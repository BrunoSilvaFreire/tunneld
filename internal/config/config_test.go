package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tunnels.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

const sshYAML = `
tunnels:
  k8s-api:
    enabled: true
    type: ssh
    depends_on:
      - bastion
    ssh:
      user: debian
      host: 1.2.3.4
      port: 2222
      identity_key_ref: id_ed25519
      local_forwards:
        - listen_address: "127.0.0.1"
          listen_port: 16443
          target_host: "127.0.0.1"
          target_port: 6443
    health:
      type: tcp
      address: "127.0.0.1:16443"
      interval: 5s
      timeout: 2s
      startup_timeout: 30s
    restart:
      policy: always
      delay: 5s
`

const kubectlYAML = `
tunnels:
  redis:
    enabled: true
    type: kubectl
    kubectl:
      kubeconfig_ref: prod-config
      context: my-context
      namespace: cityscape
      resource: svc/my-redis
      api_server: "https://127.0.0.1:16443"
      insecure_skip_tls_verify: true
      forwards:
        - local_address: "127.0.0.1"
          local_port: 6379
          remote_port: 6379
    health:
      type: tcp
      address: "127.0.0.1:6379"
    restart:
      policy: always
      delay: 5s
`

func TestLoadSSHTunnel(t *testing.T) {
	cfg, err := Load(writeTemp(t, sshYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tc, ok := cfg.Tunnels["k8s-api"]
	if !ok {
		t.Fatal("expected tunnel k8s-api")
	}
	if !tc.Enabled {
		t.Error("expected enabled=true")
	}
	if tc.Type != "ssh" {
		t.Errorf("type: got %q, want %q", tc.Type, "ssh")
	}
	if len(tc.DependsOn) != 1 || tc.DependsOn[0] != "bastion" {
		t.Errorf("depends_on: got %v, want [bastion]", tc.DependsOn)
	}
	if tc.SSH == nil {
		t.Fatal("expected non-nil SSH config")
	}
	if tc.SSH.User != "debian" {
		t.Errorf("ssh.user: got %q, want %q", tc.SSH.User, "debian")
	}
	if tc.SSH.Host != "1.2.3.4" {
		t.Errorf("ssh.host: got %q, want %q", tc.SSH.Host, "1.2.3.4")
	}
	if tc.SSH.Port != 2222 {
		t.Errorf("ssh.port: got %d, want 2222", tc.SSH.Port)
	}
	if tc.SSH.IdentityKeyRef != "id_ed25519" {
		t.Errorf("ssh.identity_key_ref: got %q, want %q", tc.SSH.IdentityKeyRef, "id_ed25519")
	}
	if len(tc.SSH.LocalForwards) != 1 {
		t.Fatalf("ssh.local_forwards: got %d, want 1", len(tc.SSH.LocalForwards))
	}
	fwd := tc.SSH.LocalForwards[0]
	if fwd.ListenPort != 16443 || fwd.TargetPort != 6443 {
		t.Errorf("forward ports: listen=%d target=%d, want 16443/6443", fwd.ListenPort, fwd.TargetPort)
	}
	if tc.Restart.Policy != "always" {
		t.Errorf("restart.policy: got %q, want %q", tc.Restart.Policy, "always")
	}
	if tc.Restart.Delay != 5*time.Second {
		t.Errorf("restart.delay: got %v, want 5s", tc.Restart.Delay)
	}
}

func TestLoadKubectlTunnel(t *testing.T) {
	cfg, err := Load(writeTemp(t, kubectlYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tc, ok := cfg.Tunnels["redis"]
	if !ok {
		t.Fatal("expected tunnel redis")
	}
	if tc.Type != "kubectl" {
		t.Errorf("type: got %q, want %q", tc.Type, "kubectl")
	}
	if tc.Kubectl == nil {
		t.Fatal("expected non-nil Kubectl config")
	}
	if tc.Kubectl.KubeconfigRef != "prod-config" {
		t.Errorf("kubeconfig_ref: got %q, want %q", tc.Kubectl.KubeconfigRef, "prod-config")
	}
	if tc.Kubectl.Context != "my-context" {
		t.Errorf("context: got %q, want %q", tc.Kubectl.Context, "my-context")
	}
	if tc.Kubectl.Namespace != "cityscape" {
		t.Errorf("namespace: got %q, want %q", tc.Kubectl.Namespace, "cityscape")
	}
	if tc.Kubectl.APIServer != "https://127.0.0.1:16443" {
		t.Errorf("api_server: got %q, want %q", tc.Kubectl.APIServer, "https://127.0.0.1:16443")
	}
	if !tc.Kubectl.InsecureSkipTLSVerify {
		t.Error("expected insecure_skip_tls_verify=true")
	}
	if len(tc.Kubectl.Forwards) != 1 {
		t.Fatalf("kubectl.forwards: got %d, want 1", len(tc.Kubectl.Forwards))
	}
	fwd := tc.Kubectl.Forwards[0]
	if fwd.LocalPort != 6379 || fwd.RemotePort != 6379 {
		t.Errorf("forward ports: local=%d remote=%d, want 6379/6379", fwd.LocalPort, fwd.RemotePort)
	}
}

func TestConfigValidate_Valid(t *testing.T) {
	cfg, err := Load(writeTemp(t, sshYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

func TestConfigValidate_UnknownType(t *testing.T) {
	const badYAML = `
tunnels:
  broken:
    enabled: true
    type: bogus
`
	cfg, err := Load(writeTemp(t, badYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for unknown type, got nil")
	}
}

func TestConfigToSpec_SSH(t *testing.T) {
	tc := TunnelConfig{
		Enabled:   true,
		Type:      "ssh",
		DependsOn: []string{"bastion"},
		SSH: &SSHConfig{
			User: "debian",
			Host: "1.2.3.4",
			Port: 22,
		},
	}
	spec, err := tc.ToSpec("my-tunnel")
	if err != nil {
		t.Fatalf("ToSpec: %v", err)
	}
	if spec.Name() != "my-tunnel" {
		t.Errorf("Name(): got %q, want %q", spec.Name(), "my-tunnel")
	}
	if spec.Type() != "ssh" {
		t.Errorf("Type(): got %q, want %q", spec.Type(), "ssh")
	}
	if len(spec.Dependencies()) != 1 || spec.Dependencies()[0] != "bastion" {
		t.Errorf("Dependencies(): got %v, want [bastion]", spec.Dependencies())
	}
}

func TestConfigToSpec_Kubectl(t *testing.T) {
	tc := TunnelConfig{
		Enabled: true,
		Type:    "kubectl",
		Kubectl: &KubectlConfig{
			KubeconfigRef: "prod-config",
			Context:       "my-context",
			Namespace:     "cityscape",
			Resource:      "svc/redis",
		},
	}
	spec, err := tc.ToSpec("redis")
	if err != nil {
		t.Fatalf("ToSpec: %v", err)
	}
	if spec.Name() != "redis" {
		t.Errorf("Name(): got %q, want %q", spec.Name(), "redis")
	}
	if spec.Type() != "kubectl" {
		t.Errorf("Type(): got %q, want %q", spec.Type(), "kubectl")
	}
	proto := spec.ToProto()
	if proto.GetKubectl() == nil {
		t.Fatal("ToProto().GetKubectl() returned nil")
	}
	if proto.GetKubectl().KubeconfigRef != "prod-config" {
		t.Errorf("KubeconfigRef: got %q, want %q", proto.GetKubectl().KubeconfigRef, "prod-config")
	}
}
