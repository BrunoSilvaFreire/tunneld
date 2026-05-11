package config

import (
	"fmt"
	"os"
	"time"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"

	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Tunnels map[string]TunnelConfig `yaml:"tunnels"`
}

type TunnelConfig struct {
	Enabled         bool               `yaml:"enabled"`
	Type            string             `yaml:"type"`
	DependsOn       []string           `yaml:"depends_on"`
	SSH             *SSHConfig         `yaml:"ssh,omitempty"`
	Kubectl         *KubectlConfig     `yaml:"kubectl,omitempty"`
	Health          HealthCheckConfig  `yaml:"health"`
	Restart         RestartPolicyConfig `yaml:"restart"`
	StartupTimeout  time.Duration      `yaml:"startup_timeout"`
	ShutdownTimeout time.Duration      `yaml:"shutdown_timeout"`
}

type HealthCheckConfig struct {
	Type           string        `yaml:"type"`
	Address        string        `yaml:"address,omitempty"`
	Interval       time.Duration `yaml:"interval,omitempty"`
	Timeout        time.Duration `yaml:"timeout,omitempty"`
	StartupTimeout time.Duration `yaml:"startup_timeout,omitempty"`
}

type RestartPolicyConfig struct {
	Policy      string        `yaml:"policy"` // always, on-failure, never
	Delay       time.Duration `yaml:"delay"`
	MaxAttempts int           `yaml:"max_attempts"`
}

type SSHConfig struct {
	User          string            `yaml:"user"`
	Host          string            `yaml:"host"`
	Port          int               `yaml:"port"`
	IdentityFile  string            `yaml:"identity_file"`
	LocalForwards []SSHForward      `yaml:"local_forwards"`
	Options       map[string]string `yaml:"options"`
}

type SSHForward struct {
	ListenAddress string `yaml:"listen_address"`
	ListenPort    int    `yaml:"listen_port"`
	TargetHost    string `yaml:"target_host"`
	TargetPort    int    `yaml:"target_port"`
}

type KubectlConfig struct {
	Kubeconfig            string           `yaml:"kubeconfig"`
	Context               string           `yaml:"context"`
	Namespace             string           `yaml:"namespace"`
	Resource              string           `yaml:"resource"`
	Forwards              []KubectlForward `yaml:"forwards"`
	APIServer             string           `yaml:"api_server"`
	InsecureSkipTLSVerify bool             `yaml:"insecure_skip_tls_verify"`
}

type KubectlForward struct {
	LocalAddress string `yaml:"local_address"`
	LocalPort    int    `yaml:"local_port"`
	RemotePort   int    `yaml:"remote_port"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (t *TunnelConfig) ToProto(name string) (*pb.TunnelSpec, error) {
	p := &pb.TunnelSpec{
		Name:            name,
		DependsOn:       t.DependsOn,
		StartupTimeout:  durationpb.New(t.StartupTimeout),
		ShutdownTimeout: durationpb.New(t.ShutdownTimeout),
		Health: &pb.HealthCheckSpec{
			Type:           t.Health.Type,
			Address:        t.Health.Address,
			Interval:       durationpb.New(t.Health.Interval),
			Timeout:        durationpb.New(t.Health.Timeout),
			StartupTimeout: durationpb.New(t.Health.StartupTimeout),
		},
		Restart: &pb.RestartPolicySpec{
			Policy:      t.Restart.Policy,
			Delay:       durationpb.New(t.Restart.Delay),
			MaxAttempts: int32(t.Restart.MaxAttempts),
		},
	}

	switch t.Type {
	case "ssh":
		if t.SSH == nil {
			return nil, fmt.Errorf("ssh config is required for type ssh")
		}
		forwards := make([]*pb.SSHForward, len(t.SSH.LocalForwards))
		for i, f := range t.SSH.LocalForwards {
			forwards[i] = &pb.SSHForward{
				ListenAddress: f.ListenAddress,
				ListenPort:    int32(f.ListenPort),
				TargetHost:    f.TargetHost,
				TargetPort:    int32(f.TargetPort),
			}
		}
		p.Type = &pb.TunnelSpec_Ssh{
			Ssh: &pb.SSHSpec{
				User:          t.SSH.User,
				Host:          t.SSH.Host,
				Port:          int32(t.SSH.Port),
				IdentityFile:  t.SSH.IdentityFile,
				LocalForwards: forwards,
				Options:       t.SSH.Options,
			},
		}
	case "kubectl":
		if t.Kubectl == nil {
			return nil, fmt.Errorf("kubectl config is required for type kubectl")
		}
		forwards := make([]*pb.KubectlForward, len(t.Kubectl.Forwards))
		for i, f := range t.Kubectl.Forwards {
			forwards[i] = &pb.KubectlForward{
				LocalAddress: f.LocalAddress,
				LocalPort:    int32(f.LocalPort),
				RemotePort:   int32(f.RemotePort),
			}
		}
		p.Type = &pb.TunnelSpec_Kubectl{
			Kubectl: &pb.KubectlSpec{
				Kubeconfig:            t.Kubectl.Kubeconfig,
				Context:               t.Kubectl.Context,
				Namespace:             t.Kubectl.Namespace,
				Resource:              t.Kubectl.Resource,
				Forwards:              forwards,
				ApiServer:             t.Kubectl.APIServer,
				InsecureSkipTlsVerify: t.Kubectl.InsecureSkipTLSVerify,
			},
		}
	default:
		return nil, fmt.Errorf("unknown tunnel type %q", t.Type)
	}

	return p, nil
}

func (t *TunnelConfig) ToSpec(name string) (tunnel.Spec, error) {
	p, err := t.ToProto(name)
	if err != nil {
		return nil, err
	}
	return tunnel.FromProto(p)
}

func ParseTunnel(name string, data []byte) (tunnel.Spec, error) {
	var t TunnelConfig
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return t.ToSpec(name)
}

func (c *Config) ToSpecs() (map[string]tunnel.Spec, error) {
	specs := make(map[string]tunnel.Spec)
	for name, t := range c.Tunnels {
		if !t.Enabled {
			continue
		}
		spec, err := t.ToSpec(name)
		if err != nil {
			return nil, fmt.Errorf("tunnel %q: %v", name, err)
		}
		specs[name] = spec
	}
	return specs, nil
}

func (c *Config) Validate() error {
	for name, t := range c.Tunnels {
		if !t.Enabled {
			continue
		}
		if t.Type == "" {
			return fmt.Errorf("tunnel %q: type is required", name)
		}
		if _, err := t.ToSpec(name); err != nil {
			return fmt.Errorf("tunnel %q: %v", name, err)
		}
	}
	return nil
}
