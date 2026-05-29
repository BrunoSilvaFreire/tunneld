package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

func TestSSHSpec_BuildCommand(t *testing.T) {
	tmpDir := t.TempDir()
	
	keyPath := filepath.Join(tmpDir, "id_rsa")
	err := os.WriteFile(keyPath, []byte("fake-key"), 0600)
	if err != nil {
		t.Fatalf("Failed to write mock key: %v", err)
	}

	spec := NewSSHSpec(
		"test-ssh",
		[]string{"dep1"},
		"user",
		"example.com",
		2222,
		keyPath,
		"",
		[]SSHForward{
			{ListenAddress: "127.0.0.1", ListenPort: 8080, TargetHost: "localhost", TargetPort: 80},
		},
		map[string]string{"ServerAliveInterval": "60"},
		&pb.HealthCheckSpec{},
		&pb.RestartPolicySpec{},
		5*time.Second,
		5*time.Second,
	)

	if spec.Name() != "test-ssh" {
		t.Errorf("Expected name test-ssh, got %s", spec.Name())
	}
	if spec.Type() != "ssh" {
		t.Errorf("Expected type ssh, got %s", spec.Type())
	}

	cmd, err := spec.BuildCommand(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("BuildCommand failed: %v", err)
	}

	args := strings.Join(cmd.Args, " ")
	
	expectedSubstrings := []string{
		"ssh",
		"-N",
		"-o BatchMode=yes",
		"-o StrictHostKeyChecking=accept-new",
		"-p 2222",
		"-i " + keyPath,
		"-o ServerAliveInterval=60",
		"-L 127.0.0.1:8080:localhost:80",
		"user@example.com",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(args, sub) {
			t.Errorf("Command args missing expected substring: %q. Args: %q", sub, args)
		}
	}
}

func TestFromProto(t *testing.T) {
	p := &pb.TunnelSpec{
		Type: &pb.TunnelSpec_Ssh{
			Ssh: &pb.SSHSpec{Host: "example.com"},
		},
	}
	
	spec, err := FromProto(p)
	if err != nil {
		t.Fatalf("FromProto failed: %v", err)
	}
	
	if spec.Type() != "ssh" {
		t.Errorf("Expected type ssh, got %s", spec.Type())
	}
}
