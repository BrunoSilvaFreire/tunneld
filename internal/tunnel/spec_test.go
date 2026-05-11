package tunnel

import (
	"context"
	"strings"
	"testing"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

func TestSSHBuildCommand(t *testing.T) {
	spec := NewSSHSpec(
		"test-ssh",
		nil,
		"user",
		"host.com",
		2222,
		"/path/to/id",
		[]SSHForward{
			{ListenAddress: "127.0.0.1", ListenPort: 8080, TargetHost: "remote", TargetPort: 80},
		},
		map[string]string{"StrictHostKeyChecking": "no"},
		&pb.HealthCheckSpec{},
		&pb.RestartPolicySpec{},
		0,
		0,
	)

	cmd, err := spec.BuildCommand(context.Background())
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	expectedArgs := []string{
		"ssh", "-N", "-p", "2222", "-i", "/path/to/id",
		"-o", "StrictHostKeyChecking=no",
		"-L", "127.0.0.1:8080:remote:80",
		"user@host.com",
	}

	actualArgs := cmd.Args
	if len(actualArgs) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d: %v", len(expectedArgs), len(actualArgs), actualArgs)
	}

	for i, arg := range expectedArgs {
		if i < len(actualArgs) && actualArgs[i] != arg {
			t.Errorf("arg[%d] expected %q, got %q", i, arg, actualArgs[i])
		}
	}
}

func TestKubectlBuildCommand(t *testing.T) {
	spec := NewKubectlSpec(
		"test-kube",
		[]string{"test-ssh"},
		"/path/to/kubeconfig",
		"prod",
		"myns",
		"svc/mysvc",
		[]KubectlForward{
			{LocalAddress: "127.0.0.1", LocalPort: 8443, RemotePort: 443},
		},
		"https://api.server",
		true,
		&pb.HealthCheckSpec{},
		&pb.RestartPolicySpec{},
		0,
		0,
	)

	cmd, err := spec.BuildCommand(context.Background())
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	args := strings.Join(cmd.Args, " ")
	expectedParts := []string{
		"kubectl",
		"--kubeconfig /path/to/kubeconfig",
		"--context prod",
		"--server https://api.server",
		"--insecure-skip-tls-verify=true",
		"-n myns",
		"port-forward",
		"--address 127.0.0.1",
		"svc/mysvc",
		"8443:443",
	}

	for _, part := range expectedParts {
		if !strings.Contains(args, part) {
			t.Errorf("expected command to contain %q, but got %q", part, args)
		}
	}
}
