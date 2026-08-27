//go:build windows

package security

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
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

// Vault persists DPAPI-protected secret values in a user-owned private file.
// The vault is shared by several services whose background goroutines
// (validation loops, pollers) read tokens concurrently with request
// goroutines saving or removing secrets, so every method is safe for
// concurrent use: a mutex serializes each read-modify-write cycle and
// prevents lost updates between simultaneous Put and Delete calls.
type Vault struct {
	path string
	mu   sync.Mutex
}

// NewVault creates a Windows DPAPI-backed vault under application data.
func NewVault(root string) (*Vault, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create secret vault directory: %w", err)
	}
	return &Vault{path: filepath.Join(root, "secrets.json")}, nil
}

// List returns metadata without decrypting values.
func (v *Vault) List() ([]SecretMetadata, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
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

// Put encrypts and atomically saves one secret under its stable reference name.
func (v *Vault) Put(name, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	records, err := v.read()
	if err != nil {
		return err
	}
	ciphertext, err := protect([]byte(value))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	record := records[name]
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.Value, record.UpdatedAt = base64.StdEncoding.EncodeToString(ciphertext), now
	records[name] = record
	return v.write(records)
}

// Get decrypts a secret for backend-only provider and template use.
func (v *Vault) Get(name string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	records, err := v.read()
	if err != nil {
		return "", err
	}
	record, exists := records[name]
	if !exists {
		return "", fmt.Errorf("secret %q not found", name)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Value)
	if err != nil {
		return "", fmt.Errorf("decode secret %q: %w", name, err)
	}
	plaintext, err := unprotect(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Delete removes a secret value from the vault.
func (v *Vault) Delete(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
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

func protect(data []byte) ([]byte, error) {
	input := blob(data)
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, fmt.Errorf("protect secret with DPAPI: %w", err)
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data))) }()
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}

func unprotect(data []byte) ([]byte, error) {
	input := blob(data)
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, fmt.Errorf("unprotect secret with DPAPI: %w", err)
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data))) }()
	return append([]byte(nil), unsafe.Slice(output.Data, output.Size)...), nil
}

func blob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}
