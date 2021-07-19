package main

import (
	"os"

	"github.com/pkg/errors"
	ingConfig "github.com/tsuru/networkapi-ingress-controller/config"
	ingController "github.com/tsuru/networkapi-ingress-controller/controller"
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func run() error {
	cfg, err := ingConfig.Get()
	if err != nil {
		return errors.Wrap(err, "unable to read config")
	}
	log.SetLogger(zap.New(zap.Level(zapcore.Level(cfg.LogLevel * -1))))

	entryLog := log.Log.WithName("run")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		return errors.Wrap(err, "unable to set up overall controller manager")
	}

	ingressReconciler := ingController.NewReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(ingConfig.IngressControllerName),
		cfg,
	)

	c, err := controller.New(ingConfig.IngressControllerName, mgr, controller.Options{
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
