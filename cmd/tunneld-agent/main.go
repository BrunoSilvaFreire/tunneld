package main

import (
	"context"
	"flag"
	"os"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
	"github.com/BrunoSilvaFreire/tunneld/k8s/agent"
)

func main() {
	var (
		metricsAddr string
		probeAddr   string
		socketPath  string
		nodeName    string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "metrics listen address")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "health probe listen address")
	flag.StringVar(&socketPath, "socket", "/run/tunneld/tunneld.sock", "path to tunneld Unix socket")
	flag.StringVar(&nodeName, "node-name", os.Getenv("NODE_NAME"), "node this agent represents (defaults to $NODE_NAME)")
	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if nodeName == "" {
		ctrl.Log.Error(nil, "--node-name (or $NODE_NAME) is required")
		os.Exit(1)
	}

	combined := clientgoscheme.Scheme
	utilruntime.Must(tunneldv1.AddToScheme(combined))

	// Agents do NOT participate in leader election — each node has its own.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 combined,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	daemon, err := agent.Dial(context.Background(), socketPath)
	if err != nil {
		ctrl.Log.Error(err, "unable to dial tunneld socket", "path", socketPath)
		os.Exit(1)
	}
	defer daemon.Close()

	if err := (&agent.TunnelReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		NodeName: nodeName,
		Daemon:   daemon,
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up agent Tunnel reconciler")
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

	ctrl.Log.Info("starting tunneld-agent manager", "node", nodeName, "socket", socketPath)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
