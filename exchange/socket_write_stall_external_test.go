package exchange_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func stallDuration(t testing.TB, value time.Duration) temporal.Duration {
	t.Helper()
	duration, err := temporal.NewDuration(value)
	if err != nil {
		t.Fatal(err)
	}
	return duration
}

// Actual TCP HTTP: the client observes a flushed first record, then a real
// timer crosses the server's old whole-response deadline before the final
// record. The operation is quiet computation, not a stalled socket write.
func TestSocketWriteStallQuietNativeHTTP(t *testing.T) {
	t.Parallel()
	configuration := serverRuntimeConfiguration(t, "127.0.0.1:0")
	configuration.Policy.WriteTimeout = stallDuration(t, 250*time.Millisecond)
	handlerDone := make(chan error, 1)
	quietDone := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := exchange.NewSocketServerCall(w, r)
		if err == nil {
			err = exchange.ServeSocketWithWriteStall(exchange.SocketWriteStallRequest{Call: call, Timeout: configuration.Policy.WriteTimeout, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.WriteString(w, "first\n"); err != nil {
					return
				}
				if err := http.NewResponseController(w).Flush(); err != nil {
					return
				}
				quiet, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: r.Context(), Duration: stallDuration(t, 750*time.Millisecond)})
				if err != nil {
					t.Error(err)
					return
				}
				defer cancel()
				<-quiet.Done()
				close(quietDone)
				_, _ = io.WriteString(w, "last\n")
				_ = http.NewResponseController(w).Flush()
			})})
		}
		handlerDone <- err
	})
	runtime, err := exchange.NewServerRuntime(configuration, handler)
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- runtime.Serve() }()
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Error(err)
		}
		select {
		case err := <-served:
			if err != nil {
				t.Error(err)
			}
		case <-exchangeFixtureBackstop(t, testDeadlockBackstop):
			t.Error("owned server did not join")
		}
	})
	if err := <-runtime.Ready(); err != nil {
		t.Fatal(err)
	}
	address, err := runtime.Address()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address.String()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	first, err := reader.ReadString('\n')
	if err != nil || first != "first\n" {
		t.Fatalf("first=%q/%v, want the flushed first record", first, err)
	}
	<-quietDone
	tail, readErr := io.ReadAll(reader)
	if err := <-handlerDone; err != nil || readErr != nil || string(tail) != "last\n" {
		t.Fatalf("quiet response tail=%q read=%v handler=%v, want exact final record after old response deadline", tail, readErr, err)
	}
}

// Native net.Pipe uses Go's real write-deadline implementation without kernel
// socket-buffer guesses. Its peer never consumes a byte, so entered is a real
// blocked write. Synctest controls deadline/cancel facts, not scheduler sleeps.
type stallPipeWriter struct {
	connection net.Conn
	header     http.Header
	entered    chan struct{}
	once       sync.Once
}

func (w *stallPipeWriter) Header() http.Header { return w.header }
func (w *stallPipeWriter) WriteHeader(int)     {}
func (w *stallPipeWriter) Write(b []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	return w.connection.Write(b)
}
func (w *stallPipeWriter) FlushError() error { return nil }
func (w *stallPipeWriter) SetWriteDeadline(deadline time.Time) error {
	return w.connection.SetWriteDeadline(deadline)
}

func TestSocketWriteStallBlockedNativeWriter(t *testing.T) {
	t.Parallel()
	for _, canceled := range []bool{false, true} {
		name := "socket deadline ends blocked write"
		if canceled {
			name = "parent cancellation expires blocked write and preserves cause"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				server, peer := net.Pipe()
				defer server.Close()
				defer peer.Close()
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)
				writer := &stallPipeWriter{connection: server, header: make(http.Header), entered: make(chan struct{})}
				request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
				call, err := exchange.NewSocketServerCall(writer, request)
				if err != nil {
					t.Fatal(err)
				}
				finished := make(chan error, 1)
				go func() {
					finished <- exchange.ServeSocketWithWriteStall(exchange.SocketWriteStallRequest{Call: call, Timeout: stallDuration(t, time.Second), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("blocked")) })})
				}()
				synctest.Wait()
				select {
				case <-writer.entered:
				default:
					t.Fatal("handler never entered its owned native write")
				}
				cause := errors.New("owned caller canceled")
				if canceled {
					cancel(cause)
				} else if err := temporal.Wait(temporal.WaitRequest{Context: t.Context(), Duration: stallDuration(t, time.Second)}); err != nil {
					t.Fatal(err)
				}
				synctest.Wait()
				err = <-finished
				if !errors.Is(err, core.ErrExchangeWrite) || !errors.Is(err, os.ErrDeadlineExceeded) || canceled && (!errors.Is(err, cause) || !errors.Is(err, context.Canceled)) {
					t.Fatalf("blocked writer=%v, want native deadline and exact optional caller cause", err)
				}
			})
		})
	}
}

type stallRecordWriter struct {
	header        http.Header
	deadline      time.Time
	deadlineCalls int
	writes        int
	writeErr      error
	deadlineErr   error
	duringWrite   func()
}

func (w *stallRecordWriter) Header() http.Header { return w.header }
func (w *stallRecordWriter) WriteHeader(int)     {}
func (w *stallRecordWriter) Write(b []byte) (int, error) {
	w.writes++
	if w.duringWrite != nil {
		w.duringWrite()
	}
	return len(b), w.writeErr
}
func (w *stallRecordWriter) FlushError() error { return nil }
func (w *stallRecordWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlineCalls++
	w.deadline = deadline
	return w.deadlineErr
}

func TestSocketWriteStallAdmissionAndFinalBound(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"zero timeout", "unsupported native writer", "native deadline refusal", "canceled ingress", "neutral empty response retains final flush bound", "write failure is never rearmed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			writer := &stallRecordWriter{header: make(http.Header)}
			var native http.ResponseWriter = writer
			timeout := stallDuration(t, time.Second)
			cause := errors.New("native owned refusal")
			var want error
			switch name {
			case "zero timeout":
				timeout, want = temporal.Duration{}, core.ErrExchangeContract
			case "unsupported native writer":
				native, want = httptest.NewRecorder(), http.ErrNotSupported
			case "native deadline refusal":
				writer.deadlineErr, want = cause, cause
			case "canceled ingress":
				cancel(cause)
				want = cause
			case "write failure is never rearmed":
				writer.writeErr, want = cause, cause
			}
			call, err := exchange.NewSocketServerCall(native, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			err = exchange.ServeSocketWithWriteStall(exchange.SocketWriteStallRequest{Call: call, Timeout: timeout, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Context() != ctx {
					t.Error("request context changed")
				}
				if writer.writeErr != nil {
					_, first := w.Write([]byte("one"))
					_, second := w.Write([]byte("two"))
					if !errors.Is(first, cause) || !errors.Is(second, cause) {
						t.Errorf("write errors=%v/%v, want original cause twice", first, second)
					}
				}
			})})
			if want != nil {
				if !errors.Is(err, want) || calls != 0 && name != "write failure is never rearmed" {
					t.Fatalf("admission=%v calls=%d, want cause %v before handler effects", err, calls, want)
				}
			} else if err != nil || calls != 1 || writer.deadline.IsZero() {
				t.Fatalf("empty response=%v calls=%d deadline=%v, want once with bounded final flush", err, calls, writer.deadline)
			}
			if name == "write failure is never rearmed" && (writer.writes != 1 || writer.deadlineCalls != 2) {
				t.Fatalf("failure writes=%d deadlines=%d, want one admitted write and no subsequent rearm", writer.writes, writer.deadlineCalls)
			}
		})
	}
}

func TestSocketWriteStallBufferedExpiryCannotRearm(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		writer := &stallRecordWriter{header: make(http.Header), duringWrite: func() {
			if err := temporal.Wait(temporal.WaitRequest{Context: t.Context(), Duration: stallDuration(t, 2*time.Second)}); err != nil {
				t.Error(err)
			}
		}}
		call, err := exchange.NewSocketServerCall(writer, httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		err = exchange.ServeSocketWithWriteStall(exchange.SocketWriteStallRequest{Call: call, Timeout: stallDuration(t, time.Second), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, first := w.Write([]byte("buffered"))
			_, second := w.Write([]byte("later"))
			if !errors.Is(first, os.ErrDeadlineExceeded) || !errors.Is(second, os.ErrDeadlineExceeded) {
				t.Errorf("buffered deadline=%v/%v, want persistent expiry", first, second)
			}
		})})
		if !errors.Is(err, os.ErrDeadlineExceeded) || writer.writes != 1 || writer.deadlineCalls != 2 {
			t.Fatalf("expired response=%v writes=%d deadline calls=%d, want no expired rearm", err, writer.writes, writer.deadlineCalls)
		}
	})
}
