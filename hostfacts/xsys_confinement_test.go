package hostfacts

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// platformEscapeLeafFiles are the exact production files permitted to take the
// golang.org/x/sys escape. Section 8.4 of the policy requires the escape to
// live in build-tagged leaf files that a structural ratchet names, so the
// exception stays visible instead of spreading; before this test the package
// had ten escape files and nothing that would go red when an eleventh
// appeared in an untagged file.
func platformEscapeLeafFiles() map[string]bool {
	return map[string]bool{
		"device_darwin.go":          true,
		"device_linux.go":           true,
		"disk_darwin.go":            true,
		"disk_linux.go":             true,
		"physical_memory_darwin.go": true,
		"physical_memory_linux.go":  true,
		"root_unix.go":              true,
		"root_windows.go":           true,
		"storage_linux.go":          true,
		"terminal_unix.go":          true,
		"terminal_windows.go":       true,
	}
}

// TestPlatformEscapeStaysInNamedLeafFiles fails when any unnamed production
// file imports golang.org/x/sys, and when a named leaf stops taking the
// escape, so the inventory can neither grow silently nor rot into vacuous
// rows.
func TestPlatformEscapeStaysInNamedLeafFiles(t *testing.T) {
	t.Parallel()

	allowed := platformEscapeLeafFiles()
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
			if !strings.HasPrefix(path, "golang.org/x/sys") {
				continue
			}
			if !allowed[name] {
				t.Errorf("production file %s imports %s, want the escape confined to the named platform leaves", name, path)
			}
			seen[name] = true
		}
	}
	for name := range allowed {
		if !seen[name] {
			t.Errorf("named platform leaf %s no longer imports golang.org/x/sys, want the inventory row removed with the escape", name)
		}
	}
}
