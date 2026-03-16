package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGet_EnvSet(t *testing.T) {
	os.Setenv("TEST_GET_VAR", "hello")
	defer os.Unsetenv("TEST_GET_VAR")
	got := Get("TEST_GET_VAR", "default")
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestGet_EnvUnset(t *testing.T) {
	os.Unsetenv("TEST_UNSET_VAR_XYZ")
	got := Get("TEST_UNSET_VAR_XYZ", "default")
	if got != "default" {
		t.Errorf("expected 'default', got %q", got)
	}
}

func TestGet_EnvEmpty(t *testing.T) {
	os.Setenv("TEST_EMPTY_VAR", "")
	defer os.Unsetenv("TEST_EMPTY_VAR")
	got := Get("TEST_EMPTY_VAR", "fallback")
	if got != "fallback" {
		t.Errorf("expected 'fallback' for empty env var, got %q", got)
	}
}

func TestMustAbs_Relative(t *testing.T) {
	rel := "some/path"
	abs := MustAbs(rel)
	if !filepath.IsAbs(abs) {
		t.Errorf("expected absolute path, got %q", abs)
	}
}

func TestMustAbs_AlreadyAbsolute(t *testing.T) {
	abs := "/tmp/test"
	result := MustAbs(abs)
	if result != abs {
		t.Errorf("expected %q, got %q", abs, result)
	}
}
