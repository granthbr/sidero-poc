# Sidero

<!-- textlint-disable -->
> [!CAUTION]
> Sidero Labs is no longer actively developing Sidero Metal.
> For an alternative, please see [Omni](https://github.com/siderolabs/omni.git).
> Unless you have an existing support contract covering Sidero Metal, all support will be provided by the community (including questions in our Slack workspace).
<!-- textlint-enable -->

Kubernetes Bare Metal Lifecycle Management.
Sidero Metal provides lightweight, composable tools that can be used to create bare-metal Talos + Kubernetes clusters.
Sidero Metal is an open-source project from [Sidero Labs](https://www.SideroLabs.com).

## Documentation

Visit the project [site](https://www.sidero.dev).

## Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

|                        | v1alpha3 (v0.3) | v1alpha4 (v0.4) | v1beta1 (v1.x) |
| ---------------------- | --------------- | --------------- | -------------- |
| Sidero Provider (v0.5) |                 |                 | ✓              |
| Sidero Provider (v0.6) |                 |                 | ✓              |

This provider's versions are able to install and manage the following versions of Kubernetes:

|                        | v1.19 | v1.20 | v1.21 | v1.22 | v1.23 | v1.24 | v1.25 | v1.26 | v1.27 | v1.28 | v1.29 | v1.30 | v1.31 |
| ---------------------- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- |
| Sidero Provider (v0.5) | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     |       |       |       |       |
| Sidero Provider (v0.6) |       |       |       |       |       | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     |

This provider's versions are compatible with the following versions of Talos:

|                        | v0.12  | v0.13 | v0.14 | v1.0  | v1.1  | v1.2  | v1.3  | v1.4  | v1.5  | v1.6  | v1.7  | v1.8  | v1.9  | v1.10 | v1.11 |
| ---------------------- | ------ | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- | ----- |
| Sidero Provider (v0.5) | ✓ (+)  | ✓ (+) | ✓     | ✓     | ✓     | ✓     | ✓     |       |       |       |       |       |       |       |       |
| Sidero Provider (v0.6) |        |       |       | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     |       |       |       |
| Sidero Provider (v0.7) |        |       |       | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     | ✓     |

**Note**: v0.7 (development) includes modernization for Talos 1.10+ with Boot Asset support for systemd-boot/UKI.

## Getting Started

### Prerequisites

- **Management Cluster**: A Kubernetes cluster (v1.24+) to run Sidero Metal controllers
- **Cluster API**: clusterctl CLI tool installed ([installation guide](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl))
- **Network Access**: Management cluster must have network access to bare-metal servers' IPMI/BMC interfaces
- **DHCP Server**: For PXE booting (Sidero provides iPXE boot, but DHCP is external)
- **Bare Metal Servers**: Servers with IPMI/BMC support and PXE boot capability

### Quick Start

#### 1. Install Sidero Metal on Management Cluster

```bash
# Initialize Cluster API with Sidero provider
clusterctl init --infrastructure sidero

# Wait for Sidero controllers to be ready
kubectl wait --for=condition=Available --timeout=300s \
  deployment -n sidero-system sidero-controller-manager
```

#### 2. Create a Talos Environment

An Environment defines which Talos Linux version to boot on your bare-metal servers.

**For Talos 1.10+ (Recommended - UKI/systemd-boot)**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: talos-1-11
spec:
  # Boot Asset method (Talos 1.10+)
  bootAsset:
    url: https://github.com/siderolabs/talos/releases/download/v1.11.5/metal-amd64.raw.xz
    sha512: <checksum-from-github-release>
    kernelArgs:
      - console=tty0
      - console=ttyS0
      - talos.platform=metal
```

**For Legacy Boot (Talos < 1.10 or BIOS-only systems)**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: talos-1-8-legacy
spec:
  kernel:
    url: https://github.com/siderolabs/talos/releases/download/v1.8.0/vmlinuz-amd64
    sha512: <checksum>
    args:
      - console=tty0
      - console=ttyS0
      - talos.platform=metal
  initrd:
    url: https://github.com/siderolabs/talos/releases/download/v1.8.0/initramfs-amd64.xz
    sha512: <checksum>
```

Apply the environment:
```bash
kubectl apply -f environment.yaml
```

#### 3. Configure Server Discovery

Sidero automatically discovers servers that PXE boot from the management network. Servers will appear as `Server` resources:

```bash
# Watch for discovered servers
kubectl get servers -w

# View server details
kubectl describe server <server-uuid>
```

#### 4. Create a Server Class (Optional)

Group servers with similar characteristics:

```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: ServerClass
metadata:
  name: worker-nodes
spec:
  environmentRef:
    name: talos-1-11
  qualifiers:
    cpu:
      - manufacturer: Intel
    systemInformation:
      - manufacturer: Dell Inc.
```

#### 5. Create a Workload Cluster

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: production-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
        - 10.244.0.0/16
    services:
      cidrBlocks:
        - 10.96.0.0/12
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: MetalCluster
    name: production-cluster
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
    kind: TalosControlPlane
    name: production-cluster-cp
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalCluster
metadata:
  name: production-cluster
spec:
  controlPlaneEndpoint:
    host: 192.168.1.100
    port: 6443
---
apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
kind: TalosControlPlane
metadata:
  name: production-cluster-cp
spec:
  version: v1.32.0
  replicas: 3
  infrastructureTemplate:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: MetalMachineTemplate
    name: production-cluster-cp
  controlPlaneConfig:
    controlplane:
      generateType: controlplane
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalMachineTemplate
metadata:
  name: production-cluster-cp
spec:
  template:
    spec:
      serverClassRef:
        apiVersion: metal.sidero.dev/v1alpha2
        kind: ServerClass
        name: control-plane-nodes
```

### Advanced Features

#### Air-Gap Deployments

For offline/disconnected environments, configure registry mirrors:

```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: talos-airgap
spec:
  bootAsset:
    url: https://artifacts.local/talos/v1.11.5/metal-amd64.raw.xz
  airGap:
    enabled: true
    # Mirror for Talos release assets
    assetMirror: https://artifacts.local/talos
    # Container registry mirrors
    registryMirrors:
      docker.io:
        endpoints:
          - https://registry.local:5000/docker.io
      ghcr.io:
        endpoints:
          - https://registry.local:5000/ghcr.io
      registry.k8s.io:
        endpoints:
          - https://registry.local:5000/registry.k8s.io
    # Optional: Local Image Factory for custom boot images
    localImageFactory:
      endpoint: https://factory.local
      registry: registry.local:5000
```

See [AIRGAP_INTEGRATION.md](AIRGAP_INTEGRATION.md) for detailed air-gap setup instructions.

#### Using Image Factory (Custom Extensions)

For systems requiring custom kernel modules or extensions:

```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: talos-custom
spec:
  bootAsset:
    # Use Image Factory to generate custom boot image
    url: https://factory.talos.dev/image/<schematic-id>/v1.11.5/metal-amd64.raw.xz
    schematicID: <your-schematic-id>
    extensions:
      - siderolabs/intel-ucode
      - siderolabs/i915-ucode
```

Generate schematics at [factory.talos.dev](https://factory.talos.dev).

### Common Operations

#### View Discovered Servers
```bash
kubectl get servers
kubectl describe server <uuid>
```

#### Check Environment Status
```bash
kubectl get environments
kubectl describe environment talos-1-11
```

#### Get Cluster Status
```bash
kubectl get clusters
kubectl get machines
kubectl get metalmachines
```

#### Access Workload Cluster
```bash
# Get kubeconfig
clusterctl get kubeconfig production-cluster > production-kubeconfig.yaml

# Use it
kubectl --kubeconfig=production-kubeconfig.yaml get nodes
```

#### Scale Cluster
```bash
# Scale control plane
kubectl patch taloscontrolplane production-cluster-cp \
  --type=merge -p '{"spec":{"replicas":5}}'

# Scale workers (via MachineDeployment)
kubectl scale machinedeployment production-cluster-workers --replicas=10
```

### Troubleshooting

#### Server Not Booting
```bash
# Check server status
kubectl describe server <uuid>

# Check environment assets are downloaded
kubectl get environment talos-1-11 -o yaml | grep -A 10 status

# View iPXE server logs
kubectl logs -n sidero-system deployment/sidero-controller-manager | grep ipxe
```

#### Server Not Discovered
- Verify DHCP is pointing to Sidero iPXE endpoint
- Check network connectivity from server to management cluster
- Ensure PXE boot is enabled in BIOS/UEFI

#### Cluster Creation Stuck
```bash
# Check machine status
kubectl describe machine <machine-name>

# Check metal machine status
kubectl describe metalmachine <metalmachine-name>

# View metadata server logs
kubectl logs -n sidero-system deployment/sidero-controller-manager | grep metadata
```

### Documentation

- **Full Documentation**: [sidero.dev](https://www.sidero.dev)
- **Talos 1.11 Upgrade Guide**: [UPGRADE_TO_TALOS_1.11.md](UPGRADE_TO_TALOS_1.11.md)
- **Air-Gap Deployments**: [AIRGAP_INTEGRATION.md](AIRGAP_INTEGRATION.md)
- **Modernization Summary**: [MODERNIZATION_SUMMARY.md](MODERNIZATION_SUMMARY.md)

## Support

Join our [Slack](https://slack.dev.talos-systems.io)!
