//go:build darwin || linux

package gotoolchain

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// The wrapper records its real OS identity before execing the real Go binary.
// Each command must lead its own group. This fails when AnalyzePackage bypasses
// the capability's contained command by invoking packages.Load's private go.
func TestAnalysisCommandsUseTheResolvedContainedCapability(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	limits, err := DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := Open(t.Context(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(directory, "groups")
	script := "#!/bin/sh\nset -eu\nprintf '%s ' \"$$\" >> " + shellFixtureQuote(marker) + "\n/bin/ps -o pgid= -p \"$$\" >> " + shellFixtureQuote(marker) + "\nexec " + shellFixtureQuote(capability.command.String()) + " \"$@\"\n"
	wrapper := filepath.Join(directory, "go-probe")
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/groupfixture\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "group.go"), []byte("package groupfixture\nconst Value = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	capability.command, err = core.ParseAbsolutePath(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := gomodule.ParseImportPath("example.com/groupfixture")
	if err != nil {
		t.Fatal(err)
	}
	got, err := capability.AnalyzePackage(t.Context(), AnalysisRequest{WorkingDirectory: root, Package: imported})
	if err != nil || len(got.Units) != 1 || got.Units[0].Types.Scope().Lookup("Value") == nil {
		t.Fatalf("analysis = %+v/%v, want one compiler-checked Value declaration", got, err)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read command group observations = %v, want retained OS identities", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		t.Fatalf("command group observations = %q, want two PID/PGID pairs", data)
	}
	for i := 0; i < len(fields); i += 2 {
		pid, pidErr := strconv.Atoi(fields[i])
		group, groupErr := strconv.Atoi(fields[i+1])
		if pidErr != nil || groupErr != nil || pid <= 0 || pid != group {
			t.Fatalf("command PID/PGID = %d/%d (%v/%v), want positive equal identities", pid, group, pidErr, groupErr)
		}
	}
}

func shellFixtureQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
