package health

import (
	"context"
	"net"
	"time"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

type Checker interface {
	Check(ctx context.Context) error
}

type TCPChecker struct {
	address string
	timeout time.Duration
}

func NewTCPChecker(spec *pb.HealthCheckSpec) *TCPChecker {
	timeout := spec.Timeout.AsDuration()
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &TCPChecker{
		address: spec.Address,
		timeout: timeout,
	}
}

func (c *TCPChecker) Check(ctx context.Context) error {
	d := net.Dialer{Timeout: c.timeout}
	conn, err := d.DialContext(ctx, "tcp", c.address)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

type NoneChecker struct{}

func (c *NoneChecker) Check(ctx context.Context) error {
	return nil
}

func NewChecker(spec *pb.HealthCheckSpec) Checker {
	if spec == nil {
		return &NoneChecker{}
	}
	switch spec.Type {
	case "tcp":
		return NewTCPChecker(spec)
	default:
		return &NoneChecker{}
	}
}
