package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The real ResponseController invokes an opaque setter. Only the selected
// setter blocks in native pipe I/O, and applies its deadline AFTER the read.
// That ordering exposes a stale arm/clear overwriting an earlier cancellation.
type stallOrderingWriter struct {
	header      http.Header
	pipe        *io.PipeWriter
	entered     chan struct{}
	expired     chan struct{}
	expiredOnce sync.Once
	mutex       sync.Mutex
	calls       int
	blockCall   int
	deadline    time.Time
	initial     time.Time
	setterErr   error
	writes      int
}

func (w *stallOrderingWriter) Header() http.Header { return w.header }
func (w *stallOrderingWriter) WriteHeader(int)     {}
func (w *stallOrderingWriter) Write(data []byte) (int, error) {
	w.writes++
	return len(data), nil
}
func (w *stallOrderingWriter) SetWriteDeadline(deadline time.Time) error {
	w.mutex.Lock()
	w.calls++
	block := w.calls == w.blockCall
	w.mutex.Unlock()
	if block {
		close(w.entered)
		if _, err := w.pipe.Write([]byte{0xa5}); err != nil {
			return err
		}
	}
	w.mutex.Lock()
	w.deadline = deadline
	w.mutex.Unlock()
	if !deadline.IsZero() && !deadline.After(w.initial) {
		w.expiredOnce.Do(func() { close(w.expired) })
	}
	if block {
		return w.setterErr
	}
	return nil
}

func TestSocketWriteStallOpaqueSetterOrdering(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		blockCall int
		cancel    bool
		refuse    bool
	}{
		{name: "blocked arm permits cancellation and cannot rearm it", blockCall: 2, cancel: true},
		{name: "blocked clear permits cancellation and cannot erase it", blockCall: 3, cancel: true},
		{name: "blocked arm refusal preserves native cause", blockCall: 2, refuse: true},
		{name: "blocked clear refusal preserves native cause", blockCall: 3, refuse: true},
		{name: "released arm retains successful write and final bound", blockCall: 2},
		{name: "released clear retains successful write and final bound", blockCall: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(nil)
				reader, writer := io.Pipe()
				defer reader.Close()
				defer writer.Close()
				initial, err := socketWriteDeadline(temporal.Duration{})
				if err != nil {
					t.Fatal(err)
				}
				nativeCause := errors.New("owned deadline setter refusal")
				native := &stallOrderingWriter{header: make(http.Header), pipe: writer, entered: make(chan struct{}), expired: make(chan struct{}), blockCall: tc.blockCall, initial: initial}
				if tc.refuse {
					native.setterErr = nativeCause
				}
				call, err := NewSocketServerCall(native, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
				if err != nil {
					t.Fatal(err)
				}
				timeout, err := temporal.NewDuration(time.Second)
				if err != nil {
					t.Fatal(err)
				}
				borrowed := make(chan *socketStallWriter, 1)
				finished := make(chan error, 1)
				go func() {
					finished <- ServeSocketWithWriteStall(SocketWriteStallRequest{Call: call, Timeout: timeout, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						borrowed <- w.(*socketStallWriter)
						_, _ = w.Write([]byte("owned response"))
					})})
				}()
				synctest.Wait()
				select {
				case <-native.entered:
				default:
					t.Fatal("opaque setter entered native I/O = false, want true")
				}
				owned := <-borrowed
				mutexAvailable := owned.mutex.TryLock()
				if !mutexAvailable {
					t.Error("state mutex available during native setter I/O = false, want true")
				} else {
					owned.mutex.Unlock()
				}
				callerCause := errors.New("owned caller cancellation")
				if tc.cancel {
					cancel(callerCause)
					// Mutex waits are not durably blocked in synctest. The old
					// implementation cannot advance its callback until the pipe
					// is released, so retain that failed fact without wedging the
					// test's own barrier behind the known held mutex.
					if mutexAvailable {
						synctest.Wait()
					}
					select {
					case <-native.expired:
					default:
						t.Error("cancellation expired response before releasing older setter = false, want true")
					}
				}
				var octet [1]byte
				if n, err := io.ReadFull(reader, octet[:]); n != 1 || err != nil || octet[0] != 0xa5 {
					t.Fatalf("native setter I/O = (%d,%v,%x), want (1,nil,a5)", n, err, octet)
				}
				synctest.Wait()
				var got error
				select {
				case got = <-finished:
				default:
					t.Fatal("handler and owned cancellation callback joined = false, want true")
				}
				if tc.cancel {
					if !errors.Is(got, context.Canceled) || !errors.Is(got, callerCause) || !errors.Is(got, core.ErrExchangeWrite) {
						t.Errorf("canceled response error = %v, want exact caller cause and write failure", got)
					}
					if native.deadline.IsZero() || native.deadline.After(initial) {
						t.Errorf("deadline after released stale setter = %v, want expired at or before %v", native.deadline, initial)
					}
				} else if tc.refuse {
					if !errors.Is(got, nativeCause) || !errors.Is(got, core.ErrExchangeWrite) {
						t.Errorf("setter refusal error = %v, want exact native cause and write failure", got)
					}
				} else if got != nil || native.writes != 1 || !native.deadline.After(initial) {
					t.Errorf("released response = (%v,%d,%v), want nil, one write and finite future deadline", got, native.writes, native.deadline)
				}
			})
		})
	}
}
