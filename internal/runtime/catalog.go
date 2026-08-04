// Package runtime also installs the official llama.cpp runtime and public GGUF models.
package runtime

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

const officialLlamaReleasesURL = "https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=16"

var (
	llamaVersionPattern = regexp.MustCompile(`^b[0-9]+$`)
	cpuAssetPattern     = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-cpu-x64\.zip$`)
	cudaAssetPattern    = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-cuda-(12\.4|13\.3)-x64\.zip$`)
	cudartAssetPattern  = regexp.MustCompile(`^cudart-llama-bin-win-cuda-(12\.4|13\.3)-x64\.zip$`)
	vulkanAssetPattern  = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-vulkan-x64\.zip$`)
	hipAssetPattern     = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-hip-radeon-x64\.zip$`)
	validRepository     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	quantizationPattern = regexp.MustCompile(`(?i)(?:^|[._-])(IQ[1-4]_[A-Z]{2}|Q[2-8](?:_[A-Z0-9]+){0,2}|F16|F32)(?:[._-]|$)`)
	profileAvatarURL    = regexp.MustCompile(`https://cdn-avatars\.huggingface\.co/[^\s"'<>]+`)
)

// LlamaCatalog discovers and installs official Windows x64 llama.cpp releases
// beneath application data. Downloads are resumed, checked, and atomically published.
type LlamaCatalog struct {
	root        string
	http        *http.Client
	releasesURL string
}

// ProgressReporter receives throttled install progress on the caller's goroutine.
// A nil reporter disables progress reporting.
type ProgressReporter func(domain.InstallProgress)

type downloadProgress struct {
	downloadedBytes int64
	totalBytes      int64
	bytesPerSecond  float64
}

// NewLlamaCatalog creates a catalog rooted in a user-owned app-data directory.
func NewLlamaCatalog(root string) *LlamaCatalog {
	return &LlamaCatalog{root: root, http: &http.Client{Timeout: 0}, releasesURL: officialLlamaReleasesURL}
}

type releaseManifest struct {
	release domain.LlamaRuntimeRelease
	cudart  domain.RuntimeArtifact
}

// List fetches compatible official Windows x64 releases.
func (c *LlamaCatalog) List(ctx context.Context) ([]domain.LlamaRuntimeRelease, error) {
	manifests, err := c.releases(ctx)
	if err != nil {
		return nil, err
	}
	releases := make([]domain.LlamaRuntimeRelease, 0, len(manifests))
	for _, manifest := range manifests {
		releases = append(releases, manifest.release)
	}
	return releases, nil
}

// Status reports runtime builds already installed locally.
func (c *LlamaCatalog) Status(selectedVersion string) (domain.LlamaRuntimeCatalogStatus, error) {
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return domain.LlamaRuntimeCatalogStatus{}, fmt.Errorf("create llama.cpp runtime directory: %w", err)
	}
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return domain.LlamaRuntimeCatalogStatus{}, fmt.Errorf("read llama.cpp runtime directory: %w", err)
	}
	installed := make([]domain.InstalledLlamaRuntime, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !llamaVersionPattern.MatchString(entry.Name()) {
			continue
		}
		versionRoot := filepath.Join(c.root, entry.Name())
		item := domain.InstalledLlamaRuntime{
			Version:         entry.Name(),
			CPUInstalled:    runtimeFileExists(filepath.Join(versionRoot, string(domain.RuntimeCPU), "llama-server.exe")),
			CUDAInstalled:   runtimeFileExists(filepath.Join(versionRoot, string(domain.RuntimeCUDA), "llama-server.exe")),
			VulkanInstalled: runtimeFileExists(filepath.Join(versionRoot, string(domain.RuntimeVulkan), "llama-server.exe")),
			HIPInstalled:    runtimeFileExists(filepath.Join(versionRoot, string(domain.RuntimeHIP), "llama-server.exe")),
		}
		if item.CPUInstalled || item.CUDAInstalled || item.VulkanInstalled || item.HIPInstalled {
			installed = append(installed, item)
		}
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Version > installed[j].Version })
	return domain.LlamaRuntimeCatalogStatus{Root: c.root, SelectedVersion: strings.TrimSpace(selectedVersion), Installed: installed}, nil
}

// Install downloads and publishes one selected acceleration build.
func (c *LlamaCatalog) Install(ctx context.Context, request domain.LlamaRuntimeInstallRequest) error {
	return c.InstallWithProgress(ctx, request, nil)
}

// InstallWithProgress downloads and atomically publishes a selected runtime build.
func (c *LlamaCatalog) InstallWithProgress(ctx context.Context, request domain.LlamaRuntimeInstallRequest, report ProgressReporter) error {
	if goruntime.GOOS != "windows" {
		return errors.New("llama.cpp runtime installation is currently available on Windows only")
	}
	if !llamaVersionPattern.MatchString(strings.TrimSpace(request.Version)) {
		return errors.New("choose a valid llama.cpp release")
	}
	if !installableRuntimeMode(request.Mode) {
		return errors.New("choose CPU, CUDA, Vulkan, or HIP")
	}
	manifests, err := c.releases(ctx)
	if err != nil {
		return err
	}
	var selected *releaseManifest
	for index := range manifests {
		if manifests[index].release.Version == request.Version {
			selected = &manifests[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("llama.cpp release %s is no longer available", request.Version)
	}
	artifacts, err := selectedArtifacts(*selected, request.Mode)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return fmt.Errorf("create llama.cpp runtime directory: %w", err)
	}
	archives := make([]string, 0, len(artifacts))
	totalBytes := int64(0)
	for _, item := range artifacts {
		totalBytes += item.artifact.Size
	}
	completedBytes := int64(0)
	lastBytesPerSecond := 0.0
	for _, item := range artifacts {
		currentBytes := int64(0)
		reportInstallProgress(report, "runtime", "downloading", item.name, completedBytes, totalBytes, 0)
		archive, downloadErr := c.downloadWithProgress(ctx, request.Version, item, func(progress downloadProgress) {
			currentBytes = progress.downloadedBytes
			if progress.bytesPerSecond > 0 {
				lastBytesPerSecond = progress.bytesPerSecond
			}
			reportInstallProgress(report, "runtime", "downloading", item.name, completedBytes+progress.downloadedBytes, totalBytes, progress.bytesPerSecond)
		})
		if downloadErr != nil {
			return downloadErr
		}
		archives = append(archives, archive)
		completedBytes += currentBytes
	}
	reportInstallProgress(report, "runtime", "installing", "Installing runtime", completedBytes, totalBytes, lastBytesPerSecond)
	if err := c.publish(request, archives); err != nil {
		return err
	}
	reportInstallProgress(report, "runtime", "installed", "Runtime installed", completedBytes, totalBytes, lastBytesPerSecond)
	return nil
}

// ResolveBinary chooses an installed binary. Auto prefers CUDA only when the
// NVIDIA driver command and the matching installed runtime are both available.
func (c *LlamaCatalog) ResolveBinary(version string, requested domain.RuntimeMode) (domain.RuntimeMode, string, error) {
	if goruntime.GOOS != "windows" {
		return "", "", errors.New("the managed llama.cpp runtime is currently Windows-only")
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return "", "", errors.New("install and select a llama.cpp runtime in Settings")
	}
	mode := requested
	if mode == domain.RuntimeAuto || mode == "" {
		cuda := c.RuntimeBinary(version, domain.RuntimeCUDA)
		if _, err := exec.LookPath("nvidia-smi"); err == nil && runtimeFileExists(cuda) {
			return domain.RuntimeCUDA, cuda, nil
		}
		mode = domain.RuntimeCPU
	}
	if !installableRuntimeMode(mode) {
		return "", "", fmt.Errorf("unsupported llama.cpp runtime mode %q", mode)
	}
	binary := c.RuntimeBinary(version, mode)
	if !runtimeFileExists(binary) {
		return "", "", fmt.Errorf("%s llama.cpp runtime %s is not installed", strings.ToUpper(string(mode)), version)
	}
	return mode, binary, nil
}

// RuntimeBinary returns the expected managed binary location.
func (c *LlamaCatalog) RuntimeBinary(version string, mode domain.RuntimeMode) string {
	return filepath.Join(c.root, version, string(mode), "llama-server.exe")
}

type namedArtifact struct {
	name     string
	artifact domain.RuntimeArtifact
}

func selectedArtifacts(manifest releaseManifest, mode domain.RuntimeMode) ([]namedArtifact, error) {
	switch mode {
	case domain.RuntimeCPU:
		return []namedArtifact{{name: "CPU runtime", artifact: manifest.release.CPU}}, nil
	case domain.RuntimeCUDA:
		if manifest.release.CUDA.URL == "" || manifest.cudart.URL == "" {
			return nil, fmt.Errorf("llama.cpp release %s does not provide a complete CUDA runtime", manifest.release.Version)
		}
		return []namedArtifact{{name: "CUDA runtime", artifact: manifest.release.CUDA}, {name: "CUDA libraries", artifact: manifest.cudart}}, nil
	case domain.RuntimeVulkan:
		if manifest.release.Vulkan.URL == "" {
			return nil, fmt.Errorf("llama.cpp release %s does not provide a Vulkan runtime", manifest.release.Version)
		}
		return []namedArtifact{{name: "Vulkan runtime", artifact: manifest.release.Vulkan}}, nil
	case domain.RuntimeHIP:
		if manifest.release.HIP.URL == "" {
			return nil, fmt.Errorf("llama.cpp release %s does not provide a HIP runtime", manifest.release.Version)
		}
		return []namedArtifact{{name: "HIP runtime", artifact: manifest.release.HIP}}, nil
	default:
		return nil, fmt.Errorf("unsupported llama.cpp runtime mode %q", mode)
	}
}

func (c *LlamaCatalog) downloadWithProgress(ctx context.Context, version string, item namedArtifact, report func(downloadProgress)) (string, error) {
	if item.artifact.URL == "" {
		return "", fmt.Errorf("llama.cpp %s download URL is unavailable", item.name)
	}
	downloads := filepath.Join(c.root, ".downloads")
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		return "", fmt.Errorf("create runtime download directory: %w", err)
	}
	archive := filepath.Join(downloads, version+"-"+safeArchivePart(item.name)+".zip")
	return downloadResumableWithProgress(ctx, c.http, item.artifact.URL, archive, item.artifact.Size, item.artifact.SHA256, "Neuropipe/0.1", report)
}

func (c *LlamaCatalog) publish(request domain.LlamaRuntimeInstallRequest, archives []string) error {
	if len(archives) == 0 {
		return errors.New("no llama.cpp archives to install")
	}
	versionRoot := filepath.Join(c.root, request.Version)
	if err := os.MkdirAll(versionRoot, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(versionRoot, "."+string(request.Mode)+"-install-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	if err := extractServer(archives[0], staging); err != nil {
		return err
	}
	for _, archive := range archives[1:] {
		if err := copyArchiveFiles(archive, staging); err != nil {
			return err
		}
	}
	if !runtimeFileExists(filepath.Join(staging, "llama-server.exe")) {
		return errors.New("llama.cpp archive did not contain llama-server.exe")
	}
	final := filepath.Join(versionRoot, string(request.Mode))
	backup := final + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, backup); err != nil {
			return fmt.Errorf("replace existing llama.cpp runtime: %w", err)
		}
	}
	if err := os.Rename(staging, final); err != nil {
		if _, restoreErr := os.Stat(backup); restoreErr == nil {
			_ = os.Rename(backup, final)
		}
		return fmt.Errorf("publish llama.cpp runtime: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous llama.cpp runtime: %w", err)
	}
	return nil
}

func (c *LlamaCatalog) releases(ctx context.Context) ([]releaseManifest, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.releasesURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Neuropipe/0.1")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch llama.cpp releases: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("llama.cpp release lookup returned %s", response.Status)
	}
	var raw []struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, err
	}
	manifests := make([]releaseManifest, 0, len(raw))
	for _, release := range raw {
		if release.Draft || release.Prerelease || !llamaVersionPattern.MatchString(release.TagName) {
			continue
		}
		manifest := releaseManifest{release: domain.LlamaRuntimeRelease{Version: release.TagName, PublishedAt: release.PublishedAt}}
		cudaServers := map[string]domain.RuntimeArtifact{}
		cudaLibraries := map[string]domain.RuntimeArtifact{}
		for _, asset := range release.Assets {
			artifact := domain.RuntimeArtifact{URL: asset.BrowserDownloadURL, Size: asset.Size, SHA256: strings.TrimPrefix(asset.Digest, "sha256:")}
			switch {
			case matchVersion(cpuAssetPattern, asset.Name, release.TagName):
				manifest.release.CPU = artifact
			case cudaAssetPattern.MatchString(asset.Name):
				matches := cudaAssetPattern.FindStringSubmatch(asset.Name)
				if matches[1] == release.TagName {
					cudaServers[matches[2]] = artifact
				}
			case cudartAssetPattern.MatchString(asset.Name):
				matches := cudartAssetPattern.FindStringSubmatch(asset.Name)
				cudaLibraries[matches[1]] = artifact
			case matchVersion(vulkanAssetPattern, asset.Name, release.TagName):
				manifest.release.Vulkan = artifact
			case matchVersion(hipAssetPattern, asset.Name, release.TagName):
				manifest.release.HIP = artifact
			}
		}
		for _, toolkit := range []string{"12.4", "13.3"} {
			if server, serverOK := cudaServers[toolkit]; serverOK {
				if libraries, librariesOK := cudaLibraries[toolkit]; librariesOK {
					manifest.release.CUDA, manifest.cudart = server, libraries
					break
				}
			}
		}
		if manifest.release.CPU.URL != "" {
			manifests = append(manifests, manifest)
		}
	}
	if len(manifests) == 0 {
		return nil, errors.New("no compatible Windows x64 llama.cpp releases are currently available")
	}
	return manifests, nil
}

func matchVersion(pattern *regexp.Regexp, name, version string) bool {
	matches := pattern.FindStringSubmatch(name)
	return len(matches) > 1 && matches[1] == version
}

// ModelCatalog discovers public GGUF model repositories and installs a selected file.
type ModelCatalog struct {
	root      string
	http      *http.Client
	hubURL    string
	avatarMu  sync.Mutex
	avatarURL map[string]string
}

// NewModelCatalog creates a model catalog rooted in user-owned app data.
func NewModelCatalog(root string) *ModelCatalog {
	return &ModelCatalog{root: root, http: &http.Client{Timeout: 0}, hubURL: "https://huggingface.co", avatarURL: make(map[string]string)}
}

// Installed returns completed GGUF files from the configured Neuropipe model
// folder. It never follows symlinks, excludes partial downloads, and checks the
// caller's context while walking potentially large local model collections.
func (c *ModelCatalog) Installed(ctx context.Context) ([]domain.LocalModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(c.root); errors.Is(err, os.ErrNotExist) {
		return []domain.LocalModel{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect model folder: %w", err)
	}

	models := make([]domain.LocalModel, 0)
	err := filepath.WalkDir(c.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".gguf") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect model %q: %w", path, err)
		}
		relative, err := filepath.Rel(c.root, path)
		if err != nil {
			return fmt.Errorf("make model path relative: %w", err)
		}
		model := domain.LocalModel{
			ID:   strings.TrimSuffix(filepath.ToSlash(relative), filepath.Ext(relative)),
			Name: filepath.Base(path),
			Path: path,
			Size: info.Size(),
		}
		if metadata, ok := readInstalledModelMetadata(path); ok {
			if metadata.AvatarURL == "" && metadata.Author != "" {
				metadata.AvatarURL = c.creatorAvatarURL(ctx, metadata.Author)
				if metadata.AvatarURL != "" {
					_ = writeInstalledModelMetadataRecord(path, metadata)
				}
			}
			model.Repository = metadata.Repository
			model.Author = metadata.Author
			model.AvatarURL = metadata.AvatarURL
			model.Downloads = metadata.Downloads
			model.Likes = metadata.Likes
			model.LastModified = metadata.LastModified
			model.Tags = metadata.Tags
			model.Quantization = metadata.Quantization
			model.SHA256 = metadata.SHA256
			model.InstalledAt = metadata.InstalledAt
		}
		models = append(models, model)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list installed GGUF models: %w", err)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

// RemoveInstalled deletes one completed GGUF rooted inside the configured
// content folder. Partial paths and files outside that folder are rejected.
func (c *ModelCatalog) RemoveInstalled(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path = filepath.Clean(strings.TrimSpace(path))
	relative, err := filepath.Rel(c.root, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) || !strings.EqualFold(filepath.Ext(path), ".gguf") {
		return errors.New("model must be an installed GGUF inside the configured content folder")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect installed model: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("model is not a regular GGUF file")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove installed model: %w", err)
	}
	if err := os.Remove(installedModelMetadataPath(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("model was removed, but its metadata could not be removed: %w", err)
	}
	return nil
}

func installedModelMetadataPath(modelPath string) string {
	return modelPath + ".neuropipe.json"
}

func readInstalledModelMetadata(modelPath string) (domain.InstalledModelMetadata, bool) {
	metadataPath := installedModelMetadataPath(modelPath)
	info, err := os.Lstat(metadataPath)
	if err != nil || !info.Mode().IsRegular() {
		return domain.InstalledModelMetadata{}, false
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return domain.InstalledModelMetadata{}, false
	}
	var metadata domain.InstalledModelMetadata
	if err := json.Unmarshal(data, &metadata); err != nil || metadata.SchemaVersion != 1 || !validRepository.MatchString(metadata.Repository) || !safeModelFile(metadata.File) {
		return domain.InstalledModelMetadata{}, false
	}
	return metadata, true
}

func writeInstalledModelMetadata(modelPath string, detail domain.ModelDetail, file domain.ModelFile) error {
	metadata := domain.InstalledModelMetadata{
		SchemaVersion: 1,
		Repository:    detail.ID,
		File:          file.Name,
		Quantization:  file.Quantization,
		SHA256:        file.SHA256,
		Author:        detail.Author,
		AvatarURL:     detail.AvatarURL,
		Downloads:     detail.Downloads,
		Likes:         detail.Likes,
		LastModified:  detail.LastModified,
		Tags:          detail.Tags,
		InstalledAt:   time.Now().UTC().Format(time.RFC3339),
	}
	return writeInstalledModelMetadataRecord(modelPath, metadata)
}

func writeInstalledModelMetadataRecord(modelPath string, metadata domain.InstalledModelMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode installed model metadata: %w", err)
	}
	metadataPath := installedModelMetadataPath(modelPath)
	temporaryPath := metadataPath + ".tmp"
	if err := os.WriteFile(temporaryPath, data, 0o600); err != nil {
		return fmt.Errorf("write installed model metadata: %w", err)
	}
	// os.Rename cannot replace an existing file on Windows. A metadata update is
	// a small companion write after the GGUF is already durable, so replacing it
	// before publishing the new sidecar keeps repeat installs reliable.
	if err := os.Remove(metadataPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("replace installed model metadata: %w", err)
	}
	if err := os.Rename(temporaryPath, metadataPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("publish installed model metadata: %w", err)
	}
	return nil
}

// Search returns public GGUF repositories ordered using the selected catalog sort.
func (c *ModelCatalog) Search(ctx context.Context, query domain.ModelSearchRequest) ([]domain.ModelSearchResult, error) {
	var sortBy string
	switch strings.TrimSpace(query.Sort) {
	case "recent":
		sortBy = "lastModified"
	case "recommended", "", "downloads":
		// Hugging Face does not expose an editorial recommendation signal to this
		// unauthenticated endpoint. Downloads are an honest local default.
		sortBy = "downloads"
	default:
		return nil, fmt.Errorf("unsupported model catalog sort %q", query.Sort)
	}
	values := url.Values{"search": {strings.TrimSpace(query.Query)}, "filter": {"gguf"}, "gated": {"false"}, "full": {"true"}, "limit": {"24"}, "sort": {sortBy}, "direction": {"-1"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.hubURL+"/api/models?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("search GGUF models: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("model hub search returned %s", response.Status)
	}
	var raw []struct {
		ID           string   `json:"id"`
		Author       string   `json:"author"`
		Downloads    int64    `json:"downloads"`
		Likes        int      `json:"likes"`
		LastModified string   `json:"lastModified"`
		Tags         []string `json:"tags"`
		Gated        any      `json:"gated"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, err
	}
	models := make([]domain.ModelSearchResult, 0, len(raw))
	for _, model := range raw {
		if !validRepository.MatchString(model.ID) || isGated(model.Gated) {
			continue
		}
		author := model.Author
		if author == "" {
			author = strings.SplitN(model.ID, "/", 2)[0]
		}
		models = append(models, domain.ModelSearchResult{ID: model.ID, Author: author, Downloads: model.Downloads, Likes: model.Likes, LastModified: model.LastModified, Tags: model.Tags})
	}
	c.populateSearchAvatars(ctx, models)
	return models, nil
}

type hubModel struct {
	ID           string   `json:"id"`
	Author       string   `json:"author"`
	Downloads    int64    `json:"downloads"`
	Likes        int      `json:"likes"`
	LastModified string   `json:"lastModified"`
	Tags         []string `json:"tags"`
	Gated        any      `json:"gated"`
	Siblings     []struct {
		Name string `json:"rfilename"`
		Size int64  `json:"size"`
		LFS  struct {
			OID  string `json:"oid"`
			Size int64  `json:"size"`
		} `json:"lfs"`
	} `json:"siblings"`
}

// Detail returns public metadata, verified GGUF choices, and a bounded model card.
func (c *ModelCatalog) Detail(ctx context.Context, repository string) (domain.ModelDetail, error) {
	if !validRepository.MatchString(repository) {
		return domain.ModelDetail{}, errors.New("invalid Hugging Face repository")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.hubURL+"/api/models/"+repository+"?blobs=true", nil)
	if err != nil {
		return domain.ModelDetail{}, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return domain.ModelDetail{}, fmt.Errorf("fetch model detail: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return domain.ModelDetail{}, fmt.Errorf("model hub lookup returned %s", response.Status)
	}
	var raw hubModel
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return domain.ModelDetail{}, err
	}
	if isGated(raw.Gated) {
		return domain.ModelDetail{}, errors.New("this Hugging Face repository is gated and cannot be installed without an account")
	}
	files := filesFromHub(raw.Siblings)
	readme, err := c.readme(ctx, repository)
	if err != nil {
		return domain.ModelDetail{}, err
	}
	return domain.ModelDetail{ID: repository, Author: raw.Author, AvatarURL: c.creatorAvatarURL(ctx, raw.Author), Downloads: raw.Downloads, Likes: raw.Likes, LastModified: raw.LastModified, Tags: raw.Tags, Readme: readme, Files: files}, nil
}

// creatorAvatarURL resolves the public CDN image embedded in a Hugging Face
// creator profile. The URL is cached per catalog lifetime and only fetched for
// the model currently opened in the detail view.
func (c *ModelCatalog) creatorAvatarURL(ctx context.Context, creator string) string {
	creator = strings.TrimSpace(creator)
	if creator == "" {
		return ""
	}
	c.avatarMu.Lock()
	cached, found := c.avatarURL[creator]
	c.avatarMu.Unlock()
	if found {
		return cached
	}
	requestContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, c.hubURL+"/"+url.PathEscape(creator), nil)
	if err != nil {
		return ""
	}
	response, err := c.http.Do(request)
	if err != nil {
		return ""
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 512*1024))
	if err != nil {
		return ""
	}
	avatar := profileAvatarURL.FindString(string(data))
	parsed, err := url.Parse(avatar)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "cdn-avatars.huggingface.co") {
		return ""
	}
	c.avatarMu.Lock()
	c.avatarURL[creator] = avatar
	c.avatarMu.Unlock()
	return avatar
}

func (c *ModelCatalog) populateSearchAvatars(ctx context.Context, models []domain.ModelSearchResult) {
	const parallelProfileRequests = 4
	semaphore := make(chan struct{}, parallelProfileRequests)
	var group sync.WaitGroup
	for index := range models {
		if models[index].Author == "" {
			continue
		}
		group.Add(1)
		go func(index int) {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			models[index].AvatarURL = c.creatorAvatarURL(ctx, models[index].Author)
		}(index)
	}
	group.Wait()
}

// Files remains a compact compatibility API for callers that only need files.
func (c *ModelCatalog) Files(ctx context.Context, repository string) ([]domain.ModelFile, error) {
	detail, err := c.Detail(ctx, repository)
	if err != nil {
		return nil, err
	}
	return detail.Files, nil
}

func filesFromHub(siblings []struct {
	Name string `json:"rfilename"`
	Size int64  `json:"size"`
	LFS  struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs"`
}) []domain.ModelFile {
	files := make([]domain.ModelFile, 0, len(siblings))
	for _, file := range siblings {
		if !strings.HasSuffix(strings.ToLower(file.Name), ".gguf") {
			continue
		}
		size := file.Size
		if file.LFS.Size > 0 {
			size = file.LFS.Size
		}
		files = append(files, domain.ModelFile{Name: file.Name, Size: size, SHA256: file.LFS.OID, Quantization: quantization(file.Name)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	if index := recommendedFile(files); index >= 0 {
		files[index].Recommended = true
	}
	return files
}

func quantization(name string) string {
	match := quantizationPattern.FindStringSubmatch(strings.ToUpper(name))
	if len(match) < 2 {
		return ""
	}
	return strings.ToUpper(match[1])
}

func recommendedFile(files []domain.ModelFile) int {
	for _, preferred := range []string{"Q4_K_M", "Q5_K_M"} {
		for index := range files {
			if files[index].Quantization == preferred {
				return index
			}
		}
	}
	best := -1
	for index := range files {
		if best < 0 || (files[index].Size > 0 && files[index].Size < files[best].Size) {
			best = index
		}
	}
	return best
}

func (c *ModelCatalog) readme(ctx context.Context, repository string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.hubURL+"/"+repository+"/raw/main/README.md", nil)
	if err != nil {
		return "", err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch model card: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("model card returned %s", response.Status)
	}
	const maxReadmeBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxReadmeBytes+1))
	if err != nil {
		return "", fmt.Errorf("read model card: %w", err)
	}
	if len(data) > maxReadmeBytes {
		data = data[:maxReadmeBytes]
	}
	return string(data), nil
}

// Install downloads, verifies, and atomically publishes a selected public GGUF model.
func (c *ModelCatalog) Install(ctx context.Context, request domain.ModelInstallRequest) (domain.LocalModel, error) {
	return c.InstallWithProgress(ctx, request, nil)
}

// InstallWithProgress downloads and atomically selects a public GGUF model.
func (c *ModelCatalog) InstallWithProgress(ctx context.Context, request domain.ModelInstallRequest, report ProgressReporter) (domain.LocalModel, error) {
	if !validRepository.MatchString(request.Repository) {
		return domain.LocalModel{}, errors.New("invalid Hugging Face repository")
	}
	if !safeModelFile(request.File) {
		return domain.LocalModel{}, errors.New("invalid Hugging Face model file")
	}
	detail, err := c.Detail(ctx, request.Repository)
	if err != nil {
		return domain.LocalModel{}, err
	}
	files := detail.Files
	var selected *domain.ModelFile
	for index := range files {
		if files[index].Name == request.File {
			selected = &files[index]
			break
		}
	}
	if selected == nil {
		return domain.LocalModel{}, errors.New("selected GGUF model is no longer available")
	}
	destination := filepath.Join(c.root, safePathPart(request.Repository), safePathPart(request.File))
	segments := strings.Split(strings.ReplaceAll(request.File, "\\", "/"), "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	downloadURL := c.hubURL + "/" + request.Repository + "/resolve/main/" + strings.Join(segments, "/")
	reportInstallProgress(report, "model", "downloading", request.File, 0, selected.Size, 0)
	lastBytesPerSecond := 0.0
	path, err := downloadResumableWithProgress(ctx, c.http, downloadURL, destination, selected.Size, selected.SHA256, "Neuropipe/0.1", func(progress downloadProgress) {
		if progress.bytesPerSecond > 0 {
			lastBytesPerSecond = progress.bytesPerSecond
		}
		reportInstallProgress(report, "model", "downloading", request.File, progress.downloadedBytes, progress.totalBytes, progress.bytesPerSecond)
	})
	if err != nil {
		return domain.LocalModel{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.LocalModel{}, err
	}
	if err := writeInstalledModelMetadata(path, detail, *selected); err != nil {
		return domain.LocalModel{}, err
	}
	relative, err := filepath.Rel(c.root, path)
	if err != nil {
		return domain.LocalModel{}, err
	}
	reportInstallProgress(report, "model", "installed", request.File, info.Size(), selected.Size, lastBytesPerSecond)
	return domain.LocalModel{
		ID:           strings.TrimSuffix(filepath.ToSlash(relative), ".gguf"),
		Name:         request.File,
		Path:         path,
		Size:         info.Size(),
		Repository:   detail.ID,
		Author:       detail.Author,
		AvatarURL:    detail.AvatarURL,
		Downloads:    detail.Downloads,
		Likes:        detail.Likes,
		LastModified: detail.LastModified,
		Tags:         detail.Tags,
		Quantization: selected.Quantization,
		SHA256:       selected.SHA256,
		InstalledAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func downloadResumableWithProgress(ctx context.Context, client *http.Client, sourceURL, destination string, expectedSize int64, expectedSHA256, userAgent string, report func(downloadProgress)) (string, error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", err
	}
	if info, err := os.Stat(destination); err == nil {
		if (expectedSize <= 0 || info.Size() == expectedSize) && checksumMatches(destination, expectedSHA256) {
			reportDownloadProgress(report, info.Size(), expectedSize, 0)
			return destination, nil
		}
		return "", errors.New("an existing download has an unexpected size or checksum; remove it before retrying")
	}
	partial := destination + ".part"
	offset := int64(0)
	if info, err := os.Stat(partial); err == nil {
		offset = info.Size()
		if (expectedSize <= 0 || offset == expectedSize) && checksumMatches(partial, expectedSHA256) {
			if err := os.Rename(partial, destination); err != nil {
				return "", err
			}
			reportDownloadProgress(report, offset, expectedSize, 0)
			return destination, nil
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", userAgent)
	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download file: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("download returned %s", response.Status)
	}
	if response.StatusCode == http.StatusOK {
		offset = 0
	}
	if expectedSize <= 0 && response.ContentLength >= 0 {
		expectedSize = response.ContentLength + offset
	}
	reportDownloadProgress(report, offset, expectedSize, 0)
	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	target, err := os.OpenFile(partial, flags, 0o600)
	if err != nil {
		return "", err
	}
	buffer := make([]byte, 256*1024)
	writer := &progressWriter{writer: target, offset: offset, total: expectedSize, lastEmit: time.Now(), report: report}
	_, copyErr := io.CopyBuffer(writer, response.Body, buffer)
	closeErr := target.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	info, err := os.Stat(partial)
	if err != nil {
		return "", err
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return "", fmt.Errorf("download size validation failed: received %d bytes, expected %d", info.Size(), expectedSize)
	}
	if !checksumMatches(partial, expectedSHA256) {
		return "", errors.New("download checksum validation failed")
	}
	reportDownloadProgress(report, info.Size(), expectedSize, writer.bytesPerSecond())
	if err := os.Rename(partial, destination); err != nil {
		return "", err
	}
	return destination, nil
}

type progressWriter struct {
	writer          io.Writer
	offset          int64
	total           int64
	written         int64
	lastEmit        time.Time
	lastEmitWritten int64
	lastSpeed       float64
	report          func(downloadProgress)
}

func (w *progressWriter) Write(value []byte) (int, error) {
	written, err := w.writer.Write(value)
	w.written += int64(written)
	now := time.Now()
	if now.Sub(w.lastEmit) >= 250*time.Millisecond || (w.total > 0 && w.offset+w.written >= w.total) {
		elapsed := now.Sub(w.lastEmit).Seconds()
		if elapsed > 0 {
			w.lastSpeed = float64(w.written-w.lastEmitWritten) / elapsed
		}
		reportDownloadProgress(w.report, w.offset+w.written, w.total, w.lastSpeed)
		w.lastEmit, w.lastEmitWritten = now, w.written
	}
	return written, err
}

func (w *progressWriter) bytesPerSecond() float64 { return w.lastSpeed }

func reportDownloadProgress(report func(downloadProgress), downloadedBytes, totalBytes int64, bytesPerSecond float64) {
	if report != nil {
		report(downloadProgress{downloadedBytes: downloadedBytes, totalBytes: totalBytes, bytesPerSecond: bytesPerSecond})
	}
}

func reportInstallProgress(report ProgressReporter, kind, stage, label string, downloadedBytes, totalBytes int64, bytesPerSecond float64) {
	if report == nil {
		return
	}
	percentage := 0
	if totalBytes > 0 {
		percentage = int(downloadedBytes * 100 / totalBytes)
		if percentage > 100 {
			percentage = 100
		}
	}
	report(domain.InstallProgress{Kind: kind, Stage: stage, Label: label, DownloadedBytes: downloadedBytes, TotalBytes: totalBytes, BytesPerSecond: bytesPerSecond, Percentage: percentage})
}

func extractServer(archive, destination string) error {
	scratch, err := os.MkdirTemp(filepath.Dir(destination), ".extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := extractArchive(archive, scratch); err != nil {
		return err
	}
	server := ""
	err = filepath.WalkDir(scratch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "llama-server.exe") {
			server = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}
	if server == "" {
		return errors.New("llama.cpp archive did not contain llama-server.exe")
	}
	return copyDirectory(filepath.Dir(server), destination)
}

func copyArchiveFiles(archive, destination string) error {
	scratch, err := os.MkdirTemp(filepath.Dir(destination), ".extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := extractArchive(archive, scratch); err != nil {
		return err
	}
	return filepath.WalkDir(scratch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		return copyFile(path, filepath.Join(destination, entry.Name()))
	})
}

func extractArchive(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open llama.cpp archive: %w", err)
	}
	defer func() { _ = reader.Close() }()
	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if filepath.IsAbs(name) || name == "." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in archive: %s", file.Name)
		}
		target := filepath.Join(destination, name)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in archive: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		copyErr := copyStream(source, target)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		return copyFile(path, target)
	})
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	return copyStream(input, destination)
}

func copyStream(source io.Reader, destination string) error {
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func checksumMatches(path, expected string) bool {
	expected = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(expected)), "sha256:")
	if expected == "" {
		return true
	}
	if len(expected) != 64 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(expected) {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return false
	}
	return hex.EncodeToString(digest.Sum(nil)) == expected
}

func installableRuntimeMode(mode domain.RuntimeMode) bool {
	return mode == domain.RuntimeCPU || mode == domain.RuntimeCUDA || mode == domain.RuntimeVulkan || mode == domain.RuntimeHIP
}

func runtimeFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func safeArchivePart(name string) string {
	return strings.NewReplacer(" ", "-", "/", "-", "\\", "-").Replace(strings.ToLower(name))
}

func isGated(value any) bool {
	switch status := value.(type) {
	case bool:
		return status
	case string:
		return strings.TrimSpace(status) != "" && !strings.EqualFold(status, "false")
	default:
		return false
	}
}

func safeModelFile(name string) bool {
	return name != "" && !strings.Contains(name, "..") && !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "\\")
}

func safePathPart(value string) string {
	return strings.NewReplacer("/", "__", "\\", "__", ":", "_").Replace(value)
}
