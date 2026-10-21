//go:build unix

package process_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestStreamsWriteOutputNativeBrokenPipeReturnsThroughDefers(t *testing.T) {
	t.Parallel()
	for _, stream := range []process.Stream{process.StreamStdout, process.StreamStderr} {
		t.Run(stream.String(), func(t *testing.T) {
			t.Parallel()
			marker := filepath.Join(t.TempDir(), "returned")
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := read.Close(); err != nil {
				t.Fatal(err)
			}
			defer write.Close()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), executable, "-test.run=^TestStreamsPipeErrorChild$")
			command.Env = append(os.Environ(), "PRIMITIVE_PIPE_ERROR_MARKER="+marker, "PRIMITIVE_PIPE_ERROR_STREAM="+stream.String())
			var diagnostics bytes.Buffer
			if stream == process.StreamStdout {
				command.Stdout = write
				command.Stderr = &diagnostics
			} else {
				command.Stdout = &diagnostics
				command.Stderr = write
			}
			if err := command.Run(); err != nil {
				t.Fatalf("child=%v/%q, want returned EPIPE rather than signal exit", err, diagnostics.Bytes())
			}
			data, err := os.ReadFile(marker)
			if err != nil || string(data) != "defer completed" {
				t.Fatalf("marker=%q/%v, want completed defer", data, err)
			}
		})
	}
}

func TestStreamsPipeErrorChild(t *testing.T) {
	t.Parallel()
	marker := os.Getenv("PRIMITIVE_PIPE_ERROR_MARKER")
	if marker == "" {
		return
	}
	stream := process.StreamStdout
	if os.Getenv("PRIMITIVE_PIPE_ERROR_STREAM") == process.StreamStderr.String() {
		stream = process.StreamStderr
	}
	observed := func() error {
		defer func() {
			if err := os.WriteFile(marker, []byte("defer completed"), 0600); err != nil {
				os.Exit(3)
			}
		}()
		streams, err := process.StandardStreams()
		if err != nil {
			return err
		}
		switch os.Getenv("PRIMITIVE_PIPE_ERROR_MODE") {
		case "empty":
			count, err := streams.WriteOutput(stream, nil)
			if count.Uint64() != 0 || err != nil {
				return core.ErrProcessContract
			}
			return nil
		case "success":
			for range 2 {
				count, err := streams.WriteOutput(stream, []byte("one"))
				if count.Uint64() != 3 || err != nil {
					return core.ErrProcessContract
				}
			}
			return nil
		}
		count, err := streams.WriteOutput(stream, []byte("native-broken-pipe"))
		if count.Uint64() != 0 || !errors.Is(err, core.ErrProcessStream) || !errors.Is(err, syscall.EPIPE) {
			return core.ErrProcessContract
		}
		return nil
	}()
	if observed != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestStreamsWriteOutputNativeSuccessAndEmpty(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"success", "empty"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			marker := filepath.Join(t.TempDir(), "returned")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), executable, "-test.run=^TestStreamsPipeErrorChild$")
			command.Env = append(os.Environ(), "PRIMITIVE_PIPE_ERROR_MARKER="+marker, "PRIMITIVE_PIPE_ERROR_MODE="+mode)
			var output, diagnostics bytes.Buffer
			command.Stdout = &output
			command.Stderr = &diagnostics
			if mode == "empty" {
				read, write, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				if err := read.Close(); err != nil {
					t.Fatal(err)
				}
				defer write.Close()
				command.Stdout = write
			}
			if err := command.Run(); err != nil {
				t.Fatalf("child=%v/%q, want native success", err, diagnostics.Bytes())
			}
			want := "oneone"
			if mode == "empty" {
				want = ""
			}
			if output.String() != want {
				t.Fatalf("output=%q, want %q", output.String(), want)
			}
			data, err := os.ReadFile(marker)
			if err != nil || string(data) != "defer completed" {
				t.Fatalf("marker=%q/%v, want completed defer", data, err)
			}
		})
	}
}
