package health

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewChecker_TCP(t *testing.T) {
	spec := &pb.HealthCheckSpec{Type: "tcp", Address: "127.0.0.1:9999"}
	checker := NewChecker(spec)
	if _, ok := checker.(*TCPChecker); !ok {
		t.Errorf("NewChecker(tcp): got %T, want *TCPChecker", checker)
	}
}

func TestNewChecker_None(t *testing.T) {
	spec := &pb.HealthCheckSpec{Type: "none"}
	checker := NewChecker(spec)
	if _, ok := checker.(*NoneChecker); !ok {
		t.Errorf("NewChecker(none): got %T, want *NoneChecker", checker)
	}
}

func TestNewChecker_NilSpec(t *testing.T) {
	checker := NewChecker(nil)
	if _, ok := checker.(*NoneChecker); !ok {
		t.Errorf("NewChecker(nil): got %T, want *NoneChecker", checker)
	}
}

func TestNoneChecker_AlwaysHealthy(t *testing.T) {
	checker := &NoneChecker{}
	if err := checker.Check(context.Background()); err != nil {
		t.Errorf("NoneChecker.Check: unexpected error: %v", err)
	}
}

func TestTCPChecker_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	spec := &pb.HealthCheckSpec{
		Type:    "tcp",
		Address: ln.Addr().String(),
	}
	checker := NewTCPChecker(spec)

	if err := checker.Check(context.Background()); err != nil {
		t.Errorf("TCPChecker.Check against open listener: %v", err)
	}
}

func TestTCPChecker_Failure(t *testing.T) {
	// Open a listener to get a free port, then close it so connections are refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	spec := &pb.HealthCheckSpec{
		Type:    "tcp",
		Address: addr,
		Timeout: durationpb.New(500 * time.Millisecond),
	}
	checker := NewTCPChecker(spec)

	if err := checker.Check(context.Background()); err == nil {
		t.Error("TCPChecker.Check against closed port: expected error, got nil")
	}
}
