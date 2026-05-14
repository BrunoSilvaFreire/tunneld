package agent

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

// DaemonClient is a thin wrapper around the generated gRPC clients connected
// over the local tunneld Unix socket.
type DaemonClient struct {
	conn   *grpc.ClientConn
	Tunnel pb.TunnelServiceClient
	Keys   pb.KeyServiceClient
}

func Dial(ctx context.Context, socketPath string) (*DaemonClient, error) {
	conn, err := grpc.DialContext(
		ctx,
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "unix", socketPath)
		}),
	)
	if err != nil {
		return nil, err
	}
	return &DaemonClient{
		conn:   conn,
		Tunnel: pb.NewTunnelServiceClient(conn),
		Keys:   pb.NewKeyServiceClient(conn),
	}, nil
}

func (c *DaemonClient) Close() error { return c.conn.Close() }
