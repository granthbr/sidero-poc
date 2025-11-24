// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package managementapi

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	metalv1 "github.com/siderolabs/sidero/app/sidero-controller-manager/api/v1alpha2"
)

// SyncService handles synchronization between AdoptedServers and the Management API.
type SyncService struct {
	client *Client
	logger logr.Logger
}

// NewSyncService creates a new sync service.
func NewSyncService(client *Client, logger logr.Logger) *SyncService {
	return &SyncService{
		client: client,
		logger: logger,
	}
}

// SyncAdoptedServer synchronizes an AdoptedServer with the Management API.
// It handles both cluster and node registration/updates.
func (s *SyncService) SyncAdoptedServer(ctx context.Context, as *metalv1.AdoptedServer) error {
	if as.Spec.ManagementAPI == nil || !as.Spec.ManagementAPI.Enabled {
		return fmt.Errorf("Management API integration not enabled for AdoptedServer %s", as.Name)
	}

	// Step 1: Register or update cluster
	clusterID := as.Spec.ManagementAPI.ClusterID
	if clusterID == "" {
		// Cluster not yet registered, register it
		s.logger.Info("Registering cluster with Management API", "cluster", as.Spec.ManagementAPI.ClusterName)
		resp, err := s.registerCluster(ctx, as)
		if err != nil {
			return errors.Wrap(err, "failed to register cluster")
		}
		clusterID = resp.ClusterID
		s.logger.Info("Cluster registered successfully", "cluster_id", clusterID)

		// Update the AdoptedServer spec with the cluster ID (this will be persisted by the controller)
		as.Spec.ManagementAPI.ClusterID = clusterID
	} else {
		// Cluster already registered, update status
		s.logger.Info("Updating cluster status in Management API", "cluster_id", clusterID)
		if err := s.updateClusterStatus(ctx, as, clusterID); err != nil {
			return errors.Wrap(err, "failed to update cluster status")
		}
	}

	// Step 2: Register or update node
	// For adopted servers, each AdoptedServer represents a single node
	if err := s.registerOrUpdateNode(ctx, as, clusterID); err != nil {
		return errors.Wrap(err, "failed to register/update node")
	}

	return nil
}

// registerCluster registers a new cluster with the Management API.
func (s *SyncService) registerCluster(ctx context.Context, as *metalv1.AdoptedServer) (*ClusterRegisterResponse, error) {
	req := &ClusterRegisterRequest{
		Name:              as.Spec.ManagementAPI.ClusterName,
		Location:          getLocationFromLabels(as),
		EndpointIP:        extractIPFromEndpoint(as.Spec.Talos.Endpoint),
		EndpointPort:      extractPortFromEndpoint(as.Spec.Talos.Endpoint),
		TalosVersion:      as.Spec.Talos.TalosVersion,
		KubernetesVersion: as.Spec.Talos.KubernetesVersion,
		Labels:            as.Spec.Labels,
		Managed:           true, // Adopted servers are managed
	}

	return s.client.RegisterCluster(ctx, req)
}

// updateClusterStatus updates the cluster status in the Management API.
func (s *SyncService) updateClusterStatus(ctx context.Context, as *metalv1.AdoptedServer, clusterID string) error {
	status := "unknown"
	health := "unknown"

	if as.Status.Connected {
		status = "active"
	}

	if as.Status.Health != nil {
		health = as.Status.Health.Status
	}

	addresses := make([]string, 0, len(as.Status.Addresses))
	for _, addr := range as.Status.Addresses {
		addresses = append(addresses, addr.Address)
	}

	req := &StatusUpdateRequest{
		Status:    status,
		Health:    health,
		Addresses: addresses,
		Metadata: map[string]string{
			"talos_version":      as.Spec.Talos.TalosVersion,
			"kubernetes_version": as.Spec.Talos.KubernetesVersion,
			"node_type":          as.Spec.Talos.NodeType,
		},
	}

	if as.Status.LastContactTime != nil {
		req.LastContact = as.Status.LastContactTime.Time
	}

	_, err := s.client.UpdateClusterStatus(ctx, clusterID, req)
	return err
}

// registerOrUpdateNode registers or updates a node in the Management API.
func (s *SyncService) registerOrUpdateNode(ctx context.Context, as *metalv1.AdoptedServer, clusterID string) error {
	hostname := as.Spec.Talos.Hostname
	if hostname == "" {
		hostname = as.Name
	}

	ipAddress := extractIPFromEndpoint(as.Spec.Talos.Endpoint)

	// Check if node already exists (we could store node_id in status, but for now we'll try to register)
	// The Management API should handle idempotent registration
	req := &NodeRegisterRequest{
		ClusterID:         clusterID,
		Hostname:          hostname,
		IPAddress:         ipAddress,
		NodeType:          as.Spec.Talos.NodeType,
		TalosVersion:      as.Spec.Talos.TalosVersion,
		KubernetesVersion: as.Spec.Talos.KubernetesVersion,
	}

	_, err := s.client.RegisterNode(ctx, req)
	return err
}

// UnregisterAdoptedServer removes an AdoptedServer from the Management API.
func (s *SyncService) UnregisterAdoptedServer(ctx context.Context, as *metalv1.AdoptedServer) error {
	// TODO: Implement node and cluster deletion in the Management API
	// For now, this is a no-op as the Management API doesn't expose deletion endpoints yet
	s.logger.Info("Unregistering AdoptedServer from Management API", "name", as.Name)
	return nil
}

// Helper functions

// getLocationFromLabels extracts the location from AdoptedServer labels.
func getLocationFromLabels(as *metalv1.AdoptedServer) string {
	if as.Spec.Labels != nil {
		if location, ok := as.Spec.Labels["location"]; ok {
			return location
		}
	}
	return "unknown"
}

// extractIPFromEndpoint extracts the IP address from a Talos endpoint.
// Endpoint format: "IP:PORT" or just "IP"
func extractIPFromEndpoint(endpoint string) string {
	// Simple implementation - split on colon and take first part
	for i, c := range endpoint {
		if c == ':' {
			return endpoint[:i]
		}
	}
	return endpoint
}

// extractPortFromEndpoint extracts the port from a Talos endpoint.
// Returns 50000 as default if no port specified.
func extractPortFromEndpoint(endpoint string) int {
	// Simple implementation - split on colon and parse second part
	for i, c := range endpoint {
		if c == ':' {
			portStr := endpoint[i+1:]
			var port int
			fmt.Sscanf(portStr, "%d", &port)
			if port > 0 && port <= 65535 {
				return port
			}
		}
	}
	return 50000 // Default Talos API port
}

// addressesToStrings converts a slice of corev1.NodeAddress to strings.
func addressesToStrings(addresses []corev1.NodeAddress) []string {
	result := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		result = append(result, addr.Address)
	}
	return result
}
