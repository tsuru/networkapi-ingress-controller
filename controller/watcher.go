package controller

import (
	"sync"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1Beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type endpointWatcher struct {
	sync.RWMutex
	ingressToEndpoint map[types.NamespacedName]types.NamespacedName
}

func (w *endpointWatcher) mapFunc(obj client.Object) []reconcile.Request {
	w.RLock()
	defer w.RUnlock()
	fullName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
	var reqs []reconcile.Request
	for ing, endpoint := range w.ingressToEndpoint {
		if endpoint == fullName {
			reqs = append(reqs, reconcile.Request{NamespacedName: ing})
		}
	}
	return reqs
}

func (w *endpointWatcher) addIngressEndpoint(ingName, endpointName types.NamespacedName) {
	w.Lock()
	defer w.Unlock()
	w.ingressToEndpoint[ingName] = endpointName
}

func (w *endpointWatcher) removeIngress(ingName types.NamespacedName) {
	w.Lock()
	defer w.Unlock()
	delete(w.ingressToEndpoint, ingName)
}

func hasClass(ingressClassName string) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		ing, ok := obj.(*networkingv1.Ingress)
		if ok && ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
			return *ing.Spec.IngressClassName == ingressClassName
		}
		ingClass := obj.GetAnnotations()[networkingv1Beta1.AnnotationIngressClass]
		return ingClass == ingressClassName
	})
}

func (r *reconcileIngress) Watch(c controller.Controller) error {
	err := c.Watch(&source.Kind{Type: &networkingv1.Ingress{}}, &handler.EnqueueRequestForObject{}, hasClass(r.cfg.IngressClassName))
	if err != nil {
		return errors.Wrap(err, "unable to watch Ingress")
	}

	err = c.Watch(&source.Kind{Type: &corev1.Endpoints{}}, handler.EnqueueRequestsFromMapFunc(r.endpoints.mapFunc))
	if err != nil {
		return errors.Wrap(err, "unable to watch Endpoints")
	}
	return nil
}
