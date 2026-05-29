package api

import (
	"context"
	"path/filepath"
	"testing"
	"net"

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

func setupKeyTest(t *testing.T) (pb.KeyServiceClient, func()) {
	t.Helper()

	keyDir := t.TempDir()
	planner := dependency.NewPlanner(nil)
	cfg := &config.Config{Tunnels: make(map[string]config.TunnelConfig)}
	supervisor := daemon.NewSupervisor(planner, cfg, keyDir, "")

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterKeyServiceServer(s, NewKeyServer(supervisor))

	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := pb.NewKeyServiceClient(conn)

	cleanup := func() {
		conn.Close()
		s.Stop()
		lis.Close()
	}
	return client, cleanup
}

func TestKeyServer_AddKey(t *testing.T) {
	client, cleanup := setupKeyTest(t)
	defer cleanup()

	ctx := context.Background()

	// Empty name
	_, err := client.AddKey(ctx, &pb.AddKeyRequest{Content: []byte("ssh-rsa AAAA")})
	if err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Expected InvalidArgument for empty name, got %v", err)
	}

	// Empty content
	_, err = client.AddKey(ctx, &pb.AddKeyRequest{Name: "my-key"})
	if err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Expected InvalidArgument for empty content, got %v", err)
	}

	// Valid
	_, err = client.AddKey(ctx, &pb.AddKeyRequest{Name: "my-key", Content: []byte("ssh-rsa AAAA")})
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}
}

func TestKeyServer_ListKeys(t *testing.T) {
	client, cleanup := setupKeyTest(t)
	defer cleanup()

	ctx := context.Background()

	// Initial empty list
	resp, err := client.ListKeys(ctx, &pb.ListKeysRequest{})
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}
	if len(resp.Names) != 0 {
		t.Fatalf("Expected 0 keys, got %d", len(resp.Names))
	}

	// Add key
	_, err = client.AddKey(ctx, &pb.AddKeyRequest{Name: "key1", Content: []byte("content1")})
	if err != nil {
		t.Fatalf("Failed to add key1: %v", err)
	}
	_, err = client.AddKey(ctx, &pb.AddKeyRequest{Name: "key2", Content: []byte("content2")})
	if err != nil {
		t.Fatalf("Failed to add key2: %v", err)
	}

	// List again
	resp, err = client.ListKeys(ctx, &pb.ListKeysRequest{})
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}
	if len(resp.Names) != 2 {
		t.Fatalf("Expected 2 keys, got %d", len(resp.Names))
	}

	found1, found2 := false, false
	for _, n := range resp.Names {
		if n == "key1" {
			found1 = true
		}
		if n == "key2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("Expected key1 and key2, got %v", resp.Names)
	}
}

func TestKeyServer_DeleteKey(t *testing.T) {
	client, cleanup := setupKeyTest(t)
	defer cleanup()

	ctx := context.Background()

	// Empty name
	_, err := client.DeleteKey(ctx, &pb.DeleteKeyRequest{})
	if err == nil || status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Expected InvalidArgument for empty name, got %v", err)
	}

	// Not found
	_, err = client.DeleteKey(ctx, &pb.DeleteKeyRequest{Name: "non-existent"})
	if err == nil || status.Code(err) != codes.NotFound {
		t.Fatalf("Expected NotFound, got %v", err)
	}

	// Valid add and delete
	_, err = client.AddKey(ctx, &pb.AddKeyRequest{Name: "my-key", Content: []byte("content")})
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	_, err = client.DeleteKey(ctx, &pb.DeleteKeyRequest{Name: "my-key"})
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// Verify it's gone
	resp, err := client.ListKeys(ctx, &pb.ListKeysRequest{})
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}
	if len(resp.Names) != 0 {
		t.Fatalf("Expected 0 keys after deletion, got %d", len(resp.Names))
	}
}

func TestKeyServer_ListKeys_MissingDir(t *testing.T) {
	// Custom setup to point to missing dir
	planner := dependency.NewPlanner(nil)
	cfg := &config.Config{Tunnels: make(map[string]config.TunnelConfig)}
	
	// Create a dir, then delete it so it's missing
	tmpDir := t.TempDir()
	missingDir := filepath.Join(tmpDir, "missing")
	
	supervisor := daemon.NewSupervisor(planner, cfg, missingDir, "")
	server := NewKeyServer(supervisor)
	
	resp, err := server.ListKeys(context.Background(), &pb.ListKeysRequest{})
	if err != nil {
		t.Fatalf("ListKeys should handle missing dir gracefully, got error: %v", err)
	}
	if len(resp.Names) != 0 {
		t.Fatalf("Expected 0 keys for missing dir, got %v", resp.Names)
	}
}
