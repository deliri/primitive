package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestDelimitedReaderConservesEveryNativeByteAndEmptyField(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "\n", "\t", "\t\t", "\t\n", "a", "a\t", "a\t\n", "a\tb\nlast", "\r\n", "\u2003a\t界\n", "\xff\t\xfe\n", strings.Repeat("界", 4097) + "\tlast\n", strings.Repeat(" ", 65537) + "\tx"} {
		for _, capacity := range []uint64{1, 17, 4096} {
			t.Run(strconv.Quote(body[:min(len(body), 20)])+"/"+strconv.FormatUint(capacity, 10), func(t *testing.T) { t.Parallel(); compareDelimitedReaderWithBytes(t, []byte(body), capacity) })
		}
	}
}

func compareDelimitedReaderWithBytes(t *testing.T, body []byte, capacity uint64) {
	t.Helper()
	buffer, err := core.NewByteCount(capacity)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := lineio.NewDelimitedReader(lineio.Request{Source: bytes.NewReader(body), BufferBytes: buffer})
	if err != nil {
		t.Fatal(err)
	}
	var reconstructed []byte
	field := uint64(1)
	completed := 0
	for {
		fragment, err := reader.ReadFragment(t.Context())
		if err == io.EOF {
			break
		}
		if err != nil || fragment.Validate() != nil {
			t.Fatalf("native fragment/error=%+v/%v,want validated bytes", fragment, err)
		}
		if fragment.Field != field || int64(fragment.Offset) != int64(len(reconstructed)) {
			t.Fatalf("field/offset=%d/%d,want %d/%d", fragment.Field, fragment.Offset, field, len(reconstructed))
		}
		if utf8.Valid(body) && !utf8.Valid(fragment.Bytes) {
			t.Fatal("valid source rune was split across fragments")
		}
		reconstructed = append(reconstructed, fragment.Bytes...)
		switch fragment.Boundary {
		case lineio.DelimitedBoundaryMore:
		case lineio.DelimitedBoundaryTab:
			reconstructed = append(reconstructed, '\t')
			field++
			completed++
		case lineio.DelimitedBoundaryLF:
			reconstructed = append(reconstructed, '\n')
			field = 1
			completed++
		case lineio.DelimitedBoundaryEOF:
			completed++
		default:
			t.Fatal("foreign native delimiter")
		}
	}
	wantFields := bytes.Count(body, []byte{'\t'}) + bytes.Count(body, []byte{'\n'})
	if len(body) > 0 && body[len(body)-1] != '\n' {
		wantFields++
	}
	if !bytes.Equal(reconstructed, body) || completed != wantFields {
		t.Fatalf("native bytes/fields=%x/%d,want %x/%d", reconstructed, completed, body, wantFields)
	}
}

func FuzzDelimitedReaderMatchesIndependentOriginalByteFraming(f *testing.F) {
	for _, body := range []string{"", "\t", "a\t\n", "\xff界\t\r\n", "a\nb\t\tc"} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, body []byte) { compareDelimitedReaderWithBytes(t, body, 17) })
}

type delimitedClosingFile struct {
	*os.File
	reads  int
	cancel context.CancelCauseFunc
}

func (s *delimitedClosingFile) Read(p []byte) (int, error) {
	if s.reads > 0 && s.cancel == nil {
		if err := s.File.Close(); err != nil {
			return 0, err
		}
	}
	s.reads++
	n, err := s.File.Read(p)
	if s.cancel != nil {
		s.cancel(os.ErrPermission)
	}
	return n, err
}

func TestDelimitedReaderConservesNativeCloseAndCancelCauses(t *testing.T) {
	t.Parallel()
	for _, cancelDuring := range []bool{false, true} {
		t.Run(strconv.FormatBool(cancelDuring), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "native-fields"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := io.WriteString(file, strings.Repeat("x", 65537)); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(context.Canceled)
			source := &delimitedClosingFile{File: file}
			want := error(fs.ErrClosed)
			if cancelDuring {
				source.cancel = cancel
				want = context.Canceled
			}
			buffer, err := core.NewByteCount(4096)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := lineio.NewDelimitedReader(lineio.Request{Source: source, BufferBytes: buffer})
			if err != nil {
				t.Fatal(err)
			}
			for {
				fragment, err := reader.ReadFragment(ctx)
				if err == nil {
					if fragment.Boundary != lineio.DelimitedBoundaryMore {
						t.Fatalf("refused source published a complete field: %+v", fragment)
					}
					continue
				}
				if len(fragment.Bytes) != 0 || fragment.Offset != 0 || fragment.Field != 0 || fragment.Boundary != lineio.DelimitedBoundaryUnknown {
					t.Fatalf("refused fragment retained bytes: %+v", fragment)
				}
				if !errors.Is(err, want) || cancelDuring && !errors.Is(err, os.ErrPermission) {
					t.Fatalf("native refusal=%v,want %v and custom cause when canceled", err, want)
				}
				break
			}
		})
	}
}

func BenchmarkDelimitedReaderNativeWorkingMemory(b *testing.B) {
	for _, extent := range []int64{64 * 1024, 16 * 1024 * 1024, 64 * 1024 * 1024} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			file, err := os.Create(filepath.Join(b.TempDir(), "native-delimited-benchmark"))
			if err != nil {
				b.Fatal(err)
			}
			defer file.Close()
			block := []byte(strings.Repeat("x", 4095) + "\t")
			for written := int64(0); written < extent; written += int64(len(block)) {
				if _, err := file.Write(block); err != nil {
					b.Fatal(err)
				}
			}
			buffer, err := core.NewByteCount(4096)
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(extent)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					b.Fatal(err)
				}
				reader, err := lineio.NewDelimitedReader(lineio.Request{Source: file, BufferBytes: buffer})
				if err != nil {
					b.Fatal(err)
				}
				var consumed int64
				fields := uint64(0)
				for {
					fragment, err := reader.ReadFragment(b.Context())
					if err == io.EOF {
						break
					}
					if err != nil || int64(fragment.Offset) != consumed {
						b.Fatalf("native offset/error=%d/%v,want %d", fragment.Offset, err, consumed)
					}
					consumed += int64(len(fragment.Bytes))
					if fragment.Boundary == lineio.DelimitedBoundaryTab {
						consumed++
						fields++
					}
				}
				if consumed != extent || fields != uint64(extent/4096) {
					b.Fatalf("native bytes/fields=%d/%d,want %d/%d", consumed, fields, extent, extent/4096)
				}
			}
		})
	}
}
