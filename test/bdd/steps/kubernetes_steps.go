package steps

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (tc *testContext) k3dClusterIsRunning(ctx context.Context) error {
	if _, err := tc.run(ctx, 10*time.Second, "k3d", "cluster", "list", "tunneld-it"); err != nil {
		return err
	}
	return tc.eventually(ctx, 60*time.Second, 2*time.Second, func(ctx context.Context) error {
		_, err := tc.run(ctx, 10*time.Second, "kubectl", "get", "nodes")
		return err
	})
}

func (tc *testContext) kubernetesEchoServiceIsReady(ctx context.Context) error {
	if _, err := tc.run(ctx, 10*time.Second, "kubectl", "get", "svc", "echo"); err != nil {
		return err
	}
	_, err := tc.run(ctx, 120*time.Second, "kubectl", "rollout", "status", "deploy/echo", "--timeout=120s")
	return err
}

func (tc *testContext) startKubectlTunnel(ctx context.Context, service string, port int) error {
	if service != "echo" || port != 8080 {
		return fmt.Errorf("unsupported kubectl fixture service %q port %d", service, port)
	}
	if err := tc.ensureTunneld(ctx); err != nil {
		return err
	}
	return tc.createAndWait(ctx, "kubernetes-echo", "kubectl-port-forward.yaml")
}

func (tc *testContext) kubernetesTunnelShouldBeHealthy(ctx context.Context) error {
	return tc.statusContains(ctx, "kubernetes-echo", "Status: running")
}

func (tc *testContext) localhostPortShouldReturn(ctx context.Context, port int, want string) error {
	return tc.nodeClientShouldReceiveFromLocalPort(ctx, strings.TrimSpace(want), port)
}
