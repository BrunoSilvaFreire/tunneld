package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
)

// TunnelKeyReconciler validates that the referenced Secret exists and exposes
// the named key. Actual installation onto a node is performed by the agent
// (which patches status.installedNodes); the controller only flips Ready.
type TunnelKeyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelkeys/status,verbs=get;update;patch

func (r *TunnelKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var key tunneldv1.TunnelKey
	if err := r.Get(ctx, req.NamespacedName, &key); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if key.Spec.Source.SecretRef == nil {
		return r.setPhase(ctx, &key, tunneldv1.TunnelKeyPhaseFailed, "spec.source.secretRef is required")
	}

	var sec corev1.Secret
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Spec.Source.SecretRef.Name}, &sec)
	if apierrors.IsNotFound(err) {
		return r.setPhase(ctx, &key, tunneldv1.TunnelKeyPhasePending, fmt.Sprintf("secret %q not found", key.Spec.Source.SecretRef.Name))
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if _, ok := sec.Data[key.Spec.Source.SecretRef.Key]; !ok {
		return r.setPhase(ctx, &key, tunneldv1.TunnelKeyPhaseFailed,
			fmt.Sprintf("secret %q has no key %q", sec.Name, key.Spec.Source.SecretRef.Key))
	}

	return r.setPhase(ctx, &key, tunneldv1.TunnelKeyPhaseReady, "")
}

func (r *TunnelKeyReconciler) setPhase(ctx context.Context, key *tunneldv1.TunnelKey, phase tunneldv1.TunnelKeyPhase, msg string) (ctrl.Result, error) {
	if key.Status.Phase == phase && key.Status.LastError == msg && key.Status.ObservedGeneration == key.Generation {
		return ctrl.Result{}, nil
	}
	key.Status.Phase = phase
	key.Status.LastError = msg
	key.Status.ObservedGeneration = key.Generation
	return ctrl.Result{}, r.Status().Update(ctx, key)
}

func (r *TunnelKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunneldv1.TunnelKey{}).
		Complete(r)
}
