//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/testserial"
)

const (
	pipeDescriptorProbeEnvironment = "PRIMITIVE_FILESTORE_PIPE_DESCRIPTOR_PROBE"
	pipeDescriptorProbeMarker      = "native-capacity"
	pipeDescriptorProbeMaximum     = 64
)

// Only the owned subprocess changes its descriptor limit. Zero and one free
// slots must refuse atomically; two must produce a usable pair and restore both
// slots when the caller closes it. Native os.Pipe supplies the error oracle.
func TestPipeNativeDescriptorCapacityLayerTriad(t *testing.T) {
	if os.Getenv(pipeDescriptorProbeEnvironment) == pipeDescriptorProbeMarker {
		runPipeNativeDescriptorCapacityCases(t)
		return
	}
	t.Parallel()
	ctx, cancel := newFilesystemBackstop(t.Context(), t, time.Minute)
	defer cancel()
	// Resolve the actual compiled test symbol, so a rename changes child selection.
	symbol := runtime.FuncForPC(reflect.ValueOf(TestPipeNativeDescriptorCapacityLayerTriad).Pointer()).Name()
	name := symbol[strings.LastIndexByte(symbol, '.')+1:]
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+name+"$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), pipeDescriptorProbeEnvironment+"="+pipeDescriptorProbeMarker)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	// cmd/go finds the first raw coverage line in test output. Preserve the
	// complete child transcript without letting its narrower scope impersonate
	// the parent package's coverage summary.
	t.Logf("native descriptor subprocess output (base64): %s", base64.StdEncoding.EncodeToString(output.Bytes()))
	if err != nil || ctx.Err() != nil {
		t.Fatalf("native descriptor subprocess = (%v,%v), want complete successful probe", err, ctx.Err())
	}
}

func runPipeNativeDescriptorCapacityCases(t *testing.T) {
	t.Helper()
	for _, tc := range []struct {
		name      string
		available int
		wantPair  bool
	}{
		{name: "zero free descriptors refuse without partial custody"},
		{name: "one free descriptor cannot leak half a pipe", available: 1},
		{name: "two free descriptors transfer exact native capacity", available: 2, wantPair: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			directory := t.TempDir()
			name := filepath.Join(directory, "retained")
			payload := []byte{0, 255, 1, 127}
			if err := os.WriteFile(name, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			// Warm Go's native pipe/poller before freezing descriptor capacity.
			warmReader, warmWriter, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(warmReader.Close(), warmWriter.Close()); err != nil {
				t.Fatal(err)
			}
			var prior syscall.Rlimit
			if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &prior); err != nil {
				t.Fatal(err)
			}
			limited := prior
			limited.Cur = min(prior.Cur, pipeDescriptorProbeMaximum)
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limited); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &prior); err != nil {
					t.Error(err)
				}
			}()
			held := make([]*os.File, 0, pipeDescriptorProbeMaximum)
			defer func() {
				for _, file := range held {
					if file != nil {
						if err := file.Close(); err != nil {
							t.Error(err)
						}
					}
				}
			}()
			exhausted := false
			for range pipeDescriptorProbeMaximum {
				file, err := os.Open(name)
				if errors.Is(err, syscall.EMFILE) {
					exhausted = true
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				held = append(held, file)
			}
			if !exhausted || len(held) < tc.available {
				t.Fatalf("descriptor fixture = (%t,%d), want exhausted capacity with %d releasable slots", exhausted, len(held), tc.available)
			}
			for i := range tc.available {
				index := len(held) - 1 - i
				if err := held[index].Close(); err != nil {
					t.Fatal(err)
				}
				held[index] = nil
			}
			nativeReader, nativeWriter, nativeErr := os.Pipe()
			if tc.wantPair {
				if nativeErr != nil || nativeReader == nil || nativeWriter == nil {
					t.Fatalf("native pair = (%v,%v,%v), want two endpoints", nativeReader, nativeWriter, nativeErr)
				}
				if err := errors.Join(nativeReader.Close(), nativeWriter.Close()); err != nil {
					t.Fatal(err)
				}
			} else {
				if nativeReader != nil {
					_ = nativeReader.Close()
				}
				if nativeWriter != nil {
					_ = nativeWriter.Close()
				}
				if nativeReader != nil || nativeWriter != nil || !errors.Is(nativeErr, syscall.EMFILE) {
					t.Fatalf("native refusal = (%v,%v,%v), want nil endpoints and EMFILE", nativeReader, nativeWriter, nativeErr)
				}
			}
			pipe, gotErr := filestore.OpenPipe(t.Context())
			if tc.wantPair {
				if gotErr != nil || pipe.Validate() != nil {
					t.Fatalf("available pair = (%+v,%v), want admitted custody", pipe, gotErr)
				}
				n, writeErr := pipe.Writer.Write(payload)
				writerCloseErr := pipe.Writer.Close()
				got := make([]byte, len(payload))
				read, readErr := io.ReadFull(pipe.Reader, got)
				var extra [1]byte
				eofN, eofErr := pipe.Reader.Read(extra[:])
				readerCloseErr := pipe.Reader.Close()
				if n != len(payload) || writeErr != nil || writerCloseErr != nil || read != len(payload) || readErr != nil || !bytes.Equal(got, payload) || eofN != 0 || !errors.Is(eofErr, io.EOF) || readerCloseErr != nil {
					t.Fatalf("capacity pair transfer = (%d,%v,%v,%v,%d,%v,%d,%v,%v), want exact bytes and EOF", n, writeErr, writerCloseErr, got, read, readErr, eofN, eofErr, readerCloseErr)
				}
			} else {
				if pipe.Reader != nil {
					_ = pipe.Reader.Close()
				}
				if pipe.Writer != nil && pipe.Writer != pipe.Reader {
					_ = pipe.Writer.Close()
				}
				var native, got *os.SyscallError
				if pipe != (filestore.Pipe{}) || !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.As(nativeErr, &native) || !errors.As(gotErr, &got) || got.Syscall != native.Syscall || !errors.Is(gotErr, native.Err) {
					t.Fatalf("capacity refusal = (%+v,%v), want zero and native source %v", pipe, gotErr, nativeErr)
				}
			}
			// Refill exactly the previously free slots. This detects a leaked native
			// descriptor even when the public failure returned two nil pointers.
			for range tc.available {
				file, err := os.Open(name)
				if err != nil {
					t.Fatalf("restored capacity = %v, want all %d original slots", err, tc.available)
				}
				held = append(held, file)
			}
			extra, err := os.Open(name)
			if extra != nil {
				_ = extra.Close()
			}
			if extra != nil || !errors.Is(err, syscall.EMFILE) {
				t.Fatalf("remaining capacity = (%v,%v), want original exhausted boundary", extra, err)
			}
		})
	}
}
