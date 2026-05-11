package api

import (
	"context"
	"time"
	"github.com/BrunoSilvaFreire/tunneld/internal/daemon"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TunnelServer struct {
	pb.UnimplementedTunnelServiceServer
	supervisor *daemon.Supervisor
}

func NewTunnelServer(supervisor *daemon.Supervisor) *TunnelServer {
	return &TunnelServer{
		supervisor: supervisor,
	}
}

func (s *TunnelServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	infos := s.supervisor.ListTunnels()
	var tunnels []*pb.TunnelStatus

	for _, info := range infos {
		if req.Name != "" && info.Name != req.Name {
			continue
		}
		tunnels = append(tunnels, &pb.TunnelStatus{
			Name:   info.Name,
			Status: string(info.Status),
			Spec:   info.Spec.ToProto(),
		})
	}

	if req.Name != "" && len(tunnels) == 0 {
		return nil, status.Errorf(codes.NotFound, "tunnel %q not found", req.Name)
	}

	return &pb.StatusResponse{Tunnels: tunnels}, nil
}

func (s *TunnelServer) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	if err := s.supervisor.StartTunnel(ctx, req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.StartResponse{}, nil
}

func (s *TunnelServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	if err := s.supervisor.StopTunnel(req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.StopResponse{}, nil
}

func (s *TunnelServer) Wait(ctx context.Context, req *pb.WaitRequest) (*pb.WaitResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	st, err := s.supervisor.WaitHealthy(ctx, req.Name, timeout)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.WaitResponse{Status: string(st)}, nil
}

func (s *TunnelServer) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	if req.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}

	spec, err := tunnel.FromProto(req.Spec)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid spec: %v", err)
	}

	if err := s.supervisor.AddTunnel(ctx, spec); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateResponse{}, nil
}

func (s *TunnelServer) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if err := s.supervisor.RemoveTunnel(req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.DeleteResponse{}, nil
}
