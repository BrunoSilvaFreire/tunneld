package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TunnelSpec mirrors api/v1/spec.proto:TunnelSpec, plus a K8s-only Expose section.
type TunnelSpec struct {
	// DependsOn lists Tunnel names (same namespace) that must reach Healthy
	// before this one starts. Mirrors the daemon's DAG.
	DependsOn []string `json:"dependsOn,omitempty"`

	// Exactly one of SSH or Kubectl must be set.
	SSH     *SSHSpec     `json:"ssh,omitempty"`
	Kubectl *KubectlSpec `json:"kubectl,omitempty"`

	Health  *HealthCheckSpec   `json:"health,omitempty"`
	Restart *RestartPolicySpec `json:"restart,omitempty"`

	StartupTimeout  *metav1.Duration `json:"startupTimeout,omitempty"`
	ShutdownTimeout *metav1.Duration `json:"shutdownTimeout,omitempty"`

	// NodeSelector restricts which nodes the controller will schedule this tunnel onto.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	Expose *ExposeSpec `json:"expose,omitempty"`
}

// TunnelPhase is the high-level lifecycle state of a Tunnel.
// +kubebuilder:validation:Enum=Pending;Healthy;Degraded;Failed;Stopped
type TunnelPhase string

const (
	TunnelPhasePending  TunnelPhase = "Pending"
	TunnelPhaseHealthy  TunnelPhase = "Healthy"
	TunnelPhaseDegraded TunnelPhase = "Degraded"
	TunnelPhaseFailed   TunnelPhase = "Failed"
	TunnelPhaseStopped  TunnelPhase = "Stopped"
)

// TunnelStatus is the observed state of a Tunnel.
type TunnelStatus struct {
	Phase             TunnelPhase        `json:"phase,omitempty"`
	Node              string             `json:"node,omitempty"`
	ResolvedForwards  []ResolvedForward  `json:"resolvedForwards,omitempty"`
	ServiceRef        *LocalObjectRef    `json:"serviceRef,omitempty"`
	ObservedGeneration int64             `json:"observedGeneration,omitempty"`
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
	LastError         string             `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=tun
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.node`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Tunnel is a single SSH or kubectl port-forward tunnel managed by tunneld.
type Tunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TunnelSpec   `json:"spec,omitempty"`
	Status            TunnelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TunnelList is the list wrapper for Tunnel.
type TunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tunnel{}, &TunnelList{})
}
