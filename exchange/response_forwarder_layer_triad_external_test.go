package exchange_test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/testserial"
)

var errForwardedNativeWrite = errors.New("native write refused")

// forwardedNativeWriter records exactly what reached the native writer. It
// never commits an implicit status, so any status it observes was forwarded.
type forwardedNativeWriter struct {
	header      http.Header
	writeErr    error
	body        []byte
	statuses    [4]int
	accept      int
	statusCalls int
	writes      int
}

func newForwardedNativeWriter() *forwardedNativeWriter {
	return &forwardedNativeWriter{header: http.Header{}, body: make([]byte, 0, 64), accept: -1}
}

func (w *forwardedNativeWriter) Header() http.Header { return w.header }

func (w *forwardedNativeWriter) WriteHeader(status int) {
	if w.statusCalls < len(w.statuses) {
		w.statuses[w.statusCalls] = status
	}
	w.statusCalls++
}

func (w *forwardedNativeWriter) Write(body []byte) (int, error) {
	w.writes++
	accepted := len(body)
	if w.accept >= 0 && w.accept < accepted {
		accepted = w.accept
	}
	w.body = append(w.body, body[:accepted]...)
	return accepted, w.writeErr
}

type flushingNativeWriter struct {
	*forwardedNativeWriter
	flushes int
}

func (w *flushingNativeWriter) Flush() { w.flushes++ }

type hijackingNativeWriter struct {
	*forwardedNativeWriter
	hijacks int
}

func (w *hijackingNativeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacks++
	return nil, nil, nil
}

// unwrapOnlyDecorator is a caller decorator that exposes its wrapped writer to
// Go's ResponseController but implements neither Flush nor Hijack itself.
type unwrapOnlyDecorator struct{ http.ResponseWriter }

func (d unwrapOnlyDecorator) Unwrap() http.ResponseWriter { return d.ResponseWriter }

func mustResponseForwarder(t *testing.T, writer http.ResponseWriter) exchange.ResponseForwarder {
	t.Helper()
	forwarder, err := exchange.NewResponseForwarder(writer)
	if err != nil {
		t.Fatalf("NewResponseForwarder() error = %v, want nil", err)
	}
	return forwarder
}

// The forwarder is the effect owner for caller-owned writer decorators. Each
// row proves one native fact reaches, or is refused before, the wrapped writer.
func TestResponseForwarderLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive status precedes body with no added header or framing", func(t *testing.T) {
		t.Parallel()
		native := newForwardedNativeWriter()
		forwarder := mustResponseForwarder(t, native)
		if err := forwarder.WriteHeader(http.StatusEarlyHints); err != nil {
			t.Fatal(err)
		}
		if err := forwarder.WriteHeader(http.StatusCreated); err != nil {
			t.Fatal(err)
		}
		n, err := forwarder.Write([]byte("a\x00b"))
		if n != 3 || err != nil {
			t.Fatalf("Write() = (%d, %v), want (3, nil)", n, err)
		}
		if native.statusCalls != 2 || native.statuses[0] != http.StatusEarlyHints || native.statuses[1] != http.StatusCreated || !bytes.Equal(native.body, []byte("a\x00b")) || native.writes != 1 || len(native.header) != 0 {
			t.Fatalf("native = statuses %v (%d) body %q writes %d header %v, want 103,201 then exact body once and no header", native.statuses, native.statusCalls, native.body, native.writes, native.header)
		}
	})
	t.Run("positive flush follows Unwrap to the native flusher", func(t *testing.T) {
		t.Parallel()
		native := &flushingNativeWriter{forwardedNativeWriter: newForwardedNativeWriter()}
		if err := mustResponseForwarder(t, unwrapOnlyDecorator{native}).Flush(); err != nil {
			t.Fatalf("Flush() error = %v, want nil", err)
		}
		if native.flushes != 1 || native.statusCalls != 0 {
			t.Fatalf("native flushes = %d statuses = %d, want one flush and no forwarder-invented status", native.flushes, native.statusCalls)
		}
	})
	t.Run("positive hijack hands the real loopback connection to the caller", func(t *testing.T) {
		t.Parallel()
		handled := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			handled <- hijackAndAnswer(writer)
		}))
		t.Cleanup(server.Close)
		client := server.Client()
		t.Cleanup(client.CloseIdleConnections)
		response, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr, <-handled); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusAccepted || string(body) != "raw" {
			t.Fatalf("hijacked response = (%d, %q), want (202, %q) written on the taken connection", response.StatusCode, body, "raw")
		}
	})
	t.Run("negative absent and typed-nil writers are refused before any effect", func(t *testing.T) {
		t.Parallel()
		for _, writer := range []http.ResponseWriter{nil, (*httptest.ResponseRecorder)(nil)} {
			if _, err := exchange.NewResponseForwarder(writer); !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("NewResponseForwarder(%T) error = %v, want %v", writer, err, core.ErrExchangeContract)
			}
		}
		var zero exchange.ResponseForwarder
		_, _, hijackErr := zero.Hijack()
		n, writeErr := zero.Write([]byte("x"))
		for _, effect := range []struct {
			err  error
			name string
		}{{name: "Validate", err: zero.Validate()}, {name: "WriteHeader", err: zero.WriteHeader(http.StatusOK)}, {name: "Write", err: writeErr}, {name: "Flush", err: zero.Flush()}, {name: "Hijack", err: hijackErr}} {
			if !errors.Is(effect.err, core.ErrExchangeContract) {
				t.Errorf("zero forwarder %s error = %v, want %v", effect.name, effect.err, core.ErrExchangeContract)
			}
		}
		if n != 0 {
			t.Fatalf("zero forwarder Write count = %d, want 0", n)
		}
	})
	t.Run("negative native partial write keeps its count and exact cause", func(t *testing.T) {
		t.Parallel()
		native := newForwardedNativeWriter()
		native.accept, native.writeErr = 2, errForwardedNativeWrite
		n, err := mustResponseForwarder(t, native).Write([]byte("abcd"))
		if n != 2 || !errors.Is(err, errForwardedNativeWrite) || string(native.body) != "ab" {
			t.Fatalf("Write() = (%d, %v) body %q, want (2, the native cause) and native prefix %q", n, err, native.body, "ab")
		}
	})
	t.Run("negative unsupported flush reports Go's sentinel and commits nothing", func(t *testing.T) {
		t.Parallel()
		native := newForwardedNativeWriter()
		if err := mustResponseForwarder(t, native).Flush(); !errors.Is(err, http.ErrNotSupported) {
			t.Fatalf("Flush() error = %v, want %v", err, http.ErrNotSupported)
		}
		if native.statusCalls != 0 || native.writes != 0 {
			t.Fatalf("native statuses = %d writes = %d, want no effect", native.statusCalls, native.writes)
		}
	})
	t.Run("negative hijack does not reach past a decorator that declined it", func(t *testing.T) {
		t.Parallel()
		native := &hijackingNativeWriter{forwardedNativeWriter: newForwardedNativeWriter()}
		conn, buffered, err := mustResponseForwarder(t, unwrapOnlyDecorator{native}).Hijack()
		if !errors.Is(err, http.ErrNotSupported) || conn != nil || buffered != nil || native.hijacks != 0 {
			t.Fatalf("Hijack() = (%v, %v, %v) native hijacks %d, want unsupported without reaching the native hijacker", conn, buffered, err, native.hijacks)
		}
		if _, _, err := mustResponseForwarder(t, native).Hijack(); err != nil || native.hijacks != 1 {
			t.Fatalf("direct Hijack() error = %v hijacks = %d, want the native hijacker once", err, native.hijacks)
		}
	})
	t.Run("negative invalid status keeps net/http's own refusal", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		forwarder := mustResponseForwarder(t, recorder)
		recovered := func() (value any) {
			defer func() { value = recover() }()
			_ = forwarder.WriteHeader(99)
			return nil
		}()
		if recovered == nil || recorder.Code != http.StatusOK || recorder.Body.Len() != 0 {
			t.Fatalf("WriteHeader(99) recovered %v recorder (%d, %q), want net/http's panic and no committed response", recovered, recorder.Code, recorder.Body.Bytes())
		}
	})
	t.Run("neutral empty body is one zero-byte native write without an invented status", func(t *testing.T) {
		t.Parallel()
		native := newForwardedNativeWriter()
		n, err := mustResponseForwarder(t, native).Write(nil)
		if n != 0 || err != nil || native.writes != 1 || native.statusCalls != 0 || len(native.body) != 0 || len(native.header) != 0 {
			t.Fatalf("Write(nil) = (%d, %v) native writes %d statuses %d body %q header %v, want one empty native write and nothing else", n, err, native.writes, native.statusCalls, native.body, native.header)
		}
	})
}

func hijackAndAnswer(writer http.ResponseWriter) error {
	forwarder, err := exchange.NewResponseForwarder(writer)
	if err != nil {
		return err
	}
	conn, buffered, err := forwarder.Hijack()
	if err != nil {
		return err
	}
	_, writeErr := buffered.WriteString("HTTP/1.1 202 Accepted\r\nContent-Length: 3\r\nConnection: close\r\n\r\nraw")
	return errors.Join(writeErr, buffered.Flush(), conn.Close())
}

// Production decorators forward precomputed status and body slices on their
// hottest path, so the forwarder must add no allocation to that path.
func TestResponseForwarderHotPathAddsNoAllocation(t *testing.T) {
	// AllocsPerRun observes process-wide allocation and sets GOMAXPROCS to 1,
	// so this test runs serially under the declared isolation hazard.
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation,
		Scope:  core.TestIsolationScopePackageProcess,
	})
	native := newForwardedNativeWriter()
	blob := []byte("precomputed response blob")
	var forwardErr error
	allocations := testing.AllocsPerRun(100, func() {
		native.body = native.body[:0]
		forwarder, err := exchange.NewResponseForwarder(native)
		if err == nil {
			err = forwarder.WriteHeader(http.StatusOK)
		}
		if err == nil {
			_, err = forwarder.Write(blob)
		}
		forwardErr = errors.Join(forwardErr, err)
	})
	if forwardErr != nil || allocations != 0 || !bytes.Equal(native.body, blob) {
		t.Fatalf("hot path allocations = %v error = %v body %q, want 0 allocations, nil and the exact blob", allocations, forwardErr, native.body)
	}
}
