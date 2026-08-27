package kv

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

// newEmbeddedService returns a KV service whose embedded stores persist under
// a throwaway app-data root.
func newEmbeddedService(t *testing.T) *Service {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := New(store, nil, t.TempDir())
	t.Cleanup(func() { _ = service.Close() })
	return service
}

// registerEmbedded saves an embedded SugarDB connection, optionally anchored
// to an explicit data directory.
func registerEmbedded(t *testing.T, service *Service, name string, path string) domain.Database {
	t.Helper()
	request := domain.SaveDatabaseRequest{
		Name: name, Driver: domain.DatabaseDriverSugarDB, Path: path,
	}
	item, err := service.Register(context.Background(), request)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return item
}

func embeddedCommand(t *testing.T, service *Service, id string, command string, args ...string) domain.KVCommandResult {
	t.Helper()
	result, err := service.ExecuteCommand(context.Background(), domain.KVCommandRequest{
		DatabaseID: id, Command: command, Args: args,
	})
	if err != nil {
		t.Fatalf("ExecuteCommand(%s) error = %v", command, err)
	}
	return result
}

func TestEmbeddedRegisterPingList(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "Local store", "")
	if item.Driver != domain.DatabaseDriverSugarDB {
		t.Fatalf("registered item driver = %q", item.Driver)
	}
	status, err := service.Ping(context.Background(), item.ID)
	if err != nil || status != domain.DatabaseStatusConnected {
		t.Fatalf("Ping() = %v, %v", status, err)
	}
	list, err := service.List(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != item.ID {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	if list[0].Status != domain.DatabaseStatusConnected {
		t.Fatalf("status after register = %q", list[0].Status)
	}
}

func TestEmbeddedStringCommands(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "Strings", "")

	result := embeddedCommand(t, service, item.ID, "SET", "greeting", "hello sugardb")
	if result.IsNil {
		t.Fatal("SET reported nil")
	}
	result = embeddedCommand(t, service, item.ID, "GET", "greeting")
	if result.Value != "hello sugardb" {
		t.Fatalf("GET greeting = %#v", result.Value)
	}
	result = embeddedCommand(t, service, item.ID, "INCR", "counter")
	if result.Value != int64(1) {
		t.Fatalf("INCR counter = %#v (expected 1)", result.Value)
	}
	result = embeddedCommand(t, service, item.ID, "GET", "missing")
	if !result.IsNil {
		t.Fatalf("GET missing = %#v (expected nil)", result.Value)
	}
}

func TestEmbeddedStructures(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "Structures", "")

	embeddedCommand(t, service, item.ID, "HSET", "user:1", "name", "Ada", "role", "admin")
	value, err := service.KeyValue(context.Background(), item.ID, "user:1")
	if err != nil {
		t.Fatalf("KeyValue() error = %v", err)
	}
	if value.Type != "hash" {
		t.Fatalf("hash key type = %q", value.Type)
	}
	fields, ok := value.Value.(map[string]string)
	if !ok || fields["name"] != "Ada" || fields["role"] != "admin" {
		t.Fatalf("hash value = %#v", value.Value)
	}

	embeddedCommand(t, service, item.ID, "LPUSH", "queue", "a", "b", "c")
	value, err = service.KeyValue(context.Background(), item.ID, "queue")
	if err != nil {
		t.Fatalf("KeyValue(list) error = %v", err)
	}
	if value.Type != "list" {
		t.Fatalf("list key type = %q", value.Type)
	}
	items, ok := value.Value.([]string)
	if !ok || len(items) != 3 {
		t.Fatalf("list value = %#v", value.Value)
	}

	embeddedCommand(t, service, item.ID, "SADD", "tags", "x", "y")
	embeddedCommand(t, service, item.ID, "ZADD", "rank", "10", "ten", "20", "twenty")
	value, err = service.KeyValue(context.Background(), item.ID, "rank")
	if err != nil {
		t.Fatalf("KeyValue(zset) error = %v", err)
	}
	if value.Type != "zset" {
		t.Fatalf("zset key type = %q", value.Type)
	}
}

func TestEmbeddedScanFallback(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "Scan", "")

	for i := 0; i < 7; i++ {
		embeddedCommand(t, service, item.ID, "SET", "key:"+string(rune('a'+i)), "v")
	}

	// First page of three; SugarDB has no SCAN so the service falls back to
	// KEYS with offset-based pagination.
	page, err := service.ScanKeys(context.Background(), item.ID, domain.KVScanRequest{Count: 3})
	if err != nil {
		t.Fatalf("ScanKeys() error = %v", err)
	}
	if len(page.Keys) != 3 || page.Keys[0].Name != "key:a" {
		t.Fatalf("first page = %#v", page.Keys)
	}
	if page.NextCursor == 0 {
		t.Fatal("expected another page")
	}
	seen := map[string]bool{}
	for _, key := range page.Keys {
		seen[key.Name] = true
		if key.Type != "string" || key.TTL != -1 {
			t.Fatalf("enriched key = %#v", key)
		}
	}

	// Walk the remaining pages.
	cursor := page.NextCursor
	pages := 1
	for cursor != 0 && pages < 5 {
		page, err = service.ScanKeys(context.Background(), item.ID, domain.KVScanRequest{Cursor: cursor, Count: 3})
		if err != nil {
			t.Fatalf("ScanKeys(cursor=%d) error = %v", cursor, err)
		}
		for _, key := range page.Keys {
			if seen[key.Name] {
				t.Fatalf("duplicate key %q across pages", key.Name)
			}
			seen[key.Name] = true
		}
		cursor = page.NextCursor
		pages++
	}
	if len(seen) != 7 {
		t.Fatalf("walked %d unique keys, want 7", len(seen))
	}

	// Pattern matching works through the same fallback.
	page, err = service.ScanKeys(context.Background(), item.ID, domain.KVScanRequest{Match: "key:[ab]", Count: 100})
	if err != nil {
		t.Fatalf("ScanKeys(match) error = %v", err)
	}
	if len(page.Keys) != 2 || page.NextCursor != 0 {
		t.Fatalf("matched page = %#v next=%d", page.Keys, page.NextCursor)
	}
}

func TestEmbeddedTTLAndDelete(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "TTLs", "")

	embeddedCommand(t, service, item.ID, "SET", "fleeting", "value")
	if err := service.SetTTL(context.Background(), item.ID, "fleeting", 90); err != nil {
		t.Fatalf("SetTTL() error = %v", err)
	}
	value, err := service.KeyValue(context.Background(), item.ID, "fleeting")
	if err != nil {
		t.Fatalf("KeyValue() error = %v", err)
	}
	if value.TTL < 1 || value.TTL > 90 {
		t.Fatalf("ttl after SetTTL = %d", value.TTL)
	}
	if err := service.SetTTL(context.Background(), item.ID, "fleeting", -1); err != nil {
		t.Fatalf("SetTTL(persist) error = %v", err)
	}
	value, err = service.KeyValue(context.Background(), item.ID, "fleeting")
	if err != nil {
		t.Fatalf("KeyValue() error = %v", err)
	}
	if value.TTL != -1 {
		t.Fatalf("ttl after persist = %d", value.TTL)
	}
	deleted, err := service.DeleteKeys(context.Background(), item.ID, []string{"fleeting"})
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteKeys() = %d, %v", deleted, err)
	}
}

func TestEmbeddedInfo(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "Info", "")
	embeddedCommand(t, service, item.ID, "SET", "k", "v")

	info, err := service.Info(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.Flavor != "sugardb" {
		t.Fatalf("flavor = %q", info.Flavor)
	}
	if info.Version == "" {
		t.Fatal("expected engine version in info")
	}
	if info.TotalKeys != 1 {
		t.Fatalf("total keys = %d", info.TotalKeys)
	}
	if len(info.Databases) != 1 || info.Databases[0].Keys != 1 {
		t.Fatalf("databases = %#v", info.Databases)
	}
}

func TestEmbeddedPubSubRoundTrip(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "PubSub", "")

	subscription, err := service.Subscribe(context.Background(), item.ID, []string{"events"}, nil)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer func() { _ = subscription.Close() }()

	// The subscription registers asynchronously once the server has read the
	// SUBSCRIBE command; wait for PUBSUB NUMSUB to observe it so the publish
	// below cannot race ahead of the registration. NUMSUB replies as a flat
	// array of [channel, count] pairs.
	counts := func() int {
		result, err := service.ExecuteCommand(context.Background(), domain.KVCommandRequest{
			DatabaseID: item.ID, Command: "PUBSUB", Args: []string{"NUMSUB", "events"},
		})
		if err != nil {
			t.Fatalf("PUBSUB NUMSUB error = %v", err)
		}
		pairs, ok := result.Value.([]any)
		if !ok {
			return 0
		}
		for _, pair := range pairs {
			entry, ok := pair.([]any)
			if !ok || len(entry) < 2 {
				continue
			}
			if entry[0] != "events" {
				continue
			}
			switch count := entry[1].(type) {
			case int:
				return count
			case int64:
				return int(count)
			}
		}
		return 0
	}
	ready := false
	for attempt := 0; attempt < 100 && !ready; attempt++ {
		if counts() >= 1 {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("subscription never became active")
	}

	embeddedCommand(t, service, item.ID, "PUBLISH", "events", "payload-1")

	receiveCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	message, err := subscription.Receive(receiveCtx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if message.Channel != "events" || message.Payload != "payload-1" {
		t.Fatalf("message = %#v", message)
	}
}

func TestEmbeddedRestartKeepsData(t *testing.T) {
	service := newEmbeddedService(t)
	dir := t.TempDir()
	item := registerEmbedded(t, service, "Durable", dir)
	embeddedCommand(t, service, item.ID, "SET", "persisted", "yes")

	// Simulate a configuration change: the engine restarts on next use.
	service.embedded.stop(item.ID)
	service.closeConnection(item.ID)

	result := embeddedCommand(t, service, item.ID, "GET", "persisted")
	if result.Value != "yes" {
		t.Fatalf("value after engine restart = %#v (AOF restore should keep it)", result.Value)
	}
}

func TestEmbeddedRestartAppliesNewSettings(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "Editable", "")
	embeddedCommand(t, service, item.ID, "SET", "before", "1")

	updated, err := service.Update(context.Background(), domain.SaveDatabaseRequest{
		ID: item.ID, Name: "Edited", Driver: domain.DatabaseDriverSugarDB, Path: t.TempDir(), DBIndex: 1,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Path == "" {
		t.Fatal("expected persisted path on updated item")
	}
	// The new logical database index starts empty.
	result := embeddedCommand(t, service, item.ID, "GET", "before")
	if !result.IsNil {
		t.Fatalf("key from db 0 leaked into db 1: %#v", result.Value)
	}
}

func TestEmbeddedTestConnectionDirectoryProbe(t *testing.T) {
	service := newEmbeddedService(t)

	item, err := BuildDatabase(domain.SaveDatabaseRequest{
		Name: "Probe", Driver: domain.DatabaseDriverSugarDB, Path: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildDatabase() error = %v", err)
	}
	status, err := service.TestConnection(context.Background(), item, "")
	if err != nil || status != domain.DatabaseStatusConnected {
		t.Fatalf("TestConnection(valid dir) = %v, %v", status, err)
	}

	// A path routed through a regular file can never become a directory on
	// any platform, so preparing the data directory must fail (the Windows
	// CI runners turn a hardcoded "/proc/..." path into a perfectly creatable
	// "C:\proc\..." path, which is why the probe is staged inside TempDir).
	notADir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notADir, []byte("file"), 0o644); err != nil {
		t.Fatalf("stage file: %v", err)
	}
	item.Path = filepath.Join(notADir, "sugardb")
	if _, err := service.TestConnection(context.Background(), item, ""); err == nil || !strings.Contains(err.Error(), "prepare sugardb data directory") {
		t.Fatalf("TestConnection(path through file) error = %v, want prepare failure", err)
	}

	// A directory occupying the probe file name makes the write probe itself
	// fail on every platform (EISDIR on Unix, access denied on Windows).
	occupied := t.TempDir()
	if err := os.Mkdir(filepath.Join(occupied, ".neuropipe-probe"), 0o755); err != nil {
		t.Fatalf("stage probe dir: %v", err)
	}
	item.Path = occupied
	if _, err := service.TestConnection(context.Background(), item, ""); err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("TestConnection(occupied probe) error = %v, want not-writable failure", err)
	}

	// The plain permission case only binds on Unix and outside root: a
	// read-only directory must surface as a not-writable rejection.
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		locked := t.TempDir()
		if err := os.Chmod(locked, 0o500); err != nil {
			t.Fatalf("chmod locked dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		item.Path = locked
		if _, err := service.TestConnection(context.Background(), item, ""); err == nil || !strings.Contains(err.Error(), "not writable") {
			t.Fatalf("TestConnection(0500 dir) error = %v, want not-writable failure", err)
		}
	}
}

func TestBuildSugarDatabaseValidation(t *testing.T) {
	valid, err := BuildDatabase(domain.SaveDatabaseRequest{
		Name: "Valid", Driver: domain.DatabaseDriverSugarDB, DBIndex: 3,
	})
	if err != nil {
		t.Fatalf("BuildDatabase() error = %v", err)
	}
	if valid.Path != "" || valid.DBIndex != 3 || valid.Host != "" {
		t.Fatalf("built item = %#v", valid)
	}

	if _, err := BuildDatabase(domain.SaveDatabaseRequest{
		Name: "Bad", Driver: domain.DatabaseDriverSugarDB, Host: "example.com",
	}); err == nil || !strings.Contains(err.Error(), "data directory") {
		t.Fatalf("host rejection error = %v", err)
	}

	if _, err := BuildDatabase(domain.SaveDatabaseRequest{
		Name: "Bad", Driver: domain.DatabaseDriverSugarDB, DBIndex: 5000,
	}); err == nil {
		t.Fatal("expected out-of-range database index to fail")
	}

	if _, err := BuildDatabase(domain.SaveDatabaseRequest{
		Name: "Bad", Driver: "mongodb",
	}); err == nil {
		t.Fatal("expected unknown driver to fail")
	}
}

func TestEmbeddedDeleteKeepsFiles(t *testing.T) {
	service := newEmbeddedService(t)
	dir := t.TempDir()
	item := registerEmbedded(t, service, "DoNotDelete", dir)
	embeddedCommand(t, service, item.ID, "SET", "keep", "me")

	if err := service.Delete(context.Background(), item.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Info(context.Background(), item.ID); err == nil {
		t.Fatal("expected Info to fail after delete")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("stat data dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("unregistering should keep the persisted data files")
	}
}

func TestEmbeddedScanCommandEmulation(t *testing.T) {
	service := newEmbeddedService(t)
	item := registerEmbedded(t, service, "ScanCommand", "")

	for i := 0; i < 5; i++ {
		embeddedCommand(t, service, item.ID, "SET", "scan:"+string(rune('a'+i)), "v")
	}

	// The KV Scan node and the console issue raw SCAN commands; the service
	// must emulate the [cursor, keys] reply shape over KEYS.
	result := embeddedCommand(t, service, item.ID, "SCAN", "0", "MATCH", "scan:*", "COUNT", "2")
	pair, ok := result.Value.([]any)
	if !ok || len(pair) != 2 {
		t.Fatalf("scan reply = %#v (expected [cursor, keys])", result.Value)
	}
	next, ok := pair[0].(int64)
	if !ok {
		t.Fatalf("emulated cursor = %#v", pair[0])
	}
	keys, ok := pair[1].([]any)
	if !ok || len(keys) != 2 || keys[0] != "scan:a" {
		t.Fatalf("first scan page = %#v", pair[1])
	}
	if next == 0 {
		t.Fatal("expected another page")
	}

	seen := map[string]bool{}
	for _, key := range keys {
		seen[key.(string)] = true
	}
	cursor := next
	for cursor != 0 {
		result = embeddedCommand(t, service, item.ID, "SCAN", strconv.FormatInt(cursor, 10), "MATCH", "scan:*", "COUNT", "2")
		pair = result.Value.([]any)
		keys = pair[1].([]any)
		for _, key := range keys {
			name := key.(string)
			if seen[name] {
				t.Fatalf("duplicate key %q across pages", name)
			}
			seen[name] = true
		}
		cursor = pair[0].(int64)
	}
	if len(seen) != 5 {
		t.Fatalf("walked %d keys, want 5", len(seen))
	}

	if _, err := service.ExecuteCommand(context.Background(), domain.KVCommandRequest{
		DatabaseID: item.ID, Command: "SCAN", Args: []string{"not-a-cursor"},
	}); err == nil {
		t.Fatal("expected invalid cursor to fail")
	}
	if _, err := service.ExecuteCommand(context.Background(), domain.KVCommandRequest{
		DatabaseID: item.ID, Command: "SCAN", Args: []string{"0", "BOGUS", "x"},
	}); err == nil {
		t.Fatal("expected unknown SCAN option to fail")
	}
}

func TestEmbeddedIsolationBetweenStores(t *testing.T) {
	service := newEmbeddedService(t)
	first := registerEmbedded(t, service, "First", "")
	second := registerEmbedded(t, service, "Second", "")

	embeddedCommand(t, service, first.ID, "SET", "owner", "first")
	embeddedCommand(t, service, second.ID, "SET", "owner", "second")

	if got := embeddedCommand(t, service, first.ID, "GET", "owner").Value; got != "first" {
		t.Fatalf("first store owner = %#v", got)
	}
	if got := embeddedCommand(t, service, second.ID, "GET", "owner").Value; got != "second" {
		t.Fatalf("second store owner = %#v", got)
	}
}
