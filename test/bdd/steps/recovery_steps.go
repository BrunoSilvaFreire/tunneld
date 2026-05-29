package steps

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (tc *testContext) sshTunnelIsHealthy(ctx context.Context) error {
	if err := tc.ensureTunneld(ctx); err != nil {
		return err
	}
	return tc.createAndWait(ctx, "ssh-tunnel", "ssh-tunnel.yaml")
}

func (tc *testContext) nodeBastionIsDisconnected(ctx context.Context) error {
	// We run `docker network disconnect tunneld-it node-bastion` on the host
	_, err := tc.run(ctx, 10*time.Second, "docker", "network", "disconnect", "tunneld-it", "node-bastion")
	return err
}

func (tc *testContext) sshTunnelIsMarkedUnhealthy(ctx context.Context) error {
	// Wait until the status changes to Failed or Error
	return tc.eventually(ctx, 30*time.Second, 1*time.Second, func(ctx context.Context) error {
		out, err := tc.execNode(ctx, 5*time.Second, "tunnelctl", "--socket", socketPath, "status", "ssh-tunnel")
		if err != nil {
			return err
		}
		if strings.Contains(out, "Failed") || strings.Contains(out, "failed") {
			return nil
		}
		return fmt.Errorf("tunnel is not marked unhealthy yet:\n%s", out)
	})
}

func (tc *testContext) nodeBastionIsReconnected(ctx context.Context) error {
	// We run `docker network connect tunneld-it node-bastion` on the host
	_, err := tc.run(ctx, 10*time.Second, "docker", "network", "connect", "tunneld-it", "node-bastion")
	return err
}

func (tc *testContext) sshTunnelIsHealthyAgain(ctx context.Context) error {
	// Wait for the restart policy to kick in and become healthy again
	return tc.eventually(ctx, 60*time.Second, 2*time.Second, func(ctx context.Context) error {
		return tc.statusContains(ctx, "ssh-tunnel", "Running")
	})
}
