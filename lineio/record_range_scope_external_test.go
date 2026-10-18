package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func rangeScopeRequest(t testing.TB, source lineio.RecordSource, offset int64, body string, framing lineio.RecordFraming) lineio.RecordFragmentRequest {
	t.Helper()
	extent, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	window, err := core.NewByteCount(17)
	if err != nil {
		t.Fatal(err)
	}
	return lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Offset: lineio.SourceByteOffset(offset), Bytes: extent, Framing: framing}, BufferBytes: window}
}

func TestRecordRangeScopeConservesNativeBytesAndExpiresEveryExit(t *testing.T) {
	t.Parallel()
	for _, size := range []int{1, 16, 17, 18, 4096, 65537} {
		for _, framing := range []lineio.RecordFraming{lineio.RecordFramingLF, lineio.RecordFramingEOF, lineio.RecordFramingOpaque} {
			for _, consume := range []int{0, 1, -1} {
				t.Run(fmt.Sprintf("bytes_%d/framing_%d/consume_%d", size, framing, consume), func(t *testing.T) {
					t.Parallel()
					body := strings.Repeat("x", size)
					if framing == lineio.RecordFramingLF {
						body += "\n"
					}
					if framing == lineio.RecordFramingOpaque {
						body = "\x00λ\n" + body + "\r\n"
					}
					prefix := "unchanged prefix\n"
					source := strings.NewReader(prefix + body + "untouched suffix")
					if _, err := source.Seek(3, io.SeekStart); err != nil {
						t.Fatal(err)
					}
					var borrowed io.Reader
					var got []byte
					err := lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{Range: rangeScopeRequest(t, source, int64(len(prefix)), body, framing), Use: func(_ context.Context, reader io.Reader) error {
						borrowed = reader
						if consume == 0 {
							return nil
						}
						if consume == 1 {
							var b [1]byte
							if _, err := io.ReadFull(reader, b[:]); err != nil {
								return err
							}
							got = append(got, b[0])
							return fs.ErrPermission
						}
						var err error
						got, err = io.ReadAll(reader)
						return err
					}})
					if consume == 1 {
						if !errors.Is(err, fs.ErrPermission) {
							t.Fatalf("refusal = %v, want native cause", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					if consume < 0 && string(got) != body || consume == 1 && string(got) != body[:1] {
						t.Fatalf("loan bytes = %q, want exact selected extent %q", got, body)
					}
					if current, err := source.Seek(0, io.SeekCurrent); err != nil || current != 3 {
						t.Fatalf("positional read moved source to %d: %v", current, err)
					}
					if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, core.ErrLineIOContract) {
						t.Fatalf("expired range reader = %v, want native contract refusal", err)
					}
				})
			}
		}
	}
}

func TestRecordRangeScopeRejectsFalsePhysicalClaimsAndIgnoredFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, claimed string
		framing               lineio.RecordFraming
		want                  error
	}{
		{name: "range extends past native EOF", source: "{}", claimed: "{} ", framing: lineio.RecordFramingEOF, want: io.ErrUnexpectedEOF},
		{name: "LF missing", source: "{}", claimed: "{}", framing: lineio.RecordFramingLF, want: core.ErrLineIOScan},
		{name: "LF ends before claimed range", source: "{}\nmore", claimed: "{}\nmore", framing: lineio.RecordFramingLF, want: core.ErrLineIOScan},
		{name: "EOF claim contains LF", source: "{}\n", claimed: "{}\n", framing: lineio.RecordFramingEOF, want: core.ErrLineIOScan},
		{name: "opaque range still requires exact extent", source: "{}\n", claimed: "{}\nx", framing: lineio.RecordFramingOpaque, want: io.ErrUnexpectedEOF},
	} {
		for _, read := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ignored_read_%t", tc.name, read), func(t *testing.T) {
				t.Parallel()
				source := strings.NewReader(tc.source)
				err := lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{Range: rangeScopeRequest(t, source, 0, tc.claimed, tc.framing), Use: func(_ context.Context, r io.Reader) error {
					if read {
						_, _ = io.Copy(io.Discard, r)
					}
					return nil
				}})
				if !errors.Is(err, tc.want) || !errors.Is(err, core.ErrLineIOScan) {
					t.Fatalf("false physical claim = %v, want scan refusal with %v", err, tc.want)
				}
			})
		}
	}
}

func TestRecordRangeScopeRefusesInvalidAdmissionAndPreservesCancellation(t *testing.T) {
	t.Parallel()
	valid := rangeScopeRequest(t, strings.NewReader("{}\n"), 0, "{}\n", lineio.RecordFramingLF)
	canceled, cancel := context.WithCancelCause(t.Context())
	cancel(fs.ErrPermission)
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		request lineio.RecordFragmentRequest
		use     bool
		want    error
	}{
		{name: "nil context", ctx: nil, request: valid, use: true, want: core.ErrNilContext},
		{name: "cancelled context", ctx: canceled, request: valid, use: true, want: context.Canceled},
		{name: "missing source", ctx: t.Context(), request: lineio.RecordFragmentRequest{}, use: true, want: core.ErrLineIOContract},
		{name: "missing consumer", ctx: t.Context(), request: valid, want: core.ErrLineIOContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			request := lineio.RecordRangeReadRequest{Range: tc.request}
			if tc.use {
				request.Use = func(context.Context, io.Reader) error { calls++; return nil }
			}
			err := lineio.WithRecordRange(tc.ctx, request)
			if !errors.Is(err, tc.want) || calls != 0 {
				t.Fatalf("admission = (%d,%v), want no loan with %v", calls, err, tc.want)
			}
			if tc.ctx == canceled && !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("custom cancellation cause disappeared: %v", err)
			}
		})
	}
	t.Run("cancel cached fragment", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		err := lineio.WithRecordRange(ctx, lineio.RecordRangeReadRequest{Range: valid, Use: func(_ context.Context, r io.Reader) error {
			var first [1]byte
			if _, err := io.ReadFull(r, first[:]); err != nil {
				return err
			}
			cancel(fs.ErrPermission)
			if n, err := r.Read(first[:]); n != 0 || !errors.Is(err, context.Canceled) {
				t.Fatalf("cached fragment after cancellation = (%d,%v)", n, err)
			}
			return nil
		}})
		if !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("cancelled range = %v, want complete native cause", err)
		}
	})
}

func TestRecordRangeScopeExpiresPanicAndGoexitLoans(t *testing.T) {
	t.Parallel()
	for _, goexit := range []bool{false, true} {
		t.Run(fmt.Sprintf("goexit_%t", goexit), func(t *testing.T) {
			t.Parallel()
			var borrowed io.Reader
			done := make(chan error, 1)
			go func() {
				defer func() {
					value := recover()
					if !goexit && value != fs.ErrPermission {
						done <- fmt.Errorf("panic = %v, want exact native cause", value)
						return
					}
					done <- nil
				}()
				_ = lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{Range: rangeScopeRequest(t, strings.NewReader("{}\n"), 0, "{}\n", lineio.RecordFramingLF), Use: func(_ context.Context, r io.Reader) error {
					borrowed = r
					var b [1]byte
					if _, err := io.ReadFull(r, b[:]); err != nil {
						return err
					}
					if goexit {
						runtime.Goexit()
					}
					panic(fs.ErrPermission)
				}})
			}()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if borrowed == nil {
				t.Fatal("loan was never admitted")
			}
			if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, core.ErrLineIOContract) {
				t.Fatalf("abandoned loan = %v, want expired", err)
			}
		})
	}
}

func TestRecordRangeScopeDrainsClosedNativeFileAfterConsumerRefusal(t *testing.T) {
	t.Parallel()
	directory, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	body := strings.Repeat("x", 257) + "\n"
	location := filestore.Location{Root: root, Path: mustRelativePath(t, "range.txt")}
	if _, err := filestore.Write(t.Context(), filestore.WriteRequest{Source: strings.NewReader(body), Location: location, Temporary: mustRelativePath(t, ".range.stage"), Mode: 0600, Install: filestore.InstallCreate}); err != nil {
		t.Fatal(err)
	}
	file, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	err = lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{Range: rangeScopeRequest(t, file, 0, body, lineio.RecordFramingLF), Use: func(_ context.Context, r io.Reader) error {
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		return fs.ErrPermission
	}})
	var native *fs.PathError
	if !errors.Is(err, fs.ErrPermission) || !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrLineIOScan) || !errors.As(err, &native) {
		t.Fatalf("consumer plus native drain failure = %v, want both causes and PathError", err)
	}
}

func FuzzRecordRangeScopeMatchesIndependentSelectedBytes(f *testing.F) {
	f.Add([]byte("prefix\x00λ\nrange\xffsuffix"), uint16(3), uint16(7), uint16(2), false)
	f.Add([]byte{}, uint16(0), uint16(0), uint16(0), false)
	f.Add([]byte("body"), uint16(0), uint16(4), uint16(1), true)
	f.Fuzz(func(t *testing.T, data []byte, start, length, consume uint16, refuse bool) {
		if len(data) == 0 {
			return
		}
		offset := int(start) % len(data)
		extent := 1 + int(length)%(len(data)-offset)
		want := data[offset : offset+extent]
		source := bytes.NewReader(data)
		var got []byte
		err := lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{Range: rangeScopeRequest(t, source, int64(offset), string(want), lineio.RecordFramingOpaque), Use: func(_ context.Context, r io.Reader) error {
			if refuse {
				_, err := io.CopyN(io.Discard, r, int64(int(consume)%extent))
				if err != nil {
					return err
				}
				return fs.ErrPermission
			}
			var err error
			got, err = io.ReadAll(r)
			return err
		}})
		if refuse {
			if !errors.Is(err, fs.ErrPermission) {
				t.Fatal(err)
			}
		} else if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("range = (%q,%v), want independent selected bytes %q", got, err, want)
		}
		if current, err := source.Seek(0, io.SeekCurrent); err != nil || current != 0 {
			t.Fatalf("range changed sequential source position %d: %v", current, err)
		}
	})
}
