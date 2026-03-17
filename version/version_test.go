package version

import "testing"

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("expected non-empty Version")
	}
}
