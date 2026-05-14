package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TunnelKeyType is the kind of credential a TunnelKey carries.
// +kubebuilder:validation:Enum=ssh-private-key;kubeconfig
type TunnelKeyType string

const (
	TunnelKeyTypeSSHPrivateKey TunnelKeyType = "ssh-private-key"
	TunnelKeyTypeKubeconfig    TunnelKeyType = "kubeconfig"
)

// TunnelKeySource locates the actual key bytes.
type TunnelKeySource struct {
	// SecretRef pulls bytes from a Kubernetes Secret in the same namespace.
	SecretRef *SecretKeyRef `json:"secretRef,omitempty"`
}

// TunnelKeySpec is the desired state of a TunnelKey.
type TunnelKeySpec struct {
	Type   TunnelKeyType   `json:"type"`
	Source TunnelKeySource `json:"source"`

	// NodeSelector restricts which nodes are allowed to materialize this key.
	// If unset, the key is materialized on every node hosting a referencing Tunnel.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// TunnelKeyPhase is the high-level state of a TunnelKey.
// +kubebuilder:validation:Enum=Pending;Ready;Failed
type TunnelKeyPhase string

const (
	TunnelKeyPhasePending TunnelKeyPhase = "Pending"
	TunnelKeyPhaseReady   TunnelKeyPhase = "Ready"
	TunnelKeyPhaseFailed  TunnelKeyPhase = "Failed"
)

// TunnelKeyStatus is the observed state of a TunnelKey.
type TunnelKeyStatus struct {
	Phase              TunnelKeyPhase     `json:"phase,omitempty"`
	InstalledNodes     []string           `json:"installedNodes,omitempty"`
	Fingerprint        string             `json:"fingerprint,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastError          string             `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=tkey
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TunnelKey is authentication material (SSH private key or kubeconfig) used by Tunnels.
type TunnelKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TunnelKeySpec   `json:"spec,omitempty"`
	Status            TunnelKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TunnelKeyList is the list wrapper for TunnelKey.
type TunnelKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TunnelKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TunnelKey{}, &TunnelKeyList{})
}
