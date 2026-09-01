package runtime

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// newOutboundHTTPClient builds the HTTP client used for all outbound catalog
// traffic (GitHub, Hugging Face). Unlike http.DefaultTransport it also honors
// the Windows system proxy on top of the usual environment variables, so a
// desktop behind Clash, v2rayN, or a corporate forward proxy reaches exactly
// the hosts the user's browser can reach.
func newOutboundHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 outboundProxy,
		DialContext:           (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	return &http.Client{Transport: transport}
}

// outboundProxy prefers the platform system proxy and falls back to the
// standard environment-variable configuration.
func outboundProxy(request *http.Request) (*url.URL, error) {
	if lookup := systemProxyLookup(); lookup != nil {
		if proxy, err := lookup(request); err == nil && proxy != nil {
			return proxy, nil
		}
	}
	return http.ProxyFromEnvironment(request)
}

// windowsProxySetting is the parsed WinINET ProxyServer value.
type windowsProxySetting struct {
	general   string
	perScheme map[string]string
}

// parseWindowsProxyServer accepts both ProxyServer registry formats:
//
//	"proxy.example:8080"                      one proxy for every protocol
//	"http=p:80;https=p:443;socks=p:1080"      one proxy per protocol
func parseWindowsProxyServer(value string) (windowsProxySetting, bool) {
	setting := windowsProxySetting{perScheme: map[string]string{}}
	value = strings.TrimSpace(value)
	if value == "" {
		return setting, false
	}
	if !strings.Contains(value, "=") {
		setting.general = value
		return setting, true
	}
	for _, part := range strings.Split(value, ";") {
		scheme, address, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		scheme = strings.ToLower(strings.TrimSpace(scheme))
		address = strings.TrimSpace(address)
		if scheme == "" || address == "" {
			continue
		}
		if scheme == "socks" && !strings.Contains(address, "://") {
			address = "socks5://" + address
		}
		setting.perScheme[scheme] = address
	}
	return setting, len(setting.perScheme) > 0
}

// proxyForRequest picks the proxy address for a request. With the
// per-protocol format, a socks entry applies to every scheme and unlisted
// protocols connect directly, which matches how desktop proxy tools configure
// WinINET closely enough for the hosts this app downloads from. Loopback
// traffic is never proxied.
func (setting windowsProxySetting) proxyForRequest(request *http.Request) (*url.URL, error) {
	if requestBypassesProxy(request) {
		return nil, nil
	}
	scheme := strings.ToLower(request.URL.Scheme)
	address := setting.perScheme[scheme]
	if address == "" {
		address = setting.perScheme["socks"]
	}
	if address == "" {
		address = setting.general
	}
	if address == "" {
		return nil, nil
	}
	return proxyURL(address)
}

// proxyURL turns a "host:port" (or "scheme://host:port") address into the URL
// http.Transport expects, assuming a plain HTTP CONNECT proxy for bare
// addresses.
func proxyURL(address string) (*url.URL, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, nil
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, nil
	}
	return parsed, nil
}

// requestBypassesProxy reports whether a request targets this machine, which
// must never be routed through a proxy.
func requestBypassesProxy(request *http.Request) bool {
	host := request.URL.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}
