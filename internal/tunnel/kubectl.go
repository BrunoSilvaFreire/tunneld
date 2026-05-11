package tunnel

import (
	"context"
	"fmt"
	"os/exec"
	"time"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"

	"google.golang.org/protobuf/types/known/durationpb"
)

type KubectlSpec struct {
	pbSpec *pb.TunnelSpec
}

type KubectlForward struct {
	LocalAddress string
	LocalPort    int
	RemotePort   int
}

func NewKubectlSpec(name string, dependsOn []string, kubeconfig, context, namespace, resource string, forwards []KubectlForward, apiServer string, skipTLS bool, health *pb.HealthCheckSpec, restart *pb.RestartPolicySpec, startup, shutdown time.Duration) *KubectlSpec {
	pbForwards := make([]*pb.KubectlForward, len(forwards))
	for i, f := range forwards {
		pbForwards[i] = &pb.KubectlForward{
			LocalAddress: f.LocalAddress,
			LocalPort:    int32(f.LocalPort),
			RemotePort:   int32(f.RemotePort),
		}
	}

	return &KubectlSpec{
		pbSpec: &pb.TunnelSpec{
			Name:            name,
			DependsOn:       dependsOn,
			Health:          health,
			Restart:         restart,
			StartupTimeout:  durationpb.New(startup),
			ShutdownTimeout: durationpb.New(shutdown),
			Type: &pb.TunnelSpec_Kubectl{
				Kubectl: &pb.KubectlSpec{
					Kubeconfig:            kubeconfig,
					Context:               context,
					Namespace:             namespace,
					Resource:              resource,
					Forwards:              pbForwards,
					ApiServer:             apiServer,
					InsecureSkipTlsVerify: skipTLS,
				},
			},
		},
	}
}

func (s *KubectlSpec) Name() string         { return s.pbSpec.Name }
func (s *KubectlSpec) Type() string         { return "kubectl" }
func (s *KubectlSpec) Dependencies() []string { return s.pbSpec.DependsOn }
func (s *KubectlSpec) HealthCheck() *pb.HealthCheckSpec { return s.pbSpec.Health }
func (s *KubectlSpec) RestartPolicy() *pb.RestartPolicySpec { return s.pbSpec.Restart }
func (s *KubectlSpec) StartupTimeout() time.Duration { return s.pbSpec.StartupTimeout.AsDuration() }
func (s *KubectlSpec) ShutdownTimeout() time.Duration { return s.pbSpec.ShutdownTimeout.AsDuration() }
func (s *KubectlSpec) ToProto() *pb.TunnelSpec { return s.pbSpec }

func (s *KubectlSpec) BuildCommand(ctx context.Context) (*exec.Cmd, error) {
	k := s.pbSpec.GetKubectl()
	args := []string{}

	if k.Kubeconfig != "" {
		args = append(args, "--kubeconfig", k.Kubeconfig)
	}
	if k.Context != "" {
		args = append(args, "--context", k.Context)
	}
	if k.ApiServer != "" {
		args = append(args, "--server", k.ApiServer)
	}
	if k.InsecureSkipTlsVerify {
		args = append(args, "--insecure-skip-tls-verify=true")
	}
	if k.Namespace != "" {
		args = append(args, "-n", k.Namespace)
	}

	args = append(args, "port-forward")

	if len(k.Forwards) > 0 && k.Forwards[0].LocalAddress != "" {
		args = append(args, "--address", k.Forwards[0].LocalAddress)
	}

	args = append(args, k.Resource)

	for _, f := range k.Forwards {
		args = append(args, fmt.Sprintf("%d:%d", f.LocalPort, f.RemotePort))
	}

	return exec.CommandContext(ctx, "kubectl", args...), nil
}
