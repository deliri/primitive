//go:build darwin || linux

package gotoolchain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// The wrapper records each actual process then execs the actual Go binary.
// A growing batch must still execute one environment observation and one load.
func TestAnalysisBatchAmortizesRealCompilerCommands(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		packages  int
		limited   bool
		wantCalls int
		wantErr   error
	}{
		{name: "single subject keeps exact command path", packages: 1, wantCalls: 2},
		{name: "two subjects share one compiler load", packages: 2, wantCalls: 2},
		{name: "larger batch does not add per-package processes", packages: 17, wantCalls: 2},
		{name: "empty batch cannot launch a command", wantErr: core.ErrGoToolchainContract},
		{name: "package bound refuses before process launch", packages: 2, limited: true, wantErr: core.ErrGoToolchainContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			limits, err := DefaultLimits()
			if err != nil {
				t.Fatal(err)
			}
			if tc.limited {
				limits.PackageMaximum = 1
			}
			capability, err := Open(t.Context(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits})
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(directory, "commands")
			wrapper := filepath.Join(directory, "go-probe")
			script := "#!/bin/sh\nset -eu\nprintf x >> " + shellFixtureQuote(marker) + "\nexec " + shellFixtureQuote(capability.command.String()) + " \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			capability.command, err = core.ParseAbsolutePath(wrapper)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/commands\n\ngo 1.27.1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			request := AnalysisBatchRequest{WorkingDirectory: root}
			for i := range tc.packages {
				name := "p" + strconv.Itoa(100+i)
				if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, name, "value.go"), []byte("package subject\nconst Value = 1\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				pkg, err := gomodule.ParseImportPath("example.com/commands/" + name)
				if err != nil {
					t.Fatal(err)
				}
				request.Packages = append(request.Packages, pkg)
			}
			var seen atomic.Int64
			err = capability.AnalyzePackages(t.Context(), request, func(_ context.Context, result AnalysisResult) error {
				if result.Err != nil {
					return result.Err
				}
				if len(result.Analysis.Units) != 1 || result.Analysis.Units[0].Types.Scope().Lookup("Value") == nil {
					t.Errorf("compiler units = %d, want one with Value declaration", len(result.Analysis.Units))
				}
				seen.Add(1)
				return nil
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("batch = %v, want %v", err, tc.wantErr)
			}
			recorded, readErr := os.ReadFile(marker)
			if tc.wantCalls == 0 {
				if !errors.Is(readErr, os.ErrNotExist) || seen.Load() != 0 {
					t.Fatalf("refused request executed: %q/%v, callbacks %d", recorded, readErr, seen.Load())
				}
			} else if readErr != nil || len(recorded) != tc.wantCalls || seen.Load() != int64(tc.packages) {
				t.Fatalf("compiler processes/results = %d/%d (%v), want %d/%d", len(recorded), seen.Load(), readErr, tc.wantCalls, tc.packages)
			}
		})
	}
}
