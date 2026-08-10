package twitch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/twitchspec"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type memoryVault struct {
	values map[string]string
	puts   []string
}

func (v *memoryVault) Get(key string) (string, error) {
	value, found := v.values[key]
	if !found {
		return "", fmt.Errorf("missing %s", key)
	}
	return value, nil
}
func (v *memoryVault) Put(key, value string) error {
	v.values[key] = value
	v.puts = append(v.puts, key)
	return nil
}
func (v *memoryVault) Delete(key string) error { delete(v.values, key); return nil }

func TestTokensAreOneAtomicVaultRecord(t *testing.T) {
	vault := &memoryVault{values: map[string]string{}}
	service := New(vault, nil, nil, nil, nil)
	if err := service.putTokens("bot", tokenBundle{AccessToken: "access", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if len(vault.puts) != 1 || vault.puts[0] != tokenKey("bot") {
		t.Fatalf("token writes = %#v", vault.puts)
	}
	got, err := service.tokens("bot")
	if err != nil || got.AccessToken != "access" || got.RefreshToken != "refresh" {
		t.Fatalf("tokens = %#v, %v", got, err)
	}
}

func TestSubscriptionConditionResolvesChannelLogin(t *testing.T) {
	descriptor, found := twitchspec.Find("channel.chat.message")
	if !found {
		t.Fatal("chat event is missing")
	}
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)
	service.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/helix/users" || request.URL.Query().Get("login") != "channel" {
			t.Fatalf("unexpected channel lookup %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"42"}]}`))}, nil
	})}
	condition, err := service.resolvedSubscriptionCondition(context.Background(), descriptor, map[string]any{"channel": "channel"}, domain.TwitchIdentity{UserID: "bot"}, "token")
	if err != nil {
		t.Fatal(err)
	}
	if condition["broadcaster_user_id"] != "42" || condition["user_id"] != "bot" || len(condition) != 2 {
		t.Fatalf("condition = %#v", condition)
	}
	if _, err := service.resolvedSubscriptionCondition(context.Background(), descriptor, map[string]any{}, domain.TwitchIdentity{UserID: "bot"}, "token"); err == nil {
		t.Fatal("missing channel was accepted")
	}
}

func TestIdentityScopesAndMessageDeduplication(t *testing.T) {
	if identityHasScopes(domain.TwitchIdentity{Scopes: []string{"one", "two"}}, []string{"two", "one"}) == false {
		t.Fatal("granted scopes were rejected")
	}
	if identityHasScopes(domain.TwitchIdentity{Scopes: []string{"one"}}, []string{"two"}) {
		t.Fatal("missing scope was accepted")
	}
	service := New(&memoryVault{values: map[string]string{}}, nil, nil, nil, nil)
	if !service.markMessage("delivery") || service.markMessage("delivery") {
		t.Fatal("duplicate delivery was not suppressed")
	}
	if err := service.recordIdentity(context.Background(), domain.TwitchIdentity{ID: "bot", Status: domain.TwitchIdentityConnected}); err != nil {
		t.Fatal(err)
	}
	if service.settings.DefaultBotIdentityID != "bot" {
		t.Fatalf("default identity = %q", service.settings.DefaultBotIdentityID)
	}
}
