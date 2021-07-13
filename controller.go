package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1Beta1 "k8s.io/api/networking/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ingressClassName       = "globo-networkapi"
	forceReconcileInterval = 5 * time.Minute
)

type reconcileIngress struct {
	client client.Client
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileIngress{}

var hasClass = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	ing, ok := obj.(*networkingv1.Ingress)
	if ok && ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
		return *ing.Spec.IngressClassName == ingressClassName
	}
	ingClass := obj.GetAnnotations()[networkingv1Beta1.AnnotationIngressClass]
	return ingClass == ingressClassName
})

func (r *reconcileIngress) Watch(c controller.Controller) error {
	err := c.Watch(&source.Kind{Type: &networkingv1.Ingress{}}, &handler.EnqueueRequestForObject{}, hasClass)
	if err != nil {
		return errors.Wrap(err, "unable to watch Ingress")
	}
	return nil
}

func (r *reconcileIngress) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	result := reconcile.Result{
		RequeueAfter: forceReconcileInterval,
	}

	ing := &networkingv1.Ingress{}
	err := r.client.Get(ctx, request.NamespacedName, ing)
	if k8sErrors.IsNotFound(err) {
		log.Error(nil, "Could not find Ingress")
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("could not fetch Ingress: %+v", err)
	}

	log.Info("Reconciling Ingress", "container name", ing)

	netapiCli := networkapi.Client()
	_ = netapiCli

	// Restrictions:
	//
	// 1. Only a single host rule with a single path is allowed;
	//
	// 2. ing.spec.rules.http.host field is ignored;
	//
	// 3. ing.spec.rules.http.paths.path must either not be set or set to `/`;
	//
	// 4. If ing.spec.defaultBackend is set than ing.spec.rules should not be set;
	//
	// 5. The backend must be a Service.

	// LB creation flow:
	//
	// 1. cli.CreateEquipment() for each service endpoint if service is
	// ClusterIP type, if service is LoadBalancer create a single Equipment for
	// the LB. Equipment type must be "Server";
	//
	// 2. cli.CreateEquipmentIP() for each created equipment with it's IP;
	//
	// 3. cli.CreatePool() for each port in the service;
	//
	// 4. cli.SetReals() for each created Pool with each Equipment created;
	//
	// 5. cli.CreateVIP() adding all created pools.

	return result, nil
}
