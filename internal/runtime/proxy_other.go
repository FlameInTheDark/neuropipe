//go:build !windows

package runtime

import (
	"net/http"
	"net/url"
)

// systemProxyLookup is a no-op outside Windows: environment variables remain
// the only proxy source on other platforms.
func systemProxyLookup() func(*http.Request) (*url.URL, error) { return nil }
