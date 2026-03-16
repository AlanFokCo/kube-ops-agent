package env

import (
	"os"
	"path/filepath"
)

// Get returns env var value or default if unset/empty.
func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// MustAbs returns absolute path; returns def or empty on error.
func MustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
