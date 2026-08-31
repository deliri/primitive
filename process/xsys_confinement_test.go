package process

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// substrateEscapeLeafFiles are the exact production files permitted to import
// syscall or golang.org/x/sys. The package-wide import allowlist admits those
// paths for the package, which left the escape free to spread into any file;
// section 8.4 wants the leaf files named, so adding syscall.Kill to run.go is
// a red test rather than a green build.
const substrateEscapeLeafMaximum = 7

func substrateEscapeLeafFiles() map[string]string {
	return map[string]string{
		"alive_unix.go":        "observes process liveness through the Unix kernel",
		"alive_windows.go":     "observes process liveness through the Windows kernel",
		"containment_unix.go":  "installs and signals Unix process-group containment",
		"sightings_windows.go": "enumerates Windows process identities",
		"termination_unix.go":  "decodes the Unix termination signal",
		"usage_linux.go":       "decodes Linux rusage peak-resident-memory units",
		"usage_darwin.go":      "decodes Darwin rusage peak-resident-memory units",
	}
}

// TestSubstrateEscapeStaysInNamedLeafFiles fails when any unnamed production
// file imports syscall or golang.org/x/sys, and when a named leaf stops
// taking the escape, so the inventory can neither grow silently nor rot into
// vacuous rows.
func TestSubstrateEscapeStaysInNamedLeafFiles(t *testing.T) {
	t.Parallel()

	allowed := substrateEscapeLeafFiles()
	if len(allowed) > substrateEscapeLeafMaximum {
		t.Fatalf("substrate escape leaf count = %d, want at most ratcheted maximum %d", len(allowed), substrateEscapeLeafMaximum)
	}
	for name, reason := range allowed {
		if reason == "" {
			t.Fatalf("substrate escape leaf %s reason is empty, want the exact kernel failure class it owns", name)
		}
	}
	seen := map[string]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v, want nil", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", name, err)
		}
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if path != "syscall" && !strings.HasPrefix(path, "golang.org/x/sys") {
				continue
			}
			if _, admitted := allowed[name]; !admitted {
				t.Errorf("production file %s imports %s, want the escape confined to the named platform leaves", name, path)
			}
			seen[name] = true
		}
	}
	for name := range allowed {
		if !seen[name] {
			t.Errorf("named platform leaf %s no longer imports syscall or golang.org/x/sys, want the inventory row removed with the escape", name)
		}
	}
}
