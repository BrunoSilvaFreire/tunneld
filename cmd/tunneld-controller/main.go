package main

import (
	"flag"
	"os"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
	"github.com/BrunoSilvaFreire/tunneld/k8s/controllers"
)

func main() {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "metrics listen address")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "health probe listen address")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true, "enable leader election")
	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	combined := clientgoscheme.Scheme
	utilruntime.Must(tunneldv1.AddToScheme(combined))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 combined,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "tunneld-controller.tunneld.io",
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controllers.TunnelReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up Tunnel controller")
		os.Exit(1)
	}
	if err := (&controllers.TunnelKeyReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up TunnelKey controller")
		os.Exit(1)
	}
	if err := (&controllers.TunnelGroupReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up TunnelGroup controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to register healthz")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to register readyz")
		os.Exit(1)
	}

	ctrl.Log.Info("starting tunneld-controller manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
