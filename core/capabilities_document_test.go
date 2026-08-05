package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// capabilitiesDocumentName is the operator-facing index that answers "does
	// Primitive already do this?" before somebody builds a second one.
	capabilitiesDocumentName = "CAPABILITIES.md"
	// capabilityDescriptionMinimumBytes is the floor for a row's description.
	// A row can name a package and say nothing, and a row that says nothing
	// sends the reader looking somewhere else, which is the failure this
	// document exists to prevent.
	capabilityDescriptionMinimumBytes = 40
	// capabilityRowPrefix and capabilityRowSeparator are the exact shape of one
	// package-table row.
	capabilityRowPrefix    = "| `"
	capabilityRowSeparator = "` |"
)

// TestCapabilitiesDocumentDescribesEveryPackage is the ratchet against
// rebuilding something Primitive already owns.
//
// A capability nobody can find is a capability somebody rebuilds, and a second
// implementation of a wire type is the one duplication that cannot be
// reconciled afterwards: both ends must agree on the bytes, so two owners means
// two answers.
//
// There is deliberately no list of package names in this file. The repository
// is the source of truth and the document is checked against it directly; a
// constant list here would be a second copy of the answer, free to drift from
// both the disk and the prose.
func TestCapabilitiesDocumentDescribesEveryPackage(t *testing.T) {
	t.Parallel()

	described := describedPackages(t)
	for _, name := range packageDirectoriesOnDisk(t) {
		description, found := described[name]
		if !found {
			t.Errorf("package %q exists and %s has no row for it", name, capabilitiesDocumentName)
			continue
		}
		if len(description) < capabilityDescriptionMinimumBytes {
			t.Errorf("%s describes %q in %d bytes, want at least %d",
				capabilitiesDocumentName, name, len(description), capabilityDescriptionMinimumBytes)
		}
	}
}

// TestCapabilitiesDocumentNamesNothingThatDoesNotExist is the other direction.
// A row for a package that was removed is worse than a missing row: it sends a
// reader to import something that is not there.
func TestCapabilitiesDocumentNamesNothingThatDoesNotExist(t *testing.T) {
	t.Parallel()

	onDisk := map[string]bool{}
	for _, name := range packageDirectoriesOnDisk(t) {
		onDisk[name] = true
	}
	for name := range describedPackages(t) {
		if !onDisk[name] {
			t.Errorf("%s has a row for %q, which is not a package in this repository",
				capabilitiesDocumentName, name)
		}
	}
}

// describedPackages reads only rows of the package table, keyed by the package
// each row names.
//
// Matching the row shape matters. A looser search passes as long as the name
// appears anywhere in the file, and every package named in the table is also
// named in the guidance below it, so a package dropped from the real table
// would still look described.
func describedPackages(t *testing.T) map[string]string {
	t.Helper()

	document, err := os.ReadFile(filepath.Join("..", capabilitiesDocumentName))
	if err != nil {
		t.Fatalf("reading %s error = %v, want nil", capabilitiesDocumentName, err)
	}
	rows := map[string]string{}
	for line := range strings.SplitSeq(string(document), "\n") {
		name, description, ok := capabilityRow(line)
		if !ok {
			continue
		}
		if _, duplicate := rows[name]; duplicate {
			t.Errorf("%s has two rows for %q, want one per package", capabilitiesDocumentName, name)
		}
		rows[name] = description
	}
	if len(rows) == 0 {
		t.Fatalf("%s has no package rows, want the ratchet to read a real table", capabilitiesDocumentName)
	}
	return rows
}

func capabilityRow(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, capabilityRowPrefix) {
		return "", "", false
	}
	columns := strings.SplitN(strings.TrimPrefix(trimmed, capabilityRowPrefix), capabilityRowSeparator, 2)
	if len(columns) != 2 {
		return "", "", false
	}
	description := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(columns[1]), "|"))
	return columns[0], description, true
}

// packageDirectoriesOnDisk returns every directory beside core that holds Go
// source, which is exactly the set a caller can import.
func packageDirectoriesOnDisk(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatalf("reading the repository root error = %v, want nil", err)
	}
	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		if !directoryHoldsGoSource(t, filepath.Join("..", entry.Name())) {
			continue
		}
		found = append(found, entry.Name())
	}
	if len(found) == 0 {
		t.Fatalf("no package directories found, want the ratchet to observe the repository")
	}
	return found
}

func directoryHoldsGoSource(t *testing.T, directory string) bool {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("reading %s error = %v, want nil", directory, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}
