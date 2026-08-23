// Package executord implements the standalone Neuropipe pipeline executor:
// a headless daemon that stores deployed pipeline bundles, runs them with the
// full Blueprint engine, hosts trusted cron schedules autonomously, and
// serves the gRPC contract from internal/proto/executor/v1.
package executord

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultListenAddress is the gRPC endpoint executors bind by default.
const DefaultListenAddress = ":47777"

// DefaultMaxConcurrentRuns bounds parallel executions until configured.
const DefaultMaxConcurrentRuns = 4

// TokenEnvVar carries the shared secret as an alternative to a token file.
const TokenEnvVar = "NEUROPIPE_EXECUTOR_TOKEN"

// BootConfig contains static settings that are intentionally not writable
// over the network: the listen address, data directory, transport secrets,
// and the shared auth token source.
type BootConfig struct {
	ListenAddress string `json:"listen"`
	DataDir       string `json:"dataDir"`
	TokenFile     string `json:"tokenFile,omitempty"`
	TLSCert       string `json:"tlsCert,omitempty"`
	TLSKey        string `json:"tlsKey,omitempty"`
}

// LoadBootConfig reads the executor boot file. A missing file yields defaults
// so `neuropipe-executor` runs with no configuration at all.
func LoadBootConfig(path string) (BootConfig, error) {
	config := BootConfig{ListenAddress: DefaultListenAddress}
	if path == "" {
		return config, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return BootConfig{}, fmt.Errorf("read executor config %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return BootConfig{}, fmt.Errorf("parse executor config %q: %w", path, err)
	}
	if strings.TrimSpace(config.ListenAddress) == "" {
		config.ListenAddress = DefaultListenAddress
	}
	return config, nil
}

// HasTLS reports whether TLS material was configured.
func (c BootConfig) HasTLS() bool { return c.TLSCert != "" && c.TLSKey != "" }

// PrepareDataDir creates the on-disk layout used by every store.
func PrepareDataDir(root string) error {
	for _, sub := range []string{"deployed", "runs"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o700); err != nil {
			return fmt.Errorf("create executor data directory: %w", err)
		}
	}
	return nil
}
