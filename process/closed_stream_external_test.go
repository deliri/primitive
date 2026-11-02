//go:build unix

package process_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// A closed native destination keeps Go's own os.ErrClosed cause beside the
// stream identity, and no byte reaches the file.
func TestStreamsWriteOutputClosedNativeFileKeepsNativeCause(t *testing.T) {
	t.Parallel()
	for _, stream := range []process.Stream{process.StreamStdout, process.StreamStderr} {
		t.Run(stream.String(), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "closed")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			streams := process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard}
			if stream == process.StreamStdout {
				streams.Stdout = file
			} else {
				streams.Stderr = file
			}
			written, err := streams.WriteOutput(stream, []byte("refused"))
			if !errors.Is(err, os.ErrClosed) || !errors.Is(err, core.ErrProcessStream) || written.Uint64() != 0 {
				t.Fatalf("closed %s write=%d/%v, want 0 and os.ErrClosed with process stream identity", stream, written.Uint64(), err)
			}
			got, err := os.ReadFile(path)
			if err != nil || len(got) != 0 {
				t.Fatalf("closed destination file=%q/%v, want empty", got, err)
			}
		})
	}
}
