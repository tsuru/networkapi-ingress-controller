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

type serviceWatcher struct {
	sync.RWMutex
	ingressToService map[types.NamespacedName]types.NamespacedName
}

func (w *serviceWatcher) mapFunc(obj client.Object) []reconcile.Request {
	w.RLock()
	defer w.RUnlock()
	fullName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
	var reqs []reconcile.Request
	for ing, svc := range w.ingressToService {
		if svc == fullName {
			reqs = append(reqs, reconcile.Request{NamespacedName: ing})
		}
	}
	return reqs
}

func (w *serviceWatcher) addIngressService(ingName, svcName types.NamespacedName) {
	w.Lock()
	defer w.Unlock()
	w.ingressToService[ingName] = svcName
}

func (w *serviceWatcher) removeIngress(ingName types.NamespacedName) {
	w.Lock()
	defer w.Unlock()
	delete(w.ingressToService, ingName)
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

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(r.serviceWatcher.mapFunc))
	if err != nil {
		return errors.Wrap(err, "unable to watch Service")
	}

	err = c.Watch(&source.Kind{Type: &corev1.Endpoints{}}, handler.EnqueueRequestsFromMapFunc(r.serviceWatcher.mapFunc))
	if err != nil {
		return errors.Wrap(err, "unable to watch Endpoints")
	}
	return nil
}
