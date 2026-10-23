package process_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestZeroOutputStreamRefusesWithoutBytes(t *testing.T) {
	t.Parallel()
	var output process.OutputStream
	n, err := output.Write([]byte("must not escape"))
	if n != 0 || !errors.Is(err, core.ErrProcessContract) || !errors.Is(output.Close(), core.ErrProcessContract) || !errors.Is(output.Validate(), core.ErrProcessContract) {
		t.Fatalf("zero OutputStream.Write=%d/%v, want zero bytes and contract refusal on write/close/validation", n, err)
	}
}

// Global stdout belongs only to this isolated helper process. Every parent
// command has cancellation, exactly one Wait and the package backstop.
func TestStandardOutputCopyHelper(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessOutput, Scope: core.TestIsolationScopePackageProcess})
	role, _, selected := helperBehavior()
	if !selected {
		return
	}
	if role == "nil-output" {
		os.Stdout = nil
		output, err := process.StandardOutput()
		if !errors.Is(err, core.ErrProcessContract) || output.Validate() == nil {
			os.Exit(91)
		}
		os.Exit(0)
	}
	var nativeClosed error
	if role == "closed-output" {
		if err := os.Stdout.Close(); err != nil {
			os.Exit(92)
		}
		raw, err := os.Stdout.SyscallConn()
		nativeClosed = err
		if err == nil {
			called := false
			nativeClosed = raw.Control(func(uintptr) { called = true })
			if called {
				os.Exit(98)
			}
		}
		if nativeClosed == nil {
			os.Exit(99)
		}
	}
	output, err := process.StandardOutput()
	if role == "closed-output" {
		if !errors.Is(err, core.ErrProcessStream) || !errors.Is(err, nativeClosed) || output.Validate() == nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(93)
		}
		os.Exit(0)
	}
	if err != nil || output.Validate() != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(94)
	}
	_, copyErr := io.Copy(output, os.Stdin)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		_, _ = fmt.Fprintln(os.Stderr, errors.Join(copyErr, closeErr))
		os.Exit(95)
	}
	nativeCount, nativeErr := os.Stdout.Write([]byte("must remain closed"))
	if nativeCount != 0 || !errors.Is(nativeErr, os.ErrClosed) {
		_, _ = fmt.Fprintln(os.Stderr, "owned stdout after Close got", nativeCount, nativeErr, "want zero and closed")
		os.Exit(97)
	}
	n, writeErr := output.Write([]byte("after close"))
	again := output.Close()
	if n != 0 || !errors.Is(writeErr, core.ErrProcessStream) || !errors.Is(writeErr, os.ErrClosed) || !errors.Is(again, core.ErrProcessStream) || !errors.Is(again, os.ErrClosed) {
		os.Exit(96)
	}
	os.Exit(0)
}

func FuzzStandardOutputPreservesBytes(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("native output"))
	f.Add(bytes.Repeat([]byte{0, 0xff, 1, 2}, 17500))
	f.Fuzz(func(t *testing.T, data []byte) {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputCopyHelper$", "--", "copy-output")
		var output, diagnostics bytes.Buffer
		cmd.Stdin = bytes.NewReader(data)
		cmd.Stdout = &output
		cmd.Stderr = &diagnostics
		if err := cmd.Run(); err != nil || !bytes.Equal(output.Bytes(), data) {
			t.Fatalf("native OutputStream bytes=%d/%d, run=%v, stderr=%q; want exact bytes and joined clean close", output.Len(), len(data), err, diagnostics.String())
		}
	})
}

func TestStandardOutputRefusalAndPipeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"nil-output", "empty-output", "copy-output"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputCopyHelper$", "--", role)
			var output, diagnostics bytes.Buffer
			payload := bytes.Repeat([]byte("ordinary pipe bytes\x00"), 1000)
			if role == "empty-output" {
				payload = nil
			}
			cmd.Stdin = bytes.NewReader(payload)
			cmd.Stdout = &output
			cmd.Stderr = &diagnostics
			err = cmd.Run()
			want := 0
			if role == "copy-output" {
				want = len(payload)
			}
			if err != nil || output.Len() != want || role == "copy-output" && !bytes.Equal(output.Bytes(), payload) {
				t.Fatalf("StandardOutput(%s) bytes=%d, run=%v, stderr=%q; want %d exact bytes and clean child exit", role, output.Len(), err, diagnostics.String(), want)
			}
		})
	}
}

// Standard output remains a Process capability. Raw socket shutdown remains
// a cataloged native transport effect, never a second application executor.
func TestStandardOutputUsesPrimitiveCatalogAuthority(t *testing.T) {
	t.Parallel()
	catalog, err := capabilities.All()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for capability := range catalog.Capabilities() {
		if capability.Package == core.PackageProcess {
			found = capability.Owns(capabilities.EffectProcess)
		}
	}
	path, err := gomodule.ParseImportPath("syscall")
	if err != nil {
		t.Fatal(err)
	}
	selector, err := capabilities.ParseSymbolName("Shutdown")
	if err != nil {
		t.Fatal(err)
	}
	fact, err := capabilities.ResolveStandardSymbol(capabilities.StandardSymbol{ImportPath: path, Selector: selector})
	if !found || err != nil || fact.Classification.Disposition != capabilities.StandardSymbolEffect || fact.Classification.Effect != capabilities.EffectTransport {
		t.Fatalf("Process owner=%t, native Shutdown=%+v/%v; want compiler-owned Process and raw transport classification", found, fact, err)
	}
}

func TestStandardOutputRegularFile(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	payload := bytes.Repeat([]byte{0x61, 0, 0xff}, 3000)
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputCopyHelper$", "--", "copy-output")
	var diagnostics bytes.Buffer
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = file
	cmd.Stderr = &diagnostics
	if err := cmd.Run(); err != nil {
		t.Fatalf("regular stdout child=%v, stderr=%q; want clean owned close", err, diagnostics.String())
	}
	got, err := os.ReadFile(file.Name())
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("regular stdout bytes=%d/%d, read=%v; want exact bytes after ordinary close", len(got), len(payload), err)
	}
}
