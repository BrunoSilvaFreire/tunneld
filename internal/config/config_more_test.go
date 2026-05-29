package config

import (
	"testing"
	"time"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestConfig_SaveAndSetEnabled(t *testing.T) {
	cfg, err := Load(writeTemp(t, sshYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Test SetEnabled
	err = cfg.SetEnabled("k8s-api", false)
	if err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}

	if cfg.Tunnels["k8s-api"].Enabled {
		t.Errorf("Expected enabled to be false")
	}

	// Reload to verify it was saved
	cfg2, err := Load(cfg.filePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg2.Tunnels["k8s-api"].Enabled {
		t.Errorf("Expected enabled to be false after reload")
	}

	// Test SetEnabled non-existent
	err = cfg.SetEnabled("non-existent", true)
	if err == nil {
		t.Errorf("Expected error for non-existent tunnel")
	}

	// Test Save missing filepath
	cfg.filePath = ""
	err = cfg.Save()
	if err == nil {
		t.Errorf("Expected error when saving with missing filepath")
	}
}

func TestMarshalTunnelAndFromProto(t *testing.T) {
	spec := &pb.TunnelSpec{
		Name:      "test-ssh",
		DependsOn: []string{"dep1"},
		Type: &pb.TunnelSpec_Ssh{
			Ssh: &pb.SSHSpec{
				User: "testuser",
				Host: "testhost",
				Port: 22,
			},
		},
		Health: &pb.HealthCheckSpec{
			Type:     "tcp",
			Interval: durationpb.New(10 * time.Second),
		},
		Restart: &pb.RestartPolicySpec{
			Policy: "always",
			Backoff: &pb.BackoffSpec{
				Multiplier: 2.0,
				MaxDelay:   durationpb.New(100 * time.Second),
			},
		},
	}

	data, err := MarshalTunnel(spec)
	if err != nil {
		t.Fatalf("MarshalTunnel: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("MarshalTunnel returned empty data")
	}

	// Test ParseTunnel
	parsed, err := ParseTunnel("test-ssh", data)
	if err != nil {
		t.Fatalf("ParseTunnel: %v", err)
	}

	if parsed.Name() != "test-ssh" {
		t.Errorf("ParseTunnel name: got %q, want %q", parsed.Name(), "test-ssh")
	}
}

func TestFromProto_Kubectl(t *testing.T) {
	spec := &pb.TunnelSpec{
		Name: "test-kube",
		Type: &pb.TunnelSpec_Kubectl{
			Kubectl: &pb.KubectlSpec{
				Context:   "ctx",
				Namespace: "ns",
				Resource:  "res",
				Forwards: []*pb.KubectlForward{
					{LocalPort: 8080, RemotePort: 80},
				},
			},
		},
		Health: &pb.HealthCheckSpec{},
		Restart: &pb.RestartPolicySpec{},
	}

	tc := FromProto(spec)
	if tc.Type != "kubectl" {
		t.Errorf("Expected type kubectl, got %q", tc.Type)
	}
	if tc.Kubectl.Context != "ctx" {
		t.Errorf("Expected context ctx, got %q", tc.Kubectl.Context)
	}
	if len(tc.Kubectl.Forwards) != 1 || tc.Kubectl.Forwards[0].LocalPort != 8080 {
		t.Errorf("Expected local port 8080")
	}
}

func TestConfig_ToSpecs(t *testing.T) {
	cfg, err := Load(writeTemp(t, sshYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	specs, err := cfg.ToSpecs()
	if err != nil {
		t.Fatalf("ToSpecs: %v", err)
	}

	if len(specs) != 1 {
		t.Errorf("Expected 1 spec, got %d", len(specs))
	}

	if _, ok := specs["k8s-api"]; !ok {
		t.Errorf("Expected k8s-api spec")
	}
}

func TestConfigValidate_MissingType(t *testing.T) {
	const missingTypeYAML = `
tunnels:
  broken:
    enabled: true
`
	cfg, err := Load(writeTemp(t, missingTypeYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for missing type, got nil")
	}
}

func TestConfigValidate_PortConflict(t *testing.T) {
	const conflictYAML = `
tunnels:
  t1:
    enabled: true
    type: ssh
    ssh:
      user: u
      host: h
      port: 22
      local_forwards:
        - listen_address: "127.0.0.1"
          listen_port: 8080
          target_host: t
          target_port: 80
    health:
      type: tcp
      address: "127.0.0.1:8080"
    restart:
      policy: always
  t2:
    enabled: true
    type: ssh
    ssh:
      user: u
      host: h
      port: 22
      local_forwards:
        - listen_address: "127.0.0.1"
          listen_port: 8080
          target_host: t
          target_port: 80
    health:
      type: tcp
      address: "127.0.0.1:8080"
    restart:
      policy: always
`
	cfg, err := Load(writeTemp(t, conflictYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for port conflict, got nil")
	}
}
