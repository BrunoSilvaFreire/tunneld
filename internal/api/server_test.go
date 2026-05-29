package api

import (
	"context"
	"net"
	"testing"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/daemon"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T) (*grpc.Server, pb.TunnelServiceClient, *daemon.Supervisor) {
	t.Helper()
	listener := bufconn.Listen(bufSize)

	// create supervisor
	planner := dependency.NewPlanner(nil)
	cfg := &config.Config{}
	sup := daemon.NewSupervisor(planner, cfg, t.TempDir(), t.TempDir())

	// create server
	srv := grpc.NewServer()
	tunnelServer := NewTunnelServer(sup)
	pb.RegisterTunnelServiceServer(srv, tunnelServer)

	go func() {
		if err := srv.Serve(listener); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	client := pb.NewTunnelServiceClient(conn)
	return srv, client, sup
}

func TestStatus_Empty(t *testing.T) {
	_, client, _ := setupTestServer(t)
	resp, err := client.Status(context.Background(), &pb.StatusRequest{})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(resp.Tunnels) != 0 {
		t.Errorf("Expected 0 tunnels, got %d", len(resp.Tunnels))
	}
}

func TestCreate_InvalidSpec(t *testing.T) {
	_, client, _ := setupTestServer(t)
	_, err := client.Create(context.Background(), &pb.CreateRequest{})
	if err == nil {
		t.Fatal("Expected error on empty spec, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestCreate_ValidSpec(t *testing.T) {
	_, client, sup := setupTestServer(t)
	
	spec := &pb.TunnelSpec{
		Name: "test-tunnel",
		Type: &pb.TunnelSpec_Ssh{
			Ssh: &pb.SSHSpec{
				Host: "example.com",
			},
		},
	}
	
	_, err := client.Create(context.Background(), &pb.CreateRequest{Spec: spec})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	if len(sup.ListTunnels()) != 1 {
		t.Errorf("Expected 1 tunnel in supervisor, got %d", len(sup.ListTunnels()))
	}
	
	resp, err := client.Status(context.Background(), &pb.StatusRequest{Name: "test-tunnel"})
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(resp.Tunnels) != 1 || resp.Tunnels[0].Name != "test-tunnel" {
		t.Errorf("Unexpected status response: %v", resp)
	}
}

func TestDelete(t *testing.T) {
	_, client, _ := setupTestServer(t)
	
	spec := &pb.TunnelSpec{
		Name: "test-delete",
		Type: &pb.TunnelSpec_Ssh{
			Ssh: &pb.SSHSpec{Host: "example.com"},
		},
	}
	
	client.Create(context.Background(), &pb.CreateRequest{Spec: spec})
	
	_, err := client.Delete(context.Background(), &pb.DeleteRequest{Name: "test-delete"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	resp, _ := client.Status(context.Background(), &pb.StatusRequest{})
	if len(resp.Tunnels) != 0 {
		t.Errorf("Expected 0 tunnels after delete, got %d", len(resp.Tunnels))
	}
}

func TestWait_Timeout(t *testing.T) {
	_, client, _ := setupTestServer(t)
	
	spec := &pb.TunnelSpec{
		Name: "test-wait",
		Type: &pb.TunnelSpec_Ssh{
			Ssh: &pb.SSHSpec{Host: "example.com"},
		},
	}
	
	client.Create(context.Background(), &pb.CreateRequest{Spec: spec})
	
	// Fast timeout since the tunnel is just a mock and won't actually start correctly
	_, err := client.Wait(context.Background(), &pb.WaitRequest{
		Name: "test-wait",
		TimeoutSeconds: 1,
	})
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}
