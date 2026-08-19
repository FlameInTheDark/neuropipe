//go:build !windows

// Package security provides a vault for secrets on Windows (DPAPI-backed) and
// a portable plaintext fallback for non-Windows platforms. The fallback is
// intended for development and testing only; production builds target Windows.
package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SecretMetadata is safe to return to the UI; it deliberately excludes values.
type SecretMetadata struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type secretRecord struct {
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Vault persists secret values in a user-owned private file. This non-Windows
// build stores values as plaintext JSON; the Windows build uses DPAPI.
type Vault struct {
	path string
}

// NewVault creates a portable vault under application data.
func NewVault(root string) (*Vault, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create secret vault directory: %w", err)
	}
	return &Vault{path: filepath.Join(root, "secrets.json")}, nil
}

// List returns metadata without decrypting values.
func (v *Vault) List() ([]SecretMetadata, error) {
	records, err := v.read()
	if err != nil {
		return nil, err
	}
	result := make([]SecretMetadata, 0, len(records))
	for name, record := range records {
		result = append(result, SecretMetadata{Name: name, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// Put saves one secret under its stable reference name.
func (v *Vault) Put(name, value string) error {
	records, err := v.read()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	record := records[name]
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.Value, record.UpdatedAt = value, now
	records[name] = record
	return v.write(records)
}

// Get returns a secret value.
func (v *Vault) Get(name string) (string, error) {
	records, err := v.read()
	if err != nil {
		return "", err
	}
	record, exists := records[name]
	if !exists {
		return "", fmt.Errorf("secret %q not found", name)
	}
	return record.Value, nil
}

// Delete removes a secret value from the vault.
func (v *Vault) Delete(name string) error {
	records, err := v.read()
	if err != nil {
		return err
	}
	delete(records, name)
	return v.write(records)
}

func (v *Vault) read() (map[string]secretRecord, error) {
	records := make(map[string]secretRecord)
	data, err := os.ReadFile(v.path)
	if os.IsNotExist(err) {
		return records, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read secret vault: %w", err)
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode secret vault: %w", err)
	}
	return records, nil
}

func (v *Vault) write(records map[string]secretRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode secret vault: %w", err)
	}
	temporary := v.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write secret vault: %w", err)
	}
	if err := os.Rename(temporary, v.path); err != nil {
		return fmt.Errorf("replace secret vault: %w", err)
	}
	return nil
}
