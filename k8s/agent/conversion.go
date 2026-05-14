package agent

import (
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

// daemonKeyName returns the name under which a key is stored in tunneld's keyDir.
// Namespaced to avoid collisions across multiple CRD namespaces sharing one daemon.
func daemonKeyName(namespace, keyName string) string {
	return namespace + "--" + keyName
}

// daemonTunnelName is the name used inside tunneld for a given Tunnel CR.
// Namespaced for the same reason as keys.
func daemonTunnelName(namespace, tunnelName string) string {
	return namespace + "--" + tunnelName
}

// ToProto translates a Tunnel CR to the daemon's TunnelSpec proto.
// Key references are resolved to the namespaced daemonKeyName; the caller is
// responsible for having already uploaded those keys via KeyService.AddKey.
func ToProto(t *tunneldv1.Tunnel) (*pb.TunnelSpec, error) {
	if (t.Spec.SSH == nil) == (t.Spec.Kubectl == nil) {
		return nil, fmt.Errorf("exactly one of ssh or kubectl must be set")
	}

	out := &pb.TunnelSpec{
		Name:            daemonTunnelName(t.Namespace, t.Name),
		StartupTimeout:  toDurationPB(t.Spec.StartupTimeout),
		ShutdownTimeout: toDurationPB(t.Spec.ShutdownTimeout),
	}
	for _, d := range t.Spec.DependsOn {
		out.DependsOn = append(out.DependsOn, daemonTunnelName(t.Namespace, d))
	}
	if h := t.Spec.Health; h != nil {
		out.Health = &pb.HealthCheckSpec{
			Type:           h.Type,
			Address:        h.Address,
			Interval:       toDurationPB(h.Interval),
			Timeout:        toDurationPB(h.Timeout),
			StartupTimeout: toDurationPB(h.StartupTimeout),
		}
	}
	if r := t.Spec.Restart; r != nil {
		rp := &pb.RestartPolicySpec{
			Policy:      r.Policy,
			Delay:       toDurationPB(r.Delay),
			MaxAttempts: r.MaxAttempts,
		}
		if r.Backoff != nil {
			rp.Backoff = &pb.BackoffSpec{
				Multiplier: float32(r.Backoff.MultiplierMilli) / 1000.0,
				MaxDelay:   toDurationPB(r.Backoff.MaxDelay),
			}
		}
		out.Restart = rp
	}

	if t.Spec.SSH != nil {
		ssh := t.Spec.SSH
		keyRef := ""
		if ssh.IdentityKeyRef != nil {
			keyRef = daemonKeyName(t.Namespace, ssh.IdentityKeyRef.Name)
		} else if ssh.IdentityKeySecretRef != nil {
			keyRef = daemonKeyName(t.Namespace, ssh.IdentityKeySecretRef.Name)
		}
		s := &pb.SSHSpec{
			User:           ssh.User,
			Host:           ssh.Host,
			Port:           ssh.Port,
			IdentityKeyRef: keyRef,
			Options:        ssh.Options,
		}
		for _, f := range ssh.LocalForwards {
			s.LocalForwards = append(s.LocalForwards, &pb.SSHForward{
				ListenAddress: f.ListenAddress,
				ListenPort:    f.ListenPort,
				TargetHost:    f.TargetHost,
				TargetPort:    f.TargetPort,
			})
		}
		out.Type = &pb.TunnelSpec_Ssh{Ssh: s}
	} else {
		kc := t.Spec.Kubectl
		keyRef := ""
		if kc.KubeconfigRef != nil {
			keyRef = daemonKeyName(t.Namespace, kc.KubeconfigRef.Name)
		} else if kc.KubeconfigSecretRef != nil {
			keyRef = daemonKeyName(t.Namespace, kc.KubeconfigSecretRef.Name)
		}
		k := &pb.KubectlSpec{
			KubeconfigRef:         keyRef,
			Context:               kc.Context,
			Namespace:             kc.Namespace,
			Resource:              kc.Resource,
			ApiServer:             kc.APIServer,
			InsecureSkipTlsVerify: kc.InsecureSkipTLSVerify,
		}
		for _, f := range kc.Forwards {
			k.Forwards = append(k.Forwards, &pb.KubectlForward{
				LocalAddress: f.LocalAddress,
				LocalPort:    f.LocalPort,
				RemotePort:   f.RemotePort,
			})
		}
		out.Type = &pb.TunnelSpec_Kubectl{Kubectl: k}
	}

	return out, nil
}

func toDurationPB(d *metav1.Duration) *durationpb.Duration {
	if d == nil {
		return nil
	}
	return durationpb.New(d.Duration)
}
