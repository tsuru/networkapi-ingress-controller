package controller

import (
	"context"
	"fmt"
	"net"
	"reflect"

	"github.com/pkg/errors"
	"github.com/tsuru/networkapi-ingress-controller/config"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	nameCommonPrefix = "kube-napi-ingress"
)

var _ reconcile.Reconciler = &reconcileIngress{}

type reconcileIngress struct {
	client         client.Client
	serviceWatcher *serviceWatcher
	cfg            config.Config
}

func NewReconciler(client client.Client) *reconcileIngress {
	return &reconcileIngress{
		client:         client,
		cfg:            config.Get(),
		serviceWatcher: &serviceWatcher{},
	}
}

func (r *reconcileIngress) vipName(ing *networkingv1.Ingress) string {
	return fmt.Sprintf("%s_%s/%s/%s", nameCommonPrefix, r.cfg.ClusterName, ing.Namespace, ing.Name)
}

func (r *reconcileIngress) poolName(ing *networkingv1.Ingress) string {
	return fmt.Sprintf("%s_%s/%s/%s", nameCommonPrefix, r.cfg.ClusterName, ing.Namespace, ing.Name)
}

func validateIngress(ing *networkingv1.Ingress) error {
	if ing.Spec.DefaultBackend == nil && len(ing.Spec.Rules) == 0 {
		return fmt.Errorf("Ingress must have either default backend or one rule")
	}
	if ing.Spec.DefaultBackend != nil && len(ing.Spec.Rules) > 0 {
		return errors.New("Ingress can't have a DefaultBackend and Rules at the same time")
	}
	if len(ing.Spec.Rules) > 1 {
		return errors.New("Ingress can have only one Rule")
	}
	if len(ing.Spec.Rules) > 0 {
		paths := ing.Spec.Rules[0].HTTP.Paths
		if len(paths) > 1 {
			return errors.New("Ingress can have only one Path")
		}
		if paths[0].Path != "" && paths[0].Path != "/" && paths[0].Path != "/*" {
			return errors.New("Ingress path must be unset, / or /*")
		}
		if paths[0].Backend.Service == nil {
			return errors.New("Ingress path must have a Service")
		}
	}
	if ing.Spec.DefaultBackend != nil {
		if ing.Spec.DefaultBackend.Service == nil {
			return errors.New("Ingress default backend must have a Service")
		}
	}
	return nil
}

func backendFromIngress(ing *networkingv1.Ingress) *networkingv1.IngressServiceBackend {
	if len(ing.Spec.Rules) > 0 && len(ing.Spec.Rules[0].HTTP.Paths) > 0 {
		return ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		return ing.Spec.DefaultBackend.Service
	}
	return nil
}

func (r *reconcileIngress) svcAndPortFromIngress(ctx context.Context, ing *networkingv1.Ingress) (*corev1.Service, *corev1.ServicePort, error) {
	backend := backendFromIngress(ing)
	if backend == nil {
		return nil, nil, errors.Errorf("ingress has no backends")
	}

	svcFullName := types.NamespacedName{
		Namespace: ing.Namespace,
		Name:      backend.Name,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcFullName, svc)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not fetch backend service")
	}

	if len(svc.Spec.Ports) == 0 {
		return nil, nil, errors.Errorf("backend service %s has no ports", svcFullName.String())
	}

	var port *corev1.ServicePort
	for _, p := range svc.Spec.Ports {
		if backend.Port.Name == p.Name || backend.Port.Number == p.Port {
			port = &p
			break
		}
	}

	if port == nil {
		hasExplicitPort := backend.Port.Name != "" || backend.Port.Number != 0
		if hasExplicitPort {
			return nil, nil, errors.Errorf("backend service %s has no port matching %#v", svcFullName.String(), backend.Port)
		}
		if len(svc.Spec.Ports) > 1 {
			return nil, nil, errors.Errorf("backend service %s has more than one port, ingress must choose one", svcFullName.String())
		}
		for _, p := range svc.Spec.Ports {
			port = &p
		}
	}

	return svc, port, nil
}

type target struct {
	IP   net.IP
	Port int
}

func (r *reconcileIngress) targetsForService(ctx context.Context, svcName types.NamespacedName, port *corev1.ServicePort) ([]target, error) {
	var targets []target

	var endpoints corev1.Endpoints
	err := r.client.Get(ctx, svcName, &endpoints)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch endpoints")
	}

	var svcPortNumber int32
	if port.TargetPort.IntVal == 0 && port.TargetPort.StrVal == "" {
		svcPortNumber = port.Port
	}

	for _, subset := range endpoints.Subsets {
		var portNumber int
		for _, subsetPort := range subset.Ports {
			if (port.Name != "" && port.Name == subsetPort.Name) || svcPortNumber == subsetPort.Port {
				portNumber = int(subsetPort.Port)
				break
			}
		}
		if portNumber == 0 {
			continue
		}
		for _, address := range subset.Addresses {
			ip := net.ParseIP(address.IP)
			if ip != nil {
				targets = append(targets, target{IP: ip, Port: portNumber})
			}
		}
	}

	return targets, nil
}

func (r *reconcileIngress) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx).WithName("reconcile").WithValues("ingress", request.NamespacedName.String())

	result := reconcile.Result{}

	ing := &networkingv1.Ingress{}
	err := r.client.Get(ctx, request.NamespacedName, ing)
	if k8sErrors.IsNotFound(err) {
		log.Error(nil, "Could not find Ingress")
		r.serviceWatcher.removeIngress(request.NamespacedName)
		return result, nil
	}
	if err != nil {
		return result, errors.Wrap(err, "could not fetch Ingress")
	}

	log.Info("Reconciling Ingress")

	err = validateIngress(ing)
	if err != nil {
		return result, err
	}

	svc, port, err := r.svcAndPortFromIngress(ctx, ing)
	if err != nil {
		return result, err
	}

	svcName := types.NamespacedName{
		Namespace: svc.Namespace,
		Name:      svc.Name,
	}
	r.serviceWatcher.addIngressService(request.NamespacedName, svcName)

	targets, err := r.targetsForService(ctx, svcName, port)
	if err != nil {
		return result, err
	}

	netapiCli := networkapi.Client()

	wantedPool := &networkapi.Pool{
		Name: r.poolName(ing),
	}

	for _, tg := range targets {
		netIP, err := netapiCli.GetIP(ctx, tg.IP)
		if err != nil && !networkapi.IsNotFound(err) {
			return result, err
		}
		if networkapi.IsNotFound(err) {
			netIP, err = netapiCli.CreateIP(ctx, &networkapi.IP{
				IP: tg.IP,
				// TODO: NetworkID: <infer network ID from IP CIDR>,
			})
		}
		if err != nil {
			return result, err
		}
		wantedPool.Reals = append(wantedPool.Reals, networkapi.PoolMember{
			IPID: netIP.ID,
			Port: tg.Port,
		})
	}

	vipPool, err := netapiCli.GetPool(ctx, r.poolName(ing))
	if err != nil && !networkapi.IsNotFound(err) {
		return result, err
	}

	if networkapi.IsNotFound(err) {
		vipPool, err = netapiCli.CreatePool(ctx, wantedPool)
	} else if !reflect.DeepEqual(vipPool, wantedPool) {
		vipPool, err = netapiCli.UpdatePool(ctx, wantedPool)
	}
	if err != nil {
		return result, err
	}

	vipIP, err := netapiCli.GetIPByName(ctx, r.vipName(ing))
	if err != nil && !networkapi.IsNotFound(err) {
		return result, err
	}
	if networkapi.IsNotFound(err) {
		// TODO: get vip environment id from annotations
		vipIP, err = netapiCli.CreateVIPIPv4(ctx, r.vipName(ing), r.cfg.DefaultVIPEnvironmentID)
	}
	if err != nil {
		return result, err
	}

	existingVIP, err := netapiCli.GetVIP(ctx, r.vipName(ing))
	if err != nil && !networkapi.IsNotFound(err) {
		return result, err
	}

	wantedVIP := &networkapi.VIP{
		Name:    r.vipName(ing),
		IPv4ID:  vipIP.ID,
		PoolIDs: []int{vipPool.ID},
	}
	if networkapi.IsNotFound(err) {
		err = netapiCli.CreateVIP(ctx, wantedVIP)
	} else if !reflect.DeepEqual(existingVIP, wantedVIP) {
		err = netapiCli.UpdateVIP(ctx, wantedVIP)
	}
	if err != nil {
		return result, err
	}

	result.RequeueAfter = r.cfg.ReconcileInterval
	return result, nil
}
