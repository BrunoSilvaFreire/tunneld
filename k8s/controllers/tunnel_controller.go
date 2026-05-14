package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
)

const requeueAfterErr = 15 * time.Second

// TunnelReconciler reconciles Tunnel objects from the central controller side:
//   - validates the spec (oneof ssh/kubectl, key refs)
//   - schedules the tunnel onto a Node by writing status.node
//   - manages the public Service/Endpoints once the agent reports an actualPort
//
// It deliberately does NOT talk to tunneld — that's the agent's job. The
// controller is purely a Kubernetes-API actor.
type TunnelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tunneld.io,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnels/finalizers,verbs=update
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelkeys,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var t tunneldv1.Tunnel
	if err := r.Get(ctx, req.NamespacedName, &t); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Validate exactly one of ssh/kubectl is set.
	if (t.Spec.SSH == nil) == (t.Spec.Kubectl == nil) {
		return r.fail(ctx, &t, "exactly one of spec.ssh or spec.kubectl must be set")
	}

	// 1. Schedule onto a node if not already scheduled (or selected node disappeared).
	if t.Status.Node == "" || !r.nodeStillValid(ctx, t.Status.Node, t.Spec.NodeSelector) {
		node, _, err := pickNode(ctx, r.Client, t.Spec.NodeSelector)
		if err != nil {
			return r.degrade(ctx, &t, fmt.Sprintf("scheduling: %v", err))
		}
		t.Status.Node = node
		t.Status.Phase = tunneldv1.TunnelPhasePending
		if err := r.Status().Update(ctx, &t); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("scheduled tunnel", "tunnel", t.Name, "node", node)
		// Requeue once status is persisted so we can manage Service in the next pass.
		return ctrl.Result{Requeue: true}, nil
	}

	// 2. Manage Service exposure when the agent has reported an actualPort.
	exposeRequested := t.Spec.Expose != nil && t.Spec.Expose.Service
	actualPort := pickActualPort(&t)
	if exposeRequested && actualPort > 0 {
		_, nodeIP, err := r.nodeAddr(ctx, t.Status.Node)
		if err != nil {
			return r.degrade(ctx, &t, fmt.Sprintf("node lookup: %v", err))
		}
		svcName, err := reconcileTunnelService(ctx, r.Client, r.Scheme, &t, nodeIP, actualPort)
		if err != nil {
			return ctrl.Result{}, err
		}
		if svcName != "" {
			t.Status.ServiceRef = &tunneldv1.LocalObjectRef{Name: svcName}
			if err := r.Status().Update(ctx, &t); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else if !exposeRequested && t.Status.ServiceRef != nil {
		if err := deleteTunnelService(ctx, r.Client, t.Namespace, t.Status.ServiceRef.Name); err != nil {
			return ctrl.Result{}, err
		}
		t.Status.ServiceRef = nil
		if err := r.Status().Update(ctx, &t); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) nodeStillValid(ctx context.Context, name string, selector map[string]string) bool {
	var n corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: name}, &n); err != nil {
		return false
	}
	for k, v := range selector {
		if n.Labels[k] != v {
			return false
		}
	}
	return isNodeReady(&n)
}

func (r *TunnelReconciler) nodeAddr(ctx context.Context, name string) (nodeName, ip string, err error) {
	var n corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: name}, &n); err != nil {
		return "", "", err
	}
	return name, nodeInternalIP(&n), nil
}

func (r *TunnelReconciler) degrade(ctx context.Context, t *tunneldv1.Tunnel, msg string) (ctrl.Result, error) {
	t.Status.Phase = tunneldv1.TunnelPhaseDegraded
	t.Status.LastError = msg
	t.Status.ObservedGeneration = t.Generation
	_ = r.setCondition(ctx, t, "Ready", metav1.ConditionFalse, "Degraded", msg)
	return ctrl.Result{RequeueAfter: requeueAfterErr}, r.Status().Update(ctx, t)
}

func (r *TunnelReconciler) fail(ctx context.Context, t *tunneldv1.Tunnel, msg string) (ctrl.Result, error) {
	t.Status.Phase = tunneldv1.TunnelPhaseFailed
	t.Status.LastError = msg
	t.Status.ObservedGeneration = t.Generation
	_ = r.setCondition(ctx, t, "Ready", metav1.ConditionFalse, "Invalid", msg)
	return ctrl.Result{}, r.Status().Update(ctx, t)
}

func (r *TunnelReconciler) setCondition(_ context.Context, t *tunneldv1.Tunnel, ctype string, st metav1.ConditionStatus, reason, msg string) error {
	cond := metav1.Condition{
		Type:               ctype,
		Status:             st,
		Reason:             reason,
		Message:            msg,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: t.Generation,
	}
	// Replace any existing condition of the same type.
	for i, c := range t.Status.Conditions {
		if c.Type == ctype {
			t.Status.Conditions[i] = cond
			return nil
		}
	}
	t.Status.Conditions = append(t.Status.Conditions, cond)
	return nil
}

// pickActualPort returns the first resolved actualPort the agent has reported,
// using Expose.ForwardIndex when set. Zero means "not reported yet".
func pickActualPort(t *tunneldv1.Tunnel) int32 {
	idx := int32(0)
	if t.Spec.Expose != nil {
		idx = t.Spec.Expose.ForwardIndex
	}
	if int(idx) >= len(t.Status.ResolvedForwards) {
		return 0
	}
	return t.Status.ResolvedForwards[idx].ActualPort
}

func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunneldv1.Tunnel{}).
		Complete(r)
}
