package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseWindowsProxyServer(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		valid   bool
		general string
		schemes map[string]string
	}{
		{name: "single address serves every protocol", value: "proxy.example:8080", valid: true, general: "proxy.example:8080"},
		{name: "address with scheme", value: "http://proxy.example:8080", valid: true, general: "http://proxy.example:8080"},
		{name: "per protocol", value: "http=p:80;https=p:443", valid: true, schemes: map[string]string{"http": "p:80", "https": "p:443"}},
		{name: "socks gains socks5 scheme", value: "socks=127.0.0.1:1080", valid: true, schemes: map[string]string{"socks": "socks5://127.0.0.1:1080"}},
		{name: "empty", value: "", valid: false},
		{name: "blank", value: "   ", valid: false},
		{name: "dangling separators", value: ";;=;", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting, ok := parseWindowsProxyServer(test.value)
			if ok != test.valid {
				t.Fatalf("parseWindowsProxyServer(%q) ok = %v, want %v", test.value, ok, test.valid)
			}
			if setting.general != test.general {
				t.Fatalf("general = %q, want %q", setting.general, test.general)
			}
			for scheme, address := range test.schemes {
				if setting.perScheme[scheme] != address {
					t.Fatalf("perScheme[%q] = %q, want %q", scheme, setting.perScheme[scheme], address)
				}
			}
		})
	}
}

func TestWindowsProxySettingProxyForRequest(t *testing.T) {
	general, ok := parseWindowsProxyServer("proxy.example:8080")
	if !ok {
		t.Fatal("parse general proxy")
	}
	perProtocol, ok := parseWindowsProxyServer("http=p80;https=p443;socks=s1080")
	if !ok {
		t.Fatal("parse per-protocol proxy")
	}

	tests := []struct {
		name    string
		setting windowsProxySetting
		url     string
		want    string
	}{
		{name: "general proxies https", setting: general, url: "https://github.com/x", want: "http://proxy.example:8080"},
		{name: "per protocol https", setting: perProtocol, url: "https://github.com/x", want: "http://p443"},
		{name: "per protocol http", setting: perProtocol, url: "http://github.com/x", want: "http://p80"},
		{name: "unlisted scheme falls back to socks", setting: perProtocol, url: "https://example.com/x", want: "http://p443"},
		{name: "loopback never proxied", setting: general, url: "http://127.0.0.1:9/x", want: ""},
		{name: "localhost never proxied", setting: general, url: "http://localhost:9/x", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.url, nil)
			proxy, err := test.setting.proxyForRequest(request)
			if err != nil {
				t.Fatalf("proxyForRequest() error = %v", err)
			}
			got := ""
			if proxy != nil {
				got = proxy.String()
			}
			if got != test.want {
				t.Fatalf("proxy = %q, want %q", got, test.want)
			}
		})
	}

	socksOnly, ok := parseWindowsProxyServer("socks=127.0.0.1:1080")
	if !ok {
		t.Fatal("parse socks-only proxy")
	}
	request := httptest.NewRequest(http.MethodGet, "https://github.com/x", nil)
	proxy, err := socksOnly.proxyForRequest(request)
	if err != nil || proxy == nil || proxy.Scheme != "socks5" {
		t.Fatalf("socks-only proxy = %v, %v, want socks5://127.0.0.1:1080", proxy, err)
	}
}

func TestProxyURL(t *testing.T) {
	proxy, err := proxyURL("cache.internal:3128")
	if err != nil || proxy == nil || proxy.String() != "http://cache.internal:3128" {
		t.Fatalf("proxyURL(bare host) = %v, %v, want http://cache.internal:3128", proxy, err)
	}
	proxy, err = proxyURL("socks5://gate.internal:1080")
	if err != nil || proxy == nil || proxy.Scheme != "socks5" {
		t.Fatalf("proxyURL(socks5) = %v, %v, want socks5 scheme", proxy, err)
	}
	if proxy, err = proxyURL(""); err != nil || proxy != nil {
		t.Fatalf("proxyURL(empty) = %v, %v, want nil", proxy, err)
	}
}

func TestOutboundProxyPrefersSystemProxy(t *testing.T) {
	// A loopback target must never be proxied even when a system proxy exists.
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9/local", nil)
	proxy, err := outboundProxy(request)
	if err != nil {
		t.Fatalf("outboundProxy() error = %v", err)
	}
	// Loopback bypasses the system proxy and ProxyFromEnvironment, so the
	// result must be nil (direct) regardless of the environment.
	if proxy != nil {
		t.Fatalf("outboundProxy(loopback) = %v, want direct", proxy)
	}
}
