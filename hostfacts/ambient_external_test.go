package hostfacts_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
)

// TestObserveHostnameReportsTheAdmittedPlatformName pins the door to the
// platform oracle and the admitted form: what the host reports is what the
// observation carries, validated.
func TestObserveHostnameReportsTheAdmittedPlatformName(t *testing.T) {
	t.Parallel()

	got, err := hostfacts.ObserveHostname()
	if err != nil {
		t.Fatalf("hostfacts.ObserveHostname() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("ObserveHostname().Validate() error = %v, want nil", err)
	}
	want, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname() oracle error = %v, want nil", err)
	}
	if got.String() != want {
		t.Fatalf("ObserveHostname() = %q, want the platform's own %q", got.String(), want)
	}
	var zero hostfacts.Hostname
	if err := zero.Validate(); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("zero Hostname.Validate() error = %v, want errors.Is %v", err, core.ErrHostFactsContract)
	}
}

// TestUserConfigDirectoryIsTheAdmittedPlatformBase pins the config base to
// the platform oracle through the path admission.
func TestUserConfigDirectoryIsTheAdmittedPlatformBase(t *testing.T) {
	t.Parallel()

	got, err := hostfacts.UserConfigDirectory()
	if err != nil {
		t.Fatalf("hostfacts.UserConfigDirectory() error = %v, want nil", err)
	}
	want, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() oracle error = %v, want nil", err)
	}
	if got.String() != want {
		t.Fatalf("UserConfigDirectory() = %q, want the platform's own %q", got.String(), want)
	}
}

// TestTemporaryDirectoryIsTheAdmittedPlatformBase pins the scratch base to
// the platform oracle through the path admission, tolerating exactly the one
// trailing-separator respelling the admission performs.
func TestTemporaryDirectoryIsTheAdmittedPlatformBase(t *testing.T) {
	t.Parallel()

	got, err := hostfacts.TemporaryDirectory()
	if err != nil {
		t.Fatalf("hostfacts.TemporaryDirectory() error = %v, want nil", err)
	}
	want, err := core.ParseAbsolutePath(filepath.Clean(os.TempDir()))
	if err != nil {
		t.Fatalf("ParseAbsolutePath(Clean(os.TempDir())) oracle error = %v, want nil", err)
	}
	if got != want {
		t.Fatalf("TemporaryDirectory() = %q, want the platform's own %q", got.String(), want.String())
	}
}
