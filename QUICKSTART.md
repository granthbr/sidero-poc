# Quick Start: Adopting Existing Talos Nodes with Sidero Metal

This guide will help you quickly adopt an existing Talos node into Sidero Metal for monitoring and management.

## Prerequisites

1. **Sidero Metal** running in a Kubernetes cluster
2. **Talos Management API** running and accessible
3. **Existing Talos node** you want to adopt
4. `kubectl` configured for your Sidero cluster

## Method 1: Adopt via Management API (Recommended)

This method uses the Talos Management API's REST endpoint to adopt a node.

### Step 1: Ensure the cluster exists in Management API

```bash
# List existing clusters
curl http://localhost:8090/api/v1/clusters

# Create cluster if needed
curl -X POST http://localhost:8090/api/v1/clusters \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-spokane",
    "location": "spokane",
    "endpoint_ip": "192.168.1.10",
    "endpoint_port": 6443
  }'
```

### Step 2: Adopt the node

```bash
curl -X POST http://localhost:8090/api/v1/sidero/adopt-server \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_name": "production-spokane",
    "node_name": "node-1",
    "talos_endpoint": "192.168.1.10:50000",
    "node_type": "controlplane",
    "talos_version": "v1.8.3",
    "kubernetes_version": "v1.31.1",
    "hostname": "node-1",
    "siderolink_enabled": true
  }'
```

### Step 3: Verify adoption

```bash
# Check via Management API
curl http://localhost:8090/api/v1/sidero/server/node-1

# Check via kubectl
kubectl get adoptedservers node-1
```

## Method 2: Adopt via Kubernetes CRD

This method creates an AdoptedServer resource directly in Kubernetes.

### Step 1: Create AdoptedServer YAML

```yaml
# adopted-server.yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: AdoptedServer
metadata:
  name: node-1
  namespace: default
spec:
  talos:
    endpoint: "192.168.1.10:50000"
    nodeType: controlplane
    talosVersion: "v1.8.3"
    kubernetesVersion: "v1.31.1"
    hostname: node-1
  managementAPI:
    enabled: true
    endpoint: "http://management-api:8090"
    clusterName: "production-spokane"
  sideroLink:
    enabled: true
  accepted: true
  labels:
    location: spokane
```

### Step 2: Apply to cluster

```bash
kubectl apply -f adopted-server.yaml
```

### Step 3: Monitor status

```bash
# Watch for status updates
kubectl get adoptedservers -w

# Check detailed status
kubectl describe adoptedserver node-1
```

## Adopting Multiple Nodes

### Bash Script Example

```bash
#!/bin/bash

CLUSTER_NAME="production-spokane"
MANAGEMENT_API="http://localhost:8090"

# Array of nodes to adopt
declare -a NODES=(
  "192.168.1.10|node-1|controlplane"
  "192.168.1.20|node-2|worker"
  "192.168.1.21|node-3|worker"
)

for node_info in "${NODES[@]}"; do
  IFS='|' read -r ip name type <<< "$node_info"

  echo "Adopting $name ($ip)..."

  curl -X POST ${MANAGEMENT_API}/api/v1/sidero/adopt-server \
    -H "Content-Type: application/json" \
    -d "{
      \"cluster_name\": \"${CLUSTER_NAME}\",
      \"node_name\": \"${name}\",
      \"talos_endpoint\": \"${ip}:50000\",
      \"node_type\": \"${type}\",
      \"siderolink_enabled\": true
    }"

  echo ""
done

echo "Adoption complete! Check status:"
echo "kubectl get adoptedservers"
```

### Python Script Example

```python
#!/usr/bin/env python3
import asyncio
from core.sidero_client import SideroClient, adopt_existing_node

async def adopt_cluster_nodes():
    """Adopt multiple nodes into Sidero"""

    # Initialize Sidero client
    client = SideroClient()

    nodes = [
        {
            "name": "node-1",
            "endpoint": "192.168.1.10:50000",
            "type": "controlplane"
        },
        {
            "name": "node-2",
            "endpoint": "192.168.1.20:50000",
            "type": "worker"
        },
        {
            "name": "node-3",
            "endpoint": "192.168.1.21:50000",
            "type": "worker"
        }
    ]

    for node in nodes:
        print(f"Adopting {node['name']}...")

        result = await adopt_existing_node(
            sidero_client=client,
            cluster_name="production-spokane",
            node_name=node["name"],
            talos_endpoint=node["endpoint"],
            node_type=node["type"],
            management_api_endpoint="http://management-api:8090",
            siderolink_enabled=True,
        )

        print(f"✓ {node['name']} adopted successfully")

    print("\nAll nodes adopted!")

if __name__ == "__main__":
    asyncio.run(adopt_cluster_nodes())
```

## Monitoring Adopted Nodes

### View All Adopted Servers

```bash
# Via kubectl
kubectl get adoptedservers
kubectl get adoptedservers -o wide

# Via Management API
curl http://localhost:8090/api/v1/sidero/list-servers
```

### Check Node Health

```bash
# View detailed status
kubectl get adoptedserver node-1 -o yaml

# Check conditions
kubectl get adoptedserver node-1 -o jsonpath='{.status.conditions}' | jq

# Watch for changes
kubectl get adoptedservers -w
```

### View Controller Logs

```bash
# Sidero controller logs
kubectl logs -n sidero-system deployment/sidero-controller-manager -f

# Filter for AdoptedServer reconciliation
kubectl logs -n sidero-system deployment/sidero-controller-manager | grep AdoptedServer
```

## Unadopting a Node

To stop managing a node (without affecting the actual Talos node):

### Via Management API

```bash
curl -X DELETE http://localhost:8090/api/v1/sidero/server/node-1
```

### Via kubectl

```bash
kubectl delete adoptedserver node-1
```

## Troubleshooting

### Node Shows as "Not Connected"

1. **Verify endpoint reachability**:
   ```bash
   # Test Talos API connectivity
   talosctl -n 192.168.1.10 version
   ```

2. **Check firewall rules**: Ensure port 50000 is accessible

3. **Verify endpoint format**: Should be `IP:PORT` (e.g., `192.168.1.10:50000`)

### Management API Sync Failing

1. **Check Management API is accessible from Sidero cluster**:
   ```bash
   kubectl exec -it deployment/sidero-controller-manager -n sidero-system -- \
     curl http://management-api:8090/health
   ```

2. **Verify cluster exists in Management API**:
   ```bash
   curl http://localhost:8090/api/v1/clusters
   ```

3. **Check logs for detailed errors**:
   ```bash
   kubectl logs -n sidero-system deployment/sidero-controller-manager | grep "Management API"
   ```

### AdoptedServer Stuck in "Not Ready"

1. **Check acceptance status**:
   ```bash
   kubectl get adoptedserver node-1 -o jsonpath='{.spec.accepted}'
   ```

   If false, set to true:
   ```bash
   kubectl patch adoptedserver node-1 --type=merge -p '{"spec":{"accepted":true}}'
   ```

2. **View conditions for detailed status**:
   ```bash
   kubectl describe adoptedserver node-1
   ```

## Configuration Reference

### Minimal AdoptedServer

Bare minimum configuration:

```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: AdoptedServer
metadata:
  name: my-node
spec:
  talos:
    endpoint: "192.168.1.10:50000"
    nodeType: worker
  accepted: true
```

### Full AdoptedServer with All Options

Complete configuration with all available options:

```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: AdoptedServer
metadata:
  name: my-node
  namespace: default
  labels:
    location: spokane
    environment: production
    role: controlplane
spec:
  talos:
    endpoint: "192.168.1.10:50000"
    nodeType: controlplane
    talosVersion: "v1.8.3"
    kubernetesVersion: "v1.31.1"
    hostname: my-node
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
  annotations:
    description: "Production control plane node"
```

## Next Steps

1. **Monitor your adopted nodes** via kubectl or the Management API dashboard
2. **Set up SideroLink** for advanced monitoring and log streaming
3. **Integrate with your CI/CD** pipelines for automated node adoption
4. **Explore bulk operations** for adopting entire clusters at once

For more detailed information, see [ADOPTION_FEATURE_SUMMARY.md](./ADOPTION_FEATURE_SUMMARY.md).
