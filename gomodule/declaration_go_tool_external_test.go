package gomodule_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// A declared local module need not satisfy the downloadable-module domain.
// The reference observation comes from the actual Go tool, not another parser.
func TestModuleDeclarationPreservesGoToolLocalIdentity(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"app", "App", "app_local"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := "module " + name + "\ngo 1.27.2\n"
			observed := observeGoToolModuleIdentity(t, body)
			if observed != name {
				t.Fatalf("Go tool identity=%q want fixture %q", observed, name)
			}
			got, err := gomodule.ObserveDeclaration(t.Context(), gomodule.DeclarationRequest{Source: strings.NewReader(body)})
			if err != nil || got.Presence != gomodule.DeclarationPresent || got.Path.String() != observed {
				t.Fatalf("module projection=%+v/%v, want Go tool %q", got, err, observed)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			if _, err := gomodule.ParsePath(name); !errors.Is(err, core.ErrGoModuleContract) {
				t.Fatalf("local identity acquired downloadable-module authority: %v", err)
			}
		})
	}
}
func observeGoToolModuleIdentity(t *testing.T, body string) string {
	t.Helper()
	directory, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
		path, err := core.ParseRelativePath("go.mod")
		if err != nil {
			return err
		}
		temporary, err := core.ParseRelativePath("go.mod.stage")
		if err != nil {
			return err
		}
		_, err = filestore.Write(ctx, filestore.WriteRequest{Source: strings.NewReader(body), Location: filestore.Location{Root: root, Path: path}, Temporary: temporary, Mode: 0o600, Install: filestore.InstallCreate})
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Validate(); err != nil {
		t.Fatal(err)
	}
	name, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatal(err)
	}
	command, err := process.Resolve(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}
	arguments, err := process.ParseArguments([]string{"list", "-m"})
	if err != nil {
		t.Fatal(err)
	}
	wait, err := temporal.ParseDuration("5s")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	observation, err := process.Run(t.Context(), process.Request{WaitDelay: wait, Command: command, WorkingDirectory: directory, Arguments: arguments, Environment: process.Environment{Mode: process.EnvironmentModeInherit}, Streams: process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}})
	if err != nil {
		t.Fatalf("Go module producer=%v stderr=%q", err, stderr.String())
	}
	exit, err := observation.ExitCode()
	if err != nil {
		t.Fatal(err)
	}
	success, err := exit.Success()
	if err != nil || !success {
		t.Fatalf("Go tool exit=%v/%v stderr=%q", exit, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}
