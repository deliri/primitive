package exchange_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const multipartJoinBackstop = 10 * time.Second

type multipartBlockedFile struct {
	file        *os.File
	entered     chan struct{}
	closed      atomic.Int64
	readStarted atomic.Bool
}

// multipartExitingFile observes real native input before exiting its producer
// goroutine. Only the Goexit is a seam; the blocked/read/close effects are os.Pipe.
type multipartExitingFile struct {
	file            *multipartBlockedFile
	reads           atomic.Int64
	closeBeforeExit bool
}

func (s *multipartExitingFile) Read(data []byte) (int, error) {
	_, _ = s.file.Read(data)
	s.reads.Add(1)
	if s.closeBeforeExit {
		// The ownership seam expires the actual native descriptor so deferred
		// Close produces Go's real *PathError rather than a fabricated cause.
		_ = s.file.file.Close()
	}
	runtime.Goexit()
	return 0, io.EOF
}
func (s *multipartExitingFile) Close() error { return s.file.Close() }

func TestMultipartGoexitJoinsNativeOwnedLifetimesAndPreservesFutureCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		kind uint8
	}{
		{name: "consumer exits while native source Read is blocked", kind: 1},
		{name: "iterator exits after admitted native source finishes", kind: 2},
		{name: "source exits after its actual native Read", kind: 3},
		{name: "source exit retains actual native close failure", kind: 4},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			input, output, err := os.Pipe()
			if err != nil {
				t.Fatalf("native input pipe = %v, want acquired", err)
			}
			t.Cleanup(func() { _ = input.Close(); _ = output.Close() })
			active := &multipartBlockedFile{file: input, entered: make(chan struct{})}
			exiting := &multipartExitingFile{file: active, closeBeforeExit: row.kind == 4}
			var source io.ReadCloser = active
			if row.kind >= 3 {
				source = exiting
			}
			future := &multipartCloseObserver{reader: &multipartPatternSource{left: 1}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			writerDone := make(chan struct{})
			writerResult := make(chan error, 1)
			if row.kind != 1 {
				go func() {
					defer close(writerDone)
					_, writeErr := output.Write([]byte{17})
					writerResult <- errors.Join(writeErr, output.Close())
				}()
			} else {
				close(writerDone)
			}
			name, filename := multipartFieldName(t, "file"), multipartFilename(t, "workout.wav")
			done := make(chan struct{})
			borrowed := make(chan io.Reader, 1)
			result := make(chan error, 1)
			var returned, consumerExiting, iteratorExiting atomic.Bool
			go func() {
				defer close(done)
				err := exchange.WithMultipartSource(ctx, exchange.MultipartSourceRequest{
					Parts: func(yield func(exchange.MultipartPart) bool) {
						if !yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: name, FileName: filename, Source: source}) {
							return
						}
						if row.kind == 2 {
							iteratorExiting.Store(true)
							runtime.Goexit()
						}
						yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: name, FileName: filename, Source: future})
					},
					Use: func(ctx context.Context, body exchange.MultipartSource) error {
						borrowed <- body.Reader
						if row.kind == 1 {
							_, parameters, parseErr := mime.ParseMediaType(body.ContentType.String())
							if parseErr != nil {
								return parseErr
							}
							if _, nextErr := multipart.NewReader(body.Reader, parameters["boundary"]).NextPart(); nextErr != nil {
								return nextErr
							}
							select {
							case <-active.entered:
								consumerExiting.Store(true)
								runtime.Goexit()
							case <-ctx.Done():
								return ctx.Err()
							case <-time.After(multipartJoinBackstop):
								return core.ErrExchangeContract
							}
						}
						_, readErr := io.Copy(io.Discard, body.Reader)
						return readErr
					},
				})
				returned.Store(true)
				result <- err
			}()
			defer func() {
				cancel()
				_ = output.Close()
				awaitMultipartJoin(t, writerDone, "native fixture writer")
				awaitMultipartJoin(t, done, "exited multipart scope")
			}()
			awaitMultipartJoin(t, done, "Goexit producer and consuming scope")
			awaitMultipartJoin(t, writerDone, "native fixture writer")
			if active.closed.Load() != 1 || future.closed.Load() != 0 || !active.readStarted.Load() {
				t.Fatalf("Goexit native custody = (closes%d,future%d,read%t), want one admitted close/no future close/actual native Read", active.closed.Load(), future.closed.Load(), active.readStarted.Load())
			}
			if row.kind == 1 {
				if returned.Load() || !consumerExiting.Load() {
					t.Fatalf("consumer lifetime = (returned%t,exiting%t), want actual Goexit with deferred scope join", returned.Load(), consumerExiting.Load())
				}
			} else {
				if !returned.Load() {
					t.Fatal("producer Goexit did not publish its cause and join the consumer")
				}
				resultErr := <-result
				if !errors.Is(resultErr, core.ErrExchangeCallbackExit) || errors.Is(resultErr, core.ErrExchangeCallbackPanic) {
					t.Fatalf("producer exit = %v, want callback-exit identity without callback-panic identity", resultErr)
				}
				if row.kind == 4 {
					var nativeClose *os.PathError
					if !errors.Is(resultErr, os.ErrClosed) || !errors.As(resultErr, &nativeClose) {
						t.Fatalf("producer exit close cause = %v, want retained native closed-file *PathError", resultErr)
					}
				}
				if err := <-writerResult; err != nil {
					t.Fatalf("native fixture write = %v, want admitted byte and closed fixture writer", err)
				}
			}
			if row.kind == 2 && !iteratorExiting.Load() || row.kind >= 3 && exiting.reads.Load() != 1 {
				t.Fatalf("Goexit trigger = (iterator%t,nativeReads%d), want exercised owned callback", iteratorExiting.Load(), exiting.reads.Load())
			}
			var probe [1]byte
			if _, err := (<-borrowed).Read(probe[:]); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("expired borrowed pipe = %v, want closed native endpoint", err)
			}
			if _, err := input.Read(probe[:]); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("Goexit native input = %v, want closed file identity", err)
			}
		})
	}
}

func (s *multipartBlockedFile) Read(data []byte) (int, error) {
	if s.readStarted.CompareAndSwap(false, true) {
		close(s.entered)
	}
	return s.file.Read(data)
}
func (s *multipartBlockedFile) Close() error { s.closed.Add(1); return s.file.Close() }

type multipartCloseObserver struct {
	reader   io.Reader
	closed   atomic.Int64
	closeErr error
}

func (s *multipartCloseObserver) Read(data []byte) (int, error) { return s.reader.Read(data) }
func (s *multipartCloseObserver) Close() error                  { s.closed.Add(1); return s.closeErr }

func awaitMultipartJoin(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(multipartJoinBackstop):
		t.Fatalf("%s did not join, want owned exit", name)
	}
}

func TestMultipartCancellationClosesNativeBlockedSourceAndPreservesFutureCustody(t *testing.T) {
	t.Parallel()
	for _, cancelCaller := range []bool{true, false} {
		t.Run(fmt.Sprintf("native-blocked-read-caller-cancel-%t", cancelCaller), func(t *testing.T) {
			t.Parallel()
			input, output, err := os.Pipe()
			if err != nil {
				t.Fatalf("native input pipe = %v, want acquired", err)
			}
			t.Cleanup(func() { _ = input.Close(); _ = output.Close() })
			active := &multipartBlockedFile{file: input, entered: make(chan struct{})}
			future := &multipartCloseObserver{reader: &multipartPatternSource{left: 1}}
			t.Cleanup(func() { _ = future.Close() })
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			name, filename := multipartFieldName(t, "file"), multipartFilename(t, "workout.wav")
			done := make(chan struct{})
			result := make(chan error, 1)
			go func() {
				defer close(done)
				result <- exchange.WithMultipartSource(ctx, exchange.MultipartSourceRequest{
					Parts: func(yield func(exchange.MultipartPart) bool) {
						if yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: name, FileName: filename, Source: active}) {
							yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: name, FileName: filename, Source: future})
						}
					},
					Use: func(ctx context.Context, source exchange.MultipartSource) error {
						_, parameters, err := mime.ParseMediaType(source.ContentType.String())
						if err != nil {
							return err
						}
						decoder := multipart.NewReader(source.Reader, parameters["boundary"])
						part, err := decoder.NextPart()
						if err != nil {
							return err
						}
						if !cancelCaller {
							select {
							case <-active.entered:
								return io.ErrClosedPipe
							case <-ctx.Done():
								return ctx.Err()
							case <-time.After(multipartJoinBackstop):
								return core.ErrExchangeContract
							}
						}
						_, err = io.Copy(io.Discard, part)
						return err
					},
				})
			}()
			defer func() { cancel(); awaitMultipartJoin(t, done, "multipart producer/consumer") }()
			awaitMultipartJoin(t, active.entered, "native input Read")
			if cancelCaller {
				cancel()
			}
			awaitMultipartJoin(t, done, "multipart producer/consumer")
			err = <-result
			want := error(io.ErrClosedPipe)
			if cancelCaller {
				want = context.Canceled
			}
			if !errors.Is(err, want) || active.closed.Load() != 1 || future.closed.Load() != 0 {
				t.Fatalf("native interrupted scope = (%v,active closes%d,future closes%d), want %v/one admitted close/untouched future source", err, active.closed.Load(), future.closed.Load(), want)
			}
			var probe [1]byte
			if _, err := input.Read(probe[:]); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("scope-owned native input = %v, want closed file identity", err)
			}
		})
	}
}

func TestMultipartSourceAndConsumerFailuresRetainTypedCausesLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		kind uint8
		want error
	}{
		{name: "source read-failure seam preserves IO cause", kind: 1, want: io.ErrUnexpectedEOF},
		{name: "source acknowledged partial failure", kind: 2, want: io.ErrUnexpectedEOF},
		{name: "source cleanup failure", kind: 3, want: io.ErrClosedPipe},
		{name: "producer iterator panic", kind: 4, want: core.ErrExchangeCallbackPanic},
		{name: "consumer callback panic", kind: 5, want: core.ErrExchangeCallbackPanic},
		{name: "invalid file agreement keeps caller custody", kind: 6, want: core.ErrExchangeContract},
		{name: "missing source refuses before custody", kind: 7, want: core.ErrExchangeContract},
		{name: "field/source contradiction", kind: 8, want: core.ErrExchangeContract},
		{name: "file/text contradiction", kind: 9, want: core.ErrExchangeContract},
		{name: "unknown part kind", kind: 10, want: core.ErrExchangeContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			name, filename := multipartFieldName(t, "file"), multipartFilename(t, "workout.wav")
			owned := &multipartCloseObserver{reader: &multipartErrorReader{kind: row.kind}}
			if row.kind == 3 {
				owned.reader = &multipartPatternSource{}
				owned.closeErr = io.ErrClosedPipe
			}
			part := exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: name, FileName: filename, Source: owned}
			switch row.kind {
			case 6:
				part.FileName = exchange.MultipartFileName{}
			case 7:
				part.Source = nil
			case 8:
				part.Kind = exchange.MultipartPartField
				part.Text = multipartFieldText(t, "data")
			case 9:
				part.Text = multipartFieldText(t, "")
			case 10:
				part.Kind = 255
			}
			err := exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
				Parts: func(yield func(exchange.MultipartPart) bool) {
					if row.kind == 4 {
						panic("synthetic iterator payload must stay out of error projection")
					}
					yield(part)
				},
				Use: func(ctx context.Context, source exchange.MultipartSource) error {
					if row.kind == 5 {
						panic("synthetic consumer payload must stay out of error projection")
					}
					_, err := io.Copy(io.Discard, source.Reader)
					return err
				},
			})
			if !errors.Is(err, row.want) {
				t.Fatalf("multipart refusal = %v, want retained %v", err, row.want)
			}
			if row.kind == 1 || row.kind == 2 || row.kind == 3 {
				if owned.closed.Load() != 1 {
					t.Fatalf("admitted source closes = %d, want exactly one", owned.closed.Load())
				}
			}
			if row.kind >= 6 && owned.closed.Load() != 0 {
				t.Fatalf("invalid-part source closes = %d, want untouched caller custody", owned.closed.Load())
			}
			if row.kind == 5 && owned.closed.Load() > 1 {
				t.Fatalf("consumer panic source closes = %d, want at most one actually admitted source close", owned.closed.Load())
			}
		})
	}
	produced, consumed := false, false
	err := exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{Parts: func(yield func(exchange.MultipartPart) bool) { produced = true }, Use: func(ctx context.Context, source exchange.MultipartSource) error {
		consumed = true
		_, err := io.Copy(io.Discard, source.Reader)
		return err
	}})
	if err != nil || !produced || !consumed {
		t.Fatalf("empty MIME scope = (%v,producer%t,consumer%t), want valid zero-part framing and both joined callbacks", err, produced, consumed)
	}
	for _, request := range []exchange.MultipartSourceRequest{{}, {Use: func(context.Context, exchange.MultipartSource) error {
		return core.ErrExchangeContract
	}}, {Parts: func(func(exchange.MultipartPart) bool) { panic("invalid request invoked producer") }}} {
		if err := exchange.WithMultipartSource(t.Context(), request); !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("missing callback refusal = %v, want contract without effects", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := exchange.WithMultipartSource(ctx, exchange.MultipartSourceRequest{Parts: func(func(exchange.MultipartPart) bool) { panic("cancelled request invoked producer") }, Use: func(context.Context, exchange.MultipartSource) error {
		return core.ErrExchangeContract
	}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel refusal = %v, want native cancellation without callbacks", err)
	}
}

type multipartErrorReader struct {
	kind   uint8
	called bool
}

func (r *multipartErrorReader) Read(data []byte) (int, error) {
	if r.called {
		return 0, io.EOF
	}
	r.called = true
	if r.kind == 1 {
		return 0, io.ErrUnexpectedEOF
	}
	if r.kind == 2 {
		data[0] = 1
		return 1, io.ErrUnexpectedEOF
	}
	return 0, io.EOF
}

type multipartReadCountingSource struct {
	reader io.Reader
	reads  atomic.Int64
	closed atomic.Int64
}

func (s *multipartReadCountingSource) Read(data []byte) (int, error) {
	s.reads.Add(1)
	return s.reader.Read(data)
}
func (s *multipartReadCountingSource) Close() error { s.closed.Add(1); return nil }

type multipartBackpressureSink struct {
	ctx        context.Context
	entered    chan struct{}
	release    chan struct{}
	source     *multipartReadCountingSource
	firstReads int64
	written    int64
}

func (s *multipartBackpressureSink) Write(data []byte) (int, error) {
	if s.written == 0 {
		s.firstReads = s.source.reads.Load()
		close(s.entered)
		select {
		case <-s.release:
		case <-s.ctx.Done():
			return 0, s.ctx.Err()
		}
	}
	for index, value := range data {
		if value != byte((s.written+int64(index))%251) {
			return 0, core.ErrExchangeContract
		}
	}
	s.written += int64(len(data))
	return len(data), nil
}

func TestMultipartNativePipeBackpressureStopsFilePrefetch(t *testing.T) {
	t.Parallel()
	const extent = int64(8*32768 + 7)
	// The generated source is an explicit read-count seam; the multipart writer,
	// native io.Pipe and consumer MIME parser execute their real stdlib paths.
	source := &multipartReadCountingSource{reader: &multipartPatternSource{left: extent}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sink := &multipartBackpressureSink{ctx: ctx, entered: make(chan struct{}), release: make(chan struct{}), source: source}
	name, filename := multipartFieldName(t, "file"), multipartFilename(t, "workout.wav")
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		defer close(done)
		result <- exchange.WithMultipartSource(ctx, exchange.MultipartSourceRequest{
			Parts: func(yield func(exchange.MultipartPart) bool) {
				yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: name, FileName: filename, Source: source})
			},
			Use: func(ctx context.Context, body exchange.MultipartSource) error {
				_, parameters, err := mime.ParseMediaType(body.ContentType.String())
				if err != nil {
					return err
				}
				decoder := multipart.NewReader(body.Reader, parameters["boundary"])
				part, err := decoder.NextPart()
				if err != nil {
					return err
				}
				if _, err := io.CopyBuffer(sink, part, make([]byte, 32768)); err != nil {
					return err
				}
				if next, err := decoder.NextPart(); next != nil || err != io.EOF {
					return errors.Join(core.ErrExchangeContract, err)
				}
				_, err = io.Copy(io.Discard, body.Reader)
				return err
			},
		})
	}()
	defer func() { cancel(); awaitMultipartJoin(t, done, "backpressured multipart scope") }()
	awaitMultipartJoin(t, sink.entered, "first acknowledged MIME body write")
	if got := source.reads.Load(); got != 1 {
		t.Fatalf("source reads under native pipe backpressure = %d, want exactly one fixed producer window", got)
	}
	close(sink.release)
	awaitMultipartJoin(t, done, "backpressured multipart scope")
	if err := <-result; err != nil || sink.firstReads != 1 || sink.written != extent || source.closed.Load() != 1 {
		t.Fatalf("native pipe stream = (%v,firstreads%d,written%d,closes%d), want fixed first window/exact%d/one source close", err, sink.firstReads, sink.written, source.closed.Load(), extent)
	}
}
