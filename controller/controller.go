package controller

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/tsuru/networkapi-ingress-controller/config"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &reconcileIngress{}

type reconcileIngress struct {
	client           client.Client
	serviceWatcher   *serviceWatcher
	cfg              config.Config
	events           record.EventRecorder
	networkAPIClient networkapi.NetworkAPI
}

func NewReconciler(client client.Client, evtRecorder record.EventRecorder, cfg config.Config) *reconcileIngress {
	return &reconcileIngress{
		client: client,
		cfg:    cfg,
		events: evtRecorder,
		serviceWatcher: &serviceWatcher{
			ingressToService: map[types.NamespacedName]types.NamespacedName{},
		},
	}
}

func (r *reconcileIngress) validateIngress(ing *networkingv1.Ingress) error {
	if ing == nil {
		return errors.New("Ingress cannot be nil")
	}

	if !hasIngressClass(ing, r.cfg.IngressClassName) {
		return fmt.Errorf("Invalid ingress class detected, predicate failed")
	}

	services := make(map[string]bool)
	if backend := ing.Spec.DefaultBackend; backend != nil && backend.Service != nil {
		if backend.Service.Name == "" {
			return errors.New("Service backend must have a name")
		}

		services[backend.Service.Name] = true
	}

	for _, r := range ing.Spec.Rules {
		if r.HTTP == nil {
			continue
		}

		if n := len(r.HTTP.Paths); n != 1 {
			if n > 1 {
				return errors.New("Ingress can have only one path")
			}

			continue
		}

		p := &r.HTTP.Paths[0]
		if p.Path != "" && p.Path != "/" && p.Path != "/*" {
			return errors.New("Ingress path must be unset, / or /*")
		}

		if p.Backend.Service == nil {
			return errors.New("Ingress path must have a Service")
		}

		if p.Backend.Service.Name == "" {
			return errors.New("Service backend must have a name")
		}

		services[p.Backend.Service.Name] = true
	}

	if n := len(services); n != 1 {
		if n == 0 {
			return errors.New("Ingress must have either default backend or one rule")
		}

		return errors.New("Ingress cannot have different Services by rule")
	}

	return nil
}

func backendsFromIngress(ing *networkingv1.Ingress) (backends []*networkingv1.IngressServiceBackend) {
	if len(ing.Spec.Rules) > 0 && ing.Spec.Rules[0].HTTP != nil && len(ing.Spec.Rules[0].HTTP.Paths) > 0 {
		backends = append(backends, ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service)
	} else if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		backends = append(backends, ing.Spec.DefaultBackend.Service)
	}

	if len(backends) > 0 && len(ing.Spec.TLS) > 0 {
		backends = append(backends, &networkingv1.IngressServiceBackend{
			Name: backends[0].Name,
			Port: networkingv1.ServiceBackendPort{Number: int32(443)},
		})
	}

	return
}

func namespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func (r *reconcileIngress) svcAndPortFromIngress(ctx context.Context, ing *networkingv1.Ingress) (*corev1.Service, []corev1.ServicePort, error) {
	backends := backendsFromIngress(ing)
	if len(backends) == 0 {
		return nil, nil, errors.Errorf("ingress has no backends")
	}

	svcFullName := types.NamespacedName{
		Name:      backends[0].Name,
		Namespace: ing.Namespace,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcFullName, svc)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not fetch backend service")
	}

	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return nil, nil, errors.Errorf("backend service %s must not be external name type", svcFullName.String())
	}

	if len(svc.Spec.Ports) == 0 {
		return nil, nil, errors.Errorf("backend service %s has no ports", svcFullName.String())
	}

	var ports []corev1.ServicePort
	for _, p := range svc.Spec.Ports {
		for _, b := range backends {
			if (b.Port.Name != "" && b.Port.Name == p.Name) || b.Port.Number == p.Port {
				ports = append(ports, p)
			}
		}
	}

	if len(ports) == 0 {
		return nil, nil, errors.Errorf("cannot match backend port with service ports")
	}

	return svc, ports, nil
}

type target struct {
	IP        net.IP
	Port      int
	NetworkID int
	TLS       bool
}

func (r *reconcileIngress) targetsForService(ctx context.Context, ing *networkingv1.Ingress, svc *corev1.Service, ports []corev1.ServicePort) ([]target, error) {
	var targets []target

	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return targets, nil
		}

		ip := svc.Status.LoadBalancer.Ingress[0].IP
		if ip == "" {
			return targets, nil
		}

		for _, p := range ports {
			targets = append(targets, target{
				IP:        net.ParseIP(ip),
				Port:      int(p.Port),
				TLS:       p.Port == int32(443),
				NetworkID: r.cfg.LBNetworkID,
			})
		}

		return targets, nil
	}

	var endpoints corev1.Endpoints
	err := r.client.Get(ctx, namespacedName(svc), &endpoints)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch endpoints")
	}

	for _, p := range ports {
		var svcPortNumber int32
		if p.TargetPort.IntVal == 0 && p.TargetPort.StrVal == "" {
			svcPortNumber = p.Port
		}

		for _, s := range endpoints.Subsets {
			var portNumber int
			for _, sp := range s.Ports {
				if p.Name != "" && p.Name == sp.Name {
					portNumber = int(sp.Port)
					break
				}

				if svcPortNumber == sp.Port {
					portNumber = int(sp.Port)
					break
				}
			}

			if portNumber == 0 {
				continue
			}

			for _, address := range s.Addresses {
				ip := net.ParseIP(address.IP)
				if len(ip) == 0 {
					continue
				}

				targets = append(targets, target{
					IP:        ip,
					Port:      portNumber,
					TLS:       p.Port == int32(443),
					NetworkID: r.cfg.PodNetworkID,
				})
			}
		}
	}

	return targets, nil
}

func (r *reconcileIngress) reconcileIngress(ctx context.Context, ing *networkingv1.Ingress) (reconcile.Result, error) {
	lg := log.FromContext(ctx)
	lg.Info("Reconciling Ingress")
	result := reconcile.Result{}

	err := r.validateIngress(ing)
	if err != nil {
		return result, err
	}

	svc, port, err := r.svcAndPortFromIngress(ctx, ing)
	if err != nil {
		return result, err
	}
	r.serviceWatcher.addIngressService(namespacedName(ing), namespacedName(svc))

	targets, err := r.targetsForService(ctx, ing, svc, port)
	if err != nil {
		return result, err
	}

	err = r.reconcileNetworkAPI(ctx, ing, targets)
	return result, err
}

func (r *reconcileIngress) cleanUp(ctx context.Context, ingName types.NamespacedName, ing *networkingv1.Ingress) (reconcile.Result, error) {
	var result reconcile.Result

	if takeOverVIPName := ing.Annotations[config.TakeOverAnnotation]; takeOverVIPName != "" {
		// We wont remove the VIP, cause we use take over
	} else {
		err := r.cleanupNetworkAPI(ctx, ingName)
		if err != nil {
			return result, err
		}
	}

	if ing == nil {
		r.serviceWatcher.removeIngress(ingName)
		return result, nil
	}

	var newFinalizers []string
	for _, finalizer := range ing.ObjectMeta.Finalizers {
		if finalizer != config.FinalizerName {
			newFinalizers = append(newFinalizers, finalizer)
		}
	}
	ing.ObjectMeta.Finalizers = newFinalizers
	err := r.client.Update(ctx, ing)
	if err != nil {
		return result, err
	}
	r.serviceWatcher.removeIngress(ingName)
	return result, nil
}

func (r *reconcileIngress) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	lg := log.FromContext(ctx).WithName("reconcile").WithValues("ingress", request.NamespacedName.String())
	ctx = log.IntoContext(ctx, lg)

	if r.cfg.DebugReconcileOnce {
		defer func() {
			if err != nil {
				lg.Error(err, "Failed to reconcile Ingress")
			}
			panic("DebugReconcileOnce is enabled")
		}()
	}

	ing := &networkingv1.Ingress{}
	err = r.client.Get(ctx, request.NamespacedName, ing)
	if k8sErrors.IsNotFound(err) {
		lg.Error(nil, "Could not find Ingress")
		return r.cleanUp(ctx, request.NamespacedName, nil)
	}
	if err != nil {
		return result, errors.Wrap(err, "could not fetch Ingress")
	}
	if ing.DeletionTimestamp != nil {
		return r.cleanUp(ctx, request.NamespacedName, ing)
	}

	hasFinalizer := false
	for _, finalizer := range ing.ObjectMeta.Finalizers {
		if finalizer == config.FinalizerName {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		ing.ObjectMeta.Finalizers = append(ing.ObjectMeta.Finalizers, config.FinalizerName)
		err = r.client.Update(ctx, ing)
		if err != nil {
			return result, err
		}
	}

	r.events.Event(ing, corev1.EventTypeNormal, "NetworkAPIIngressReconciling", "Ingress reconciling")
	result, err = r.reconcileIngress(ctx, ing)
	if err != nil {
		r.events.Eventf(ing, corev1.EventTypeWarning, "NetworkAPIIngressReconcileFailed", "Failed to reconcile Ingress: %v", err)
		return result, err
	}
	r.events.Event(ing, corev1.EventTypeNormal, "NetworkAPIIngressReconciled", "Ingress reconciled")

	result.RequeueAfter = r.cfg.ReconcileInterval

	return result, nil
}
