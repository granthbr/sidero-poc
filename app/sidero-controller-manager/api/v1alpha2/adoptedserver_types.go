// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ManagementAPIConfig defines the configuration for Talos Management API integration.
type ManagementAPIConfig struct {
	// Enabled indicates if Management API integration is enabled.
	// +optional
	Enabled bool `json:"enabled"`

	// Endpoint is the base URL of the Talos Management API.
	// Example: http://talos-management-api:8090
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// ClusterName is the cluster name in the Management API.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// ClusterID is the UUID of the cluster in the Management API.
	// +optional
	ClusterID string `json:"clusterID,omitempty"`

	// SecretRef references a secret containing Management API credentials.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

// SideroLinkConfig defines the configuration for SideroLink monitoring.
type SideroLinkConfig struct {
	// Enabled indicates if SideroLink monitoring is enabled.
	// When enabled, SideroLink will be configured in monitoring-only mode.
	// +optional
	Enabled bool `json:"enabled"`

	// Address is the assigned SideroLink IPv6 address for this node.
	// Format: fdae:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx:xxxx/64
	// +optional
	Address string `json:"address,omitempty"`

	// PublicKey is the Wireguard public key for this node.
	// +optional
	PublicKey string `json:"publicKey,omitempty"`
}

// TalosConfig defines Talos-specific configuration for the adopted server.
type TalosConfig struct {
	// Endpoint is the Talos API endpoint (IP:port).
	// Default port is 50000 if not specified.
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// TalosVersion is the version of Talos running on this server.
	// Example: v1.8.3
	// +optional
	TalosVersion string `json:"talosVersion,omitempty"`

	// KubernetesVersion is the version of Kubernetes running on this server.
	// Example: v1.31.1
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// NodeType indicates if this is a control plane or worker node.
	// Valid values: controlplane, worker
	// +kubebuilder:validation:Enum=controlplane;worker
	// +optional
	NodeType string `json:"nodeType,omitempty"`

	// Hostname is the node hostname.
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// AdoptedServerSpec defines the desired state of AdoptedServer.
type AdoptedServerSpec struct {
	// Talos contains Talos-specific configuration.
	// +kubebuilder:validation:Required
	Talos TalosConfig `json:"talos"`

	// ManagementAPI contains configuration for Talos Management API integration.
	// +optional
	ManagementAPI *ManagementAPIConfig `json:"managementAPI,omitempty"`

	// SideroLink contains configuration for SideroLink monitoring.
	// +optional
	SideroLink *SideroLinkConfig `json:"sideroLink,omitempty"`

	// Accepted indicates if the server has been accepted for management.
	// Similar to Server.Spec.Accepted, this controls whether Sidero will manage this node.
	// +optional
	Accepted bool `json:"accepted"`

	// Labels are custom labels to apply to this server.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are custom annotations to apply to this server.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

const (
	// ConditionConnected indicates whether the Talos API is reachable.
	ConditionConnected clusterv1.ConditionType = "Connected"
	// ConditionAdopted indicates whether the server has been successfully adopted.
	ConditionAdopted clusterv1.ConditionType = "Adopted"
	// ConditionSideroLinkReady indicates whether SideroLink is connected and functioning.
	ConditionSideroLinkReady clusterv1.ConditionType = "SideroLinkReady"
	// ConditionManagementAPISync indicates whether the server is synced with Management API.
	ConditionManagementAPISync clusterv1.ConditionType = "ManagementAPISync"
)

// AdoptedServerStatus defines the observed state of AdoptedServer.
type AdoptedServerStatus struct {
	// Ready indicates if the adopted server is ready and being monitored.
	// +optional
	Ready bool `json:"ready"`

	// Connected indicates if Sidero can reach the Talos API endpoint.
	// +optional
	Connected bool `json:"connected"`

	// LastContactTime is the last time Sidero successfully contacted this server.
	// +optional
	LastContactTime *metav1.Time `json:"lastContactTime,omitempty"`

	// Conditions defines current service state of the AdoptedServer.
	Conditions []clusterv1.Condition `json:"conditions,omitempty"`

	// Addresses lists the IP addresses discovered from the node.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// Health contains health check information from the node.
	// +optional
	Health *HealthStatus `json:"health,omitempty"`

	// ManagementAPIStatus contains sync status with the Management API.
	// +optional
	ManagementAPIStatus *ManagementAPIStatus `json:"managementAPIStatus,omitempty"`

	// SideroLinkStatus contains the status of the SideroLink connection.
	// +optional
	SideroLinkStatus *SideroLinkStatus `json:"sideroLinkStatus,omitempty"`

	// NodeInfo contains additional information about the node.
	// +optional
	NodeInfo *NodeInfo `json:"nodeInfo,omitempty"`
}

// HealthStatus contains health check information.
type HealthStatus struct {
	// Status is the overall health status: "healthy", "degraded", "unhealthy", "unknown"
	// +optional
	Status string `json:"status,omitempty"`

	// LastCheckTime is when the health check was last performed.
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// Message contains additional health information or error details.
	// +optional
	Message string `json:"message,omitempty"`
}

// ManagementAPIStatus contains sync status with the Management API.
type ManagementAPIStatus struct {
	// Synced indicates if the server is successfully synced with the Management API.
	// +optional
	Synced bool `json:"synced"`

	// LastSyncTime is when the last sync occurred.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Error contains any error from the last sync attempt.
	// +optional
	Error string `json:"error,omitempty"`
}

// SideroLinkStatus contains the status of the SideroLink connection.
type SideroLinkStatus struct {
	// Connected indicates if SideroLink is connected.
	// +optional
	Connected bool `json:"connected"`

	// LastEventTime is when the last event was received over SideroLink.
	// +optional
	LastEventTime *metav1.Time `json:"lastEventTime,omitempty"`

	// EventsReceived is the count of events received over SideroLink.
	// +optional
	EventsReceived int64 `json:"eventsReceived,omitempty"`

	// LogsReceived is the count of log entries received over SideroLink.
	// +optional
	LogsReceived int64 `json:"logsReceived,omitempty"`
}

// NodeInfo contains additional information about the node.
type NodeInfo struct {
	// Architecture of the node (e.g., amd64, arm64).
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// OperatingSystem is the OS running on the node (should be "talos").
	// +optional
	OperatingSystem string `json:"operatingSystem,omitempty"`

	// MachineID is the unique machine identifier.
	// +optional
	MachineID string `json:"machineID,omitempty"`

	// KernelVersion is the kernel version running on the node.
	// +optional
	KernelVersion string `json:"kernelVersion,omitempty"`

	// ClusterName is the Kubernetes cluster name from the node's perspective.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=as
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.talos.endpoint",description="Talos API endpoint"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.talos.nodeType",description="Node type (controlplane/worker)"
// +kubebuilder:printcolumn:name="Accepted",type="boolean",JSONPath=".spec.accepted",description="Indicates if accepted for management"
// +kubebuilder:printcolumn:name="Connected",type="boolean",JSONPath=".status.connected",description="Indicates if Talos API is reachable"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.health.status",description="Health status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"
// +kubebuilder:storageversion

// AdoptedServer is the Schema for the adoptedservers API.
// AdoptedServer represents an existing Talos node that has been adopted into
// Sidero for monitoring and management without reprovisioning.
type AdoptedServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AdoptedServerSpec   `json:"spec,omitempty"`
	Status AdoptedServerStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions from the status.
func (as *AdoptedServer) GetConditions() clusterv1.Conditions {
	return as.Status.Conditions
}

// SetConditions sets the conditions in the status.
func (as *AdoptedServer) SetConditions(conditions clusterv1.Conditions) {
	as.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// AdoptedServerList contains a list of AdoptedServer.
type AdoptedServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AdoptedServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AdoptedServer{}, &AdoptedServerList{})
}
