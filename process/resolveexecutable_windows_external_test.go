//go:build windows

package process_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

// TestResolveExecutableResolvesTheWindowsExecutableExtension proves the one
// claim the door makes that only Windows can answer: the host may resolve a
// path to a different name than the caller spelled, because LookPath applies
// the executable extension search to a path without one. The parse gate on
// the answer exists for exactly this case, so the proof pins both shapes:
// the bare spelling resolves to the extended name, and the extended spelling
// resolves to itself.
func TestResolveExecutableResolvesTheWindowsExecutableExtension(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build func(t *testing.T) (path string, wantResolved string)
		name  string
	}{
		{
			name: "a path without an extension resolves to the extended executable",
			build: func(t *testing.T) (string, string) {
				directory := t.TempDir()
				planted := filepath.Join(directory, "candidate-tool.exe")
				if err := os.WriteFile(planted, []byte("MZ"), 0o755); err != nil {
					t.Fatalf("WriteFile(%s) error = %v, want nil", planted, err)
				}
				return filepath.Join(directory, "candidate-tool"), planted
			},
		},
		{
			name: "a path spelling its extension resolves to itself",
			build: func(t *testing.T) (string, string) {
				planted := filepath.Join(t.TempDir(), "candidate-tool.exe")
				if err := os.WriteFile(planted, []byte("MZ"), 0o755); err != nil {
					t.Fatalf("WriteFile(%s) error = %v, want nil", planted, err)
				}
				return planted, planted
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, wantResolved := tc.build(t)
			got, err := process.ResolveExecutable(t.Context(), absolutePath(t, path))
			if err != nil {
				t.Fatalf("ResolveExecutable(%q) error = %v, want nil", path, err)
			}
			if got.String() != wantResolved {
				t.Fatalf("ResolveExecutable(%q) = %q, want %q", path, got.String(), wantResolved)
			}
		})
	}
}
