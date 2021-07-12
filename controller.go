package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	networkingv1 "k8s.io/api/networking/v1"
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
	ingressClassName = "globo-networkapi"
)

type reconcileIngress struct {
	client client.Client
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileIngress{}

var hasClass = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	if name := annotations["kubernetes.io/ingress.class"]; name == ingressClassName {
		return true
	}
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return false
	}
	return ing.Spec.IngressClassName != nil &&
		*ing.Spec.IngressClassName == ingressClassName
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

	ing := &networkingv1.Ingress{}
	err := r.client.Get(ctx, request.NamespacedName, ing)
	if k8sErrors.IsNotFound(err) {
		log.Error(nil, "Could not find Ingress")
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch Ingress: %+v", err)
	}

	log.Info("Reconciling Ingress", "container name", ing)
	return reconcile.Result{}, nil
}
