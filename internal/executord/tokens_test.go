package executord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTokenPrecedence(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	work := t.TempDir()
	configuredFile := filepath.Join(work, "configured.txt")
	if err := os.WriteFile(configuredFile, []byte("configured-token\n"), 0o600); err != nil {
		t.Fatalf("write configured token file: %v", err)
	}
	dataDir := filepath.Join(work, "data")
	savedFile := filepath.Join(dataDir, DefaultTokenFileName)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(savedFile, []byte("saved-token"), 0o600); err != nil {
		t.Fatalf("write saved token file: %v", err)
	}
	boot := BootConfig{DataDir: dataDir, TokenFile: configuredFile}

	assertResolution := func(t *testing.T, boot BootConfig, explicit string, wantSource TokenSource, wantValue string) {
		t.Helper()
		resolved, err := ResolveToken(boot, explicit)
		if err != nil {
			t.Fatalf("ResolveToken() error = %v", err)
		}
		if resolved.Value != wantValue || resolved.Source != wantSource || resolved.Generated {
			t.Fatalf("ResolveToken() = %+v, want %s %q not generated", resolved, wantSource, wantValue)
		}
	}

	assertResolution(t, boot, "flag-token ", TokenSourceExplicit, "flag-token")
	t.Setenv(TokenEnvVar, "env-token")
	assertResolution(t, boot, "", TokenSourceEnv, "env-token")
	t.Setenv(TokenEnvVar, "")
	assertResolution(t, boot, "", TokenSourceConfiguredFile, "configured-token")

	noFileBoot := BootConfig{DataDir: dataDir}
	assertResolution(t, noFileBoot, "", TokenSourceSaved, "saved-token")

	if err := os.Remove(savedFile); err != nil {
		t.Fatalf("remove saved token: %v", err)
	}
	resolved, err := ResolveToken(noFileBoot, "")
	if err != nil {
		t.Fatalf("ResolveToken() without any token error = %v", err)
	}
	if resolved.Value != "" || resolved.Source != "" || resolved.Generated {
		t.Fatalf("unconfigured ResolveToken() = %+v, want empty result", resolved)
	}
}

func TestEnsureTokenGeneratesOnce(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	work := t.TempDir()
	boot := BootConfig{DataDir: filepath.Join(work, "data")}

	first, err := EnsureToken(boot, "")
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if !first.Generated || first.Source != TokenSourceSaved {
		t.Fatalf("first resolution = %+v, want generated saved token", first)
	}
	if len(first.Value) != 64 || strings.Trim(first.Value, "0123456789abcdef") != "" {
		t.Fatalf("generated token = %q, want 64 hex characters", first.Value)
	}
	persisted, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("token was not persisted: %v", err)
	}
	if strings.TrimSpace(string(persisted)) != first.Value {
		t.Fatalf("persisted token = %q, want %q", string(persisted), first.Value)
	}

	second, err := EnsureToken(boot, "")
	if err != nil {
		t.Fatalf("second EnsureToken() error = %v", err)
	}
	if second.Generated {
		t.Fatalf("second resolution must not regenerate: %+v", second)
	}
	if second.Value != first.Value {
		t.Fatalf("second token = %q, want the stored %q", second.Value, first.Value)
	}
}

func TestEnsureTokenExplicitNeverPersists(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	work := t.TempDir()
	boot := BootConfig{DataDir: filepath.Join(work, "data")}

	resolved, err := EnsureToken(boot, "explicit-value")
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if resolved.Value != "explicit-value" || resolved.Source != TokenSourceExplicit || resolved.Generated {
		t.Fatalf("resolution = %+v, want explicit passthrough", resolved)
	}
	if _, err := os.Stat(filepath.Join(boot.DataDir, DefaultTokenFileName)); !os.IsNotExist(err) {
		t.Fatalf("explicit token must not be written to disk (stat err = %v)", err)
	}
}

func TestGenerateAndSaveTokenRotatesStoredSecret(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	boot := BootConfig{DataDir: filepath.Join(t.TempDir(), "data")}

	first, err := EnsureToken(boot, "")
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	rotated, err := GenerateAndSaveToken(boot)
	if err != nil {
		t.Fatalf("GenerateAndSaveToken() error = %v", err)
	}
	if !rotated.Generated || rotated.Value == first.Value {
		t.Fatalf("rotation produced %+v, want a fresh value", rotated)
	}
	reloaded, err := ResolveToken(boot, "")
	if err != nil {
		t.Fatalf("ResolveToken() after rotation error = %v", err)
	}
	if reloaded.Value != rotated.Value || reloaded.Generated {
		t.Fatalf("stored token after rotation = %+v, want the rotated secret without regeneration", reloaded)
	}
}
