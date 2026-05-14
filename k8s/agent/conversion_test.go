package agent

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

func TestToProto_SSH(t *testing.T) {
	tun := &tunneldv1.Tunnel{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "data"},
		Spec: tunneldv1.TunnelSpec{
			DependsOn: []string{"bastion"},
			SSH: &tunneldv1.SSHSpec{
				User: "ec2-user",
				Host: "bastion.example.com",
				Port: 22,
				IdentityKeyRef: &tunneldv1.LocalObjectRef{Name: "bastion-ssh"},
				LocalForwards: []tunneldv1.SSHForward{{
					ListenAddress: "0.0.0.0",
					ListenPort:    0,
					TargetHost:    "pg.internal",
					TargetPort:    5432,
				}},
				Options: map[string]string{"StrictHostKeyChecking": "no"},
			},
			Health: &tunneldv1.HealthCheckSpec{
				Type:           "tcp",
				StartupTimeout: &metav1.Duration{Duration: 30 * time.Second},
			},
			Restart: &tunneldv1.RestartPolicySpec{
				Policy: "on-failure",
				Backoff: &tunneldv1.BackoffSpec{
					MultiplierMilli: 2500, // 2.5x
					MaxDelay:        &metav1.Duration{Duration: time.Minute},
				},
			},
		},
	}

	spec, err := ToProto(tun)
	if err != nil {
		t.Fatalf("ToProto: %v", err)
	}

	if spec.Name != "data--pg" {
		t.Errorf("Name = %q, want data--pg", spec.Name)
	}
	if got, want := spec.DependsOn, []string{"data--bastion"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("DependsOn = %v, want %v", got, want)
	}

	ssh, ok := spec.Type.(*pb.TunnelSpec_Ssh)
	if !ok {
		t.Fatalf("Type = %T, want *TunnelSpec_Ssh", spec.Type)
	}
	if ssh.Ssh.IdentityKeyRef != "data--bastion-ssh" {
		t.Errorf("IdentityKeyRef = %q, want data--bastion-ssh", ssh.Ssh.IdentityKeyRef)
	}
	if len(ssh.Ssh.LocalForwards) != 1 || ssh.Ssh.LocalForwards[0].TargetPort != 5432 {
		t.Errorf("LocalForwards = %+v", ssh.Ssh.LocalForwards)
	}

	// Restart multiplier: 2500 milli-units → 2.5x.
	if got := spec.Restart.Backoff.Multiplier; got < 2.499 || got > 2.501 {
		t.Errorf("Backoff.Multiplier = %f, want ~2.5", got)
	}
}

func TestToProto_Kubectl(t *testing.T) {
	tun := &tunneldv1.Tunnel{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "prod"},
		Spec: tunneldv1.TunnelSpec{
			Kubectl: &tunneldv1.KubectlSpec{
				KubeconfigRef: &tunneldv1.LocalObjectRef{Name: "prod-kc"},
				Context:       "prod",
				Namespace:     "data",
				Resource:      "svc/postgres",
				Forwards: []tunneldv1.KubectlForward{{
					LocalAddress: "0.0.0.0",
					LocalPort:    0,
					RemotePort:   5432,
				}},
				InsecureSkipTLSVerify: true,
			},
		},
	}
	spec, err := ToProto(tun)
	if err != nil {
		t.Fatalf("ToProto: %v", err)
	}
	kc, ok := spec.Type.(*pb.TunnelSpec_Kubectl)
	if !ok {
		t.Fatalf("Type = %T, want *TunnelSpec_Kubectl", spec.Type)
	}
	if kc.Kubectl.KubeconfigRef != "prod--prod-kc" {
		t.Errorf("KubeconfigRef = %q, want prod--prod-kc", kc.Kubectl.KubeconfigRef)
	}
	if !kc.Kubectl.InsecureSkipTlsVerify {
		t.Errorf("InsecureSkipTlsVerify = false, want true")
	}
}

func TestToProto_RejectsBothSet(t *testing.T) {
	tun := &tunneldv1.Tunnel{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
		Spec: tunneldv1.TunnelSpec{
			SSH:     &tunneldv1.SSHSpec{Host: "a"},
			Kubectl: &tunneldv1.KubectlSpec{Resource: "svc/x"},
		},
	}
	if _, err := ToProto(tun); err == nil {
		t.Errorf("expected error when both ssh and kubectl set")
	}
}

func TestToProto_RejectsNeitherSet(t *testing.T) {
	tun := &tunneldv1.Tunnel{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"}}
	if _, err := ToProto(tun); err == nil {
		t.Errorf("expected error when neither ssh nor kubectl set")
	}
}
