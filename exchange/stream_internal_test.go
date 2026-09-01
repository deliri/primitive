package exchange

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type delayedProbeReader struct {
	emptyReads int
	reads      int
}

type emptyForeverReader struct{}

func (emptyForeverReader) Read([]byte) (int, error) { return 0, nil }

func (r *delayedProbeReader) Read(buffer []byte) (int, error) {
	if r.reads < r.emptyReads {
		r.reads++
		return 0, nil
	}
	buffer[0] = 'x'
	return 1, nil
}

func TestDownloadOverflowProbeDoesNotConfuseAStallWithEOF(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		want       error
		name       string
		emptyReads int
	}{
		{
			name:       "one transient empty read still discovers overflow",
			emptyReads: 1,
			want:       core.ErrExchangeBodyLimit,
		},
		{
			name:       "last admitted empty read still discovers overflow",
			emptyReads: core.ReaderConsecutiveEmptyReadMaximum - 1,
			want:       core.ErrExchangeBodyLimit,
		},
		{
			name:       "the shared empty-read ceiling refuses no progress",
			emptyReads: core.ReaderConsecutiveEmptyReadMaximum,
			want:       io.ErrNoProgress,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			reader := &delayedProbeReader{emptyReads: testCase.emptyReads}
			written, gotErr := probeDownloadEnd(downloadCopyRequest{
				context: context.Background(), source: reader,
			}, 7)
			if written != 7 || !errors.Is(gotErr, testCase.want) {
				t.Fatalf("probeDownloadEnd() = (%d, %v), want (7, %v)",
					written, gotErr, testCase.want)
			}
		})
	}
}

func TestDownloadTransferRefusesAnUnendingEmptyReader(t *testing.T) {
	t.Parallel()

	limit, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("core.NewByteCount(1) error = %v, want nil", err)
	}
	written, gotErr := copyDownload(downloadCopyRequest{
		context: context.Background(), source: emptyForeverReader{},
		destination: io.Discard, limit: limit,
	})
	if written != 0 || !errors.Is(gotErr, io.ErrNoProgress) {
		t.Fatalf("copyDownload(empty reader) = (%d, %v), want (0, %v)",
			written, gotErr, io.ErrNoProgress)
	}
}

func TestZeroExtentBoundariesDoNotConfuseAStallWithEOF(t *testing.T) {
	t.Parallel()

	t.Run("bodyless request still discovers a body after one stall", func(t *testing.T) {
		t.Parallel()

		request := &http.Request{Body: io.NopCloser(&delayedProbeReader{emptyReads: 1})}
		gotErr := refuseRequestBody(request)
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("refuseRequestBody(stalled body) error = %v, want %v and %v",
				gotErr, core.ErrExchangeRequest, core.ErrExchangeContract)
		}
	})

	t.Run("bodyless request refuses an unending stall", func(t *testing.T) {
		t.Parallel()

		request := &http.Request{Body: io.NopCloser(emptyForeverReader{})}
		gotErr := refuseRequestBody(request)
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, io.ErrNoProgress) {
			t.Fatalf("refuseRequestBody(empty reader) error = %v, want %v and %v",
				gotErr, core.ErrExchangeRequest, io.ErrNoProgress)
		}
	})

	t.Run("zero response extent still discovers a byte after one stall", func(t *testing.T) {
		t.Parallel()

		gotErr := probeEmptyResponseSource(
			context.Background(), &delayedProbeReader{emptyReads: 1},
		)
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			!errors.Is(gotErr, core.ErrExchangeBodyLimit) {
			t.Fatalf("probeEmptyResponseSource(stalled source) error = %v, want %v and %v",
				gotErr, core.ErrExchangeResponse, core.ErrExchangeBodyLimit)
		}
	})

	t.Run("zero response extent refuses an unending stall", func(t *testing.T) {
		t.Parallel()

		gotErr := probeEmptyResponseSource(context.Background(), emptyForeverReader{})
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			!errors.Is(gotErr, io.ErrNoProgress) {
			t.Fatalf("probeEmptyResponseSource(empty reader) error = %v, want %v and %v",
				gotErr, core.ErrExchangeResponse, io.ErrNoProgress)
		}
	})
}

// dirtyTransferBuffer fills every byte with a nonzero value so a partial scrub
// leaves observable residue anywhere in the extent.
func dirtyTransferBuffer(buffer *transferBuffer) {
	for index := range buffer {
		buffer[index] = byte(index%255 + 1)
	}
}

func firstNonZeroIndex(buffer *transferBuffer) int {
	for index, value := range buffer {
		if value != 0 {
			return index
		}
	}
	return -1
}

func TestTransferBufferMatchesDeclaredExtent(t *testing.T) {
	t.Parallel()

	buffer := acquireTransferBuffer()
	defer releaseTransferBuffer(buffer)

	if len(buffer) != TransferBufferBytes {
		t.Fatalf(
			"transfer buffer extent = %d, want the declared %d",
			len(buffer),
			TransferBufferBytes,
		)
	}
}

func TestTransferBufferScrubErasesEveryByte(t *testing.T) {
	t.Parallel()

	var buffer transferBuffer
	dirtyTransferBuffer(&buffer)

	scrubTransferBuffer(&buffer)

	if index := firstNonZeroIndex(&buffer); index >= 0 {
		t.Fatalf(
			"scrubTransferBuffer() byte %d = %d, want 0",
			index,
			buffer[index],
		)
	}
}

// TestTransferBufferReleaseScrubsBeforePoolReturn proves the ownership-safe
// release order structurally. A runtime test cannot inspect a buffer after Put:
// once returned to the shared pool, the test no longer owns that memory.
func TestTransferBufferReleaseScrubsBeforePoolReturn(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	file, gotParseErr := parser.ParseFile(set, "stream.go", nil, 0)
	if gotParseErr != nil {
		t.Fatalf(
			"parser.ParseFile(stream.go) error = %v, want nil",
			gotParseErr,
		)
	}
	got, gotErr := functionStatementText(
		set,
		file,
		"releaseTransferBuffer",
	)
	if gotErr != nil {
		t.Fatalf(
			"releaseTransferBuffer statement inspection error = %v, want nil",
			gotErr,
		)
	}
	want := []string{
		"scrubTransferBuffer(buffer)",
		"transferBuffers.Put(buffer)",
	}
	if !slices.Equal(got, want) {
		t.Fatalf(
			"releaseTransferBuffer statements = %q, want exactly %q",
			got,
			want,
		)
	}
}

// TestTransferBufferAcquireAlwaysYieldsScrubbedExtent proves the invariant every
// transfer depends on: an acquired buffer never carries another transfer's
// bytes, whether the pool returns a reused buffer or constructs a new one.
func TestTransferBufferAcquireAlwaysYieldsScrubbedExtent(t *testing.T) {
	t.Parallel()

	const rounds = 64
	for round := range rounds {
		buffer := acquireTransferBuffer()
		if index := firstNonZeroIndex(buffer); index >= 0 {
			t.Fatalf(
				"acquireTransferBuffer() round %d byte %d = %d, want 0",
				round,
				index,
				buffer[index],
			)
		}
		dirtyTransferBuffer(buffer)
		releaseTransferBuffer(buffer)
	}
}

// TestTransferBufferPoolIsSafeUnderConcurrentTransfers pressures the shared pool
// the way concurrent downloads do. Under the race detector this also proves no
// two transfers hold the same buffer at once.
func TestTransferBufferPoolIsSafeUnderConcurrentTransfers(t *testing.T) {
	t.Parallel()

	const (
		workers = 16
		rounds  = 32
	)
	residue := make(chan int, workers*rounds)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			for range rounds {
				buffer := acquireTransferBuffer()
				if index := firstNonZeroIndex(buffer); index >= 0 {
					residue <- index
				}
				dirtyTransferBuffer(buffer)
				releaseTransferBuffer(buffer)
			}
		})
	}
	group.Wait()
	close(residue)

	count := 0
	first := -1
	for index := range residue {
		if first < 0 {
			first = index
		}
		count++
	}
	if count != 0 {
		t.Fatalf(
			"concurrent acquire residue = %d buffers, first dirty byte %d, want 0 buffers",
			count,
			first,
		)
	}
}

// retainingWriter is a plain io.Writer. It deliberately implements neither
// io.ReaderFrom nor any other fast path, so io.CopyBuffer must route the copy
// through the supplied buffer.
type retainingWriter struct {
	written bytes.Buffer
}

func (w *retainingWriter) Write(payload []byte) (int, error) {
	return w.written.Write(payload)
}

// TestCopyBufferUsesTheSuppliedBufferOnlyForPlainWriters pins the documented
// io.CopyBuffer dispatch that decides whether the pooled buffer participates at
// all. io.CopyBuffer states that the buffer is unused when the destination
// implements io.ReaderFrom, so a destination such as io.Discard or bytes.Buffer
// exercises none of the pooled extent. Any measurement that claims to show
// buffer reuse must use a plain writer.
func TestCopyBufferUsesTheSuppliedBufferOnlyForPlainWriters(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("stream-proof-"), 4096)
	cases := []struct {
		destination func() io.Writer
		name        string
		wantUsed    bool
	}{
		{
			name:        "plain writer routes the copy through the buffer",
			destination: func() io.Writer { return &retainingWriter{} },
			wantUsed:    true,
		},
		{
			name:        "io.Discard destination never touches the buffer",
			destination: func() io.Writer { return io.Discard },
		},
		{
			name:        "bytes.Buffer destination never touches the buffer",
			destination: func() io.Writer { return bytes.NewBuffer(nil) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buffer := acquireTransferBuffer()
			defer releaseTransferBuffer(buffer)
			sentinel := bytes.Repeat([]byte{0xA5}, TransferBufferBytes)
			copy(buffer[:], sentinel)
			limited := &io.LimitedReader{
				R: bytes.NewReader(payload),
				N: int64(len(payload)),
			}

			count, gotErr := io.CopyBuffer(tc.destination(), limited, buffer[:])
			if gotErr != nil || count != int64(len(payload)) {
				t.Fatalf(
					"io.CopyBuffer() = (%d, %v), want (%d, nil)",
					count,
					gotErr,
					len(payload),
				)
			}
			gotUsed := !bytes.Equal(buffer[:], sentinel)
			if gotUsed != tc.wantUsed {
				t.Fatalf(
					"io.CopyBuffer() used the supplied buffer = %t, want %t",
					gotUsed,
					tc.wantUsed,
				)
			}
		})
	}
}

// TestTransferBufferPoolAccessIsConfinedToItsOwnHelpers is a structural ratchet.
// Reuse is only safe because every buffer is scrubbed on the way back to the
// pool, and nothing in the type system enforces that. Confining Get and Put to
// the two helpers keeps the scrub on the single return path.
func TestTransferBufferPoolAccessIsConfinedToItsOwnHelpers(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	file, gotErr := parser.ParseFile(set, "stream.go", nil, 0)
	if gotErr != nil {
		t.Fatalf("parser.ParseFile(stream.go) error = %v, want nil", gotErr)
	}
	gotGet := poolMethodCallers(file, "Get")
	gotPut := poolMethodCallers(file, "Put")
	wantGet := []string{"acquireTransferBuffer"}
	wantPut := []string{"releaseTransferBuffer"}
	if !slices.Equal(gotGet, wantGet) {
		t.Fatalf(
			"transferBuffers.Get callers = %q, want exactly %q",
			gotGet,
			wantGet,
		)
	}
	if !slices.Equal(gotPut, wantPut) {
		t.Fatalf(
			"transferBuffers.Put callers = %q, want exactly %q",
			gotPut,
			wantPut,
		)
	}
}

func poolMethodCallers(file *ast.File, method string) []string {
	callers := make([]string, 0, 1)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		if functionCallsPoolMethod(function.Body, method) {
			callers = append(callers, function.Name.Name)
		}
	}
	slices.Sort(callers)
	return callers
}

func functionCallsPoolMethod(body *ast.BlockStmt, method string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != method {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if ok && receiver.Name == "transferBuffers" {
			found = true
			return false
		}
		return true
	})
	return found
}

func functionStatementText(
	set *token.FileSet,
	file *ast.File,
	name string,
) ([]string, error) {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != name || function.Body == nil {
			continue
		}
		statements := make([]string, 0, len(function.Body.List))
		for _, statement := range function.Body.List {
			var rendered bytes.Buffer
			if err := format.Node(&rendered, set, statement); err != nil {
				return nil, err
			}
			statements = append(statements, rendered.String())
		}
		return statements, nil
	}
	return nil, nil
}
