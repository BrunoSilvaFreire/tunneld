package tunnel

import (
	"context"
	"fmt"
	"os/exec"
	"time"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"

	"google.golang.org/protobuf/types/known/durationpb"
)

type SSHSpec struct {
	pbSpec *pb.TunnelSpec
}

type SSHForward struct {
	ListenAddress string
	ListenPort    int
	TargetHost    string
	TargetPort    int
}

func NewSSHSpec(name string, dependsOn []string, user, host string, port int, identityFile string, forwards []SSHForward, options map[string]string, health *pb.HealthCheckSpec, restart *pb.RestartPolicySpec, startup, shutdown time.Duration) *SSHSpec {
	pbForwards := make([]*pb.SSHForward, len(forwards))
	for i, f := range forwards {
		pbForwards[i] = &pb.SSHForward{
			ListenAddress: f.ListenAddress,
			ListenPort:    int32(f.ListenPort),
			TargetHost:    f.TargetHost,
			TargetPort:    int32(f.TargetPort),
		}
	}

	return &SSHSpec{
		pbSpec: &pb.TunnelSpec{
			Name:            name,
			DependsOn:       dependsOn,
			Health:          health,
			Restart:         restart,
			StartupTimeout:  durationpb.New(startup),
			ShutdownTimeout: durationpb.New(shutdown),
			Type: &pb.TunnelSpec_Ssh{
				Ssh: &pb.SSHSpec{
					User:          user,
					Host:          host,
					Port:          int32(port),
					IdentityFile:  identityFile,
					LocalForwards: pbForwards,
					Options:       options,
				},
			},
		},
	}
}

func FromProto(p *pb.TunnelSpec) (Spec, error) {
	switch p.Type.(type) {
	case *pb.TunnelSpec_Ssh:
		return &SSHSpec{pbSpec: p}, nil
	case *pb.TunnelSpec_Kubectl:
		return &KubectlSpec{pbSpec: p}, nil
	default:
		return nil, fmt.Errorf("unknown tunnel type")
	}
}

func (s *SSHSpec) Name() string         { return s.pbSpec.Name }
func (s *SSHSpec) Type() string         { return "ssh" }
func (s *SSHSpec) Dependencies() []string { return s.pbSpec.DependsOn }
func (s *SSHSpec) HealthCheck() *pb.HealthCheckSpec { return s.pbSpec.Health }
func (s *SSHSpec) RestartPolicy() *pb.RestartPolicySpec { return s.pbSpec.Restart }
func (s *SSHSpec) StartupTimeout() time.Duration { return s.pbSpec.StartupTimeout.AsDuration() }
func (s *SSHSpec) ShutdownTimeout() time.Duration { return s.pbSpec.ShutdownTimeout.AsDuration() }
func (s *SSHSpec) ToProto() *pb.TunnelSpec { return s.pbSpec }

func (s *SSHSpec) BuildCommand(ctx context.Context) (*exec.Cmd, error) {
	ssh := s.pbSpec.GetSsh()
	args := []string{"-N"}

	if ssh.Port != 0 && ssh.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", ssh.Port))
	}

	if ssh.IdentityFile != "" {
		args = append(args, "-i", ssh.IdentityFile)
	}

	for k, v := range ssh.Options {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	for _, f := range ssh.LocalForwards {
		listen := f.ListenAddress
		if listen == "" {
			listen = "127.0.0.1"
		}
		args = append(args, "-L", fmt.Sprintf("%s:%d:%s:%d", listen, f.ListenPort, f.TargetHost, f.TargetPort))
	}

	dest := ssh.Host
	if ssh.User != "" {
		dest = fmt.Sprintf("%s@%s", ssh.User, ssh.Host)
	}
	args = append(args, dest)

	return exec.CommandContext(ctx, "ssh", args...), nil
}
