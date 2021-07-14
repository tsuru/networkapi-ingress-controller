package main

import (
	"os"

	"github.com/pkg/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func init() {
	log.SetLogger(zap.New())
}

func run() error {
	entryLog := log.Log.WithName("run")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		return errors.Wrap(err, "unable to set up overall controller manager")
	}

	ingressReconciler := NewReconciler(mgr.GetClient())

	c, err := controller.New("foo-controller", mgr, controller.Options{
		Reconciler: ingressReconciler,
	})
	if err != nil {
		return errors.Wrap(err, "unable to set up controller")
	}

	err = ingressReconciler.Watch(c)
	if err != nil {
		return errors.Wrap(err, "unable to watch resources")
	}

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		return errors.Wrap(err, "unable to run manager")
	}

	return nil
}

func main() {
	entryLog := log.Log.WithName("main")
	err := run()
	if err != nil {
		entryLog.Error(err, "error")
		os.Exit(1)
	}
}
