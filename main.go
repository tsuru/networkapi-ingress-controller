package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"
	ingConfig "github.com/tsuru/networkapi-ingress-controller/config"
	ingController "github.com/tsuru/networkapi-ingress-controller/controller"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	GitHash    = "HEAD"
	GitVersion = "0.0.0"
)

func run() error {
	var configFile = flag.String("config", "", "Paths to a networkapi ingress controller config.")
	var version = flag.Bool("version", false, "Display version information and exit.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	zapLog := zap.New(zap.UseFlagOptions(&opts))
	log.SetLogger(zapLog)

	if version != nil && *version {
		fmt.Printf("%s - %s\n", GitVersion, GitHash)
		return nil
	}

	if configFile == nil || *configFile == "" {
		flag.Usage()
		return errors.New("missing config file")
	}

	cfg, err := ingConfig.Get(*configFile)
	if err != nil {
		return errors.Wrap(err, "unable to read config")
	}

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
