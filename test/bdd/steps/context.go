package steps

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

const (
	nodeClient = "node-client"
	socketPath = "/run/tunneld-it/tunneld.sock"
)

type testContext struct {
	rootDir string
}

func InitializeScenario(sc *godog.ScenarioContext) {
	tc := &testContext{rootDir: findRoot()}

	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		_ = tc.stopTunneld(ctx)
		return ctx, nil
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		_ = tc.stopTunneld(ctx)
		return ctx, nil
	})

	sc.Step(`^the distributed Docker lab is running$`, tc.distributedDockerLabIsRunning)
	sc.Step(`^node-client cannot directly rely on localhost to reach node-target$`, tc.nodeClientCannotUseLocalhostForTarget)
	sc.Step(`^node-client can SSH into node-bastion$`, tc.nodeClientCanSSHIntoNodeBastion)
	sc.Step(`^tunnelD starts an SSH tunnel from node-client through node-bastion to node-target port 8080$`, tc.startSSHTunnel)
	sc.Step(`^node-client should receive "([^"]*)" from local port (\d+)$`, tc.nodeClientShouldReceiveFromLocalPort)
	sc.Step(`^tunnelD should report the SSH tunnel as healthy$`, tc.sshTunnelShouldBeHealthy)

	sc.Step(`^the k3d cluster is running$`, tc.k3dClusterIsRunning)
	sc.Step(`^the Kubernetes echo service is ready$`, tc.kubernetesEchoServiceIsReady)
	sc.Step(`^tunnelD starts a kubectl tunnel to service "([^"]*)" port (\d+)$`, tc.startKubectlTunnel)
	sc.Step(`^localhost port (\d+) should return "([^"]*)"$`, tc.localhostPortShouldReturn)
	sc.Step(`^tunnelD should report the Kubernetes tunnel as healthy$`, tc.kubernetesTunnelShouldBeHealthy)

	sc.Step(`^a kubeconfig is available only through an SSH tunnel$`, pendingDependencyWork)
	sc.Step(`^tunnelD has an SSH tunnel named "([^"]*)"$`, pendingDependencyWork)
	sc.Step(`^tunnelD has a kubectl tunnel named "([^"]*)"$`, pendingDependencyWork)
	sc.Step(`^"([^"]*)" depends on "([^"]*)"$`, pendingDependencyWork)
	sc.Step(`^tunnelD starts the tunnel graph$`, pendingDependencyWork)
	sc.Step(`^tunnelD should start "([^"]*)" before "([^"]*)"$`, pendingDependencyWork)
	sc.Step(`^an SSH tunnel through node-bastion is healthy$`, pendingRecoveryWork)
	sc.Step(`^node-bastion is disconnected from the Docker network$`, pendingRecoveryWork)
	sc.Step(`^tunnelD should mark the SSH tunnel as unhealthy or degraded$`, pendingRecoveryWork)
	sc.Step(`^node-bastion is reconnected to the Docker network$`, pendingRecoveryWork)
	sc.Step(`^tunnelD should eventually mark the SSH tunnel as healthy again$`, pendingRecoveryWork)
}

func findRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "."
		}
		wd = parent
	}
}

func pendingDependencyWork(args ...string) error {
	return fmt.Errorf("TODO: dependency graph integration is documented but blocked on synchronous health-gated dependent startup")
}

func pendingRecoveryWork(args ...string) error {
	return fmt.Errorf("TODO: recovery integration is documented but needs deterministic restart/keepalive timing")
}

func (tc *testContext) run(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, name, args...)
	cmd.Dir = tc.rootDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("%s timed out after %s: %s", commandString(name, args), timeout, out.String())
	}
	if err != nil {
		return out.String(), fmt.Errorf("%s failed: %w\n%s", commandString(name, args), err, out.String())
	}
	return out.String(), nil
}

func (tc *testContext) execNode(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	all := append([]string{"exec", "-e", "GOCOVERDIR=/artifacts/coverage", nodeClient}, args...)
	return tc.run(ctx, timeout, "docker", all...)
}

func (tc *testContext) execNodeShell(ctx context.Context, timeout time.Duration, script string) (string, error) {
	return tc.execNode(ctx, timeout, "sh", "-lc", script)
}

func (tc *testContext) eventually(ctx context.Context, timeout, interval time.Duration, check func(context.Context) error) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		if err := check(ctx); err == nil {
			return nil
		} else {
			last = err
		}
		time.Sleep(interval)
	}
	if last == nil {
		last = fmt.Errorf("condition was not met")
	}
	return fmt.Errorf("timed out after %s: %w", timeout, last)
}

func (tc *testContext) ensureTunneld(ctx context.Context) error {
	_ = tc.stopTunneld(ctx)
	_, err := tc.execNodeShell(ctx, 10*time.Second, `
set -eu
rm -f /run/tunneld-it/tunneld.sock
mkdir -p /run/tunneld-it /var/lib/tunneld-it/keys /var/lib/tunneld-it/tunnels /tmp/tunneld-it
nohup env GOCOVERDIR=/artifacts/coverage tunneld --socket /run/tunneld-it/tunneld.sock --key-dir /var/lib/tunneld-it/keys --tunnels-dir /var/lib/tunneld-it/tunnels run --no-config > /tmp/tunneld-it/tunneld.log 2>&1 &
`)
	if err != nil {
		return err
	}
	return tc.eventually(ctx, 20*time.Second, 500*time.Millisecond, func(ctx context.Context) error {
		_, err := tc.execNode(ctx, 3*time.Second, "test", "-S", socketPath)
		return err
	})
}

func (tc *testContext) stopTunneld(ctx context.Context) error {
	_, err := tc.execNodeShell(ctx, 10*time.Second, "pkill tunneld >/dev/null 2>&1 || true; while pgrep tunneld >/dev/null 2>&1; do sleep 0.1; done; rm -f /run/tunneld-it/tunneld.sock")
	return err
}

func (tc *testContext) createAndWait(ctx context.Context, name, fixture string) error {
	path := filepath.ToSlash(filepath.Join("/workspace/test/fixtures/tunneld", fixture))
	if _, err := tc.execNode(ctx, 15*time.Second, "tunnelctl", "--socket", socketPath, "create", name, "--config", path); err != nil {
		return err
	}
	if _, err := tc.execNode(ctx, 60*time.Second, "tunnelctl", "--socket", socketPath, "wait", name, "--timeout", "45"); err != nil {
		return err
	}
	return nil
}

func (tc *testContext) statusContains(ctx context.Context, name, want string) error {
	out, err := tc.execNode(ctx, 10*time.Second, "tunnelctl", "--socket", socketPath, "status", name)
	if err != nil {
		return err
	}
	if !strings.Contains(out, want) {
		return fmt.Errorf("status for %s did not contain %q:\n%s", name, want, out)
	}
	return nil
}

func (tc *testContext) curlNodePort(ctx context.Context, port int) (string, error) {
	return tc.execNode(ctx, 10*time.Second, "curl", "--fail", "--silent", "--show-error", "--max-time", "5", fmt.Sprintf("http://127.0.0.1:%d", port))
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func commandString(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}
