// Package updatecheck compares Neuropipe's release version with GitHub's
// latest published release.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLatestReleaseURL = "https://api.github.com/repos/FlameInTheDark/neuropipe/releases/latest"
	defaultRequestTimeout   = 12 * time.Second
	maxResponseBytes        = 1 << 20
)

// Release identifies a published Neuropipe version and its GitHub release page.
type Release struct {
	Version string
	URL     string
}

// Source retrieves the newest published application release.
type Source interface {
	Latest(context.Context) (Release, error)
}

// Checker compares a running version to releases supplied by a Source.
type Checker struct {
	source  Source
	current semanticVersion
	valid   bool
}

// NewChecker creates a release checker for the supplied running version.
// Development builds without a semantic version intentionally do not advertise updates.
func NewChecker(source Source, currentVersion string) *Checker {
	current, err := parseSemanticVersion(currentVersion)
	return &Checker{source: source, current: current, valid: err == nil}
}

// Check reports whether a newer stable release is available.
func (c *Checker) Check(ctx context.Context) (Release, bool, error) {
	if c == nil || c.source == nil || !c.valid {
		return Release{}, false, nil
	}

	release, err := c.source.Latest(ctx)
	if err != nil {
		return Release{}, false, fmt.Errorf("get latest release: %w", err)
	}
	latest, err := parseSemanticVersion(release.Version)
	if err != nil {
		return Release{}, false, fmt.Errorf("parse latest release version %q: %w", release.Version, err)
	}
	return release, latest.compare(c.current) > 0, nil
}

// GitHubSource retrieves the latest non-prerelease release from Neuropipe's
// public GitHub repository.
type GitHubSource struct {
	client   *http.Client
	endpoint string
}

// NewGitHubSource creates a GitHub latest-release source.
func NewGitHubSource(client *http.Client) *GitHubSource {
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimeout}
	}
	return &GitHubSource{client: client, endpoint: defaultLatestReleaseURL}
}

// Latest requests the repository's latest release metadata.
func (s *GitHubSource) Latest(ctx context.Context) (release Release, err error) {
	if s == nil || s.client == nil {
		return Release{}, fmt.Errorf("GitHub release source is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return Release{}, fmt.Errorf("create latest-release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Neuropipe-update-checker")

	response, err := s.client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("request latest release: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close latest-release response: %w", closeErr)
		}
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2048))
		if readErr != nil {
			return Release{}, fmt.Errorf("read latest-release error response: %w", readErr)
		}
		return Release{}, fmt.Errorf("latest-release request returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if strings.TrimSpace(payload.TagName) == "" || strings.TrimSpace(payload.HTMLURL) == "" {
		return Release{}, fmt.Errorf("latest release response is missing a tag or page URL")
	}
	return Release{Version: payload.TagName, URL: payload.HTMLURL}, nil
}

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []prereleaseIdentifier
}

type prereleaseIdentifier struct {
	numeric bool
	number  uint64
	text    string
}

func parseSemanticVersion(input string) (semanticVersion, error) {
	version := strings.TrimSpace(input)
	version = strings.TrimPrefix(version, "v")
	if buildIndex := strings.IndexByte(version, '+'); buildIndex >= 0 {
		build := version[buildIndex+1:]
		if build == "" {
			return semanticVersion{}, fmt.Errorf("build metadata must not be empty")
		}
		for _, identifier := range strings.Split(build, ".") {
			if identifier == "" || !isPrereleaseIdentifier(identifier) {
				return semanticVersion{}, fmt.Errorf("invalid build metadata identifier %q", identifier)
			}
		}
		version = version[:buildIndex]
	}
	core, prerelease, hasPrerelease := strings.Cut(version, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semanticVersion{}, fmt.Errorf("must contain major, minor, and patch components")
	}
	parsed := semanticVersion{}
	values := []*uint64{&parsed.major, &parsed.minor, &parsed.patch}
	for index, part := range parts {
		value, err := parseNumericIdentifier(part)
		if err != nil {
			return semanticVersion{}, fmt.Errorf("parse version component %d: %w", index+1, err)
		}
		*values[index] = value
	}
	if !hasPrerelease {
		return parsed, nil
	}
	if prerelease == "" {
		return semanticVersion{}, fmt.Errorf("prerelease must not be empty")
	}
	for _, identifier := range strings.Split(prerelease, ".") {
		if identifier == "" || !isPrereleaseIdentifier(identifier) {
			return semanticVersion{}, fmt.Errorf("invalid prerelease identifier %q", identifier)
		}
		if isNumericIdentifier(identifier) {
			number, err := parseNumericIdentifier(identifier)
			if err != nil {
				return semanticVersion{}, fmt.Errorf("parse prerelease identifier %q: %w", identifier, err)
			}
			parsed.prerelease = append(parsed.prerelease, prereleaseIdentifier{numeric: true, number: number})
			continue
		}
		parsed.prerelease = append(parsed.prerelease, prereleaseIdentifier{text: identifier})
	}
	return parsed, nil
}

func parseNumericIdentifier(value string) (uint64, error) {
	if value == "" {
		return 0, fmt.Errorf("must not be empty")
	}
	if len(value) > 1 && value[0] == '0' {
		return 0, fmt.Errorf("must not contain a leading zero")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("must contain only digits")
		}
	}
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse number: %w", err)
	}
	return number, nil
}

func isPrereleaseIdentifier(value string) bool {
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return true
}

func isNumericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func (v semanticVersion) compare(other semanticVersion) int {
	for _, pair := range [][2]uint64{{v.major, other.major}, {v.minor, other.minor}, {v.patch, other.patch}} {
		if pair[0] > pair[1] {
			return 1
		}
		if pair[0] < pair[1] {
			return -1
		}
	}
	if len(v.prerelease) == 0 && len(other.prerelease) == 0 {
		return 0
	}
	if len(v.prerelease) == 0 {
		return 1
	}
	if len(other.prerelease) == 0 {
		return -1
	}
	for index := 0; index < len(v.prerelease) && index < len(other.prerelease); index++ {
		left, right := v.prerelease[index], other.prerelease[index]
		if left.numeric && right.numeric {
			if left.number > right.number {
				return 1
			}
			if left.number < right.number {
				return -1
			}
			continue
		}
		if left.numeric != right.numeric {
			if left.numeric {
				return -1
			}
			return 1
		}
		if left.text > right.text {
			return 1
		}
		if left.text < right.text {
			return -1
		}
	}
	if len(v.prerelease) > len(other.prerelease) {
		return 1
	}
	if len(v.prerelease) < len(other.prerelease) {
		return -1
	}
	return 0
}
