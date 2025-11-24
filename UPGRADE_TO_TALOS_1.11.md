# Upgrade Guide: Sidero Metal to Talos 1.11.5

**Date**: November 24, 2025
**Target Talos Version**: v1.11.5 (from v1.8.0)
**Target Kubernetes**: v1.32.0 (from v1.30.0)

---

## Overview

This document tracks all changes required to modernize Sidero Metal from Talos 1.8.0 to Talos 1.11.5. This upgrade spans 3-4 major Talos versions with significant breaking changes.

---

## Phase 1: Foundation Updates ✅ COMPLETED

### 1.1 Go Version Update
**Files Modified**:
- `go.mod:3`
- `sfyra/go.mod:3`

**Changes**:
```diff
- go 1.22.7
+ go 1.23
```

**Reason**: Talos 1.11.5 is built with Go 1.24.9, but Go 1.23 provides compatibility while being more stable.

---

### 1.2 Core Dependencies Update
**Files Modified**: `go.mod`

**Changes**:
```diff
# Talos Machinery - CRITICAL UPDATE
- github.com/siderolabs/talos/pkg/machinery v1.8.0
+ github.com/siderolabs/talos/pkg/machinery v1.11.5

# Kubernetes Client Libraries
- k8s.io/api v0.31.0
+ k8s.io/api v0.32.0

- k8s.io/apiextensions-apiserver v0.31.0
+ k8s.io/apiextensions-apiserver v0.32.0

- k8s.io/apimachinery v0.31.0
+ k8s.io/apimachinery v0.32.0

- k8s.io/client-go v0.31.0
+ k8s.io/client-go v0.32.0

- k8s.io/component-base v0.31.0
+ k8s.io/component-base v0.32.0

# Cluster API
- sigs.k8s.io/cluster-api v1.8.3
+ sigs.k8s.io/cluster-api v1.10.0

# Controller Runtime
- sigs.k8s.io/controller-runtime v0.19.0
+ sigs.k8s.io/controller-runtime v0.20.0

# Indirect dependencies
- k8s.io/apiserver v0.31.0
+ k8s.io/apiserver v0.32.0
```

**Impact**:
- ✅ All Kubernetes APIs updated to v0.32
- ✅ Cluster API updated to v1.10 (adds support for Kubernetes 1.32)
- ✅ Controller Runtime updated for compatibility

---

### 1.3 Sfyra Test Framework Updates
**Files Modified**: `sfyra/go.mod`

**Changes**:
```diff
- go 1.22.7
+ go 1.23

# Full Talos package (used for testing)
- github.com/siderolabs/talos v1.8.0
+ github.com/siderolabs/talos v1.11.5

- github.com/siderolabs/talos/pkg/machinery v1.8.0
+ github.com/siderolabs/talos/pkg/machinery v1.11.5

# Kubernetes libraries (match main module)
- k8s.io/api v0.31.1
+ k8s.io/api v0.32.0

- k8s.io/apiextensions-apiserver v0.31.1
+ k8s.io/apiextensions-apiserver v0.32.0

- k8s.io/apimachinery v0.31.1
+ k8s.io/apimachinery v0.32.0

- k8s.io/client-go v0.31.1
+ k8s.io/client-go v0.32.0

- sigs.k8s.io/cluster-api v1.8.3
+ sigs.k8s.io/cluster-api v1.10.0

- sigs.k8s.io/controller-runtime v0.19.0
+ sigs.k8s.io/controller-runtime v0.20.0
```

---

### 1.4 Makefile Build Configuration
**Files Modified**: `Makefile:12-16`

**Changes**:
```diff
- TALOS_RELEASE ?= v1.8.0
+ TALOS_RELEASE ?= v1.11.5

- DEFAULT_K8S_VERSION ?= v1.30.0
+ DEFAULT_K8S_VERSION ?= v1.32.0

- TOOLS ?= ghcr.io/siderolabs/tools:v1.8.0-1-ga0c06c6
+ TOOLS ?= ghcr.io/siderolabs/tools:v1.11.5

- PKGS ?= v1.8.0-8-gdf1a1a5
+ PKGS ?= v1.11.5
```

**Impact**:
- ✅ All builds will use Talos 1.11.5 binaries
- ✅ Default Kubernetes version bumped to 1.32.0
- ✅ Build toolchain updated to match Talos version

---

## Phase 2: Breaking Changes - Boot Asset Migration 🚧 IN PROGRESS

### 2.1 Problem: systemd-boot & UKI (Unified Kernel Images)

**Talos 1.10+ Breaking Change**: All UEFI systems now use systemd-boot bootloader with UKIs. This makes `.machine.install.extraKernelArgs` **IGNORED**.

**Current Implementation** (`app/sidero-controller-manager/api/v1alpha2/environment_types.go:78-98`):
```go
// ❌ BROKEN in Talos 1.10+
func EnvironmentDefaultSpec(talosRelease, apiEndpoint string, apiPort uint16) *EnvironmentSpec {
    args := make([]string, 0, len(kernel.DefaultArgs)+6)
    args = append(args, kernel.DefaultArgs...)
    args = append(args, "console=tty0", "console=ttyS0", "earlyprintk=ttyS0")
    args = append(args, "initrd=initramfs.xz", "talos.platform=metal")
    sort.Strings(args)

    return &EnvironmentSpec{
        Kernel: Kernel{
            Asset: Asset{
                URL: fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/vmlinuz-amd64", talosRelease),
            },
            Args: args, // ❌ These args won't be applied!
        },
        Initrd: Initrd{
            Asset: Asset{
                URL: fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/initramfs-amd64.xz", talosRelease),
            },
        },
    }
}
```

**Required Solution**: Migrate to Talos Image Factory Boot Assets API.

---

### 2.2 Solution: Boot Assets Integration

**Files to Modify**:
- [ ] `app/sidero-controller-manager/api/v1alpha2/environment_types.go` - Add Boot Asset fields to CRD
- [ ] `app/sidero-controller-manager/controllers/environment_controller.go` - Integrate Image Factory client
- [ ] `app/sidero-controller-manager/internal/ipxe/ipxe_server.go` - Serve Boot Asset URLs
- [ ] `app/sidero-controller-manager/internal/metadata/metadata_server.go` - Provide Boot Asset metadata

**New Architecture**:
```
┌─────────────────────────────────────────────┐
│  Environment CRD (Updated)                  │
│  ┌────────────────────────────────────────┐ │
│  │ Spec:                                  │ │
│  │   - BootAssetURL (new)                │ │
│  │   - KernelArgs (embedded in asset)    │ │
│  │   - Extensions (baked into asset)     │ │
│  │   - Fallback: Kernel/Initrd URLs      │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│  EnvironmentReconciler                      │
│  ┌────────────────────────────────────────┐ │
│  │ 1. Generate Boot Asset request        │ │
│  │ 2. Call Talos Image Factory API       │ │
│  │ 3. Receive signed asset URL           │ │
│  │ 4. Cache asset locally                │ │
│  │ 5. Update Environment status          │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│  iPXE Server                                │
│  Serves: Boot Asset URL (not raw vmlinuz)  │
└─────────────────────────────────────────────┘
```

---

## Phase 3: Extension System Migration 🚧 PENDING

### 3.1 Problem: Extension Delivery Deprecated

**Talos 1.10+ Breaking Change**: `.machine.install.extensions` field is deprecated and non-functional.

**Current Implementation**: Extensions installed via machine config field.

**Required Solution**: Extensions must be pre-baked into Boot Assets using Image Factory.

**Files to Modify**:
- [ ] `app/sidero-controller-manager/api/v1alpha2/server_types.go` - Remove/deprecate extension fields
- [ ] `app/sidero-controller-manager/controllers/server_controller.go` - Update extension handling
- [ ] Update ServerClass to specify boot asset profiles (e.g., "gpu", "standard")

---

## Phase 4: API Compatibility Updates 🚧 PENDING

### 4.1 Files Requiring Updates

**Import Changes**:
- [ ] All files importing `talos/pkg/machinery/kernel` - API may have changed
- [ ] All files using `talos/pkg/machinery/config` - Validate field names

**Potential Breaking APIs**:
1. **Disk Configuration** - `.machine.disks` deprecated (backward compatible)
2. **Volume Configuration** - New VolumeConfig, UserVolumeConfig APIs
3. **Boot Configuration** - Boot partition size increased to 2 GiB
4. **Kubernetes Validation** - Image tags now required

---

## Phase 5: Testing Updates 🚧 PENDING

### 5.1 Sfyra Integration Tests

**Files to Update**:
- [ ] `sfyra/pkg/tests/environment.go` - Update for Boot Assets
- [ ] `sfyra/pkg/tests/server.go` - Update for new APIs
- [ ] `sfyra/pkg/tests/server_class.go` - Update boot asset profiles
- [ ] `sfyra/pkg/capi/cluster.go` - Kubernetes 1.32 compatibility
- [ ] `sfyra/pkg/bootstrap/cluster.go` - Update bootstrap flow

---

## Phase 6: Documentation Updates 🚧 PENDING

### 6.1 Files to Update

- [ ] `README.md` - Update compatibility matrix
- [ ] `website/` - Update documentation for Talos 1.11
- [ ] `.github/workflows/` - Update CI/CD for new versions

---

## Breaking Changes Summary

### From Talos 1.8 → 1.9
- ⚠️ DRM drivers moved to extensions
- ⚠️ udev → systemd-udevd (network interface naming may change)
- ⚠️ Registry mirror fallback behavior changed

### From Talos 1.9 → 1.10
- 🔴 **CRITICAL**: systemd-boot + UKIs (kernel args ignored)
- 🔴 **CRITICAL**: Extensions via `.machine.install.extensions` broken
- ⚠️ cgroups v1 removed
- ⚠️ Stageˣ build system (extension compatibility)

### From Talos 1.10 → 1.11
- ⚠️ IMA support dropped
- ⚠️ Kubernetes version validation enforced
- ⚠️ Boot partition size increased to 2 GiB

---

## Testing Checklist

- [ ] Unit tests pass
- [ ] Integration tests (sfyra) pass
- [ ] Boot Asset generation works
- [ ] PXE boot with UKI works
- [ ] Kernel arguments properly embedded
- [ ] Extensions properly baked into images
- [ ] Server classification works
- [ ] Cluster provisioning works
- [ ] Upgrade path from 1.8 → 1.11 tested

---

## Rollback Plan

If issues arise:
1. Revert `go.mod` and `sfyra/go.mod` to Talos 1.8.0
2. Revert `Makefile` TALOS_RELEASE to v1.8.0
3. Run `go mod tidy` to restore dependencies
4. Rebuild all binaries

---

## Migration Timeline

**Estimated Effort**: 12-17 weeks (3-4 months)

- Week 1-2: ✅ Foundation updates (COMPLETED)
- Week 3-6: 🚧 Boot Asset migration (IN PROGRESS)
- Week 7-9: Extension system redesign
- Week 10-11: API compatibility fixes
- Week 12-14: Integration testing
- Week 15-17: Documentation and release

---

## References

- [Talos 1.9 Release Notes](https://docs.siderolabs.com/talos/v1.9/getting-started/what's-new-in-talos)
- [Talos 1.10 Release Notes](https://www.talos.dev/v1.10/introduction/what-is-new/)
- [Talos 1.11 Release Notes](https://docs.siderolabs.com/talos/v1.11/getting-started/what's-new-in-talos)
- [Talos Image Factory](https://factory.talos.dev/)

---

**Last Updated**: November 24, 2025
