package steps

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (tc *testContext) nodeClientCanSSHIntoNodeBastion(ctx context.Context) error {
	_, err := tc.execNode(ctx, 15*time.Second,
		"ssh",
		"-i", "/test/keys/id_ed25519",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"tunneld@node-bastion",
		"true",
	)
	return err
}

func (tc *testContext) startSSHTunnel(ctx context.Context) error {
	if err := tc.ensureTunneld(ctx); err != nil {
		return err
	}
	return tc.createAndWait(ctx, "ssh-private-target", "ssh-tunnel.yaml")
}

func (tc *testContext) nodeClientShouldReceiveFromLocalPort(ctx context.Context, want string, port int) error {
	return tc.eventually(ctx, 30*time.Second, 500*time.Millisecond, func(ctx context.Context) error {
		out, err := tc.curlNodePort(ctx, port)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) != want {
			return fmt.Errorf("port %d returned %q, want %q", port, strings.TrimSpace(out), want)
		}
		return nil
	})
}

func (tc *testContext) sshTunnelShouldBeHealthy(ctx context.Context) error {
	return tc.statusContains(ctx, "ssh-private-target", "Status: running")
}
