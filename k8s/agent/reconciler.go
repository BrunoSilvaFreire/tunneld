package agent

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

// TunnelReconciler runs in the per-node agent. It only processes Tunnels whose
// status.node matches NodeName, and translates the CR into gRPC calls against
// the local tunneld daemon over a Unix socket.
type TunnelReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
	Daemon   *DaemonClient
}

// +kubebuilder:rbac:groups=tunneld.io,resources=tunnels,verbs=get;list;watch
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelkeys,verbs=get;list;watch
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("tunnel", req.NamespacedName)

	var t tunneldv1.Tunnel
	if err := r.Get(ctx, req.NamespacedName, &t); err != nil {
		if apierrors.IsNotFound(err) {
			// CR gone — best-effort delete from daemon.
			_, _ = r.Daemon.Tunnel.Delete(ctx, &pb.DeleteRequest{
				Name: daemonTunnelName(req.Namespace, req.Name),
			})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if t.Status.Node != r.NodeName {
		return ctrl.Result{}, nil
	}

	// 1. Upload key material referenced by the spec.
	keyName, content, err := r.resolveKey(ctx, &t)
	if err != nil {
		return r.degrade(ctx, &t, err.Error())
	}
	if keyName != "" {
		if _, err := r.Daemon.Keys.AddKey(ctx, &pb.AddKeyRequest{Name: keyName, Content: content}); err != nil {
			return r.degrade(ctx, &t, fmt.Sprintf("AddKey %q: %v", keyName, err))
		}
	}

	// 2. Push the tunnel spec via Update (create-or-update).
	spec, err := ToProto(&t)
	if err != nil {
		return r.degrade(ctx, &t, err.Error())
	}
	if _, err := r.Daemon.Tunnel.Update(ctx, &pb.UpdateRequest{Spec: spec, Persistent: false}); err != nil {
		return r.degrade(ctx, &t, fmt.Sprintf("Update: %v", err))
	}

	// 3. Poll daemon status and reflect it onto the CR.
	resp, err := r.Daemon.Tunnel.Status(ctx, &pb.StatusRequest{Name: spec.Name})
	if err != nil || len(resp.Tunnels) == 0 {
		logger.Info("daemon status pending", "err", err)
		t.Status.Phase = tunneldv1.TunnelPhasePending
		return ctrl.Result{RequeueAfter: requeuePending}, r.Status().Update(ctx, &t)
	}
	ds := resp.Tunnels[0]

	t.Status.ResolvedForwards = nil
	for _, f := range ds.ResolvedForwards {
		t.Status.ResolvedForwards = append(t.Status.ResolvedForwards, tunneldv1.ResolvedForward{
			ListenAddress:  f.LocalAddress,
			ConfiguredPort: f.ConfiguredPort,
			ActualPort:     f.ActualPort,
			RemotePort:     f.RemotePort,
		})
	}
	t.Status.Phase = mapDaemonPhase(ds.Status)
	t.Status.LastError = ds.Error
	t.Status.ObservedGeneration = t.Generation

	if err := r.Status().Update(ctx, &t); err != nil {
		return ctrl.Result{}, err
	}

	// Keep polling while not yet Healthy so the controller sees actualPort soon.
	if t.Status.Phase != tunneldv1.TunnelPhaseHealthy {
		return ctrl.Result{RequeueAfter: requeuePending}, nil
	}
	return ctrl.Result{RequeueAfter: requeueSteady}, nil
}

// resolveKey returns the daemon-side key name and bytes to upload, or empty if
// the tunnel needs no credentials.
func (r *TunnelReconciler) resolveKey(ctx context.Context, t *tunneldv1.Tunnel) (string, []byte, error) {
	var (
		secretName, secretKey string
		dKeyName              string
	)
	switch {
	case t.Spec.SSH != nil && t.Spec.SSH.IdentityKeyRef != nil:
		var k tunneldv1.TunnelKey
		if err := r.Get(ctx, client.ObjectKey{Namespace: t.Namespace, Name: t.Spec.SSH.IdentityKeyRef.Name}, &k); err != nil {
			return "", nil, fmt.Errorf("TunnelKey %q: %w", t.Spec.SSH.IdentityKeyRef.Name, err)
		}
		if k.Spec.Source.SecretRef == nil {
			return "", nil, fmt.Errorf("TunnelKey %q has no secretRef", k.Name)
		}
		secretName, secretKey = k.Spec.Source.SecretRef.Name, k.Spec.Source.SecretRef.Key
		dKeyName = daemonKeyName(t.Namespace, k.Name)
	case t.Spec.SSH != nil && t.Spec.SSH.IdentityKeySecretRef != nil:
		secretName, secretKey = t.Spec.SSH.IdentityKeySecretRef.Name, t.Spec.SSH.IdentityKeySecretRef.Key
		dKeyName = daemonKeyName(t.Namespace, secretName)
	case t.Spec.Kubectl != nil && t.Spec.Kubectl.KubeconfigRef != nil:
		var k tunneldv1.TunnelKey
		if err := r.Get(ctx, client.ObjectKey{Namespace: t.Namespace, Name: t.Spec.Kubectl.KubeconfigRef.Name}, &k); err != nil {
			return "", nil, fmt.Errorf("TunnelKey %q: %w", t.Spec.Kubectl.KubeconfigRef.Name, err)
		}
		if k.Spec.Source.SecretRef == nil {
			return "", nil, fmt.Errorf("TunnelKey %q has no secretRef", k.Name)
		}
		secretName, secretKey = k.Spec.Source.SecretRef.Name, k.Spec.Source.SecretRef.Key
		dKeyName = daemonKeyName(t.Namespace, k.Name)
	case t.Spec.Kubectl != nil && t.Spec.Kubectl.KubeconfigSecretRef != nil:
		secretName, secretKey = t.Spec.Kubectl.KubeconfigSecretRef.Name, t.Spec.Kubectl.KubeconfigSecretRef.Key
		dKeyName = daemonKeyName(t.Namespace, secretName)
	default:
		return "", nil, nil
	}

	var sec corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{Namespace: t.Namespace, Name: secretName}, &sec); err != nil {
		return "", nil, fmt.Errorf("Secret %q: %w", secretName, err)
	}
	data, ok := sec.Data[secretKey]
	if !ok {
		return "", nil, fmt.Errorf("Secret %q has no key %q", secretName, secretKey)
	}
	return dKeyName, data, nil
}

func (r *TunnelReconciler) degrade(ctx context.Context, t *tunneldv1.Tunnel, msg string) (ctrl.Result, error) {
	t.Status.Phase = tunneldv1.TunnelPhaseDegraded
	t.Status.LastError = msg
	t.Status.ObservedGeneration = t.Generation
	if err := r.Status().Update(ctx, t); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueErr}, nil
}

func mapDaemonPhase(s string) tunneldv1.TunnelPhase {
	switch s {
	case "running", "healthy":
		return tunneldv1.TunnelPhaseHealthy
	case "starting", "pending":
		return tunneldv1.TunnelPhasePending
	case "stopped":
		return tunneldv1.TunnelPhaseStopped
	case "failed":
		return tunneldv1.TunnelPhaseFailed
	default:
		return tunneldv1.TunnelPhaseDegraded
	}
}

func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only watch Tunnels assigned to this node.
	mine := predicate.NewPredicateFuncs(func(o client.Object) bool {
		t, ok := o.(*tunneldv1.Tunnel)
		return ok && t.Status.Node == r.NodeName
	})
	// Also process create/delete events with no status yet so we can clean up on delete.
	allEvents := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return true },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunneldv1.Tunnel{}, builder.WithPredicates(predicate.Or(mine, allEvents))).
		Complete(r)
}
