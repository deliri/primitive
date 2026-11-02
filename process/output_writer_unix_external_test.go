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

const (
	outputWriterMarker = "PRIMITIVE_OUTPUT_WRITER_MARKER"
	outputWriterRoute  = "PRIMITIVE_OUTPUT_WRITER_ROUTE"
	outputWriterMode   = "PRIMITIVE_OUTPUT_WRITER_MODE"
)

func TestOutputWriterNativePipesPreserveReturnsAndDefers(t *testing.T) {
	t.Parallel()
	for _, stream := range []process.Stream{process.StreamStdout, process.StreamStderr} {
		for _, mode := range []string{"broken", "empty", "success"} {
			t.Run(stream.String()+"/"+mode, func(t *testing.T) {
				t.Parallel()
				marker := filepath.Join(t.TempDir(), "returned")
				executable, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				command := exec.CommandContext(t.Context(), executable, "-test.run=^TestOutputWriterNativePipeChild$")
				command.Env = append(os.Environ(), outputWriterMarker+"="+marker, outputWriterRoute+"="+stream.String(), outputWriterMode+"="+mode)
				var output, diagnostics bytes.Buffer
				command.Stdout, command.Stderr = &output, &diagnostics
				if mode != "success" {
					read, write, err := os.Pipe()
					if err != nil {
						t.Fatal(err)
					}
					if err := read.Close(); err != nil {
						t.Fatal(err)
					}
					defer write.Close()
					if stream == process.StreamStdout {
						command.Stdout = write
					} else {
						command.Stderr = write
					}
				}
				if err := command.Run(); err != nil {
					t.Fatalf("native child = %v, stdout %q stderr %q, want returned typed observation", err, output.Bytes(), diagnostics.Bytes())
				}
				data, err := os.ReadFile(marker)
				if err != nil || string(data) != "defer completed" {
					t.Fatalf("child defer marker = %q/%v, want defer completed", data, err)
				}
				wantOutput, wantDiagnostics := "", ""
				if mode == "success" {
					if stream == process.StreamStdout {
						wantOutput = "oneone"
					} else {
						wantDiagnostics = "oneone"
					}
				}
				if output.String() != wantOutput || diagnostics.String() != wantDiagnostics {
					t.Fatalf("native outputs = %q/%q, want %q/%q", output.Bytes(), diagnostics.Bytes(), wantOutput, wantDiagnostics)
				}
			})
		}
	}
}

func TestOutputWriterNativePipeChild(t *testing.T) {
	t.Parallel()
	marker := os.Getenv(outputWriterMarker)
	if marker == "" {
		return
	}
	result := func() error {
		defer func() {
			if err := os.WriteFile(marker, []byte("defer completed"), 0600); err != nil {
				os.Exit(3)
			}
		}()
		streams, err := process.StandardStreams()
		if err != nil {
			return err
		}
		stream := process.StreamStdout
		if os.Getenv(outputWriterRoute) == process.StreamStderr.String() {
			stream = process.StreamStderr
		}
		writer, err := streams.OutputWriter(stream)
		if err != nil {
			return err
		}
		switch os.Getenv(outputWriterMode) {
		case "empty":
			n, err := writer.Write(nil)
			if n != 0 || err != nil {
				return syscall.EINVAL
			}
		case "success":
			for range 2 {
				if n, err := writer.Write([]byte("one")); n != 3 || err != nil {
					return syscall.EINVAL
				}
			}
		case "broken":
			n, err := writer.Write([]byte("native-broken-pipe"))
			if n != 0 || !errors.Is(err, core.ErrProcessStream) || !errors.Is(err, syscall.EPIPE) {
				return syscall.EINVAL
			}
		default:
			return syscall.EINVAL
		}
		return nil
	}()
	if result != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
