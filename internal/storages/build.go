package storages

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// BuildStorage validates a save request and returns the persistable item.
// It performs no I/O so both the create and update paths share one rule set.
func BuildStorage(request domain.SaveStorageRequest) (domain.Storage, error) {
	item := domain.Storage{
		ID:            strings.TrimSpace(request.ID),
		Name:          strings.TrimSpace(request.Name),
		Driver:        request.Driver,
		Endpoint:      strings.TrimSpace(request.Endpoint),
		Region:        strings.TrimSpace(request.Region),
		Bucket:        strings.TrimSpace(request.Bucket),
		AccessKey:     strings.TrimSpace(request.AccessKey),
		SecretRef:     strings.TrimSpace(request.SecretRef),
		Secure:        request.Secure,
		Host:          strings.TrimSpace(request.Host),
		Port:          request.Port,
		Username:      strings.TrimSpace(request.Username),
		PasswordRef:   strings.TrimSpace(request.PasswordRef),
		TLSMode:       request.TLSMode,
		BaseDir:       strings.TrimSpace(request.BaseDir),
		PublicBaseURL: strings.TrimRight(strings.TrimSpace(request.PublicBaseURL), "/"),
	}
	if item.Name == "" {
		return domain.Storage{}, fmt.Errorf("storage name is required")
	}
	if item.Driver == "" {
		item.Driver = domain.StorageDriverS3
	}
	if item.TLSMode == "" {
		item.TLSMode = domain.StorageTLSNone
	}
	if err := validateStorageItem(item); err != nil {
		return domain.Storage{}, err
	}
	return item, nil
}

// validateStorageItem enforces the per-driver required settings.
func validateStorageItem(item domain.Storage) error {
	if err := validatePublicBaseURL(item.PublicBaseURL); err != nil {
		return err
	}
	switch item.Driver {
	case domain.StorageDriverS3:
		if item.Bucket == "" {
			return fmt.Errorf("bucket is required for S3 storages")
		}
		if item.Endpoint != "" && strings.Contains(item.Endpoint, "://") {
			return fmt.Errorf("endpoint must be a host[:port], not a URL")
		}
	case domain.StorageDriverFTP:
		if item.Host == "" {
			return fmt.Errorf("host is required for FTP storages")
		}
		if item.Port < 0 || item.Port > 65535 {
			return fmt.Errorf("port must be between 0 and 65535")
		}
		if !domain.ValidStorageTLSMode(item.TLSMode) {
			return fmt.Errorf("unknown TLS mode %q", item.TLSMode)
		}
		if _, err := CleanRemotePath(item.BaseDir); err != nil {
			return fmt.Errorf("base directory: %w", err)
		}
	default:
		return fmt.Errorf("unknown storage driver %q", item.Driver)
	}
	return nil
}

// validatePublicBaseURL enforces the optional public base URL: an absolute
// http(s) address without credentials or a path traversal. The value is
// stored trimmed of trailing slashes.
func validatePublicBaseURL(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("public URL base: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("public URL base must start with http:// or https://")
	}
	if parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("public URL base must be a plain http(s) address without credentials")
	}
	if strings.Contains(parsed.Path, "..") {
		return fmt.Errorf("public URL base must not contain \"..\" segments")
	}
	return nil
}

// ftpPort returns the effective FTP port (21 when unset).
func ftpPort(item domain.Storage) int {
	if item.Port > 0 {
		return item.Port
	}
	return 21
}

// s3Region returns the effective S3 region (us-east-1 when unset — required
// by AWS, ignored by most S3-compatible servers).
func s3Region(item domain.Storage) string {
	if item.Region != "" {
		return item.Region
	}
	return "us-east-1"
}
