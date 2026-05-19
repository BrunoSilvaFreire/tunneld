package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
)

// reconcileTunnelService creates/updates a ClusterIP Service + Endpoints for a Tunnel.
// The Service is selector-less; we manage the Endpoints object directly,
// pointing at the scheduled node's IP and the dynamically-resolved actualPort.
func reconcileTunnelService(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	t *tunneldv1.Tunnel,
	nodeIP string,
	actualPort int32,
) (svcName string, err error) {
	if t.Spec.Expose == nil || !t.Spec.Expose.Service {
		return "", nil
	}
	if nodeIP == "" || actualPort <= 0 {
		return "", fmt.Errorf("missing nodeIP or actualPort for service exposure")
	}

	name := t.Spec.Expose.ServiceName
	if name == "" {
		name = t.Name
	}
	port := servicePort(t)
	if port == 0 {
		// Fall back to the dynamic actualPort if there's no semantic remote port.
		port = actualPort
	}

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: t.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, svc, func() error {
		svc.Labels = mergeLabels(svc.Labels, managedLabels(t))
		svc.Annotations = mergeLabels(svc.Annotations, managedAnnotations(t.Name))
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Selector = nil
		svc.Spec.Ports = []corev1.ServicePort{{
			Name:       "tunnel",
			Port:       port,
			TargetPort: intstr.FromInt32(actualPort),
			Protocol:   corev1.ProtocolTCP,
		}}
		return controllerutil.SetControllerReference(t, svc, scheme)
	}); err != nil {
		return "", fmt.Errorf("reconcile service: %w", err)
	}

	ep := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: t.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, ep, func() error {
		ep.Labels = mergeLabels(ep.Labels, managedLabels(t))
		ep.Annotations = mergeLabels(ep.Annotations, managedAnnotations(t.Name))
		ep.Subsets = []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{IP: nodeIP}},
			Ports: []corev1.EndpointPort{{
				Name:     "tunnel",
				Port:     actualPort,
				Protocol: corev1.ProtocolTCP,
			}},
		}}
		return controllerutil.SetControllerReference(t, ep, scheme)
	}); err != nil {
		return "", fmt.Errorf("reconcile endpoints: %w", err)
	}
	return name, nil
}

func deleteTunnelService(ctx context.Context, c client.Client, namespace, name string) error {
	for _, obj := range []client.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}},
		&corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}},
	} {
		if err := c.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// servicePort returns the semantic remote/target port for the chosen forward.
// Clients dial this stable value via the Service; kube-proxy maps it to the
// dynamic actualPort behind the scenes.
func servicePort(t *tunneldv1.Tunnel) int32 {
	idx := int32(0)
	if t.Spec.Expose != nil {
		idx = t.Spec.Expose.ForwardIndex
	}
	if t.Spec.SSH != nil && int(idx) < len(t.Spec.SSH.LocalForwards) {
		return t.Spec.SSH.LocalForwards[idx].TargetPort
	}
	if t.Spec.Kubectl != nil && int(idx) < len(t.Spec.Kubectl.Forwards) {
		return t.Spec.Kubectl.Forwards[idx].RemotePort
	}
	return 0
}

func managedLabels(t *tunneldv1.Tunnel) map[string]string {
	tunnelID := string(t.UID)
	if tunnelID == "" {
		tunnelID = shortHash(t.Namespace + "/" + t.Name)
	}
	return map[string]string{
		"app.kubernetes.io/managed-by": "tunneld-controller",
		"tunneld.io/tunnel-uid":        tunnelID,
	}
}

func managedAnnotations(tunnelName string) map[string]string {
	return map[string]string{
		"tunneld.io/tunnel": tunnelName,
	}
}

func mergeLabels(a, b map[string]string) map[string]string {
	if a == nil {
		a = map[string]string{}
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}
