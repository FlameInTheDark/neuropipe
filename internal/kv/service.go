package kv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	sugardblib "github.com/echovault/sugardb/sugardb"
	"github.com/redis/go-redis/v9"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	"github.com/google/uuid"
)

const (
	pingTimeout = 5 * time.Second
	// defaultScanCount is the COUNT hint passed to SCAN. Servers treat it as
	// a hint, so pages may be smaller or slightly larger.
	defaultScanCount = 100
	// maxScanCount bounds one browser page so a huge keyspace cannot produce
	// an unbounded reply.
	maxScanCount = 500
	// maxValueItems caps list/set/zset/stream members in the value viewer.
	maxValueItems = 500
)

// Service manages registered Redis-protocol connections and their pooled
// go-redis clients. It mirrors the SQL databases service: metadata lives in
// the shared databases table, secrets in the vault, and every execution path
// resolves the connection by ID so credentials never leave the backend.
// Embedded SugarDB connections run an in-process engine behind the same
// client stack (see sugardb.go).
type Service struct {
	store    *persistence.Store
	vault    *security.Vault
	embedded *embeddedManager
	mu       sync.Mutex
	clients  map[string]*redis.Client
	closed   bool
}

// New creates a KV service. vault may be nil for unauthenticated local
// servers; password-protected connections require it to resolve password
// references. dataRoot anchors the default persistence directory of embedded
// SugarDB stores (<dataRoot>/sugardb/<connection id>).
func New(store *persistence.Store, vault *security.Vault, dataRoot string) *Service {
	return &Service{store: store, vault: vault, embedded: newEmbeddedManager(dataRoot), clients: make(map[string]*redis.Client)}
}

// Close releases every cached client and stops every embedded engine.
// Subsequent calls fail explicitly.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var first error
	for id, client := range s.clients {
		if err := client.Close(); err != nil && first == nil {
			first = fmt.Errorf("close kv connection %q: %w", id, err)
		}
	}
	clear(s.clients)
	s.embedded.close()
	return first
}

// List returns every registered key/value connection: remote Redis-protocol
// servers plus embedded SugarDB stores.
func (s *Service) List(ctx context.Context) ([]domain.Database, error) {
	items, err := s.store.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Database, 0)
	for _, item := range items {
		if domain.IsKVDriver(item.Driver) {
			result = append(result, item)
		}
	}
	return result, nil
}

// Get returns one registered connection by ID.
func (s *Service) Get(ctx context.Context, id string) (domain.Database, error) {
	item, err := s.store.GetDatabase(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.Database{}, err
	}
	if err := requireKVDriver(item); err != nil {
		return domain.Database{}, err
	}
	return item, nil
}

// Create registers a new KV connection, storing its password in the vault
// and verifying reachability with PING.
func (s *Service) Create(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	return s.register(ctx, request)
}

// Register records an existing KV server. Both paths are identical because
// Redis servers hold their own data; nothing is created locally.
func (s *Service) Register(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	return s.register(ctx, request)
}

func (s *Service) register(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := BuildDatabase(request)
	if err != nil {
		return domain.Database{}, err
	}
	if err := s.applyPassword(ctx, &item, request.Password); err != nil {
		return domain.Database{}, err
	}
	secret, err := s.resolveSecret(item.PasswordRef)
	if err != nil {
		return domain.Database{}, err
	}
	status := domain.DatabaseStatusUnverified
	if item.Driver != domain.DatabaseDriverSugarDB {
		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		if err := s.pingItem(pingCtx, item, secret); err == nil {
			status = domain.DatabaseStatusConnected
		}
		cancel()
	}
	created, err := s.store.CreateDatabase(ctx, item)
	if err != nil {
		_ = s.deleteSecret(item.PasswordRef)
		return domain.Database{}, err
	}
	if status == domain.DatabaseStatusConnected {
		if err := s.store.UpdateDatabaseStatus(ctx, created.ID, domain.DatabaseStatusConnected); err != nil {
			return domain.Database{}, err
		}
	}
	if item.Driver == domain.DatabaseDriverSugarDB {
		// The embedded engine needs the persisted ID before it can start,
		// so verification happens right after the row exists.
		if _, err := s.connection(ctx, created.ID); err == nil {
			if err := s.store.UpdateDatabaseStatus(ctx, created.ID, domain.DatabaseStatusConnected); err != nil {
				return domain.Database{}, err
			}
		} else if err := s.store.UpdateDatabaseStatus(ctx, created.ID, domain.DatabaseStatusError); err != nil {
			return domain.Database{}, err
		}
	}
	return s.store.GetDatabase(ctx, created.ID)
}

// Update replaces a connection's metadata, rotating the vault password when
// a new one is supplied, and drops any cached client so the next call dials
// with the new settings.
func (s *Service) Update(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := BuildDatabase(request)
	if err != nil {
		return domain.Database{}, err
	}
	item.ID = strings.TrimSpace(request.ID)
	if item.ID == "" {
		return domain.Database{}, fmt.Errorf("database ID is required")
	}
	stored, err := s.store.GetDatabase(ctx, item.ID)
	if err != nil {
		return domain.Database{}, err
	}
	if err := requireKVDriver(stored); err != nil {
		return domain.Database{}, err
	}
	if err := s.applyPassword(ctx, &item, request.Password); err != nil {
		return domain.Database{}, err
	}
	updated, err := s.store.UpdateDatabase(ctx, item)
	if err != nil {
		return domain.Database{}, err
	}
	if item.Driver == domain.DatabaseDriverSugarDB {
		// Engine-scoped settings require a restart; a DB index change only
		// needs a fresh client (the index is applied at dial time).
		if stored.Path != item.Path || stored.PasswordRef != item.PasswordRef {
			s.closeConnection(item.ID)
			s.embedded.stop(item.ID)
		} else if stored.DBIndex != item.DBIndex {
			s.closeConnection(item.ID)
		}
	} else if stored.Host != item.Host || stored.Port != item.Port || stored.Username != item.Username ||
		stored.PasswordRef != item.PasswordRef || stored.DBIndex != item.DBIndex || stored.UseTLS != item.UseTLS ||
		stored.ClientName != item.ClientName || stored.Address != item.Address {
		s.closeConnection(item.ID)
	}
	if _, err := s.connection(ctx, updated.ID); err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, updated.ID, domain.DatabaseStatusError)
		return domain.Database{}, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, updated.ID, domain.DatabaseStatusConnected); err != nil {
		return domain.Database{}, err
	}
	return s.store.GetDatabase(ctx, updated.ID)
}

// Delete removes the connection row, its cached client, its embedded engine
// (if any), and its vault secret. Persisted SugarDB files stay on disk so
// unregistering never destroys user data.
func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("database ID is required")
	}
	item, _ := s.store.GetDatabase(ctx, id)
	s.closeConnection(id)
	s.embedded.stop(id)
	if err := s.store.DeleteDatabase(ctx, id); err != nil {
		return err
	}
	_ = s.deleteSecret(item.PasswordRef)
	return nil
}

// Ping verifies the connection and persists the resulting status.
func (s *Service) Ping(ctx context.Context, id string) (domain.DatabaseStatus, error) {
	conn, err := s.connection(ctx, id)
	if err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := conn.client.Ping(pingCtx).Err(); err != nil {
		s.closeConnection(id)
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusConnected); err != nil {
		return domain.DatabaseStatusConnected, err
	}
	return domain.DatabaseStatusConnected, nil
}

// TestConnection probes the supplied configuration without persisting
// anything. Remote servers are dialled and pinged; embedded SugarDB stores
// verify their persistence directory is writable (the engine itself only
// starts once the connection has a persisted ID).
func (s *Service) TestConnection(ctx context.Context, item domain.Database, password string) (domain.DatabaseStatus, error) {
	if item.Driver == domain.DatabaseDriverSugarDB {
		if err := s.probeEmbeddedDir(item); err != nil {
			return domain.DatabaseStatusError, err
		}
		return domain.DatabaseStatusConnected, nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := s.pingItem(pingCtx, item, password); err != nil {
		return domain.DatabaseStatusError, err
	}
	return domain.DatabaseStatusConnected, nil
}

// probeEmbeddedDir creates (if needed) and writes into the SugarDB data
// directory to surface permission problems in the connection modal.
func (s *Service) probeEmbeddedDir(item domain.Database) error {
	dir := strings.TrimSpace(item.Path)
	if dir == "" {
		root := strings.TrimSpace(s.embedded.dataRoot)
		if root == "" {
			return fmt.Errorf("sugardb data directory requires an explicit path or an app data root")
		}
		dir = filepath.Join(root, "sugardb")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("prepare sugardb data directory: %w", err)
	}
	probe := filepath.Join(dir, ".neuropipe-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o644); err != nil {
		return fmt.Errorf("sugardb data directory is not writable: %w", err)
	}
	_ = os.Remove(probe)
	return nil
}

func (s *Service) pingItem(ctx context.Context, item domain.Database, secret string) error {
	dial, err := client(item, secret)
	if err != nil {
		return err
	}
	defer func() { _ = dial.Close() }()
	return dial.Ping(ctx).Err()
}

// ExecuteCommand runs one validated command on the node execution path. The
// denylist is enforced here regardless of what the editor allowed.
func (s *Service) ExecuteCommand(ctx context.Context, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	command := normalizeCommand(request.Command)
	if command == "" {
		return domain.KVCommandResult{}, fmt.Errorf("redis command is required")
	}
	if err := validateCommand(command, request.AllowDangerous); err != nil {
		return domain.KVCommandResult{}, err
	}
	conn, err := s.connection(ctx, request.DatabaseID)
	if err != nil {
		return domain.KVCommandResult{}, err
	}
	raw, err := conn.client.Do(ctx, commandArgs(command, request.Args)...).Result()
	if err != nil {
		if strings.EqualFold(command, "scan") && commandUnsupported(err) {
			// The embedded SugarDB engine has no SCAN; emulate it with KEYS
			// so KV Scan nodes and the console keep working unchanged.
			return s.emulatedScan(ctx, conn, request)
		}
		if errors.Is(err, redis.Nil) {
			return domain.KVCommandResult{IsNil: true}, nil
		}
		return domain.KVCommandResult{}, fmt.Errorf("redis %s: %w", command, err)
	}
	value, truncated := normalizeReply(raw)
	return domain.KVCommandResult{Value: value, Truncated: truncated}, nil
}

// emulatedScan answers a SCAN-shaped request against servers without SCAN by
// listing matching keys once and treating the cursor as an offset into the
// sorted result. The reply keeps the SCAN contract: [nextCursor, keys].
func (s *Service) emulatedScan(ctx context.Context, conn kvConn, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	var cursor uint64
	match := "*"
	count := defaultScanCount
	typeFilter := ""
	args := request.Args
	if len(args) > 0 {
		parsed, err := strconv.ParseUint(strings.TrimSpace(args[0]), 10, 64)
		if err != nil {
			return domain.KVCommandResult{}, fmt.Errorf("redis SCAN: invalid cursor %q", args[0])
		}
		cursor = parsed
	}
	for index := 1; index < len(args); index++ {
		option := strings.ToUpper(strings.TrimSpace(args[index]))
		switch option {
		case "MATCH", "COUNT", "TYPE":
			if index+1 >= len(args) {
				return domain.KVCommandResult{}, fmt.Errorf("redis SCAN: %s requires a value", option)
			}
			value := strings.TrimSpace(args[index+1])
			switch option {
			case "MATCH":
				if value != "" {
					match = value
				}
			case "COUNT":
				if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
					count = parsed
				}
			case "TYPE":
				typeFilter = value
			}
			index++
		default:
			return domain.KVCommandResult{}, fmt.Errorf("redis SCAN: unsupported option %q", args[index])
		}
	}
	if count > maxScanCount {
		count = maxScanCount
	}
	page, err := s.scanViaKeys(ctx, conn, domain.KVScanRequest{Cursor: cursor, Match: match, Type: typeFilter, Count: count}, count)
	if err != nil {
		return domain.KVCommandResult{}, err
	}
	keys := make([]any, len(page.Keys))
	for index, key := range page.Keys {
		keys[index] = key.Name
	}
	return domain.KVCommandResult{Value: []any{jsSafeInt(int64(page.NextCursor)), keys}}, nil
}

// Debug runs one command from the interactive console. The console may pass
// allowDangerous after an explicit user confirmation.
func (s *Service) Debug(ctx context.Context, id string, command string, args []string, allowDangerous bool) (domain.KVCommandResult, error) {
	return s.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: id, Command: command, Args: args, AllowDangerous: allowDangerous})
}

func commandArgs(command string, args []string) []any {
	result := make([]any, 0, len(args)+1)
	result = append(result, command)
	for _, arg := range args {
		result = append(result, arg)
	}
	return result
}

// ScanKeys returns one cursor-based SCAN page enriched with per-key type,
// TTL, and memory usage. Servers without SCAN (the embedded SugarDB engine)
// fall back to KEYS with offset-based pagination so the browser stays
// fully functional.
func (s *Service) ScanKeys(ctx context.Context, id string, request domain.KVScanRequest) (domain.KVKeyPage, error) {
	conn, err := s.connection(ctx, id)
	if err != nil {
		return domain.KVKeyPage{}, err
	}
	count := request.Count
	if count <= 0 {
		count = defaultScanCount
	}
	if count > maxScanCount {
		count = maxScanCount
	}
	keys, cursor, err := conn.client.Scan(ctx, request.Cursor, request.Match, int64(count)).Result()
	if err != nil {
		if !commandUnsupported(err) {
			return domain.KVKeyPage{}, fmt.Errorf("scan keys: %w", err)
		}
		return s.scanViaKeys(ctx, conn, request, count)
	}
	page := domain.KVKeyPage{Keys: s.enrichKeys(ctx, conn, keys, request.Type), NextCursor: cursor, TotalSeen: len(keys)}
	return page, nil
}

// commandUnsupported reports whether err is the RESP error a server returns
// for a command it does not implement (Redis says "unknown command", SugarDB
// says "command X not supported"). Used to pick client-side fallbacks.
func commandUnsupported(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unknown command") || strings.Contains(message, "not supported")
}

// scanViaKeys emulates cursor pagination over KEYS for servers without SCAN:
// the full match list is fetched once, sorted, and the cursor is interpreted
// as an offset into it. Keys added or removed between pages shift subsequent
// offsets by one position - the same eventual-consistency class of noise the
// SCAN contract permits.
func (s *Service) scanViaKeys(ctx context.Context, conn kvConn, request domain.KVScanRequest, count int) (domain.KVKeyPage, error) {
	pattern := strings.TrimSpace(request.Match)
	if pattern == "" {
		pattern = "*"
	}
	all, err := conn.client.Keys(ctx, pattern).Result()
	if err != nil {
		return domain.KVKeyPage{}, fmt.Errorf("scan keys: %w", err)
	}
	sort.Strings(all)
	offset := 0
	if request.Cursor <= uint64(len(all)) {
		offset = int(request.Cursor)
	}
	end := offset + count
	if end > len(all) {
		end = len(all)
	}
	keys := all[offset:end]
	next := uint64(0)
	if end < len(all) {
		next = uint64(end)
	}
	return domain.KVKeyPage{Keys: s.enrichKeys(ctx, conn, keys, request.Type), NextCursor: next, TotalSeen: len(keys)}, nil
}

// enrichKeys collects TYPE, TTL, and (where supported) MEMORY USAGE for the
// page's keys and applies the optional type filter. Individual command
// failures leave the corresponding field zero-valued instead of failing the
// page. Remote servers use a pipeline; the embedded SugarDB engine gets
// sequential commands because its TCP layer only parses one command per
// segment - pipelined commands after the first would be dropped.
func (s *Service) enrichKeys(ctx context.Context, conn kvConn, keys []string, typeFilter string) []domain.KVKey {
	enriched := make([]domain.KVKey, 0, len(keys))
	if len(keys) == 0 {
		return enriched
	}
	// The embedded SugarDB engine has no MEMORY USAGE command; skip the
	// probe instead of failing the pipeline that carries it.
	includeSize := conn.item.Driver != domain.DatabaseDriverSugarDB
	typeCmds := make([]*redis.StatusCmd, len(keys))
	ttlCmds := make([]*redis.DurationCmd, len(keys))
	sizeCmds := make([]*redis.IntCmd, len(keys))
	if includeSize {
		pipeliner := conn.client.Pipeline()
		for index, key := range keys {
			typeCmds[index] = pipeliner.Type(ctx, key)
			ttlCmds[index] = pipeliner.TTL(ctx, key)
			sizeCmds[index] = pipeliner.MemoryUsage(ctx, key)
		}
		if _, err := pipeliner.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return enriched
		}
	} else {
		for index, key := range keys {
			typeCmds[index] = conn.client.Type(ctx, key)
			ttlCmds[index] = conn.client.TTL(ctx, key)
		}
	}
	for index, key := range keys {
		keyType := ""
		if typeCmds[index] != nil {
			keyType = typeCmds[index].Val()
		}
		if typeFilter != "" && !strings.EqualFold(keyType, typeFilter) {
			continue
		}
		entry := domain.KVKey{Name: key, Type: keyType, TTL: -1}
		if ttlCmds[index] != nil {
			if duration, err := ttlCmds[index].Result(); err == nil {
				entry.TTL = int64(duration.Seconds())
				if duration == -1 {
					entry.TTL = -1
				} else if duration == -2 {
					entry.TTL = -2
				} else if entry.TTL < 1 {
					entry.TTL = 1
				}
			}
		}
		if sizeCmds[index] != nil {
			if size, err := sizeCmds[index].Result(); err == nil {
				entry.Size = size
			}
		}
		enriched = append(enriched, entry)
	}
	return enriched
}

// KeyValue loads one key's value for the browser with per-type shaping and
// truncation. A missing key reports Type "none" rather than an error.
func (s *Service) KeyValue(ctx context.Context, id string, key string) (domain.KVKeyValue, error) {
	conn, err := s.connection(ctx, id)
	if err != nil {
		return domain.KVKeyValue{}, err
	}
	client := conn.client
	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		return domain.KVKeyValue{}, fmt.Errorf("read key type: %w", err)
	}
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		ttl = -1
	}
	value := domain.KVKeyValue{Type: keyType, TTL: int64(ttl.Seconds())}
	if ttl == -1 {
		value.TTL = -1
	} else if ttl == -2 {
		value.TTL = -2
	}
	switch keyType {
	case "none":
		value.Value = nil
	case "string":
		text, err := client.Get(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			value.Value = nil
			break
		}
		if err != nil {
			return domain.KVKeyValue{}, fmt.Errorf("read string value: %w", err)
		}
		normalized, truncated := truncateString(text)
		value.Value, value.Truncated = normalized, truncated
	case "hash":
		fields, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			return domain.KVKeyValue{}, fmt.Errorf("read hash value: %w", err)
		}
		result := make(map[string]string, len(fields))
		count := 0
		for field, item := range fields {
			if count >= maxValueItems {
				value.Truncated = true
				break
			}
			result[field] = item
			count++
		}
		value.Value = result
	case "list":
		items, err := client.LRange(ctx, key, 0, int64(maxValueItems)).Result()
		if err != nil {
			return domain.KVKeyValue{}, fmt.Errorf("read list value: %w", err)
		}
		if len(items) > maxValueItems {
			items = items[:maxValueItems]
			value.Truncated = true
		}
		value.Value = items
	case "set":
		members, err := client.SMembers(ctx, key).Result()
		if err != nil {
			return domain.KVKeyValue{}, fmt.Errorf("read set value: %w", err)
		}
		if len(members) > maxValueItems {
			sort.Strings(members)
			members = members[:maxValueItems]
			value.Truncated = true
		}
		value.Value = members
	case "zset":
		entries, err := client.ZRangeWithScores(ctx, key, 0, int64(maxValueItems)).Result()
		if err != nil {
			return domain.KVKeyValue{}, fmt.Errorf("read sorted set value: %w", err)
		}
		if len(entries) > maxValueItems {
			entries = entries[:maxValueItems]
			value.Truncated = true
		}
		result := make([]map[string]any, len(entries))
		for index, entry := range entries {
			result[index] = map[string]any{"member": entry.Member, "score": jsSafeInt(int64(entry.Score))}
		}
		value.Value = result
	case "stream":
		entries, err := client.XRange(ctx, key, "-", "+").Result()
		if err != nil {
			return domain.KVKeyValue{}, fmt.Errorf("read stream value: %w", err)
		}
		if len(entries) > maxValueItems {
			entries = entries[:maxValueItems]
			value.Truncated = true
		}
		result := make([]map[string]any, len(entries))
		for index, entry := range entries {
			fields := make(map[string]any, len(entry.Values))
			for field, item := range entry.Values {
				fields[field] = item
			}
			result[index] = map[string]any{"id": entry.ID, "fields": fields}
		}
		value.Value = result
	default:
		value.Value = nil
	}
	return value, nil
}

// DeleteKeys removes keys from the browser and reports how many existed.
func (s *Service) DeleteKeys(ctx context.Context, id string, keys []string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	conn, err := s.connection(ctx, id)
	if err != nil {
		return 0, err
	}
	deleted, err := conn.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("delete keys: %w", err)
	}
	return deleted, nil
}

// SetTTL applies a new expiry in seconds. A negative ttl persists the key.
func (s *Service) SetTTL(ctx context.Context, id string, key string, ttlSeconds int64) error {
	conn, err := s.connection(ctx, id)
	if err != nil {
		return err
	}
	client := conn.client
	if ttlSeconds < 0 {
		if err := client.Persist(ctx, key).Err(); err != nil {
			return fmt.Errorf("persist key: %w", err)
		}
		return nil
	}
	if err := client.Expire(ctx, key, time.Duration(ttlSeconds)*time.Second).Err(); err != nil {
		return fmt.Errorf("set key ttl: %w", err)
	}
	return nil
}

// Info summarises the server for the browser Info tab. Remote servers are
// read through the INFO command (missing fields stay zero-valued because
// flavours differ); the embedded SugarDB engine is queried directly since it
// has no INFO command.
func (s *Service) Info(ctx context.Context, id string) (domain.KVServerInfo, error) {
	conn, err := s.connection(ctx, id)
	if err != nil {
		return domain.KVServerInfo{}, err
	}
	if engine := s.embedded.server(id); engine != nil {
		return s.embeddedInfo(ctx, conn, engine), nil
	}
	client := conn.client
	info := domain.KVServerInfo{Flavor: "redis"}
	fullText, err := client.Info(ctx).Result()
	if err != nil {
		return domain.KVServerInfo{}, fmt.Errorf("read server info: %w", err)
	}
	serverText := fullText
	if value, ok := infoValue(serverText, "redis_version"); ok {
		info.Version = value
	}
	if strings.Contains(serverText, "valkey_version:") {
		info.Flavor = "valkey"
		if value, ok := infoValue(serverText, "valkey_version"); ok {
			info.Version = value
		}
	}
	if value, ok := infoValue(fullText, "uptime_in_seconds"); ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			info.UptimeSeconds = parsed
		}
	}
	if value, ok := infoValue(fullText, "connected_clients"); ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			info.ConnectedClients = parsed
		}
	}
	if value, ok := infoValue(fullText, "used_memory"); ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			info.UsedMemory = parsed
		}
	}
	if value, ok := infoValue(fullText, "used_memory_human"); ok {
		info.UsedMemoryHuman = value
	}
	if total, err := client.DBSize(ctx).Result(); err == nil {
		info.TotalKeys = total
	}
	info.Databases = parseKeyspace(fullText)
	return info, nil
}

// embeddedInfo builds the browser summary for a running embedded SugarDB
// engine from its native server info plus a DBSIZE probe.
func (s *Service) embeddedInfo(ctx context.Context, conn kvConn, engine *sugardblib.SugarDB) domain.KVServerInfo {
	details := engine.GetServerInfo()
	info := domain.KVServerInfo{
		Flavor:     "sugardb",
		Version:    details.Version,
		UsedMemory: details.MemoryUsed,
	}
	if total, err := conn.client.DBSize(ctx).Result(); err == nil {
		info.TotalKeys = total
		info.Databases = []domain.KVDatabaseInfo{{Index: conn.item.DBIndex, Keys: total}}
	}
	return info
}

// Subscribe opens one dedicated pub/sub subscription. The KV subscribe
// trigger service consumes it; nothing here touches pipeline execution.
func (s *Service) Subscribe(ctx context.Context, id string, channels, patterns []string) (Subscription, error) {
	conn, err := s.connection(ctx, id)
	if err != nil {
		return nil, err
	}
	var pubsub *redis.PubSub
	if len(patterns) > 0 {
		pubsub = conn.client.PSubscribe(ctx, patterns...)
	} else {
		pubsub = conn.client.Subscribe(ctx, channels...)
	}
	return &redisSubscription{pubsub: pubsub}, nil
}

// PubSubMessage is one delivered pub/sub notification.
type PubSubMessage struct {
	Channel string
	Pattern string
	Payload string
}

// Subscription is the narrow receive loop contract consumed by the KV
// subscribe trigger service.
type Subscription interface {
	Receive(ctx context.Context) (PubSubMessage, error)
	Close() error
}

type redisSubscription struct {
	pubsub *redis.PubSub
}

func (s *redisSubscription) Receive(ctx context.Context) (PubSubMessage, error) {
	message, err := s.pubsub.ReceiveMessage(ctx)
	if err != nil {
		return PubSubMessage{}, err
	}
	return PubSubMessage{Channel: message.Channel, Pattern: message.Pattern, Payload: message.Payload}, nil
}

func (s *redisSubscription) Close() error { return s.pubsub.Close() }

// kvConn bundles the live client with its registered metadata so callers can
// branch on the driver: embedded SugarDB stores skip commands the engine
// lacks (MEMORY USAGE) and read server info through the native API.
type kvConn struct {
	client *redis.Client
	item   domain.Database
}

// connection returns the cached client for id, opening one on first use.
// Remote Redis-protocol servers dial their configured address; embedded
// SugarDB stores start their in-process engine first and connect to its
// loopback listener.
func (s *Service) connection(ctx context.Context, id string) (kvConn, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return kvConn{}, fmt.Errorf("database ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return kvConn{}, fmt.Errorf("kv service is closed")
	}
	if cached := s.clients[id]; cached != nil {
		item, err := s.store.GetDatabase(ctx, id)
		if err != nil {
			return kvConn{}, err
		}
		return kvConn{client: cached, item: item}, nil
	}
	item, err := s.store.GetDatabase(ctx, id)
	if err != nil {
		return kvConn{}, err
	}
	if err := requireKVDriver(item); err != nil {
		return kvConn{}, err
	}
	secret, err := s.resolveSecret(item.PasswordRef)
	if err != nil {
		return kvConn{}, err
	}
	var opened *redis.Client
	if item.Driver == domain.DatabaseDriverSugarDB {
		opened, err = s.embeddedClient(ctx, item, secret)
	} else {
		opened, err = client(item, secret)
	}
	if err != nil {
		return kvConn{}, fmt.Errorf("open kv connection: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := opened.Ping(pingCtx).Err(); err != nil {
		_ = opened.Close()
		return kvConn{}, fmt.Errorf("open kv connection: %w", err)
	}
	s.clients[id] = opened
	return kvConn{client: opened, item: item}, nil
}

// embeddedClient starts (or reuses) the engine for item and returns a client
// bound to its loopback listener. The caller must hold s.mu.
func (s *Service) embeddedClient(_ context.Context, item domain.Database, secret string) (*redis.Client, error) {
	addr, err := s.embedded.start(item, secret, false)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(embeddedOptions(addr, item, secret)), nil
}

func (s *Service) closeConnection(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached := s.clients[id]; cached != nil {
		_ = cached.Close()
		delete(s.clients, id)
	}
}

// resolveSecret returns the plaintext password stored under ref. An empty
// ref or a missing vault yields the empty string so unauthenticated local
// servers still work.
func (s *Service) resolveSecret(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || s.vault == nil {
		return "", nil
	}
	secret, err := s.vault.Get(ref)
	if err != nil {
		return "", fmt.Errorf("load kv password: %w", err)
	}
	return secret, nil
}

// applyPassword writes a new password (if any) to the vault and updates
// item.PasswordRef. An empty password preserves the existing ref.
func (s *Service) applyPassword(_ context.Context, item *domain.Database, password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil
	}
	if s.vault == nil {
		return fmt.Errorf("kv password storage is unavailable")
	}
	ref := strings.TrimSpace(item.PasswordRef)
	if ref == "" {
		ref = "kvpw:" + uuid.NewString()
		item.PasswordRef = ref
	}
	if err := s.vault.Put(ref, password); err != nil {
		return fmt.Errorf("store kv password: %w", err)
	}
	return nil
}

func (s *Service) deleteSecret(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || s.vault == nil {
		return nil
	}
	return s.vault.Delete(ref)
}

// BuildDatabase validates a save request and returns the persistable item.
func BuildDatabase(request domain.SaveDatabaseRequest) (domain.Database, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return domain.Database{}, fmt.Errorf("database name is required")
	}
	switch request.Driver {
	case domain.DatabaseDriverSugarDB:
		return buildSugarDatabase(request, name)
	case domain.DatabaseDriverRedis:
		return buildRedisDatabase(request, name)
	default:
		return domain.Database{}, fmt.Errorf("unsupported kv database driver %q", request.Driver)
	}
}

// buildSugarDatabase validates an embedded SugarDB connection: no host,
// port, or URL applies - the engine runs inside the app - while Path selects
// the persistence directory and DBIndex namespaces keys.
func buildSugarDatabase(request domain.SaveDatabaseRequest, name string) (domain.Database, error) {
	if strings.TrimSpace(request.Address) != "" || strings.TrimSpace(request.Host) != "" {
		return domain.Database{}, fmt.Errorf("embedded sugardb connections do not use a host or connection URL; set a data directory instead")
	}
	item := domain.Database{
		Name:        name,
		Driver:      domain.DatabaseDriverSugarDB,
		Path:        strings.TrimSpace(request.Path),
		PasswordRef: strings.TrimSpace(request.PasswordRef),
		DBIndex:     request.DBIndex,
	}
	if item.DBIndex < 0 || item.DBIndex > 4096 {
		return domain.Database{}, fmt.Errorf("sugardb database index must be between 0 and 4096")
	}
	return item, nil
}

func buildRedisDatabase(request domain.SaveDatabaseRequest, name string) (domain.Database, error) {
	address := strings.TrimSpace(request.Address)
	item := domain.Database{
		Name:        name,
		Driver:      domain.DatabaseDriverRedis,
		Host:        strings.TrimSpace(request.Host),
		Port:        request.Port,
		Username:    strings.TrimSpace(request.Username),
		PasswordRef: strings.TrimSpace(request.PasswordRef),
		DBIndex:     request.DBIndex,
		UseTLS:      request.UseTLS,
		ClientName:  strings.TrimSpace(request.ClientName),
		Address:     address,
	}
	if address != "" {
		// Validate the URL form eagerly so save failures happen in the modal,
		// not on first use.
		if _, err := options(item, ""); err != nil {
			return domain.Database{}, err
		}
	} else {
		if item.Host == "" {
			return domain.Database{}, fmt.Errorf("redis host is required")
		}
		if item.Port == 0 {
			item.Port = 6379
		}
	}
	if item.DBIndex < 0 || item.DBIndex > 4096 {
		return domain.Database{}, fmt.Errorf("redis database index must be between 0 and 4096")
	}
	return item, nil
}

// requireKVDriver rejects non-key/value rows reaching the KV service.
func requireKVDriver(item domain.Database) error {
	if !domain.IsKVDriver(item.Driver) {
		return fmt.Errorf("database %q is not a key/value connection", item.Name)
	}
	return nil
}

func infoValue(section, key string) (string, bool) {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if name, value, found := strings.Cut(line, ":"); found && name == key {
			return value, true
		}
	}
	return "", false
}

// parseKeyspace reads "db0:keys=12,expires=3" lines into structured entries.
func parseKeyspace(section string) []domain.KVDatabaseInfo {
	result := make([]domain.KVDatabaseInfo, 0)
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, rest, found := strings.Cut(line, ":")
		if !found || !strings.HasPrefix(name, "db") {
			continue
		}
		index, err := strconv.Atoi(strings.TrimPrefix(name, "db"))
		if err != nil {
			continue
		}
		entry := domain.KVDatabaseInfo{Index: index}
		for _, pair := range strings.Split(rest, ",") {
			key, value, found := strings.Cut(pair, "=")
			if !found || key != "keys" {
				continue
			}
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				entry.Keys = parsed
			}
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result
}
