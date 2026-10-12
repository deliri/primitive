package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestRecordCaptureNativeExactBytesSurviveEarlyConsumerReturn(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{0, 1, 15, 16, 17, 65535, 65536, 65537, 8*1024*1024 + 1} {
		for _, ending := range []string{"\n", "\r\n", ""} {
			for _, consume := range []int{0, 1, 65537} {
				t.Run(strconv.Itoa(extent)+"/"+strconv.Quote(ending)+"/"+strconv.Itoa(consume), func(t *testing.T) {
					t.Parallel()
					body := append(bytes.Repeat([]byte{'x'}, extent), []byte(ending)...)
					memory, err := core.NewByteCount(65536)
					if err != nil {
						t.Fatal(err)
					}
					reader, err := lineio.New(lineio.Request{Source: bytes.NewReader(body), BufferBytes: memory})
					if err != nil {
						t.Fatal(err)
					}
					var capture bytes.Buffer
					got, err := reader.WithRecord(t.Context(), lineio.RecordReadRequest{Capture: &capture, Use: func(ctx context.Context, source io.Reader) error {
						_, err := io.Copy(io.Discard, io.LimitReader(source, int64(consume)))
						return err
					}})
					if len(body) == 0 {
						if got != (lineio.RecordReadObservation{}) || err != io.EOF || capture.Len() != 0 {
							t.Fatalf("empty source=%+v,%v capture=%x", got, err, capture.Bytes())
						}
						return
					}
					if err != nil || !bytes.Equal(capture.Bytes(), body) {
						t.Fatalf("native capture=%x,%v; want %d exact bytes", capture.Bytes(), err, len(body))
					}
					count, countErr := got.Bytes.Int64()
					if countErr != nil || count != int64(len(body)) {
						t.Fatalf("extent=%d,%v; want %d", count, countErr, len(body))
					}
				})
			}
		}
	}
}

type recordCaptureFaultWriter struct {
	count  int
	cause  error
	cancel context.CancelFunc
}

func (w recordCaptureFaultWriter) Write(data []byte) (int, error) {
	if w.cancel != nil {
		w.cancel()
	}
	if w.count < 0 {
		return -1, w.cause
	}
	if w.count > len(data) {
		return len(data) + 1, w.cause
	}
	return w.count, w.cause
}

func TestRecordCaptureNativeWriteFailureRetainsSourceCause(t *testing.T) {
	t.Parallel()
	for causeIndex, sourceCause := range []error{nil, io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrClosedPipe)} {
		for _, tc := range []struct {
			name   string
			writer io.Writer
			cause  error
		}{
			{name: "short", writer: recordCaptureFaultWriter{}, cause: io.ErrShortWrite},
			{name: "native", writer: recordCaptureFaultWriter{cause: io.ErrClosedPipe}, cause: io.ErrClosedPipe},
			{name: "negative", writer: recordCaptureFaultWriter{count: -1}, cause: core.ErrLineIOContract},
			{name: "over count", writer: recordCaptureFaultWriter{count: 99}, cause: core.ErrLineIOContract},
			{name: "typed nil", writer: (*bytes.Buffer)(nil), cause: core.ErrLineIOContract},
		} {
			t.Run(tc.name+"/"+strconv.Itoa(causeIndex), func(t *testing.T) {
				t.Parallel()
				var source io.Reader = bytes.NewReader([]byte("record"))
				if sourceCause != nil {
					source = io.MultiReader(source, iotest.ErrReader(sourceCause))
				}
				memory, err := core.NewByteCount(16)
				if err != nil {
					t.Fatal(err)
				}
				reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: memory})
				if err != nil {
					t.Fatal(err)
				}
				got, err := reader.WithRecord(t.Context(), lineio.RecordReadRequest{Capture: tc.writer, Use: func(ctx context.Context, source io.Reader) error { _, err := io.Copy(io.Discard, source); return err }})
				if got != (lineio.RecordReadObservation{}) || !errors.Is(err, tc.cause) {
					t.Fatalf("capture refusal=%+v,%v; want cause=%v", got, err, tc.cause)
				}
				// Typed nil refuses before touching the source. Every execution refusal
				// preserves a cause that accompanied the already borrowed native fragment.
				if tc.name != "typed nil" && sourceCause != nil && !errors.Is(err, sourceCause) {
					t.Fatalf("native source cause lost: %v; want=%v", err, sourceCause)
				}
			})
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	memory, err := core.NewByteCount(16)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := lineio.New(lineio.Request{Source: bytes.NewReader([]byte("record")), BufferBytes: memory})
	if err != nil {
		t.Fatal(err)
	}
	got, err := reader.WithRecord(ctx, lineio.RecordReadRequest{Capture: recordCaptureFaultWriter{cancel: cancel, cause: io.ErrClosedPipe}, Use: func(ctx context.Context, source io.Reader) error { _, err := io.Copy(io.Discard, source); return err }})
	if got != (lineio.RecordReadObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("cancellation erased capture cause: %+v,%v", got, err)
	}
}

func FuzzRecordCaptureNativeMatchesIndependentPhysicalBytes(f *testing.F) {
	f.Add([]byte("record\nnext\n"), uint16(1), false)
	f.Add([]byte{0, 255, 1}, uint16(0), true)
	f.Fuzz(func(t *testing.T, body []byte, consume uint16, refuse bool) {
		reader, err := lineio.New(lineio.Request{Source: bytes.NewReader(body), BufferBytes: replacementMemory(t, 17)})
		if err != nil {
			t.Fatal(err)
		}
		var capture bytes.Buffer
		got, err := reader.WithRecord(t.Context(), lineio.RecordReadRequest{Capture: &capture, Use: func(ctx context.Context, source io.Reader) error {
			_, err := io.Copy(io.Discard, io.LimitReader(source, int64(consume)))
			if refuse {
				return errors.Join(core.ErrPrimitiveContract, err)
			}
			return err
		}})
		first, _, found := bytes.Cut(body, []byte{'\n'})
		expected := first
		if found {
			expected = body[:len(first)+1]
		}
		if !bytes.Equal(capture.Bytes(), expected) {
			t.Fatalf("physical capture=%x; want=%x", capture.Bytes(), expected)
		}
		if len(body) == 0 {
			if got != (lineio.RecordReadObservation{}) || err != io.EOF {
				t.Fatalf("empty source=%+v,%v", got, err)
			}
			return
		}
		if refuse {
			if got != (lineio.RecordReadObservation{}) || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("refusal=%+v,%v", got, err)
			}
			return
		}
		count, countErr := got.Bytes.Int64()
		if err != nil || countErr != nil || count != int64(len(expected)) {
			t.Fatalf("physical extent=%+v,%v", got, err)
		}
	})
}

func BenchmarkRecordCaptureNativeFixedWorkingMemory(b *testing.B) {
	for _, extent := range []int64{65535, 65536, 65537, 8*1024*1024 + 1, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			memory := replacementMemory(b, 65536)
			b.ReportAllocs()
			b.SetBytes(extent)
			b.ResetTimer()
			for range b.N {
				reader, err := lineio.New(lineio.Request{Source: io.LimitReader(&replacementExtentSource{}, extent), BufferBytes: memory})
				if err != nil {
					b.Fatal(err)
				}
				got, err := reader.WithRecord(b.Context(), lineio.RecordReadRequest{Capture: io.Discard, Use: func(ctx context.Context, source io.Reader) error { _, err := io.Copy(io.Discard, source); return err }})
				count, countErr := got.Bytes.Int64()
				if err != nil || countErr != nil || count != extent {
					b.Fatalf("native capture benchmark=%+v,%v", got, err)
				}
			}
		})
	}
}

type recordCaptureCancelledNativeSource struct {
	cancel context.CancelFunc
	cause  error
}

func (s recordCaptureCancelledNativeSource) Read(destination []byte) (int, error) {
	s.cancel()
	return copy(destination, []byte("record")), s.cause
}

func TestRecordCaptureCancellationBeforeConsumerReadRetainsNativeCause(t *testing.T) {
	t.Parallel()
	for index, cause := range []error{nil, io.EOF, io.ErrClosedPipe, errors.Join(io.EOF, io.ErrUnexpectedEOF)} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reader, err := lineio.New(lineio.Request{Source: recordCaptureCancelledNativeSource{cancel: cancel, cause: cause}, BufferBytes: replacementMemory(t, 16)})
			if err != nil {
				t.Fatal(err)
			}
			got, err := reader.WithRecord(ctx, lineio.RecordReadRequest{Capture: io.Discard, Use: func(ctx context.Context, source io.Reader) error { return ctx.Err() }})
			if got != (lineio.RecordReadObservation{}) || !errors.Is(err, context.Canceled) || cause != nil && !errors.Is(err, cause) {
				t.Fatalf("cancel-before-consumer=%+v,%v; want cancellation and native cause=%v", got, err, cause)
			}
		})
	}
}
