package api

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/BrunoSilvaFreire/tunneld/internal/constants"
	"github.com/BrunoSilvaFreire/tunneld/internal/daemon"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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

func toProtoResolvedForwards(ms []tunnel.PortMapping) []*pb.ResolvedForward {
	out := make([]*pb.ResolvedForward, len(ms))
	for i, m := range ms {
		out[i] = &pb.ResolvedForward{
			LocalAddress:   m.LocalAddress,
			ConfiguredPort: m.ConfiguredPort,
			ActualPort:     m.ActualPort,
			RemotePort:     m.RemotePort,
		}
	}
	return out
}

func (s *TunnelServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	infos := s.supervisor.ListTunnels()
	var tunnels []*pb.TunnelStatus

	for _, info := range infos {
		if req.Name != "" && info.Name != req.Name {
			continue
		}
		tunnels = append(tunnels, &pb.TunnelStatus{
			Name:             info.Name,
			Status:           string(info.Status),
			Spec:             info.Spec.ToProto(),
			Error:            info.Error,
			ResolvedForwards: toProtoResolvedForwards(info.PortMappings),
		})
	}

	if req.Name != "" && len(tunnels) == 0 {
		return nil, status.Errorf(codes.NotFound, "tunnel %q not found", req.Name)
	}

	return &pb.StatusResponse{Tunnels: tunnels}, nil
}

// WatchStatus streams the current status of one or all tunnels on a periodic
// tick. The first frame is sent immediately so clients have a baseline; after
// that, frames arrive at the requested interval until the client disconnects.
//
// This is a simple polling stream rather than an event subscription — the
// supervisor exposes no event channel today, and polling once per cadence is
// cheap enough (a map iteration under a mutex) for the scale we care about.
// Migrating to push semantics is additive when needed.
func (s *TunnelServer) WatchStatus(req *pb.WatchStatusRequest, stream pb.TunnelService_WatchStatusServer) error {
	interval := time.Duration(req.IntervalMs) * time.Millisecond
	switch {
	case interval <= 0:
		interval = 2 * time.Second
	case interval < 200*time.Millisecond:
		interval = 200 * time.Millisecond
	case interval > 60*time.Second:
		interval = 60 * time.Second
	}

	send := func() error {
		resp, err := s.Status(stream.Context(), &pb.StatusRequest{Name: req.Name})
		if err != nil {
			// NotFound is non-fatal for an unfiltered stream — the tunnel may
			// appear later. Translate to an empty frame and keep streaming.
			if status.Code(err) == codes.NotFound && req.Name == "" {
				return stream.Send(&pb.StatusResponse{})
			}
			return err
		}
		return stream.Send(resp)
	}

	if err := send(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
}

func (s *TunnelServer) Logs(req *pb.LogsRequest, stream pb.TunnelService_LogsServer) error {
	p, err := s.supervisor.GetProcess(req.Name)
	if err != nil {
		return status.Error(codes.NotFound, err.Error())
	}

	file, err := os.Open(p.LogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No logs yet
		}
		return status.Errorf(codes.Internal, "failed to open log file: %v", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if err := stream.Send(&pb.LogsResponse{Line: line}); err != nil {
				return err
			}
		}

		if err == io.EOF {
			if !req.Follow {
				return nil
			}
			// Wait for more data
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err != nil {
			return status.Errorf(codes.Internal, "error reading logs: %v", err)
		}

		// Check if client disconnected
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
		}
	}
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

func (s *TunnelServer) Enable(ctx context.Context, req *pb.EnableRequest) (*pb.EnableResponse, error) {
	if err := s.supervisor.EnableTunnel(ctx, req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EnableResponse{}, nil
}

func (s *TunnelServer) Disable(ctx context.Context, req *pb.DisableRequest) (*pb.DisableResponse, error) {
	if err := s.supervisor.DisableTunnel(req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.DisableResponse{}, nil
}

func (s *TunnelServer) Wait(ctx context.Context, req *pb.WaitRequest) (*pb.WaitResponse, error) {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = constants.DefaultWaitTimeout
	}

	st, err := s.supervisor.WaitHealthy(ctx, req.Name, timeout)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var fwds []*pb.ResolvedForward
	if p, perr := s.supervisor.GetProcess(req.Name); perr == nil {
		fwds = toProtoResolvedForwards(p.PortMappings())
	}

	return &pb.WaitResponse{Status: string(st), ResolvedForwards: fwds}, nil
}

func (s *TunnelServer) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	if req.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}

	// Save inline keys if any
	for name, content := range req.InlineKeys {
		if err := s.supervisor.SaveKey(name, content); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to save inline key %q: %v", name, err)
		}
	}

	spec, err := tunnel.FromProto(req.Spec)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid spec: %v", err)
	}

	if err := s.supervisor.AddTunnel(ctx, spec, req.Persistent); err != nil {
		// If it already exists, check if we should fail or just return success
		if strings.Contains(err.Error(), "already exists") {
			// In a real system, we might check if specs match.
			// For now, let's treat it as success to be idempotent for Ansible.
			return &pb.CreateResponse{}, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateResponse{}, nil
}

func (s *TunnelServer) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if req.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}

	for name, content := range req.InlineKeys {
		if err := s.supervisor.SaveKey(name, content); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to save inline key %q: %v", name, err)
		}
	}

	spec, err := tunnel.FromProto(req.Spec)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid spec: %v", err)
	}

	for _, info := range s.supervisor.ListTunnels() {
		if info.Name != spec.Name() {
			continue
		}
		if proto.Equal(info.Spec.ToProto(), spec.ToProto()) {
			if err := s.supervisor.StartTunnel(ctx, spec.Name()); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			return &pb.UpdateResponse{}, nil
		}
		break
	}

	// Remove if present, then add. RemoveTunnel is a no-op equivalent for missing names
	// (it returns an error we tolerate) so Update doubles as Create-or-Update.
	_ = s.supervisor.RemoveTunnel(spec.Name())

	if err := s.supervisor.AddTunnel(ctx, spec, req.Persistent); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.UpdateResponse{}, nil
}

func (s *TunnelServer) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if err := s.supervisor.RemoveTunnel(req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.DeleteResponse{}, nil
}
