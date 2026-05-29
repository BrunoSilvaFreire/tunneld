package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

func TestSupervisor_StartStopTunnel(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "start-stop", restart: &pb.RestartPolicySpec{Policy: "never"}}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}

	p, _ := s.GetProcess("start-stop")
	p.logPath = filepath.Join(t.TempDir(), "test.log")

	// StopTunnel
	if err := s.StopTunnel("start-stop"); err != nil {
		t.Fatalf("StopTunnel: %v", err)
	}

	p, _ = s.GetProcess("start-stop")
	if p.Status() != tunnel.StatusStopped {
		t.Errorf("Expected status stopped, got %s", p.Status())
	}
	
	if p.ExpectedState() != DesiredStopped {
		t.Errorf("Expected DesiredStopped, got %s", p.ExpectedState())
	}

	// StartTunnel
	s.ctx = context.Background()
	err := s.StartTunnel(context.Background(), "start-stop")
	if err == nil {
		t.Fatalf("Expected error starting mock tunnel, got nil")
	}

	if p.ExpectedState() != DesiredRunning {
		t.Errorf("Expected DesiredRunning, got %s", p.ExpectedState())
	}
}

func TestSupervisor_EnableDisableTunnel(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "enable-disable", restart: &pb.RestartPolicySpec{Policy: "never"}}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}

	// Create dummy config file to allow saving
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfgPath, []byte("tunnels:\n  enable-disable:\n    enabled: true"), 0644)
	cfg, _ := config.Load(cfgPath)
	s.config = cfg

	p, _ := s.GetProcess("enable-disable")
	p.logPath = filepath.Join(t.TempDir(), "test.log")

	if err := s.DisableTunnel("enable-disable"); err != nil {
		t.Fatalf("DisableTunnel: %v", err)
	}

	p, _ = s.GetProcess("enable-disable")
	if p.ExpectedState() != DesiredStopped {
		t.Errorf("Expected DesiredStopped, got %s", p.ExpectedState())
	}

	s.ctx = context.Background()
	err := s.EnableTunnel(context.Background(), "enable-disable")
	if err == nil {
		t.Fatalf("Expected error enabling mock tunnel, got nil")
	}
	
	if p.ExpectedState() != DesiredRunning {
		t.Errorf("Expected DesiredRunning, got %s", p.ExpectedState())
	}
}

func TestSupervisor_HandleFailure(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{
		name: "failing-tunnel",
		restart: &pb.RestartPolicySpec{
			Policy: "always",
			MaxAttempts: 2,
		},
	}

	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}

	p, _ := s.GetProcess("failing-tunnel")
	p.logPath = filepath.Join(t.TempDir(), "test.log")
	p.setExpectedState(DesiredRunning)

	// Trigger failure handling
	s.handleFailure(context.Background(), "failing-tunnel")
	
	if p.RestartAttempts() != 1 {
		t.Errorf("Expected 1 restart attempt, got %d", p.RestartAttempts())
	}

	s.handleFailure(context.Background(), "failing-tunnel")
	
	if p.RestartAttempts() != 2 {
		t.Errorf("Expected 2 restart attempt, got %d", p.RestartAttempts())
	}
	
	s.handleFailure(context.Background(), "failing-tunnel")
	
	if p.Status() != tunnel.StatusFailed {
		t.Errorf("Expected StatusFailed after max attempts, got %s", p.Status())
	}
}

func TestSupervisor_RemoveTunnel(t *testing.T) {
	s := newTestSupervisor(t)
	spec := testSpec{name: "remove-tunnel"}
	if err := s.AddTunnel(context.Background(), spec, false); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}
	
	if err := s.RemoveTunnel("remove-tunnel"); err != nil {
		t.Fatalf("RemoveTunnel: %v", err)
	}
	
	if _, err := s.GetProcess("remove-tunnel"); err == nil {
		t.Fatal("Expected error getting removed tunnel, got nil")
	}
}
