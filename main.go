package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"
	ingConfig "github.com/tsuru/networkapi-ingress-controller/config"
	ingController "github.com/tsuru/networkapi-ingress-controller/controller"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrlConfig "sigs.k8s.io/controller-runtime/pkg/config"
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
	entryLog := log.Log.WithName("run")

	var ingressConfigFile = flag.String("ingress-config", "", "Paths to a networkapi ingress controller config.")
	var ctrlConfigFile = flag.String("controller-config", "", "Paths to a networkapi ingress controller config.")
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

	if ingressConfigFile == nil || *ingressConfigFile == "" {
		flag.Usage()
		return errors.New("missing ingress-config argument")
	}

	if ctrlConfigFile == nil || *ctrlConfigFile == "" {
		flag.Usage()
		return errors.New("missing controller-config argument")
	}

	cfg, err := ingConfig.Get(*ingressConfigFile)
	if err != nil {
		return errors.Wrap(err, "unable to read config")
	}

	mgrOpts, err := manager.Options{
		Scheme:                     scheme.Scheme,
		LeaderElectionID:           ingConfig.IngressControllerName,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
	}.AndFrom(ctrlConfig.File().AtPath(*ctrlConfigFile))
	if err != nil {
		return errors.Wrapf(err, "unable to read controller-config file")
	}

	mgr, err := manager.New(config.GetConfigOrDie(), mgrOpts)
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
