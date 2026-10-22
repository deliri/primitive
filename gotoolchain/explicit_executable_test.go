//go:build darwin || linux

package gotoolchain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestExplicitExecutableNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"owned command", "different command", "absent", "not executable", "directory", "unset", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "owned-go")
			version := "1.26.7"
			if mode == "different command" {
				version = "1.25.9"
			}
			if err := os.WriteFile(name, []byte("#!/bin/sh\nprintf '{\"Go\":\""+version+"\"}'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if mode == "absent" {
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "not executable" {
				if err := os.Chmod(name, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "directory" {
				name = directory
			}
			command, err := core.ParseAbsolutePath(name)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unset" {
				command = core.AbsolutePath{}
			}
			limits, err := DefaultLimits()
			if err != nil {
				t.Fatal(err)
			}
			env, err := process.ParseExactEnvironment([]string{"PATH=/no-ambient-go", "GOTOOLCHAIN=local"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			if mode == "cancelled" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			capability, err := OpenExecutable(ctx, command, Configuration{Environment: &env, Limits: limits, Workspace: WorkspaceModeDisabled})
			good := mode == "owned command" || mode == "different command"
			if !good {
				if err == nil {
					t.Fatal("unrunnable explicit command admitted")
				}
				if mode == "cancelled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			work, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			got, err := capability.ObserveModuleFile(ctx, ObservationRequest{WorkingDirectory: work})
			if err != nil || got.GoVersion.String() != "go"+version {
				t.Fatalf("explicit native module=%+v/%v", got, err)
			}
		})
	}
}
