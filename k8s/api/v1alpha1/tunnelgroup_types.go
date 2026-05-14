package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TunnelGroupMember declares an individual tunnel inside a TunnelGroup.
// It is a flat embedding of TunnelSpec plus a Name so the DAG stays inline.
type TunnelGroupMember struct {
	Name      string   `json:"name"`
	DependsOn []string `json:"dependsOn,omitempty"`

	SSH     *SSHSpec     `json:"ssh,omitempty"`
	Kubectl *KubectlSpec `json:"kubectl,omitempty"`

	Health  *HealthCheckSpec   `json:"health,omitempty"`
	Restart *RestartPolicySpec `json:"restart,omitempty"`

	StartupTimeout  *metav1.Duration `json:"startupTimeout,omitempty"`
	ShutdownTimeout *metav1.Duration `json:"shutdownTimeout,omitempty"`
}

// TunnelGroupExpose publishes one member of the group as a cluster Service.
type TunnelGroupExpose struct {
	Tunnel       string `json:"tunnel"`
	ServiceName  string `json:"serviceName,omitempty"`
	ForwardIndex int32  `json:"forwardIndex,omitempty"`
}

// TunnelGroupSpec is the desired state of a TunnelGroup.
// All members are co-scheduled on a single node so the local-loopback DAG works.
type TunnelGroupSpec struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tunnels      []TunnelGroupMember `json:"tunnels"`
	Expose       []TunnelGroupExpose `json:"expose,omitempty"`
}

// TunnelGroupMemberStatus mirrors phase per member.
type TunnelGroupMemberStatus struct {
	Name  string      `json:"name"`
	Phase TunnelPhase `json:"phase,omitempty"`
}

// TunnelGroupStatus is the observed state of a TunnelGroup.
type TunnelGroupStatus struct {
	Phase              TunnelPhase               `json:"phase,omitempty"`
	Node               string                    `json:"node,omitempty"`
	Members            []TunnelGroupMemberStatus `json:"members,omitempty"`
	ObservedGeneration int64                     `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition        `json:"conditions,omitempty"`
	LastError          string                    `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=tgrp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.node`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TunnelGroup is a co-scheduled DAG of related tunnels (mirrors tunnels.yaml).
type TunnelGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TunnelGroupSpec   `json:"spec,omitempty"`
	Status            TunnelGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TunnelGroupList is the list wrapper for TunnelGroup.
type TunnelGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TunnelGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TunnelGroup{}, &TunnelGroupList{})
}
