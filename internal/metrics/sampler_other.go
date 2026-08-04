//go:build !windows

package metrics

// NewProcessSampler supplies a portable no-op collector for non-Windows test
// environments. Neuropipe only enables owned-process sampling on Windows.
func NewProcessSampler() ProcessSampler { return nil }
