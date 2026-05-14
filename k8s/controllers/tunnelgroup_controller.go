package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tunneldv1 "github.com/BrunoSilvaFreire/tunneld/k8s/api/v1alpha1"
)

// TunnelGroupReconciler materializes each TunnelGroup member as a child Tunnel
// resource owned by the TunnelGroup. Co-scheduling is achieved by stamping the
// group's nodeSelector onto every child Tunnel: once the first one schedules,
// the rest match the same node by label.
type TunnelGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tunneld.io,resources=tunnelgroups/status,verbs=get;update;patch

func (r *TunnelGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var g tunneldv1.TunnelGroup
	if err := r.Get(ctx, req.NamespacedName, &g); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Materialize members.
	memberStatus := make([]tunneldv1.TunnelGroupMemberStatus, 0, len(g.Spec.Tunnels))
	for _, m := range g.Spec.Tunnels {
		tun := &tunneldv1.Tunnel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      childTunnelName(g.Name, m.Name),
				Namespace: g.Namespace,
			},
		}
		expose := exposeForMember(&g, m.Name)
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, tun, func() error {
			tun.Labels = mergeLabels(tun.Labels, map[string]string{
				"app.kubernetes.io/managed-by": "tunneld-controller",
				"tunneld.io/group":             g.Name,
				"tunneld.io/member":            m.Name,
			})
			tun.Spec = tunneldv1.TunnelSpec{
				DependsOn:       prefixDeps(g.Name, m.DependsOn),
				SSH:             m.SSH,
				Kubectl:         m.Kubectl,
				Health:          m.Health,
				Restart:         m.Restart,
				StartupTimeout:  m.StartupTimeout,
				ShutdownTimeout: m.ShutdownTimeout,
				NodeSelector:    g.Spec.NodeSelector,
				Expose:          expose,
			}
			return controllerutil.SetControllerReference(&g, tun, r.Scheme)
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		// Read back the child's current phase for aggregate status.
		var live tunneldv1.Tunnel
		_ = r.Get(ctx, client.ObjectKeyFromObject(tun), &live)
		memberStatus = append(memberStatus, tunneldv1.TunnelGroupMemberStatus{
			Name:  m.Name,
			Phase: live.Status.Phase,
		})
	}

	g.Status.Members = memberStatus
	g.Status.Phase = aggregatePhase(memberStatus)
	g.Status.ObservedGeneration = g.Generation
	return ctrl.Result{}, r.Status().Update(ctx, &g)
}

func childTunnelName(group, member string) string {
	return group + "-" + member
}

func prefixDeps(group string, deps []string) []string {
	if len(deps) == 0 {
		return nil
	}
	out := make([]string, len(deps))
	for i, d := range deps {
		out[i] = childTunnelName(group, d)
	}
	return out
}

func exposeForMember(g *tunneldv1.TunnelGroup, memberName string) *tunneldv1.ExposeSpec {
	for _, e := range g.Spec.Expose {
		if e.Tunnel == memberName {
			name := e.ServiceName
			if name == "" {
				name = memberName
			}
			return &tunneldv1.ExposeSpec{
				Service:      true,
				ServiceName:  name,
				ForwardIndex: e.ForwardIndex,
			}
		}
	}
	return nil
}

// aggregatePhase returns Failed if any member failed, Degraded if any is degraded,
// Pending if any is pending, otherwise Healthy.
func aggregatePhase(members []tunneldv1.TunnelGroupMemberStatus) tunneldv1.TunnelPhase {
	if len(members) == 0 {
		return tunneldv1.TunnelPhasePending
	}
	hasPending, hasDegraded, hasFailed := false, false, false
	for _, m := range members {
		switch m.Phase {
		case tunneldv1.TunnelPhaseFailed:
			hasFailed = true
		case tunneldv1.TunnelPhaseDegraded:
			hasDegraded = true
		case tunneldv1.TunnelPhasePending, "":
			hasPending = true
		}
	}
	switch {
	case hasFailed:
		return tunneldv1.TunnelPhaseFailed
	case hasDegraded:
		return tunneldv1.TunnelPhaseDegraded
	case hasPending:
		return tunneldv1.TunnelPhasePending
	default:
		return tunneldv1.TunnelPhaseHealthy
	}
}

func (r *TunnelGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunneldv1.TunnelGroup{}).
		Owns(&tunneldv1.Tunnel{}).
		Complete(r)
}
