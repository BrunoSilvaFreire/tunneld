package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
)

func sceneScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := tunneldv1.AddToScheme(s); err != nil {
		t.Fatalf("tunneld AddToScheme: %v", err)
	}
	return s
}

func TestReconcileTunnelService_CreatesServiceAndEndpoints(t *testing.T) {
	s := sceneScheme(t)
	tun := &tunneldv1.Tunnel{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "data", UID: "uid-1"},
		Spec: tunneldv1.TunnelSpec{
			SSH: &tunneldv1.SSHSpec{
				LocalForwards: []tunneldv1.SSHForward{{TargetPort: 5432}},
			},
			Expose: &tunneldv1.ExposeSpec{Service: true},
		},
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(tun).Build()

	svcName, err := reconcileTunnelService(context.Background(), c, s, tun, "10.0.0.5", 34211)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if svcName != "pg" {
		t.Errorf("svcName = %q, want pg", svcName)
	}

	var svc corev1.Service
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "data", Name: "pg"}, &svc); err != nil {
		t.Fatalf("get service: %v", err)
	}
	if got := svc.Spec.Ports[0].Port; got != 5432 {
		t.Errorf("service Port = %d, want 5432", got)
	}
	if got := svc.Spec.Ports[0].TargetPort.IntValue(); got != 34211 {
		t.Errorf("service TargetPort = %d, want 34211", got)
	}
	if svc.Spec.Selector != nil {
		t.Errorf("expected selector-less Service, got %v", svc.Spec.Selector)
	}

	var ep corev1.Endpoints
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "data", Name: "pg"}, &ep); err != nil {
		t.Fatalf("get endpoints: %v", err)
	}
	if len(ep.Subsets) != 1 || ep.Subsets[0].Addresses[0].IP != "10.0.0.5" {
		t.Errorf("endpoints addresses = %+v", ep.Subsets)
	}
	if ep.Subsets[0].Ports[0].Port != 34211 {
		t.Errorf("endpoints port = %d, want 34211", ep.Subsets[0].Ports[0].Port)
	}
}

func TestReconcileTunnelService_SkipsWhenExposeDisabled(t *testing.T) {
	s := sceneScheme(t)
	tun := &tunneldv1.Tunnel{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "data"},
		Spec: tunneldv1.TunnelSpec{
			SSH: &tunneldv1.SSHSpec{LocalForwards: []tunneldv1.SSHForward{{TargetPort: 5432}}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(s).Build()
	name, err := reconcileTunnelService(context.Background(), c, s, tun, "10.0.0.5", 34211)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if name != "" {
		t.Errorf("name = %q, want empty when expose disabled", name)
	}
}

func TestDeleteTunnelService_IsIdempotent(t *testing.T) {
	s := sceneScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	if err := deleteTunnelService(context.Background(), c, "data", "missing"); err != nil {
		t.Errorf("delete on missing svc returned %v, want nil", err)
	}
	// Sanity: confirm the not-found path is actually exercised.
	err := c.Get(context.Background(), types.NamespacedName{Namespace: "data", Name: "missing"}, &corev1.Service{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}
