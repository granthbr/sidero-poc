// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"fmt"
	"sort"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentDefault is an automatically created Environment.
const EnvironmentDefault = "default"

type Asset struct {
	URL    string `json:"url,omitempty"`
	SHA512 string `json:"sha512,omitempty"`
}

type Kernel struct {
	Asset `json:",inline"`

	// Args are kernel arguments. DEPRECATED in Talos 1.10+ with systemd-boot.
	// Use BootAsset with embedded args instead.
	// +optional
	Args []string `json:"args,omitempty"`
}

type Initrd struct {
	Asset `json:",inline"`
}

// BootAsset represents a Talos Image Factory boot asset with embedded configuration.
// This is the preferred method for Talos 1.10+ which uses systemd-boot and UKIs.
type BootAsset struct {
	// URL is the boot asset URL from Image Factory or custom UKI.
	// Example: https://factory.talos.dev/image/<schematic-id>/<version>/metal-amd64.raw.xz
	URL string `json:"url,omitempty"`

	// SHA512 checksum of the boot asset.
	// +optional
	SHA512 string `json:"sha512,omitempty"`

	// SchematicID is the Image Factory schematic ID (if using Image Factory).
	// +optional
	SchematicID string `json:"schematicID,omitempty"`

	// KernelArgs are kernel arguments embedded in the boot asset.
	// These are informational only - the actual args are baked into the UKI.
	// +optional
	KernelArgs []string `json:"kernelArgs,omitempty"`

	// Extensions is a list of system extensions baked into this boot asset.
	// These are informational only - extensions are baked into the image.
	// +optional
	Extensions []string `json:"extensions,omitempty"`
}

// RegistryMirror defines a container registry mirror configuration for air-gapped environments.
// Introduced in Talos 1.9+ for offline deployments.
type RegistryMirror struct {
	// Endpoints is a list of registry mirror URLs.
	// Example: ["https://registry.local:5000"]
	// +required
	Endpoints []string `json:"endpoints"`

	// SkipVerify skips TLS certificate verification.
	// Use with caution - only for development/testing.
	// +optional
	SkipVerify bool `json:"skipVerify,omitempty"`

	// OverridePath replaces the image path when pulling from mirror.
	// +optional
	OverridePath bool `json:"overridePath,omitempty"`
}

// LocalImageFactory defines configuration for a local/on-premise Image Factory instance.
// Used in air-gapped environments where factory.talos.dev is not accessible.
type LocalImageFactory struct {
	// Endpoint is the local Image Factory API endpoint.
	// Example: https://factory.local
	// +required
	Endpoint string `json:"endpoint"`

	// Registry is the local container registry used by Image Factory.
	// Example: registry.local:5000
	// +required
	Registry string `json:"registry"`

	// InsecureSkipVerify skips TLS certificate verification for Factory API.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// AirGapConfig defines configuration for air-gapped/offline deployments.
// Introduced to support Talos 1.9+ air-gap features.
type AirGapConfig struct {
	// Enabled indicates if this environment is configured for air-gapped operation.
	// When true, all assets are fetched from local sources.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AssetMirror is a local HTTP(S) server hosting Talos release assets.
	// Replaces downloads from github.com/siderolabs/talos/releases.
	// Example: https://artifacts.local/talos
	// +optional
	AssetMirror string `json:"assetMirror,omitempty"`

	// RegistryMirrors defines container registry mirror configuration.
	// Injected into Talos machine config via metadata service.
	// Map key is the upstream registry (e.g., "docker.io", "ghcr.io").
	// +optional
	RegistryMirrors map[string]RegistryMirror `json:"registryMirrors,omitempty"`

	// ImageCacheURL points to a pre-built OCI image cache file.
	// Talos 1.9+ supports embedding container images in boot assets.
	// Example: https://artifacts.local/talos/v1.11.5/image-cache.oci
	// +optional
	ImageCacheURL string `json:"imageCacheURL,omitempty"`

	// LocalImageFactory configures a local Image Factory instance.
	// Used for generating custom boot assets with extensions in air-gap.
	// +optional
	LocalImageFactory *LocalImageFactory `json:"localImageFactory,omitempty"`
}

// EnvironmentSpec defines the desired state of Environment.
type EnvironmentSpec struct {
	// BootAsset is the preferred method for Talos 1.10+ with systemd-boot.
	// When specified, Kernel and Initrd fields are ignored.
	// +optional
	BootAsset *BootAsset `json:"bootAsset,omitempty"`

	// Kernel configuration for legacy boot (Talos < 1.10 or non-UEFI systems).
	// Deprecated in favor of BootAsset for Talos 1.10+.
	// +optional
	Kernel Kernel `json:"kernel,omitempty"`

	// Initrd configuration for legacy boot (Talos < 1.10 or non-UEFI systems).
	// Deprecated in favor of BootAsset for Talos 1.10+.
	// +optional
	Initrd Initrd `json:"initrd,omitempty"`

	// AirGap configuration for offline/disconnected deployments.
	// Supports Talos 1.9+ air-gap features (registry mirrors, image cache, local Image Factory).
	// +optional
	AirGap *AirGapConfig `json:"airGap,omitempty"`
}

type AssetCondition struct {
	Asset  `json:",inline"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// EnvironmentStatus defines the observed state of Environment.
type EnvironmentStatus struct {
	Conditions []AssetCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="BootAsset",type="string",priority=0,JSONPath=".spec.bootAsset.url",description="the boot asset URL (Talos 1.10+)"
// +kubebuilder:printcolumn:name="Kernel",type="string",priority=1,JSONPath=".spec.kernel.url",description="the kernel for the environment (legacy)"
// +kubebuilder:printcolumn:name="Initrd",type="string",priority=1,JSONPath=".spec.initrd.url",description="the initrd for the environment (legacy)"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="indicates the readiness of the environment"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of this resource"
// +kubebuilder:storageversion

// Environment is the Schema for the environments API.
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environment.
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

// EnvironmentDefaultSpec returns EnvironmentDefault's spec.
// For Talos 1.10+, this uses the raw metal image which includes systemd-boot/UKI support.
// For earlier versions, falls back to separate kernel/initrd.
func EnvironmentDefaultSpec(talosRelease, apiEndpoint string, apiPort uint16) *EnvironmentSpec {
	// Get default kernel args (kernel.DefaultArgs is a function in Talos 1.11+)
	defaultArgs := kernel.DefaultArgs(nil)
	args := make([]string, 0, len(defaultArgs)+6)
	args = append(args, defaultArgs...)
	// Note: console=ttyS0 removed in Talos 1.8+ by default for bare metal
	args = append(args, "console=tty0", "console=ttyS0", "earlyprintk=ttyS0")
	args = append(args, "initrd=initramfs.xz", "talos.platform=metal")
	sort.Strings(args)

	// For Talos 1.10+, use the metal raw image which supports systemd-boot/UKI.
	// This image can boot via iPXE and includes all necessary components.
	// The kernel args are informational only - in production, use Image Factory
	// to bake custom args and extensions into a UKI.
	return &EnvironmentSpec{
		BootAsset: &BootAsset{
			// Using metal-amd64.raw.xz which supports both BIOS and UEFI boot
			// For production: Use Image Factory API to generate custom schematics
			// Example: https://factory.talos.dev/image/<schematic-id>/v1.11.5/metal-amd64.raw.xz
			URL:        fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/metal-amd64.raw.xz", talosRelease),
			KernelArgs: args, // Informational - actual args must be in schematic
		},
		// Fallback for legacy systems or explicit kernel/initrd preference
		Kernel: Kernel{
			Asset: Asset{
				URL: fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/vmlinuz-amd64", talosRelease),
			},
			Args: args, // These work for BIOS boot, ignored for UEFI systemd-boot
		},
		Initrd: Initrd{
			Asset: Asset{
				URL: fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/initramfs-amd64.xz", talosRelease),
			},
		},
	}
}

// IsReady returns aggregated Environment readiness.
// Checks both BootAsset (preferred for Talos 1.10+) and legacy Kernel/Initrd.
func (env *Environment) IsReady() bool {
	assetURLs := map[string]struct{}{}

	// Check BootAsset (preferred for Talos 1.10+)
	if env.Spec.BootAsset != nil && env.Spec.BootAsset.URL != "" {
		assetURLs[env.Spec.BootAsset.URL] = struct{}{}
	}

	// Check legacy Kernel/Initrd (fallback or explicit preference)
	if env.Spec.Kernel.URL != "" {
		assetURLs[env.Spec.Kernel.URL] = struct{}{}
	}

	if env.Spec.Initrd.URL != "" {
		assetURLs[env.Spec.Initrd.URL] = struct{}{}
	}

	// Mark assets as ready based on conditions
	for _, cond := range env.Status.Conditions {
		if cond.Status == "True" && cond.Type == "Ready" {
			delete(assetURLs, cond.URL)
		}
	}

	return len(assetURLs) == 0
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
