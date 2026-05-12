package tunnel

import (
	"context"
	"os/exec"
	"time"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

// Status represents the current state of a tunnel.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
)

// Spec defines the configuration and behavior of a tunnel.
type Spec interface {
	Name() string
	Type() string
	Dependencies() []string
	BuildCommand(ctx context.Context, keyDir string) (*exec.Cmd, error)
	HealthCheck() *pb.HealthCheckSpec
	RestartPolicy() *pb.RestartPolicySpec
	StartupTimeout() time.Duration
	ShutdownTimeout() time.Duration
	ToProto() *pb.TunnelSpec
}
