// Package localization defines the supported local application languages.
package localization

import "strings"

const (
	// English is the immutable fallback locale for Neuropipe.
	English = "en"
	German  = "de"
	French  = "fr"
	Russian = "ru"
)

// SupportedLanguages lists every locale bundled with the desktop app.
var SupportedLanguages = []string{English, German, French, Russian}

// Normalize returns a supported base language, falling back safely to English.
func Normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if index := strings.IndexAny(value, "-_"); index >= 0 {
		value = value[:index]
	}
	for _, language := range SupportedLanguages {
		if value == language {
			return language
		}
	}
	return English
}

// IsSupported reports whether value is exactly one of the bundled locales.
func IsSupported(value string) bool {
	for _, language := range SupportedLanguages {
		if value == language {
			return true
		}
	}
	return false
}
