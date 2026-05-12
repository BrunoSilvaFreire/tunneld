package api

import (
	"context"
	"os"
	"path/filepath"

	"github.com/BrunoSilvaFreire/tunneld/internal/daemon"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type KeyServer struct {
	pb.UnimplementedKeyServiceServer
	supervisor *daemon.Supervisor
}

func NewKeyServer(supervisor *daemon.Supervisor) *KeyServer {
	return &KeyServer{
		supervisor: supervisor,
	}
}

func (s *KeyServer) AddKey(ctx context.Context, req *pb.AddKeyRequest) (*pb.AddKeyResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "key name is required")
	}
	if len(req.Content) == 0 {
		return nil, status.Error(codes.InvalidArgument, "key content is required")
	}

	if err := s.supervisor.SaveKey(req.Name, req.Content); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save key: %v", err)
	}

	return &pb.AddKeyResponse{}, nil
}

func (s *KeyServer) ListKeys(ctx context.Context, req *pb.ListKeysRequest) (*pb.ListKeysResponse, error) {
	keyDir := s.supervisor.GetKeyDir()
	entries, err := os.ReadDir(keyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &pb.ListKeysResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to read key directory: %v", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	return &pb.ListKeysResponse{Names: names}, nil
}

func (s *KeyServer) DeleteKey(ctx context.Context, req *pb.DeleteKeyRequest) (*pb.DeleteKeyResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "key name is required")
	}

	keyPath := filepath.Join(s.supervisor.GetKeyDir(), req.Name)
	if err := os.Remove(keyPath); err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "key %q not found", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete key: %v", err)
	}

	return &pb.DeleteKeyResponse{}, nil
}
