// Package kv manages registered Redis-protocol key/value connections
// (Redis, Valkey, KeyDB, Dragonfly) and their pooled go-redis clients.
// Command validation, denylisting, and reply normalisation live in commands.go.
package kv

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/redis/go-redis/v9"
)

// options builds the go-redis client options for one registered connection.
// The host/port form is canonical; a non-empty Address carries a complete
// redis:// URL that overrides it. secret is the decrypted password (or the
// empty string) sourced from the vault.
func options(item domain.Database, secret string) (*redis.Options, error) {
	address := strings.TrimSpace(item.Address)
	var opts *redis.Options
	if address != "" {
		parsed, err := redis.ParseURL(address)
		if err != nil {
			return nil, fmt.Errorf("parse redis URL: %w", err)
		}
		opts = parsed
		// Host/port fields stay meaningful for the UI detail line, but the
		// URL wins for the actual dial target.
	} else {
		host := strings.TrimSpace(item.Host)
		if host == "" {
			return nil, fmt.Errorf("redis host is required")
		}
		port := item.Port
		if port == 0 {
			port = 6379
		}
		opts = &redis.Options{Addr: fmt.Sprintf("%s:%d", host, port)}
	}
	opts.Username = strings.TrimSpace(item.Username)
	if secret != "" {
		// An explicit vault password overrides any password embedded in a URL
		// so rotation never requires editing the URL itself.
		opts.Password = secret
	}
	opts.DB = item.DBIndex
	if opts.DB < 0 {
		opts.DB = 0
	}
	if name := strings.TrimSpace(item.ClientName); name != "" {
		opts.ClientName = name
	}
	if item.UseTLS {
		opts.TLSConfig = tlsConfig()
	}
	return opts, nil
}

// client builds a new client from the registered metadata. Pool sizing is
// left at go-redis defaults: local automation workloads rarely exceed them,
// and the client is shared per connection.
func client(item domain.Database, secret string) (*redis.Client, error) {
	opts, err := options(item, secret)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(opts), nil
}

func tlsConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12}
}
