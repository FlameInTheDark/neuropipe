package kv

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/alicebob/miniredis/v2"
)

func newTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.NewMiniRedis()
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := New(store, nil, t.TempDir())
	t.Cleanup(func() { _ = service.Close() })
	return service, server
}

// register connects the service to a miniredis instance.
func register(t *testing.T, service *Service, server *miniredis.Miniredis, name string) domain.Database {
	t.Helper()
	port, err := strconv.Atoi(server.Port())
	if err != nil {
		t.Fatal(err)
	}
	request := domain.SaveDatabaseRequest{
		Name: name, Driver: domain.DatabaseDriverRedis,
		Host: server.Host(), Port: port, DBIndex: 2,
	}
	item, err := service.Register(context.Background(), request)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return item
}

func TestRegisterPingAndList(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	if item.Driver != domain.DatabaseDriverRedis || item.DBIndex != 2 {
		t.Fatalf("registered item = %#v", item)
	}
	status, err := service.Ping(context.Background(), item.ID)
	if err != nil || status != domain.DatabaseStatusConnected {
		t.Fatalf("Ping() = %v, %v", status, err)
	}
	list, err := service.List(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != item.ID {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	if err := service.Delete(context.Background(), item.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.List(context.Background()); err != nil || len(list) != 1 {
		t.Fatalf("List() after delete = %#v, %v", list, err)
	}
}

func TestExecuteCommandDenylistAndNil(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	ctx := context.Background()
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "flushall"}); err == nil {
		t.Fatal("ExecuteCommand() allowed a denylisted command")
	}
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "FLUSHALL", AllowDangerous: true}); err != nil {
		t.Fatalf("ExecuteCommand() with AllowDangerous error = %v", err)
	}
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "GET;"}); err == nil {
		t.Fatal("ExecuteCommand() accepted an invalid command word")
	}
	result, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "GET", Args: []string{"missing"}})
	if err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}
	if !result.IsNil || result.Value != nil {
		t.Fatalf("missing key result = %#v", result)
	}
}

func TestExecuteCommandNormalizesReplies(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	ctx := context.Background()
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "SET", Args: []string{"user:1", "ada"}}); err != nil {
		t.Fatalf("SET error = %v", err)
	}
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "HSET", Args: []string{"user:1:meta", "email", "a@example.com", "city", "london"}}); err != nil {
		t.Fatalf("HSET error = %v", err)
	}
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "RPUSH", Args: []string{"queue", "a", "b"}}); err != nil {
		t.Fatalf("RPUSH error = %v", err)
	}
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "INCRBY", Args: []string{"counter", "5"}}); err != nil {
		t.Fatalf("INCRBY error = %v", err)
	}

	result, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "GET", Args: []string{"user:1"}})
	if err != nil || result.Value != "ada" {
		t.Fatalf("GET result = %#v, %v", result, err)
	}
	hashResult, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "HGETALL", Args: []string{"user:1:meta"}})
	if err != nil {
		t.Fatalf("HGETALL error = %v", err)
	}
	hash, ok := hashResult.Value.(map[string]any)
	if !ok || hash["email"] != "a@example.com" {
		t.Fatalf("HGETALL value = %#v", hashResult.Value)
	}
	listResult, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "LRANGE", Args: []string{"queue", "0", "-1"}})
	if err != nil {
		t.Fatalf("LRANGE error = %v", err)
	}
	list, ok := listResult.Value.([]any)
	if !ok || len(list) != 2 || list[0] != "a" {
		t.Fatalf("LRANGE value = %#v", listResult.Value)
	}
	counter, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "INCRBY", Args: []string{"counter", "5"}})
	if err != nil || counter.Value != int64(10) {
		t.Fatalf("counter result = %#v, %v", counter, err)
	}
}

func TestScanKeysEnrichesMetadata(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	ctx := context.Background()
	for _, command := range [][]string{
		{"SET", "user:1", "ada"}, {"SET", "user:2", "grace"}, {"RPUSH", "queue", "a"},
	} {
		if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: command[0], Args: command[1:]}); err != nil {
			t.Fatalf("%s error = %v", command[0], err)
		}
	}
	if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "EXPIRE", Args: []string{"user:1", "120"}}); err != nil {
		t.Fatalf("EXPIRE error = %v", err)
	}
	page, err := service.ScanKeys(ctx, item.ID, domain.KVScanRequest{Cursor: 0, Match: "user:*"})
	if err != nil {
		t.Fatalf("ScanKeys() error = %v", err)
	}
	if len(page.Keys) != 2 {
		t.Fatalf("ScanKeys() keys = %#v", page.Keys)
	}
	for _, key := range page.Keys {
		if key.Type != "string" {
			t.Fatalf("key type = %q", key.Type)
		}
		if key.Name == "user:1" && (key.TTL <= 0 || key.TTL > 120) {
			t.Fatalf("user:1 ttl = %d", key.TTL)
		}
		if key.Name == "user:2" && key.TTL != -1 {
			t.Fatalf("user:2 ttl = %d", key.TTL)
		}
	}
	// Type filter drops the string keys.
	filtered, err := service.ScanKeys(ctx, item.ID, domain.KVScanRequest{Cursor: 0, Type: "list"})
	if err != nil {
		t.Fatalf("ScanKeys(type) error = %v", err)
	}
	if len(filtered.Keys) != 1 || filtered.Keys[0].Name != "queue" {
		t.Fatalf("filtered keys = %#v", filtered.Keys)
	}
}

func TestKeyValuePerType(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	ctx := context.Background()
	for _, command := range [][]string{
		{"SET", "s", "hello"},
		{"HSET", "h", "a", "1", "b", "2"},
		{"RPUSH", "l", "x", "y", "z"},
		{"SADD", "set", "m1", "m2"},
		{"ZADD", "z", "10", "high", "1", "low"},
	} {
		if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: command[0], Args: command[1:]}); err != nil {
			t.Fatalf("%s error = %v", command[0], err)
		}
	}
	cases := []struct {
		key    string
		verify func(domain.KVKeyValue) error
	}{
		{"s", func(value domain.KVKeyValue) error {
			if value.Type != "string" || value.Value != "hello" {
				t.Fatalf("string value = %#v", value)
			}
			return nil
		}},
		{"h", func(value domain.KVKeyValue) error {
			hash, ok := value.Value.(map[string]string)
			if !ok || hash["a"] != "1" || len(hash) != 2 {
				t.Fatalf("hash value = %#v", value.Value)
			}
			return nil
		}},
		{"l", func(value domain.KVKeyValue) error {
			list, ok := value.Value.([]string)
			if !ok || len(list) != 3 || list[2] != "z" {
				t.Fatalf("list value = %#v", value.Value)
			}
			return nil
		}},
		{"set", func(value domain.KVKeyValue) error {
			members, ok := value.Value.([]string)
			if !ok || len(members) != 2 {
				t.Fatalf("set value = %#v", value.Value)
			}
			return nil
		}},
		{"z", func(value domain.KVKeyValue) error {
			entries, ok := value.Value.([]map[string]any)
			if !ok || len(entries) != 2 {
				t.Fatalf("zset value = %#v", value.Value)
			}
			if entries[0]["member"] != "low" || entries[0]["score"] != int64(1) {
				t.Fatalf("zset entries = %#v", entries)
			}
			return nil
		}},
	}
	for _, testCase := range cases {
		value, err := service.KeyValue(ctx, item.ID, testCase.key)
		if err != nil {
			t.Fatalf("KeyValue(%q) error = %v", testCase.key, err)
		}
		_ = testCase.verify(value)
	}
	missing, err := service.KeyValue(ctx, item.ID, "nope")
	if err != nil || missing.Type != "none" {
		t.Fatalf("missing key value = %#v, %v", missing, err)
	}
}

func TestDeleteKeysAndSetTTL(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	ctx := context.Background()
	for _, key := range []string{"a", "b", "c"} {
		if _, err := service.ExecuteCommand(ctx, domain.KVCommandRequest{DatabaseID: item.ID, Command: "SET", Args: []string{key, "1"}}); err != nil {
			t.Fatalf("SET error = %v", err)
		}
	}
	deleted, err := service.DeleteKeys(ctx, item.ID, []string{"a", "b", "missing"})
	if err != nil || deleted != 2 {
		t.Fatalf("DeleteKeys() = %d, %v", deleted, err)
	}
	if err := service.SetTTL(ctx, item.ID, "c", 90); err != nil {
		t.Fatalf("SetTTL() error = %v", err)
	}
	value, err := service.KeyValue(ctx, item.ID, "c")
	if err != nil || value.TTL <= 0 || value.TTL > 90 {
		t.Fatalf("ttl value = %#v, %v", value, err)
	}
	if err := service.SetTTL(ctx, item.ID, "c", -1); err != nil {
		t.Fatalf("SetTTL(persist) error = %v", err)
	}
	value, err = service.KeyValue(ctx, item.ID, "c")
	if err != nil || value.TTL != -1 {
		t.Fatalf("persisted value = %#v, %v", value, err)
	}
}

func TestInfoSummarisesServer(t *testing.T) {
	service, server := newTestService(t)
	item := register(t, service, server, "Cache")
	if _, err := service.ExecuteCommand(context.Background(), domain.KVCommandRequest{DatabaseID: item.ID, Command: "SET", Args: []string{"k", "v"}}); err != nil {
		t.Fatalf("SET error = %v", err)
	}
	info, err := service.Info(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	// miniredis omits redis_version; flavour defaulting and the key count are
	// the assertions that hold against the embedded server.
	if info.Flavor != "redis" || info.TotalKeys != 1 {
		t.Fatalf("Info() = %#v", info)
	}
}

func TestBuildDatabaseValidation(t *testing.T) {
	if _, err := BuildDatabase(domain.SaveDatabaseRequest{Name: "", Driver: domain.DatabaseDriverRedis}); err == nil {
		t.Fatal("BuildDatabase() accepted an empty name")
	}
	if _, err := BuildDatabase(domain.SaveDatabaseRequest{Name: "x", Driver: domain.DatabaseDriverSQLite}); err == nil {
		t.Fatal("BuildDatabase() accepted a non-redis driver")
	}
	if _, err := BuildDatabase(domain.SaveDatabaseRequest{Name: "x", Driver: domain.DatabaseDriverRedis}); err == nil {
		t.Fatal("BuildDatabase() accepted a missing host")
	}
	if _, err := BuildDatabase(domain.SaveDatabaseRequest{Name: "x", Driver: domain.DatabaseDriverRedis, Host: "localhost"}); err != nil {
		t.Fatalf("BuildDatabase() error = %v", err)
	}
	if _, err := BuildDatabase(domain.SaveDatabaseRequest{Name: "x", Driver: domain.DatabaseDriverRedis, Address: "redis://:pw@localhost:6379/1"}); err != nil {
		t.Fatalf("BuildDatabase(url) error = %v", err)
	}
	if _, err := BuildDatabase(domain.SaveDatabaseRequest{Name: "x", Driver: domain.DatabaseDriverRedis, Address: "http://bad"}); err == nil {
		t.Fatal("BuildDatabase() accepted an invalid URL")
	}
}

func TestNormalizeReplyTruncatesLongStrings(t *testing.T) {
	long := strings.Repeat("a", maxNormalizedStringBytes+100)
	value, truncated := normalizeReply(long)
	text, _ := value.(string)
	if !truncated || len(text) != maxNormalizedStringBytes {
		t.Fatalf("truncate result = %d chars, truncated=%v", len(text), truncated)
	}
	list := make([]any, maxNormalizedStrings+10)
	for index := range list {
		list[index] = index
	}
	value, truncated = normalizeReply(list)
	result, ok := value.([]any)
	if !ok || !truncated || len(result) != maxNormalizedStrings {
		t.Fatalf("list truncate result = %d items, truncated=%v", len(result), truncated)
	}
}
