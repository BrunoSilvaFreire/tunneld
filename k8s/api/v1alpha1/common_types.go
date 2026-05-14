package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretKeyRef points at a single key within a Kubernetes Secret in the same namespace.
type SecretKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// LocalObjectRef references another resource by name in the same namespace.
type LocalObjectRef struct {
	Name string `json:"name"`
}

// HealthCheckSpec mirrors api/v1/spec.proto:HealthCheckSpec.
type HealthCheckSpec struct {
	// +kubebuilder:default=tcp
	// +kubebuilder:validation:Enum=tcp
	Type string `json:"type,omitempty"`

	// Address overrides the auto-derived health check target.
	// Defaults to the first local forward.
	Address        string          `json:"address,omitempty"`
	Interval       *metav1.Duration `json:"interval,omitempty"`
	Timeout        *metav1.Duration `json:"timeout,omitempty"`
	StartupTimeout *metav1.Duration `json:"startupTimeout,omitempty"`
}

// BackoffSpec mirrors api/v1/spec.proto:BackoffSpec.
type BackoffSpec struct {
	// Multiplier in milli-units (e.g. 2000 = 2.0x). Kept as int to keep CRD
	// schemas portable; the agent divides by 1000 when calling the daemon.
	// +kubebuilder:default=2000
	MultiplierMilli int32            `json:"multiplierMilli,omitempty"`
	MaxDelay        *metav1.Duration `json:"maxDelay,omitempty"`
}

// RestartPolicySpec mirrors api/v1/spec.proto:RestartPolicySpec.
type RestartPolicySpec struct {
	// +kubebuilder:default=on-failure
	// +kubebuilder:validation:Enum=always;on-failure;never
	Policy      string           `json:"policy,omitempty"`
	Delay       *metav1.Duration `json:"delay,omitempty"`
	MaxAttempts int32            `json:"maxAttempts,omitempty"`
	Backoff     *BackoffSpec     `json:"backoff,omitempty"`
}

// SSHForward mirrors api/v1/spec.proto:SSHForward.
type SSHForward struct {
	// +kubebuilder:default="0.0.0.0"
	ListenAddress string `json:"listenAddress,omitempty"`
	// 0 = dynamic port allocation. Resolved port surfaces in status.
	ListenPort int32  `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int32  `json:"targetPort"`
}

// SSHSpec mirrors api/v1/spec.proto:SSHSpec.
type SSHSpec struct {
	User string `json:"user"`
	Host string `json:"host"`
	// +kubebuilder:default=22
	Port int32 `json:"port,omitempty"`

	// IdentityKeyRef references a TunnelKey in the same namespace. Preferred.
	IdentityKeyRef *LocalObjectRef `json:"identityKeyRef,omitempty"`
	// IdentityKeySecretRef references a raw Secret. Use when you don't want a TunnelKey.
	IdentityKeySecretRef *SecretKeyRef `json:"identityKeySecretRef,omitempty"`

	LocalForwards []SSHForward      `json:"localForwards"`
	Options       map[string]string `json:"options,omitempty"`
}

// KubectlForward mirrors api/v1/spec.proto:KubectlForward.
type KubectlForward struct {
	// +kubebuilder:default="0.0.0.0"
	LocalAddress string `json:"localAddress,omitempty"`
	LocalPort    int32  `json:"localPort"`
	RemotePort   int32  `json:"remotePort"`
}

// KubectlSpec mirrors api/v1/spec.proto:KubectlSpec.
type KubectlSpec struct {
	KubeconfigRef        *LocalObjectRef  `json:"kubeconfigRef,omitempty"`
	KubeconfigSecretRef  *SecretKeyRef    `json:"kubeconfigSecretRef,omitempty"`
	Context              string           `json:"context,omitempty"`
	Namespace            string           `json:"namespace,omitempty"`
	Resource             string           `json:"resource"`
	Forwards             []KubectlForward `json:"forwards"`
	APIServer            string           `json:"apiServer,omitempty"`
	InsecureSkipTLSVerify bool            `json:"insecureSkipTLSVerify,omitempty"`
}

// ExposeSpec controls how the tunnel is published to the cluster.
type ExposeSpec struct {
	// Service: when true, the controller creates a ClusterIP Service + Endpoints
	// pointing at the scheduled node's IP and the resolved actualPort.
	Service bool `json:"service,omitempty"`
	// ServiceName overrides metadata.name as the Service name.
	ServiceName string `json:"serviceName,omitempty"`
	// ForwardIndex selects which entry in localForwards/forwards to publish.
	// Defaults to 0.
	ForwardIndex int32 `json:"forwardIndex,omitempty"`
}

// ResolvedForward mirrors api/v1/spec.proto:ResolvedForward.
type ResolvedForward struct {
	ListenAddress  string `json:"listenAddress,omitempty"`
	ConfiguredPort int32  `json:"configuredPort"`
	ActualPort     int32  `json:"actualPort"`
	RemotePort     int32  `json:"remotePort"`
}
