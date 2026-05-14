package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func mustScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	return s
}

func node(name string, labels map[string]string, ready bool, internalIP string) *corev1.Node {
	cond := corev1.ConditionFalse
	if ready {
		cond = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: cond}},
			Addresses:  []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: internalIP}},
		},
	}
}

func TestPickNode_RespectsSelectorAndReadiness(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(mustScheme()).
		WithObjects(
			node("a-notready", map[string]string{"role": "tunnels"}, false, "10.0.0.1"),
			node("b-ready", map[string]string{"role": "tunnels"}, true, "10.0.0.2"),
			node("c-ready-wrong-label", map[string]string{"role": "other"}, true, "10.0.0.3"),
		).Build()

	name, ip, err := pickNode(context.Background(), c, map[string]string{"role": "tunnels"})
	if err != nil {
		t.Fatalf("pickNode: %v", err)
	}
	if name != "b-ready" {
		t.Errorf("name = %q, want b-ready", name)
	}
	if ip != "10.0.0.2" {
		t.Errorf("ip = %q, want 10.0.0.2", ip)
	}
}

func TestPickNode_NoMatch(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(mustScheme()).
		WithObjects(node("a", map[string]string{"role": "other"}, true, "10.0.0.1")).
		Build()
	if _, _, err := pickNode(context.Background(), c, map[string]string{"role": "tunnels"}); err == nil {
		t.Errorf("expected error when no node matches")
	}
}

func TestNodeInternalIP_FallsBackToExternal(t *testing.T) {
	n := &corev1.Node{Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
		{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
	}}}
	if got := nodeInternalIP(n); got != "1.2.3.4" {
		t.Errorf("got %q, want 1.2.3.4", got)
	}
}
