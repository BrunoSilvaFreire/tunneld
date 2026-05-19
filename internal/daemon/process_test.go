package daemon

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

type healthSpec struct {
	health *pb.HealthCheckSpec
}

func (h healthSpec) Name() string                                            { return "health-test" }
func (h healthSpec) Type() string                                            { return "mock" }
func (h healthSpec) Dependencies() []string                                  { return nil }
func (h healthSpec) BuildCommand(context.Context, string) (*exec.Cmd, error) { return nil, nil }
func (h healthSpec) HealthCheck() *pb.HealthCheckSpec                        { return h.health }
func (h healthSpec) RestartPolicy() *pb.RestartPolicySpec                    { return nil }
func (h healthSpec) StartupTimeout() time.Duration                           { return 0 }
func (h healthSpec) ShutdownTimeout() time.Duration                          { return 0 }
func (h healthSpec) ToProto() *pb.TunnelSpec                                 { return &pb.TunnelSpec{} }

func TestEffectiveHealthCheck_DefaultsEmptyTCPAddressToFirstForward(t *testing.T) {
	p := NewProcess(healthSpec{health: &pb.HealthCheckSpec{Type: "tcp"}}, t.TempDir())
	p.portMappings = []tunnel.PortMapping{{
		LocalAddress:   "0.0.0.0",
		ConfiguredPort: 0,
		ActualPort:     39597,
		RemotePort:     6443,
	}}

	got := p.EffectiveHealthCheck()
	if got.Address != "127.0.0.1:39597" {
		t.Fatalf("Address = %q, want 127.0.0.1:39597", got.Address)
	}
}

func TestEffectiveHealthCheck_RewritesDynamicAddress(t *testing.T) {
	p := NewProcess(healthSpec{health: &pb.HealthCheckSpec{Type: "tcp", Address: "127.0.0.1:0"}}, t.TempDir())
	p.portMappings = []tunnel.PortMapping{{
		LocalAddress:   "127.0.0.1",
		ConfiguredPort: 0,
		ActualPort:     39597,
		RemotePort:     6443,
	}}

	got := p.EffectiveHealthCheck()
	if got.Address != "127.0.0.1:39597" {
		t.Fatalf("Address = %q, want 127.0.0.1:39597", got.Address)
	}
}
