package steps

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (tc *testContext) distributedDockerLabIsRunning(ctx context.Context) error {
	for _, name := range []string{"node-client", "node-bastion", "node-target"} {
		out, err := tc.run(ctx, 10*time.Second, "docker", "inspect", "-f", "{{.State.Running}}", name)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) != "true" {
			return fmt.Errorf("%s is not running: %s", name, out)
		}
	}
	_, err := tc.run(ctx, 10*time.Second, "docker", "network", "inspect", "tunneld-it")
	return err
}

func (tc *testContext) nodeClientCannotUseLocalhostForTarget(ctx context.Context) error {
	_, err := tc.execNode(ctx, 5*time.Second, "curl", "--fail", "--silent", "--max-time", "2", "http://127.0.0.1:8080")
	if err == nil {
		return fmt.Errorf("node-client unexpectedly reached node-target through localhost:8080")
	}
	return nil
}
