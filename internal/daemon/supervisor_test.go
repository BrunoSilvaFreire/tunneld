package daemon

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

// testSpec is a minimal tunnel.Spec for supervisor tests.
// It never spawns real processes (BuildCommand returns nil).
type testSpec struct {
	name    string
	deps    []string
	restart *pb.RestartPolicySpec
}

func (m testSpec) Name() string                                          { return m.name }
func (m testSpec) Type() string                                          { return "mock" }
func (m testSpec) Dependencies() []string                                { return m.deps }
func (m testSpec) BuildCommand(context.Context, string) (*exec.Cmd, error) { return nil, nil }
func (m testSpec) HealthCheck() *pb.HealthCheckSpec                      { return &pb.HealthCheckSpec{} }
func (m testSpec) RestartPolicy() *pb.RestartPolicySpec                  { return m.restart }
func (m testSpec) StartupTimeout() time.Duration                         { return 0 }
func (m testSpec) ShutdownTimeout() time.Duration                        { return 0 }
func (m testSpec) ToProto() *pb.TunnelSpec {
	return &pb.TunnelSpec{Name: m.name, DependsOn: m.deps, Restart: m.restart}
}

func newTestSupervisor(t *testing.T) *Supervisor {
	t.Helper()
	return NewSupervisor(dependency.NewPlanner(nil), &config.Config{}, t.TempDir(), t.TempDir())
}

func TestAddTunnel_RegistersProcess(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "my-tunnel", restart: &pb.RestartPolicySpec{Policy: "never"}}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}
	if _, err := s.GetProcess("my-tunnel"); err != nil {
		t.Errorf("GetProcess after AddTunnel: %v", err)
	}
	if n := len(s.ListTunnels()); n != 1 {
		t.Errorf("ListTunnels: got %d, want 1", n)
	}
}

func TestAddTunnel_Duplicate(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "my-tunnel", restart: &pb.RestartPolicySpec{Policy: "never"}}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("first AddTunnel: %v", err)
	}
	err := s.AddTunnel(context.Background(), spec, false)
	if err == nil {
		t.Fatal("expected error on duplicate AddTunnel, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWaitHealthy_AlreadyRunning(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "my-tunnel", restart: &pb.RestartPolicySpec{Policy: "never"}}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}
	p, _ := s.GetProcess("my-tunnel")
	p.setStatus(tunnel.StatusRunning)

	status, err := s.WaitHealthy(context.Background(), "my-tunnel", 2*time.Second)
	if err != nil {
		t.Errorf("WaitHealthy: unexpected error: %v", err)
	}
	if status != tunnel.StatusRunning {
		t.Errorf("status: got %q, want %q", status, tunnel.StatusRunning)
	}
}

// TestWaitHealthy_TransientFailed_AlwaysPolicy is a regression test for the fix
// that makes WaitHealthy continue polling through StatusFailed when restart
// policy is "always", rather than returning immediately with an error.
func TestWaitHealthy_TransientFailed_AlwaysPolicy(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{
		name:    "my-tunnel",
		restart: &pb.RestartPolicySpec{Policy: "always"},
	}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}
	p, _ := s.GetProcess("my-tunnel")
	p.setStatus(tunnel.StatusFailed)

	// Simulate recovery: transition to Running well within the ticker interval (500ms).
	go func() {
		time.Sleep(100 * time.Millisecond)
		p.setStatus(tunnel.StatusRunning)
	}()

	status, err := s.WaitHealthy(context.Background(), "my-tunnel", 2*time.Second)
	if err != nil {
		t.Errorf("WaitHealthy should not fail on transient StatusFailed with restart policy=always: %v", err)
	}
	if status != tunnel.StatusRunning {
		t.Errorf("status: got %q, want %q", status, tunnel.StatusRunning)
	}
}

func TestWaitHealthy_Failed_NeverPolicy(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{
		name:    "my-tunnel",
		restart: &pb.RestartPolicySpec{Policy: "never"},
	}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}
	p, _ := s.GetProcess("my-tunnel")
	p.setStatus(tunnel.StatusFailed)

	_, err := s.WaitHealthy(context.Background(), "my-tunnel", 2*time.Second)
	if err == nil {
		t.Error("WaitHealthy: expected error when tunnel is failed and restart policy is never, got nil")
	}
}

func TestWaitHealthy_Timeout(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "my-tunnel", restart: &pb.RestartPolicySpec{Policy: "always"}}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}
	// Status remains StatusStopped — will never become Running on its own.

	_, err := s.WaitHealthy(context.Background(), "my-tunnel", 100*time.Millisecond)
	if err == nil {
		t.Error("WaitHealthy: expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}
