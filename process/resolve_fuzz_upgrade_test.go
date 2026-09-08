package process_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

func FuzzResolveAndResolveExecutableExternalIngress(f *testing.F) {
	filename := "fixture"
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	seed, err := core.ParsePathComponent(filename)
	if err != nil || seed.Validate() != nil {
		f.Fatalf("resolution seed: %v", err)
	}
	f.Add(seed.String(), uint32(os.ModePerm), false)
	f.Add(seed.String(), uint32(0), false)
	f.Add(seed.String(), uint32(os.ModePerm), true)
	f.Add("missing", uint32(os.ModePerm), false)
	f.Add("../escape", uint32(os.ModePerm), false)
	f.Add("bad\x00name", uint32(os.ModePerm), false)
	f.Fuzz(func(t *testing.T, raw string, permissions uint32, directory bool) {
		testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
		root := t.TempDir()
		fixed := filepath.Join(root, filename)
		mode := os.FileMode(permissions) & os.ModePerm
		if directory {
			if err := os.Mkdir(fixed, mode); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(fixed, nil, mode); err != nil {
			t.Fatal(err)
		}
		// Exact PATH isolates the lookup; no discovered file is executed.
		t.Setenv("PATH", root)
		name, parseErr := core.ParsePathComponent(raw)
		got, err := process.Resolve(t.Context(), name)
		if parseErr != nil {
			if !errors.Is(err, core.ErrProcessContract) || got != (core.AbsolutePath{}) {
				t.Fatalf("invalid name exposed path: %v", err)
			}
			return
		}
		want, nativeErr := exec.LookPath(name.String())
		if nativeErr != nil {
			for cause := errors.Unwrap(nativeErr); cause != nil; cause = errors.Unwrap(nativeErr) {
				nativeErr = cause
			}
			if !errors.Is(err, nativeErr) || !errors.Is(err, core.ErrProcessContract) || got != (core.AbsolutePath{}) {
				t.Fatalf("name lookup lost native refusal: %v, want %v", err, nativeErr)
			}
		} else {
			absolute, absoluteErr := core.ParseAbsolutePath(want)
			if absoluteErr != nil {
				if !errors.Is(err, core.ErrProcessContract) || got != (core.AbsolutePath{}) {
					t.Fatalf("relative lookup answer was promoted: %v", err)
				}
			} else if err != nil || got != absolute {
				t.Fatalf("lookup changed Go's path: %v, want %v, error %v", got, absolute, err)
			}
		}
		candidate, err := core.ParseAbsolutePath(filepath.Join(root, name.String()))
		if err != nil {
			t.Fatal(err)
		}
		got, err = process.ResolveExecutable(t.Context(), candidate)
		want, nativeErr = exec.LookPath(candidate.String())
		if nativeErr != nil {
			for cause := errors.Unwrap(nativeErr); cause != nil; cause = errors.Unwrap(nativeErr) {
				nativeErr = cause
			}
			if !errors.Is(err, nativeErr) || !errors.Is(err, core.ErrProcessContract) || got != (core.AbsolutePath{}) {
				t.Fatalf("absolute lookup lost native refusal: %v, want %v", err, nativeErr)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != want {
			t.Fatalf("absolute lookup changed Go's answer: %v, %v, want %q", got, err, want)
		}
	})
}
