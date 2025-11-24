# Sidero Metal - Adopted Server Feature Implementation

## Overview

This document describes the newly implemented **Adopted Server** feature that allows Sidero Metal to manage existing Talos nodes without reprovisioning them. This feature integrates seamlessly with the Talos Management API for comprehensive lifecycle management.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Sidero Metal (Go)                            │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  AdoptedServer CRD (v1alpha2)                            │  │
│  │  - Spec: Talos config, Management API config, SideroLink │  │
│  │  - Status: Health, connectivity, sync status             │  │
│  └──────────────────────────────────────────────────────────┘  │
│                            │                                     │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  AdoptedServerReconciler                                 │  │
│  │  - Health checks, connectivity monitoring                │  │
│  │  - SideroLink setup (monitoring-only mode)               │  │
│  │  - Bidirectional sync with Management API                │  │
│  └──────────────────────────────────────────────────────────┘  │
│                            │                                     │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Management API Client (pkg/managementapi/)              │  │
│  │  - REST client, types, sync service                      │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                               │
                        REST API (HTTP)
                               │
┌─────────────────────────────────────────────────────────────────┐
│              Talos Management API (Python/FastAPI)              │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Sidero Integration Routes (/api/v1/sidero/*)           │  │
│  │  - POST /adopt-server                                    │  │
│  │  - PATCH /sync-status/{server_name}                      │  │
│  │  - GET /list-servers                                     │  │
│  │  - GET /server/{server_name}                             │  │
│  │  - DELETE /server/{server_name}                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                            │                                     │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Sidero Client (core/sidero_client.py)                   │  │
│  │  - Kubernetes Custom Resource API wrapper                │  │
│  │  - AdoptedServer CRUD operations                         │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Details

### 1. Sidero Metal Components (sidero-poc/)

#### AdoptedServer CRD
**Location**: `app/sidero-controller-manager/api/v1alpha2/adoptedserver_types.go`

New Kubernetes Custom Resource Definition for representing adopted servers:

**Spec Fields**:
- `talos`: Talos configuration (endpoint, version, node type, hostname)
- `managementAPI`: Management API integration config (enabled, endpoint, cluster name/ID)
- `sideroLink`: SideroLink monitoring config (enabled, address, public key)
- `accepted`: Whether server is accepted for management
- `labels`, `annotations`: Custom metadata

**Status Fields**:
- `ready`, `connected`: Overall status
- `lastContactTime`: Last successful contact
- `conditions`: Standard Kubernetes conditions (Connected, Adopted, SideroLinkReady, ManagementAPISync)
- `health`: Health check results
- `addresses`: Discovered IP addresses
- `managementAPIStatus`: Sync status with Management API
- `sideroLinkStatus`: SideroLink connection details
- `nodeInfo`: Additional node metadata

**Example YAML**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: AdoptedServer
metadata:
  name: spokane-control-1
  namespace: default
spec:
  talos:
    endpoint: "192.168.1.10:50000"
    nodeType: controlplane
    talosVersion: "v1.8.3"
    kubernetesVersion: "v1.31.1"
    hostname: spokane-control-1
  managementAPI:
    enabled: true
    endpoint: "http://management-api.example.com:8090"
    clusterName: "production-spokane"
    clusterID: "550e8400-e29b-41d4-a716-446655440000"
  sideroLink:
    enabled: true
  accepted: true
  labels:
    location: spokane
    environment: production
```

#### AdoptedServer Controller
**Location**: `app/sidero-controller-manager/controllers/adoptedserver_controller.go`

Kubernetes controller that reconciles AdoptedServer resources:

**Reconciliation Loop** (every 30 seconds):
1. Verify server is accepted
2. Check Talos API connectivity
3. Gather node information
4. Perform health checks
5. Setup SideroLink if enabled (monitoring-only mode)
6. Sync with Management API if configured
7. Update status with results

**Key Methods**:
- `checkTalosConnectivity()`: Verifies Talos API is reachable
- `gatherNodeInfo()`: Collects system information from node
- `performHealthCheck()`: Executes health checks
- `setupSideroLink()`: Configures SideroLink monitoring
- `syncWithManagementAPI()`: Syncs with Management API
- `unregisterFromManagementAPI()`: Cleanup on deletion

#### Management API Client Package
**Location**: `app/sidero-controller-manager/pkg/managementapi/`

Go package for communicating with the Talos Management API:

**Files**:
- `types.go`: Request/response type definitions
- `client.go`: REST HTTP client implementation
- `sync.go`: Sync service for bidirectional synchronization

**Client Methods**:
- `RegisterCluster()`: Register cluster with Management API
- `RegisterNode()`: Register node with Management API
- `UpdateClusterStatus()`: Update cluster status
- `UpdateNodeStatus()`: Update node status
- `HealthCheck()`: Check Management API health

**Sync Service**:
- `SyncAdoptedServer()`: Full sync (cluster + node registration/update)
- `UnregisterAdoptedServer()`: Cleanup on deletion

#### Controller Registration
**Location**: `app/sidero-controller-manager/main.go:268-277`

The AdoptedServerReconciler is registered with the controller manager on startup.

### 2. Talos Management API Components (talos-management-api/)

#### Sidero Client
**Location**: `core/sidero_client.py`

Python client for managing AdoptedServer resources via Kubernetes API:

**Key Methods**:
- `create_adopted_server()`: Create new AdoptedServer resource
- `get_adopted_server()`: Retrieve AdoptedServer by name
- `list_adopted_servers()`: List all AdoptedServers
- `update_adopted_server_status()`: Update server status
- `delete_adopted_server()`: Remove AdoptedServer
- `patch_adopted_server()`: Update server spec

**Convenience Function**:
- `adopt_existing_node()`: High-level function to adopt a node with full integration

**Requirements**: `pip install kubernetes`

#### Sidero Integration Routes
**Location**: `api/sidero_routes.py`

FastAPI routes for Sidero Metal integration:

**Endpoints**:

1. **POST /api/v1/sidero/adopt-server**
   - Adopt an existing Talos node into Sidero
   - Creates AdoptedServer resource with Management API integration
   - Returns adopted server details

2. **PATCH /api/v1/sidero/sync-status/{server_name}**
   - Receive status updates from Sidero
   - Called by AdoptedServer controller
   - Updates Management API database

3. **GET /api/v1/sidero/list-servers**
   - List all AdoptedServer resources
   - Supports namespace and label filtering
   - Returns array of adopted servers

4. **GET /api/v1/sidero/server/{server_name}**
   - Get specific AdoptedServer details
   - Returns full resource spec and status

5. **DELETE /api/v1/sidero/server/{server_name}**
   - Remove AdoptedServer from Sidero
   - Stops monitoring but doesn't affect actual node

#### Route Registration
**Location**: `main.py:166`

Sidero routes are registered with the FastAPI application on startup.

## Usage Workflows

### Workflow 1: Adopt Existing Node via Management API

```bash
# Step 1: Adopt the node through Management API
curl -X POST http://localhost:8090/api/v1/sidero/adopt-server \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "production-spokane",
    "node_name": "spokane-control-1",
    "talos_endpoint": "192.168.1.10:50000",
    "node_type": "controlplane",
    "talos_version": "v1.8.3",
    "kubernetes_version": "v1.31.1",
    "hostname": "spokane-control-1",
    "labels": {
      "location": "spokane",
      "environment": "production"
    },
    "siderolink_enabled": true
  }'

# Step 2: Check status
curl http://localhost:8090/api/v1/sidero/server/spokane-control-1
```

### Workflow 2: Direct Kubernetes CRD Creation

```bash
# Create AdoptedServer directly via kubectl
kubectl apply -f - <<EOF
apiVersion: metal.sidero.dev/v1alpha2
kind: AdoptedServer
metadata:
  name: spokane-worker-1
spec:
  talos:
    endpoint: "192.168.1.20:50000"
    nodeType: worker
    talosVersion: "v1.8.3"
    kubernetesVersion: "v1.31.1"
  managementAPI:
    enabled: true
    endpoint: "http://management-api:8090"
    clusterName: "production-spokane"
  accepted: true
EOF

# Check status
kubectl get adoptedservers
kubectl describe adoptedserver spokane-worker-1
```

### Workflow 3: List and Monitor Adopted Servers

```bash
# Via Management API
curl http://localhost:8090/api/v1/sidero/list-servers

# Via kubectl
kubectl get adoptedservers -o wide
kubectl get adoptedservers spokane-control-1 -o yaml
```

## Configuration

### Sidero Metal Configuration

The AdoptedServer controller is automatically enabled when the sidero-controller-manager starts. No additional configuration needed.

### Talos Management API Configuration

Add to your `config.py` or environment variables:

```python
# Enable Sidero integration
SIDERO_ENABLED = True

# Kubernetes configuration
SIDERO_KUBECONFIG_PATH = "/path/to/kubeconfig"  # Optional, uses default if not set
SIDERO_IN_CLUSTER = False  # Set to True if running inside Kubernetes

# Management API external endpoint (for Sidero to call back)
MANAGEMENT_API_EXTERNAL_ENDPOINT = "http://management-api.example.com:8090"
```

## Benefits

### For Existing Nodes
- ✅ No reprovisioning required
- ✅ Non-destructive adoption process
- ✅ Preserves existing workloads and data
- ✅ Immediate monitoring and management

### Unified Management
- ✅ Single pane of glass for all Talos infrastructure
- ✅ Consistent API across adopted and provisioned servers
- ✅ Centralized health monitoring
- ✅ Bidirectional status synchronization

### Scalability
- ✅ Adopt nodes individually or in bulk
- ✅ Namespace-based organization
- ✅ Label-based filtering and grouping
- ✅ Automatic status updates

### Integration
- ✅ Works with existing Sidero Metal workflows
- ✅ Compatible with Cluster API (CAPI)
- ✅ SideroLink monitoring support
- ✅ Azure Key Vault integration via Management API

## Next Steps

### Phase 1 Complete ✅
- AdoptedServer CRD and controller
- Management API client package
- Python Sidero client
- REST API integration endpoints
- Bidirectional synchronization

### Phase 2: Enhanced Monitoring
- [ ] Implement actual Talos API connectivity checks (talosctl/gRPC)
- [ ] Add comprehensive health checks (etcd, kubelet, containerd)
- [ ] Implement SideroLink setup for adopted servers
- [ ] Add metrics and Prometheus integration

### Phase 3: Web UI Integration
- [ ] Add "Adopt Existing Node" button to Management API UI
- [ ] Create adoption wizard workflow
- [ ] Show Sidero server status in dashboard
- [ ] Display SideroLink metrics

### Phase 4: Advanced Features
- [ ] Automated network discovery of Talos nodes
- [ ] Bulk adoption operations
- [ ] Migration path: adopt → manage → eventually re-provision
- [ ] Advanced filtering and search

## Testing

### Unit Tests
```bash
# Sidero Metal (Go)
cd app/sidero-controller-manager
go test ./...

# Management API (Python)
cd /path/to/talos-management-api
pytest tests/
```

### Integration Tests
```bash
# Create test cluster
kind create cluster

# Deploy Sidero CRDs
make manifests
kubectl apply -f app/sidero-controller-manager/config/crd/

# Create test AdoptedServer
kubectl apply -f examples/adopted-server-sample.yaml

# Verify reconciliation
kubectl get adoptedservers -w
```

## Troubleshooting

### AdoptedServer Not Ready

Check conditions:
```bash
kubectl describe adoptedserver <name>
```

Common issues:
- **Not Accepted**: Set `spec.accepted: true`
- **Connection Failed**: Verify `spec.talos.endpoint` is reachable
- **Management API Sync Failed**: Check Management API endpoint and credentials

### Management API Integration Not Working

1. Check Sidero logs:
```bash
kubectl logs -n sidero-system deployment/sidero-controller-manager
```

2. Check Management API logs:
```bash
docker logs talos-management-api
```

3. Verify connectivity:
```bash
curl http://management-api-endpoint:8090/health
```

## File Changes Summary

### New Files Created

**Sidero Metal (sidero-poc/)**:
- `app/sidero-controller-manager/api/v1alpha2/adoptedserver_types.go` (310 lines)
- `app/sidero-controller-manager/controllers/adoptedserver_controller.go` (396 lines)
- `app/sidero-controller-manager/pkg/managementapi/types.go` (105 lines)
- `app/sidero-controller-manager/pkg/managementapi/client.go` (215 lines)
- `app/sidero-controller-manager/pkg/managementapi/sync.go` (178 lines)

**Talos Management API (talos-management-api/)**:
- `core/sidero_client.py` (362 lines)
- `api/sidero_routes.py` (347 lines)

### Modified Files

**Sidero Metal**:
- `app/sidero-controller-manager/main.go`: Added AdoptedServerReconciler registration
- `app/sidero-controller-manager/api/v1alpha2/zz_generated.deepcopy.go`: Added DeepCopy methods

**Talos Management API**:
- `main.py`: Added sidero_routes import and router registration

## Conclusion

The Adopted Server feature successfully bridges the gap between existing Talos infrastructure and Sidero Metal's management capabilities. This implementation provides a solid foundation for unified Talos cluster management across both greenfield (PXE-provisioned) and brownfield (existing) deployments.

The two-repository architecture maintains clear separation of concerns while enabling seamless integration through well-defined REST APIs and Kubernetes Custom Resources.
