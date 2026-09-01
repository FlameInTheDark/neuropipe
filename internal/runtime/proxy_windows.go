//go:build windows

package runtime

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// systemProxyRecheckInterval bounds how often the WinINET registry values are
// re-read, so toggling the desktop proxy is picked up without an app restart
// while each individual request still avoids a registry hit.
const systemProxyRecheckInterval = time.Minute

var (
	systemProxyMu        sync.Mutex
	systemProxyCached    func(*http.Request) (*url.URL, error)
	systemProxyCheckedAt time.Time
)

// systemProxyLookup returns the current user's WinINET manual proxy as a
// transport Proxy function, or nil when no proxy is configured. PAC scripts
// (AutoConfigURL) and the ProxyOverride bypass list are not interpreted; the
// hosts this package downloads from are never local or bypassed in practice.
func systemProxyLookup() func(*http.Request) (*url.URL, error) {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()
	if !systemProxyCheckedAt.IsZero() && time.Since(systemProxyCheckedAt) < systemProxyRecheckInterval {
		return systemProxyCached
	}
	systemProxyCached = loadWindowsSystemProxy()
	systemProxyCheckedAt = time.Now()
	return systemProxyCached
}

// loadWindowsSystemProxy reads the manual proxy the browser uses from
// HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings.
func loadWindowsSystemProxy() func(*http.Request) (*url.URL, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer func() { _ = key.Close() }()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return nil
	}
	setting, ok := parseWindowsProxyServer(server)
	if !ok {
		return nil
	}
	return setting.proxyForRequest
}
