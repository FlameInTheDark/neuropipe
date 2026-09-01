package runtime

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func assetList(tag string, names ...string) []llamaAsset {
	assets := make([]llamaAsset, 0, len(names))
	for _, name := range names {
		assets = append(assets, llamaAsset{Name: name, URL: "http://example.invalid/" + tag + "/" + name})
	}
	return assets
}

// TestClassifyLlamaReleaseToolkitPreference pins the CUDA toolkit ranking:
// the preferred 12.4 beats 13.3, and both beat unknown future toolkits.
func TestClassifyLlamaReleaseToolkitPreference(t *testing.T) {
	manifest, ok := classifyLlamaRelease("b600", "2026-02-01T00:00:00Z", assetList("b600",
		"llama-b600-bin-win-cpu-x64.zip",
		"llama-b600-bin-win-cuda-12.4-x64.zip",
		"cudart-llama-bin-win-cuda-12.4-x64.zip",
		"llama-b600-bin-win-cuda-13.3-x64.zip",
		"cudart-llama-bin-win-cuda-13.3-x64.zip",
	))
	if !ok {
		t.Fatal("classifyLlamaRelease() ok = false, want a manifest")
	}
	if manifest.Release.CUDA.URL != "http://example.invalid/b600/llama-b600-bin-win-cuda-12.4-x64.zip" {
		t.Fatalf("CUDA = %q, want the 12.4 build", manifest.Release.CUDA.URL)
	}
	if manifest.Cudart.URL != "http://example.invalid/b600/cudart-llama-bin-win-cuda-12.4-x64.zip" {
		t.Fatalf("cudart = %q, want the 12.4 runtime library", manifest.Cudart.URL)
	}

	manifest, ok = classifyLlamaRelease("b600", "", assetList("b600",
		"llama-b600-bin-win-cpu-x64.zip",
		"llama-b600-bin-win-cuda-13.3-x64.zip",
		"cudart-llama-bin-win-cuda-13.3-x64.zip",
		"llama-b600-bin-win-cuda-13.4-x64.zip",
		"cudart-llama-bin-win-cuda-13.4-x64.zip",
	))
	if !ok {
		t.Fatal("classifyLlamaRelease() ok = false, want a manifest")
	}
	if manifest.Release.CUDA.URL != "http://example.invalid/b600/llama-b600-bin-win-cuda-13.3-x64.zip" {
		t.Fatalf("CUDA = %q, want the preferred 13.3 build over the unknown 13.4 one", manifest.Release.CUDA.URL)
	}
}

// TestClassifyLlamaReleaseAcceptsUnknownToolkit keeps future toolkit names
// installable instead of silently dropping the release.
func TestClassifyLlamaReleaseAcceptsUnknownToolkit(t *testing.T) {
	manifest, ok := classifyLlamaRelease("b600", "", assetList("b600",
		"llama-b600-bin-win-cpu-x64.zip",
		"llama-b600-bin-win-cuda-14.0-x64.zip",
		"cudart-llama-bin-win-cuda-14.0-x64.zip",
	))
	if !ok {
		t.Fatal("classifyLlamaRelease() ok = false, want a manifest")
	}
	if manifest.Release.CUDA.URL == "" || manifest.Cudart.URL == "" {
		t.Fatalf("CUDA artifacts = %#v, want the 14.0 pair", manifest)
	}
}

// TestClassifyLlamaReleaseAMDBuilds covers both AMD asset spellings mapping
// onto the HIP slot.
func TestClassifyLlamaReleaseAMDBuilds(t *testing.T) {
	for _, name := range []string{"llama-b600-bin-win-rocm-7.14-x64.zip", "llama-b600-bin-win-hip-radeon-x64.zip"} {
		manifest, ok := classifyLlamaRelease("b600", "", assetList("b600", "llama-b600-bin-win-cpu-x64.zip", name))
		if !ok || manifest.Release.HIP.URL == "" {
			t.Fatalf("HIP not set for asset %q: %#v", name, manifest.Release)
		}
	}
}

// TestClassifyLlamaReleaseIgnoresNonX64AndForeignTags verifies arm64 builds
// and mismatched version prefixes never populate artifacts.
func TestClassifyLlamaReleaseIgnoresNonX64AndForeignTags(t *testing.T) {
	manifest, ok := classifyLlamaRelease("b600", "", assetList("b600",
		"llama-b599-bin-win-cpu-x64.zip", // wrong version prefix
		"llama-b600-bin-win-cuda-13.4-arm64.zip",
		"cudart-llama-bin-win-cuda-13.4-arm64.zip",
		"llama-b600-bin-macos-x64.tar.gz",
	))
	if ok {
		t.Fatalf("classifyLlamaRelease() ok = true, want rejection without a Windows x64 CPU build: %#v", manifest)
	}
	if manifest.Release.CPU.URL != "" || manifest.Release.CUDA.URL != "" || manifest.Release.Vulkan.URL != "" || manifest.Release.HIP.URL != "" {
		t.Fatalf("foreign assets leaked into the manifest: %#v", manifest.Release)
	}
}

// TestClassifyLlamaReleaseRequiresCPUAsset keeps GPU-only releases out of
// the listing, matching the historical contract Install relies on.
func TestClassifyLlamaReleaseRequiresCPUAsset(t *testing.T) {
	if _, ok := classifyLlamaRelease("b600", "", assetList("b600",
		"llama-b600-bin-win-cuda-12.4-x64.zip",
		"cudart-llama-bin-win-cuda-12.4-x64.zip",
	)); ok {
		t.Fatal("classifyLlamaRelease() ok = true, want rejection without a CPU build")
	}
}

func TestCudaToolkitRankOrder(t *testing.T) {
	if !(cudaToolkitRank("12.4") > cudaToolkitRank("13.3") && cudaToolkitRank("13.3") > cudaToolkitRank("13.4")) {
		t.Fatalf("toolkit ranks out of order: 12.4=%d 13.3=%d 13.4=%d", cudaToolkitRank("12.4"), cudaToolkitRank("13.3"), cudaToolkitRank("13.4"))
	}
	if cudaToolkitRank("13.4") <= cudaToolkitRank("12.9") {
		t.Fatalf("unknown toolkits must rank by version: 13.4=%d 12.9=%d", cudaToolkitRank("13.4"), cudaToolkitRank("12.9"))
	}
	if cudaToolkitRank("garbage") != -1 {
		t.Fatalf("unparsable toolkit rank = %d, want -1", cudaToolkitRank("garbage"))
	}
}

func TestPickCudaToolkitRequiresMatchingCudart(t *testing.T) {
	servers := map[string]domain.RuntimeArtifact{"12.4": {URL: "s124"}, "13.3": {URL: "s133"}}
	libraries := map[string]domain.RuntimeArtifact{"13.3": {URL: "l133"}}
	server, library := pickCudaToolkit(servers, libraries)
	if server.URL != "s133" || library.URL != "l133" {
		t.Fatalf("pickCudaToolkit() = %q, %q, want the only complete pair", server.URL, library.URL)
	}
	if _, library := pickCudaToolkit(servers, nil); library.URL != "" {
		t.Fatalf("pickCudaToolkit() without cudart = %q, want empty", library.URL)
	}
}

func TestSortReleaseManifestsAndBuildNumber(t *testing.T) {
	manifests := []releaseManifest{
		{Release: domain.LlamaRuntimeRelease{Version: "b99"}},
		{Release: domain.LlamaRuntimeRelease{Version: "b10721"}},
		{Release: domain.LlamaRuntimeRelease{Version: "b599"}},
	}
	sortReleaseManifests(manifests)
	if manifests[0].Release.Version != "b10721" || manifests[1].Release.Version != "b599" || manifests[2].Release.Version != "b99" {
		t.Fatalf("sortReleaseManifests() = %v, %v, %v, want numeric descending order", manifests[0].Release.Version, manifests[1].Release.Version, manifests[2].Release.Version)
	}
	if llamaBuildNumber("not-a-build") != -1 {
		t.Fatalf("llamaBuildNumber(not-a-build) = %d, want -1", llamaBuildNumber("not-a-build"))
	}
}
