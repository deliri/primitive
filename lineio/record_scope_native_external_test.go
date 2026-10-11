package lineio_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

type generatedRecordBody struct{ remaining int64 }

func (r *generatedRecordBody) Read(destination []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count := len(destination)
	if int64(count) > r.remaining {
		count = int(r.remaining)
	}
	for index := 0; index < count; index++ {
		destination[index] = 'x'
	}
	r.remaining -= int64(count)
	return count, nil
}

func TestRecordScopeNativeFramingAndBorrowedLifetimeMatrix(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{0, 1, 65535, 65536, 65537, 131071, 131072, 131073, 8<<20 + 1} {
		for _, mode := range []uint8{0, 1, 2, 3, 4, 5} {
			t.Run(fmt.Sprintf("extent_%d/mode_%d", extent, mode), func(t *testing.T) {
				t.Parallel()
				delimiter := []byte{'\n'}
				tail := []byte("next\n")
				if mode == 1 {
					delimiter = nil
					tail = nil
				}
				buffer, err := core.NewByteCount(65536)
				if err != nil {
					t.Fatal(err)
				}
				reader, err := lineio.New(lineio.Request{Source: io.MultiReader(&generatedRecordBody{remaining: extent}, bytes.NewReader(delimiter), bytes.NewReader(tail)), BufferBytes: buffer})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if mode == 4 {
					cancel()
				}
				var borrowed io.Reader
				var digest [sha256.Size]byte
				var consumed int64
				calls := 0
				var wantErr error
				if mode == 2 {
					wantErr = io.ErrShortWrite
				}
				if mode == 4 || mode == 5 {
					wantErr = context.Canceled
				}
				observation, err := reader.WithRecord(ctx, lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error {
					calls++
					borrowed = source
					if mode == 5 {
						cancel()
						return nil
					}
					hash := sha256.New()
					window := [65536]byte{}
					var readErr error
					if mode == 3 {
						consumed, readErr = io.CopyN(hash, source, 1)
					} else if mode == 2 {
						return wantErr
					} else {
						consumed, readErr = io.CopyBuffer(hash, source, window[:])
					}
					if readErr != nil {
						return readErr
					}
					copy(digest[:], hash.Sum(nil))
					return nil
				}})
				if extent == 0 && mode == 1 {
					wantErr = io.EOF
				}
				if !errors.Is(err, wantErr) {
					t.Fatalf("record result = %+v/%v, want %v", observation, err, wantErr)
				}
				if wantErr != nil {
					if observation != (lineio.RecordReadObservation{}) {
						t.Fatalf("refused observation=%+v, want zero", observation)
					}
				} else {
					total, convertErr := observation.Bytes.Int64()
					if convertErr != nil || total != extent+int64(len(delimiter)) {
						t.Fatalf("extent=%d/%v, want %d", total, convertErr, extent+int64(len(delimiter)))
					}
					framing := lineio.RecordFramingLF
					if mode == 1 {
						framing = lineio.RecordFramingEOF
					}
					if observation.Framing != framing {
						t.Fatalf("framing=%v, want %v", observation.Framing, framing)
					}
					expected := sha256.New()
					if mode == 3 {
						first := byte('x')
						if extent == 0 {
							first = '\n'
						}
						_, _ = expected.Write([]byte{first})
						if consumed != 1 {
							t.Fatalf("prefix consumed=%d, want 1", consumed)
						}
					} else {
						window := [65536]byte{}
						_, copyErr := io.CopyBuffer(expected, io.MultiReader(&generatedRecordBody{remaining: extent}, bytes.NewReader(delimiter)), window[:])
						if copyErr != nil {
							t.Fatal(copyErr)
						}
						if consumed != total {
							t.Fatalf("consumed=%d, want %d", consumed, total)
						}
					}
					var wantDigest [sha256.Size]byte
					copy(wantDigest[:], expected.Sum(nil))
					if digest != wantDigest {
						t.Fatalf("digest=%x, Native digest=%x", digest, wantDigest)
					}
				}
				if borrowed != nil {
					if count, err := borrowed.Read(make([]byte, 1)); count != 0 || !errors.Is(err, core.ErrLineIOContract) {
						t.Fatalf("expired reader=%d/%v, want 0/contract refusal", count, err)
					}
				}
				if mode == 4 {
					if calls != 0 {
						t.Fatalf("canceled callback count=%d, want 0", calls)
					}
					_, err = reader.WithRecord(t.Context(), lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error { _, err := io.Copy(io.Discard, source); return err }})
					if err != nil {
						t.Fatalf("active retry=%v, want unchanged record", err)
					}
				}
				if mode != 1 && mode != 5 {
					var next bytes.Buffer
					_, err = reader.WithRecord(t.Context(), lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error { _, err := io.Copy(&next, source); return err }})
					if err != nil || !bytes.Equal(next.Bytes(), tail) {
						t.Fatalf("next record=%q/%v, want %q/nil", next.Bytes(), err, tail)
					}
				}
			})
		}
	}
}

func FuzzRecordScopeMatchesNativePhysicalLines(f *testing.F) {
	f.Add([]byte("one\ntwo\n"), uint16(0))
	f.Add([]byte("unterminated"), uint16(65535))
	f.Add([]byte{}, uint16(1))
	f.Fuzz(func(t *testing.T, data []byte, prefix uint16) {
		buffer, err := core.NewByteCount(64)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := lineio.New(lineio.Request{Source: bytes.NewReader(data), BufferBytes: buffer})
		if err != nil {
			t.Fatal(err)
		}
		record, _, terminated := bytes.Cut(data, []byte{'\n'})
		extent := len(record)
		if terminated {
			extent++
		}
		hash := sha256.New()
		var consumed int64
		observation, err := reader.WithRecord(t.Context(), lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error {
			var readErr error
			consumed, readErr = io.CopyN(hash, source, int64(prefix))
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}})
		if len(data) == 0 {
			if err != io.EOF || consumed != 0 || observation != (lineio.RecordReadObservation{}) {
				t.Fatalf("empty source=%+v/%d/%v, want zero/0/EOF", observation, consumed, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		wantConsumed := min(int(prefix), extent)
		if consumed != int64(wantConsumed) || !bytes.Equal(hash.Sum(nil), sha256BytesForRecord(data[:wantConsumed])) {
			t.Fatalf("prefix extent=%d, want %d and Native digest", consumed, wantConsumed)
		}
		observed, err := observation.Bytes.Int64()
		if err != nil || observed != int64(extent) {
			t.Fatalf("record extent=%d/%v, want %d", observed, err, extent)
		}
	})
}
func sha256BytesForRecord(data []byte) []byte { digest := sha256.Sum256(data); return digest[:] }

type nativeRecordFailureSource struct {
	body  []byte
	cause error
}

func (r *nativeRecordFailureSource) Read(destination []byte) (int, error) {
	if len(r.body) == 0 {
		return 0, r.cause
	}
	count := copy(destination, r.body)
	r.body = r.body[count:]
	return count, r.cause
}
func TestRecordScopeNativeSourceCausesAndPanicExpiry(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrClosedPipe)} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			buffer, err := core.NewByteCount(16)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := lineio.New(lineio.Request{Source: &nativeRecordFailureSource{body: []byte("partial"), cause: cause}, BufferBytes: buffer})
			if err != nil {
				t.Fatal(err)
			}
			observation, err := reader.WithRecord(t.Context(), lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error { _, err := io.Copy(io.Discard, source); return err }})
			if observation != (lineio.RecordReadObservation{}) || !errors.Is(err, core.ErrLineIOScan) || !errors.Is(err, cause) {
				t.Fatalf("source failure=%+v/%v, want zero and both causes", observation, err)
			}
		})
	}
	t.Run("panic immediately expires borrowed source", func(t *testing.T) {
		t.Parallel()
		buffer, err := core.NewByteCount(16)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := lineio.New(lineio.Request{Source: bytes.NewReader([]byte("first\nnext\n")), BufferBytes: buffer})
		if err != nil {
			t.Fatal(err)
		}
		var borrowed io.Reader
		var gotPanic error
		func() {
			defer func() {
				if value := recover(); value != nil {
					var ok bool
					gotPanic, ok = value.(error)
					if !ok {
						t.Fatalf("panic=%v, want typed error", value)
					}
				}
			}()
			_, _ = reader.WithRecord(t.Context(), lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error { borrowed = source; panic(core.ErrPrimitiveContract) }})
		}()
		if gotPanic != core.ErrPrimitiveContract || borrowed == nil {
			t.Fatalf("panic=%v/borrowed %v, want exact panic and delivered reader", gotPanic, borrowed)
		}
		if count, err := borrowed.Read(make([]byte, 1)); count != 0 || !errors.Is(err, core.ErrLineIOContract) {
			t.Fatalf("expired reader=%d/%v, want zero/contract refusal", count, err)
		}
	})
}

func BenchmarkRecordScopeFixedWindow(b *testing.B) {
	for _, extent := range []int64{65535, 65536, 65537, 8<<20 + 1} {
		b.Run(fmt.Sprintf("bytes_%d", extent), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(extent + 1)
			buffer, err := core.NewByteCount(65536)
			if err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				reader, err := lineio.New(lineio.Request{Source: io.MultiReader(&generatedRecordBody{remaining: extent}, bytes.NewReader([]byte{'\n'})), BufferBytes: buffer})
				if err != nil {
					b.Fatal(err)
				}
				observation, err := reader.WithRecord(b.Context(), lineio.RecordReadRequest{Use: func(_ context.Context, source io.Reader) error { _, err := io.Copy(io.Discard, source); return err }})
				if err != nil {
					b.Fatal(err)
				}
				count, err := observation.Bytes.Int64()
				if err != nil || count != extent+1 {
					b.Fatalf("extent=%d/%v, want %d", count, err, extent+1)
				}
			}
		})
	}
}
