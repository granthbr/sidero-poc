// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package managementapi

import "time"

// ClusterRegisterRequest represents a request to register a cluster with the Management API.
type ClusterRegisterRequest struct {
	Name              string            `json:"name"`
	Location          string            `json:"location"`
	EndpointIP        string            `json:"endpoint_ip"`
	EndpointPort      int               `json:"endpoint_port"`
	TalosVersion      string            `json:"talos_version,omitempty"`
	KubernetesVersion string            `json:"kubernetes_version,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Managed           bool              `json:"managed"` // true for adopted servers
}

// ClusterRegisterResponse represents the response from registering a cluster.
type ClusterRegisterResponse struct {
	ClusterID string `json:"cluster_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// NodeRegisterRequest represents a request to register a node with the Management API.
type NodeRegisterRequest struct {
	ClusterID          string `json:"cluster_id"`
	Hostname           string `json:"hostname"`
	IPAddress          string `json:"ip_address"`
	NodeType           string `json:"node_type"` // controlplane, worker
	TalosVersion       string `json:"talos_version,omitempty"`
	KubernetesVersion  string `json:"kubernetes_version,omitempty"`
	MachineConfigPath  string `json:"machine_config_path,omitempty"`
}

// NodeRegisterResponse represents the response from registering a node.
type NodeRegisterResponse struct {
	NodeID    string `json:"node_id"`
	ClusterID string `json:"cluster_id"`
	Hostname  string `json:"hostname"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// StatusUpdateRequest represents a request to update cluster or node status.
type StatusUpdateRequest struct {
	Status      string            `json:"status"`
	Health      string            `json:"health,omitempty"`
	Addresses   []string          `json:"addresses,omitempty"`
	LastContact time.Time         `json:"last_contact,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StatusUpdateResponse represents the response from a status update.
type StatusUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ClusterInfoResponse represents cluster information from the Management API.
type ClusterInfoResponse struct {
	ClusterID         string            `json:"cluster_id"`
	Name              string            `json:"name"`
	Location          string            `json:"location"`
	Status            string            `json:"status"`
	EndpointIP        string            `json:"endpoint_ip"`
	EndpointPort      int               `json:"endpoint_port"`
	TalosVersion      string            `json:"talos_version"`
	KubernetesVersion string            `json:"kubernetes_version"`
	Labels            map[string]string `json:"labels"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// NodeInfoResponse represents node information from the Management API.
type NodeInfoResponse struct {
	NodeID            string    `json:"node_id"`
	ClusterID         string    `json:"cluster_id"`
	Hostname          string    `json:"hostname"`
	IPAddress         string    `json:"ip_address"`
	NodeType          string    `json:"node_type"`
	Status            string    `json:"status"`
	TalosVersion      string    `json:"talos_version"`
	KubernetesVersion string    `json:"kubernetes_version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// HealthCheckResponse represents a health check response from the Management API.
type HealthCheckResponse struct {
	Status    string `json:"status"`
	AppName   string `json:"app_name"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ErrorResponse represents an error response from the Management API.
type ErrorResponse struct {
	Detail string `json:"detail"`
	Error  string `json:"error,omitempty"`
	Code   int    `json:"code,omitempty"`
}
