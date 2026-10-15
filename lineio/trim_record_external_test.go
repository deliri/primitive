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

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestTrimRecordWhitespaceMatchesOriginalNativeBytes(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", " ", "\t\r\n", "frame", " frame \n", "\u0085\u2003frame\u00a0", "\xff\x00", "\u2003\xffframe\xff\u2003", "\ufeff frame ", strings.Repeat(" ", 4097) + "x", "x" + strings.Repeat("\u2003", 4097)} {
		for _, capacity := range []uint64{1, 3, 4096} {
			t.Run(strconv.Quote(body[:min(len(body), 24)])+"/"+strconv.FormatUint(capacity, 10), func(t *testing.T) {
				t.Parallel()
				compareTrimmedRecordWithBytes(t, []byte(body), capacity)
			})
		}
	}
}

func compareTrimmedRecordWithBytes(t *testing.T, body []byte, capacity uint64) {
	t.Helper()
	// The owning range is nonempty even when the supplied body is blank.
	data := append(append([]byte{}, body...), ' ')
	const prefix = "outside range\n"
	source := strings.NewReader(prefix + string(data) + "outside tail")
	length, err := core.NewByteLength(uint64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(capacity)
	if err != nil {
		t.Fatal(err)
	}
	got, err := lineio.TrimRecordWhitespace(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Offset: lineio.SourceByteOffset(len(prefix)), Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: buffer})
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.TrimSpace(data)
	if len(want) == 0 {
		if got != (lineio.RecordRange{}) {
			t.Fatalf("blank trim=%+v,want zero range", got)
		}
		return
	}
	offset := bytes.Index(data, want)
	if got.Validate() != nil || int64(got.Offset) != int64(len(prefix)+offset) || got.Bytes.Uint64() != uint64(len(want)) || got.Framing != lineio.RecordFramingOpaque {
		t.Fatalf("trim=%+v,want original offset/extent %d/%d", got, len(prefix)+offset, len(want))
	}
	physical := make([]byte, len(want))
	if _, err := source.ReadAt(physical, int64(got.Offset)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(physical, want) {
		t.Fatalf("trim bytes=%x,want original %x", physical, want)
	}
	position, err := source.Seek(0, io.SeekCurrent)
	if err != nil || position != 0 {
		t.Fatalf("borrowed cursor=%d/%v,want unchanged zero", position, err)
	}
}

func FuzzTrimRecordWhitespaceMatchesStandardByteTrim(f *testing.F) {
	for _, body := range []string{"frame", "\t\u2003frame\u0085", "\xff\x00\u2003", "\n\n", "\ufeffx"} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, body []byte) { compareTrimmedRecordWithBytes(t, body, 17) })
}

type trimNativeSource struct {
	*os.File
	truncate bool
	cancel   context.CancelCauseFunc
}

func (s *trimNativeSource) ReadAt(p []byte, offset int64) (int, error) {
	if s.truncate {
		if err := s.File.Truncate(0); err != nil {
			return 0, err
		}
	} else if s.cancel == nil {
		if err := s.File.Close(); err != nil {
			return 0, err
		}
	}
	n, err := s.File.ReadAt(p, offset)
	if s.cancel != nil {
		s.cancel(os.ErrPermission)
	}
	return n, err
}

func TestTrimRecordWhitespaceConservesNativeRefusals(t *testing.T) {
	t.Parallel()
	for _, selection := range []uint8{0, 1, 2, 3} {
		t.Run(strconv.Itoa(int(selection)), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "native-trim-source"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			const body = "  frame\n"
			if _, err := io.WriteString(file, body); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(context.Canceled)
			source := &trimNativeSource{File: file}
			want := error(fs.ErrClosed)
			switch selection {
			case 1:
				source.truncate = true
				want = io.ErrUnexpectedEOF
			case 2:
				source.cancel = cancel
				want = context.Canceled
			case 3:
				cancel(os.ErrPermission)
				want = context.Canceled
			}
			length, err := core.NewByteLength(uint64(len(body)))
			if err != nil {
				t.Fatal(err)
			}
			buffer, err := core.NewByteCount(4096)
			if err != nil {
				t.Fatal(err)
			}
			got, err := lineio.TrimRecordWhitespace(ctx, lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingLF}, BufferBytes: buffer})
			if got != (lineio.RecordRange{}) || !errors.Is(err, want) {
				t.Fatalf("refused trim=%+v/%v,want zero and %v", got, err, want)
			}
			if selection >= 2 && !errors.Is(err, os.ErrPermission) {
				t.Fatalf("custom native cause missing: %v", err)
			}
		})
	}
}

func TestTrimRecordWhitespaceValidatesPhysicalFraming(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		body    string
		framing lineio.RecordFraming
		valid   bool
	}{
		{body: "x\n", framing: lineio.RecordFramingLF, valid: true},
		{body: "x", framing: lineio.RecordFramingEOF, valid: true},
		{body: "x\ny", framing: lineio.RecordFramingOpaque, valid: true},
		{body: "x\ny", framing: lineio.RecordFramingLF},
		{body: "x\n", framing: lineio.RecordFramingEOF},
		{body: "x", framing: lineio.RecordFramingLF},
	} {
		t.Run(strconv.Quote(tc.body)+"/"+strconv.Itoa(int(tc.framing)), func(t *testing.T) {
			t.Parallel()
			length, err := core.NewByteLength(uint64(len(tc.body)))
			if err != nil {
				t.Fatal(err)
			}
			buffer, err := core.NewByteCount(16)
			if err != nil {
				t.Fatal(err)
			}
			got, err := lineio.TrimRecordWhitespace(t.Context(), lineio.RecordFragmentRequest{Source: strings.NewReader(tc.body), Record: lineio.RecordRange{Bytes: length, Framing: tc.framing}, BufferBytes: buffer})
			if tc.valid {
				if err != nil || got.Validate() != nil {
					t.Fatalf("valid frame=%+v/%v", got, err)
				}
			} else if got != (lineio.RecordRange{}) || !errors.Is(err, core.ErrLineIOScan) {
				t.Fatalf("invalid frame=%+v/%v,want native framing refusal", got, err)
			}
		})
	}
}

func TestTrimRecordWhitespaceRejectsNilSourceBeforeReading(t *testing.T) {
	t.Parallel()
	length, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(16)
	if err != nil {
		t.Fatal(err)
	}
	var typedNil *os.File
	for _, source := range []lineio.RecordSource{nil, typedNil} {
		got, err := lineio.TrimRecordWhitespace(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: buffer})
		if got != (lineio.RecordRange{}) || !errors.Is(err, core.ErrLineIOContract) {
			t.Fatalf("nil source=%+v/%v,want zero and native contract refusal", got, err)
		}
	}
}

type trimFillSource struct{ remaining int64 }

func (s *trimFillSource) Read(p []byte) (int, error) {
	if s.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), s.remaining)
	for i := range n {
		p[i] = 'x'
	}
	s.remaining -= n
	return int(n), nil
}

func BenchmarkTrimRecordWhitespaceNativeWorkingMemory(b *testing.B) {
	for _, extent := range []int64{65536, 16*1024*1024 + 1, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			file, err := os.Create(filepath.Join(b.TempDir(), "native-trim-benchmark"))
			if err != nil {
				b.Fatal(err)
			}
			defer file.Close()
			const prefix = " \u2003"
			written, err := io.Copy(file, io.MultiReader(strings.NewReader(prefix), &trimFillSource{remaining: extent}, strings.NewReader("\u0085\n")))
			if err != nil {
				b.Fatal(err)
			}
			length, err := core.NewByteLength(uint64(written))
			if err != nil {
				b.Fatal(err)
			}
			buffer, err := core.NewByteCount(4096)
			if err != nil {
				b.Fatal(err)
			}
			request := lineio.RecordFragmentRequest{Source: file, Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingLF}, BufferBytes: buffer}
			b.ReportAllocs()
			b.SetBytes(written)
			for b.Loop() {
				got, err := lineio.TrimRecordWhitespace(b.Context(), request)
				if err != nil || int64(got.Offset) != int64(len(prefix)) || got.Bytes.Uint64() != uint64(extent) {
					b.Fatalf("trim=%+v/%v,want native %d/%d", got, err, len(prefix), extent)
				}
			}
		})
	}
}
