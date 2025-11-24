# Air-Gap Integration Plan for Sidero Metal

**Date**: November 24, 2025
**Target**: Talos 1.9+ Air-Gap Features
**Status**: Design Phase

---

## Overview

This document outlines the integration of Talos 1.9+ air-gap capabilities into Sidero Metal, enabling fully offline bare-metal Kubernetes deployments.

---

## Talos Air-Gap Capabilities (1.9+)

### 1. Image Cache (Introduced in Talos 1.9)
**Feature**: Pre-seed container images in Talos installation media

**Benefits**:
- No external registry required for initial deployment
- Eliminates need for registry mirrors during bootstrap
- Images embedded directly in boot assets

**Command**:
```bash
talosctl images cache-create --image-cache-path ./image-cache.oci --images=-
```

**Integration Point**: Image Factory schematics can include image cache

---

### 2. Registry Mirrors
**Feature**: Redirect all image pulls to internal registries

**Configuration** (Talos machine config):
```yaml
machine:
  registries:
    mirrors:
      docker.io:
        endpoints:
          - https://registry.local:5000
      ghcr.io:
        endpoints:
          - https://registry.local:5000/ghcr
```

**Integration Point**: Sidero metadata service can inject registry config

---

### 3. Local Image Factory
**Feature**: Run Image Factory in air-gapped mode with local registry

**Requirements**:
- Local registry with copied imager images and signatures
- Script to copy artifacts: `hack/copy-artifacts.sh`

**Command**:
```bash
# Copy to local registry
crane cp ghcr.io/siderolabs/installer:v1.11.5 registry.local/installer:v1.11.5

# Run Image Factory
image-factory --image-registry registry.local
```

**Integration Point**: Sidero can host local Image Factory instance

---

## Sidero Air-Gap Architecture

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    Air-Gapped Network                            │
│                                                                   │
│  ┌──────────────────┐         ┌──────────────────┐              │
│  │ Local Container  │         │ Local Image      │              │
│  │ Registry         │◄────────│ Factory          │              │
│  │ (Docker/Harbor)  │         │ (Optional)       │              │
│  └────────┬─────────┘         └─────────┬────────┘              │
│           │                              │                       │
│           │  ┌───────────────────────────▼─────────────────┐    │
│           │  │  Sidero Controller Manager                  │    │
│           │  │  ┌────────────────────────────────────────┐ │    │
│           │  │  │ Environment Controller                 │ │    │
│           │  │  │ - Downloads from local sources         │ │    │
│           │  │  │ - Caches assets locally                │ │    │
│           │  │  │ - Generates image cache if configured  │ │    │
│           │  │  └────────────────────────────────────────┘ │    │
│           │  │  ┌────────────────────────────────────────┐ │    │
│           │  │  │ Metadata Server                        │ │    │
│           │  │  │ - Injects registry mirror config       │ │    │
│           │  │  │ - Provides image cache location        │ │    │
│           │  │  └────────────────────────────────────────┘ │    │
│           │  └────────────────────────────────────────────┘     │
│           │                              │                       │
│           └──────────────────────────────┼───────────────────┐  │
│                                          ↓                    ↓  │
│                    ┌─────────────────────────────────────────┐  │
│                    │     Bare Metal Servers                  │  │
│                    │  - PXE boot from local assets           │  │
│                    │  - Pull images from local registry      │  │
│                    │  - Use embedded image cache             │  │
│                    └─────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Environment CRD Enhancements

### New Fields for Air-Gap Support

```go
// AirGapConfig defines air-gap/offline deployment configuration.
type AirGapConfig struct {
    // Enabled indicates if this environment is for air-gapped deployments.
    Enabled bool `json:"enabled,omitempty"`

    // RegistryMirrors defines container registry mirrors for air-gapped environments.
    // These are injected into Talos machine configuration via metadata service.
    // +optional
    RegistryMirrors map[string]RegistryMirror `json:"registryMirrors,omitempty"`

    // ImageCacheURL is the URL to a pre-built OCI image cache.
    // If specified, boot assets will include this image cache.
    // +optional
    ImageCacheURL string `json:"imageCacheURL,omitempty"`

    // LocalImageFactory configures a local Image Factory instance.
    // +optional
    LocalImageFactory *LocalImageFactory `json:"localImageFactory,omitempty"`

    // AssetMirror is a local mirror for Talos release assets.
    // If not specified, assets are downloaded from GitHub releases.
    // Example: https://artifacts.local/talos
    // +optional
    AssetMirror string `json:"assetMirror,omitempty"`
}

// RegistryMirror defines a container registry mirror configuration.
type RegistryMirror struct {
    // Endpoints is a list of registry mirror URLs.
    // +required
    Endpoints []string `json:"endpoints"`

    // SkipVerify skips TLS certificate verification.
    // +optional
    SkipVerify bool `json:"skipVerify,omitempty"`

    // OverridePath replaces the image path.
    // +optional
    OverridePath bool `json:"overridePath,omitempty"`
}

// LocalImageFactory defines local Image Factory configuration.
type LocalImageFactory struct {
    // Endpoint is the local Image Factory API endpoint.
    // Example: https://factory.local
    // +required
    Endpoint string `json:"endpoint"`

    // Registry is the local container registry used by Image Factory.
    // Example: registry.local:5000
    // +required
    Registry string `json:"registry"`

    // InsecureSkipVerify skips TLS certificate verification.
    // +optional
    InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}
```

### Updated EnvironmentSpec

```go
type EnvironmentSpec struct {
    // BootAsset is the preferred method for Talos 1.10+ with systemd-boot.
    BootAsset *BootAsset `json:"bootAsset,omitempty"`

    // Kernel configuration for legacy boot.
    Kernel Kernel `json:"kernel,omitempty"`

    // Initrd configuration for legacy boot.
    Initrd Initrd `json:"initrd,omitempty"`

    // AirGap configuration for offline/disconnected deployments.
    // NEW FIELD
    // +optional
    AirGap *AirGapConfig `json:"airGap,omitempty"`
}
```

---

## Use Cases

### Use Case 1: Basic Air-Gap with Registry Mirrors

**Scenario**: Air-gapped datacenter with local container registry

**Environment Configuration**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: airgap-basic
spec:
  bootAsset:
    url: https://artifacts.local/talos/v1.11.5/metal-amd64.raw.xz
  airGap:
    enabled: true
    assetMirror: https://artifacts.local/talos
    registryMirrors:
      docker.io:
        endpoints:
          - https://registry.local:5000/docker.io
      ghcr.io:
        endpoints:
          - https://registry.local:5000/ghcr.io
      gcr.io:
        endpoints:
          - https://registry.local:5000/gcr.io
      registry.k8s.io:
        endpoints:
          - https://registry.local:5000/registry.k8s.io
```

**Flow**:
1. Sidero downloads boot assets from local artifact mirror
2. Servers PXE boot from locally cached assets
3. Metadata service injects registry mirror configuration
4. Kubernetes pulls images from local registry

---

### Use Case 2: Air-Gap with Image Cache

**Scenario**: Completely isolated network, no registry available

**Environment Configuration**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: airgap-isolated
spec:
  bootAsset:
    url: https://artifacts.local/talos/v1.11.5/metal-amd64-with-cache.raw.xz
  airGap:
    enabled: true
    assetMirror: https://artifacts.local/talos
    imageCacheURL: https://artifacts.local/talos/v1.11.5/image-cache.oci
```

**Flow**:
1. Boot asset includes embedded image cache
2. All container images pre-loaded in boot media
3. No external registry needed
4. Fully offline deployment

---

### Use Case 3: Local Image Factory

**Scenario**: Multiple server classes require different extensions

**Environment Configuration**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
metadata:
  name: airgap-factory
spec:
  airGap:
    enabled: true
    localImageFactory:
      endpoint: https://factory.local
      registry: registry.local:5000
    registryMirrors:
      docker.io:
        endpoints:
          - https://registry.local:5000/docker.io
```

**ServerClass Configuration**:
```yaml
apiVersion: metal.sidero.dev/v1alpha2
kind: ServerClass
metadata:
  name: gpu-servers
spec:
  bootAsset:
    schematicID: abc123  # Generated by local Image Factory
    extensions:
      - siderolabs/nvidia-gpu
      - siderolabs/iscsi-tools
```

**Flow**:
1. Sidero calls local Image Factory API
2. Factory generates custom schematics with extensions
3. Boot assets built using local registry
4. GPU servers get appropriate drivers baked in

---

## Implementation Phases

### Phase 1: Basic Air-Gap Support (1-2 weeks)
- [ ] Add AirGapConfig to Environment CRD
- [ ] Add RegistryMirror type
- [ ] Update Environment controller to use AssetMirror for downloads
- [ ] Update metadata server to inject registry mirrors
- [ ] Documentation for basic air-gap setup

### Phase 2: Image Cache Integration (1 week)
- [ ] Add ImageCacheURL field
- [ ] Download and cache image cache files
- [ ] Inject image cache location in metadata service
- [ ] Test with embedded image cache boot assets

### Phase 3: Local Image Factory (2-3 weeks)
- [ ] Add LocalImageFactory type
- [ ] Implement Image Factory API client
- [ ] Generate schematics with local Factory
- [ ] Handle schematic caching
- [ ] ServerClass integration for custom boot assets

### Phase 4: Advanced Features (1-2 weeks)
- [ ] Automatic image cache generation from cluster requirements
- [ ] Multi-registry support
- [ ] Registry credential management
- [ ] Asset verification and checksums

---

## Asset Mirror Strategy

### Directory Structure

```
artifacts.local/talos/
├── v1.11.5/
│   ├── metal-amd64.raw.xz
│   ├── vmlinuz-amd64
│   ├── initramfs-amd64.xz
│   ├── image-cache.oci              # Pre-built image cache
│   ├── metal-amd64-with-cache.raw.xz # Boot asset + cache
│   └── checksums.txt
├── v1.11.4/
│   └── ...
└── extensions/
    ├── nvidia-gpu-v1.0.0.tar
    ├── iscsi-tools-v1.0.0.tar
    └── ...
```

### Mirroring Script

```bash
#!/bin/bash
# mirror-talos-assets.sh - Mirror Talos assets for air-gap

TALOS_VERSION="v1.11.5"
MIRROR_DIR="/var/www/artifacts/talos"
GITHUB_BASE="https://github.com/siderolabs/talos/releases/download"

mkdir -p "${MIRROR_DIR}/${TALOS_VERSION}"

# Download boot assets
for asset in metal-amd64.raw.xz vmlinuz-amd64 initramfs-amd64.xz; do
    wget "${GITHUB_BASE}/${TALOS_VERSION}/${asset}" \
         -O "${MIRROR_DIR}/${TALOS_VERSION}/${asset}"
done

# Generate image cache
talosctl images cache-create \
    --image-cache-path "${MIRROR_DIR}/${TALOS_VERSION}/image-cache.oci" \
    --images=-

# Generate checksums
cd "${MIRROR_DIR}/${TALOS_VERSION}"
sha512sum * > checksums.txt
```

---

## Registry Mirror Strategy

### Registry Setup

**Option 1: Docker Registry**
```yaml
# docker-compose.yml
version: '3'
services:
  registry:
    image: registry:2
    ports:
      - "5000:5000"
    volumes:
      - ./registry-data:/var/lib/registry
    environment:
      REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY: /var/lib/registry
```

**Option 2: Harbor (Production)**
```bash
# Harbor provides:
# - Web UI
# - Image signing
# - Vulnerability scanning
# - Replication
# - RBAC

helm install harbor harbor/harbor \
  --set expose.type=loadBalancer \
  --set persistence.enabled=true
```

### Registry Mirroring Script

```bash
#!/bin/bash
# mirror-container-images.sh - Mirror required images

MIRROR_REGISTRY="registry.local:5000"

# Kubernetes images
for image in $(kubeadm config images list); do
    crane cp $image ${MIRROR_REGISTRY}/${image}
done

# Talos images
for image in $(talosctl images default); do
    crane cp $image ${MIRROR_REGISTRY}/${image}
done

# CNI images (Flannel example)
crane cp docker.io/flannel/flannel:v0.25.1 \
    ${MIRROR_REGISTRY}/docker.io/flannel/flannel:v0.25.1

# CoreDNS
crane cp registry.k8s.io/coredns/coredns:v1.11.1 \
    ${MIRROR_REGISTRY}/registry.k8s.io/coredns/coredns:v1.11.1
```

---

## Security Considerations

### 1. TLS Certificates
**Problem**: Air-gapped environments often use self-signed certificates

**Solution**:
```go
type RegistryMirror struct {
    Endpoints []string `json:"endpoints"`

    // CA certificate for TLS verification
    // +optional
    CACert string `json:"caCert,omitempty"`

    // Skip TLS verification (NOT recommended for production)
    // +optional
    SkipVerify bool `json:"skipVerify,omitempty"`
}
```

### 2. Registry Authentication
**Problem**: Private registries require credentials

**Solution**: Use Kubernetes secrets
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: registry-credentials
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64-encoded-config>
---
apiVersion: metal.sidero.dev/v1alpha2
kind: Environment
spec:
  airGap:
    registryMirrors:
      docker.io:
        endpoints:
          - https://registry.local:5000
        authSecretRef:
          name: registry-credentials
          namespace: sidero-system
```

### 3. Asset Integrity
**Problem**: Ensure downloaded assets haven't been tampered with

**Solution**: SHA512 verification
```go
type Asset struct {
    URL    string `json:"url,omitempty"`
    SHA512 string `json:"sha512,omitempty"`  // Existing field
}

// Verify during download
func verifyAsset(file string, expectedSHA512 string) error {
    h := sha512.New()
    f, _ := os.Open(file)
    io.Copy(h, f)
    actual := hex.EncodeToString(h.Sum(nil))

    if actual != expectedSHA512 {
        return fmt.Errorf("checksum mismatch")
    }
    return nil
}
```

---

## Testing Strategy

### Test Scenarios

1. **Basic Air-Gap Test**
   - Deploy Sidero in isolated network
   - Configure asset mirror
   - Provision cluster
   - Verify no external network calls

2. **Registry Mirror Test**
   - Deploy with registry mirrors
   - Provision cluster
   - Verify all images pulled from local registry
   - Check Talos machine config for mirror configuration

3. **Image Cache Test**
   - Build boot asset with image cache
   - Deploy completely offline
   - Verify cluster deploys without registry

4. **Local Image Factory Test**
   - Deploy local Image Factory
   - Generate custom schematics
   - Provision servers with extensions
   - Verify offline schematic generation

---

## Migration Path

### For Existing Deployments

**Step 1**: Add asset mirror (non-breaking)
```yaml
spec:
  airGap:
    assetMirror: https://artifacts.local/talos
```

**Step 2**: Add registry mirrors (non-breaking)
```yaml
spec:
  airGap:
    registryMirrors:
      docker.io:
        endpoints:
          - https://registry.local:5000
```

**Step 3**: Switch to image cache (optional)
```yaml
spec:
  bootAsset:
    url: https://artifacts.local/talos/v1.11.5/metal-with-cache.raw.xz
  airGap:
    imageCacheURL: https://artifacts.local/talos/v1.11.5/image-cache.oci
```

---

## Documentation Requirements

### User Documentation
- [ ] Air-Gap Deployment Guide
- [ ] Asset Mirroring Setup
- [ ] Registry Configuration Guide
- [ ] Image Cache Creation Tutorial
- [ ] Local Image Factory Setup
- [ ] Troubleshooting Guide

### Operator Documentation
- [ ] Air-Gap Architecture Diagram
- [ ] Configuration Reference
- [ ] Security Best Practices
- [ ] Performance Tuning

---

## Performance Considerations

### Asset Caching
- Local filesystem cache: `/var/lib/sidero/assets`
- Shared NFS mount for multiple controllers
- CDN for distributed deployments

### Registry Performance
- Use Harbor with caching layer
- Configure registry garbage collection
- Monitor storage usage

### Network Optimization
- Compress assets during transfer
- Use HTTP/2 for parallel downloads
- Enable registry proxy cache

---

## Future Enhancements

### Auto-Discovery
- Automatically detect air-gap environment
- Suggest asset mirror configuration
- Validate registry accessibility

### Smart Mirroring
- Only mirror required images
- Incremental updates
- Bandwidth throttling

### Multi-Site Support
- Replicate assets across sites
- Geo-distributed registries
- Fail-over configuration

---

## References

- [Talos Air-Gapped Environments](https://www.talos.dev/v1.10/advanced/air-gapped/)
- [Talos Image Factory](https://www.talos.dev/v1.10/learn-more/image-factory/)
- [Talos Image Cache Blog Post](https://www.siderolabs.com/blog/air-gapped-kubernetes-with-talos-linux/)
- [Image Factory GitHub](https://github.com/siderolabs/image-factory)

---

**Status**: Design Complete, Ready for Implementation
**Next Step**: Implement Phase 1 (Basic Air-Gap Support)
