package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestTrimRecordSuffixMatchesExactOriginalBytes(t *testing.T) {
	t.Parallel()
	for index, body := range []string{"", "\r\n", "\r\r\r", "frame\r\r\n", "frame \r\n", " frame\t\r\n", "\xff\x00\r\n", "a\rb\ninside\r", "\u2003frame\u00a0", "x" + strings.Repeat("\r", 65537)} {
		for _, window := range []uint64{1, 17, 4096} {
			for _, cutset := range []string{"", "\r\n", " \t\r\n"} {
				t.Run(fmt.Sprintf("body_%d/window_%d/cutset_%x", index, window, cutset), func(t *testing.T) {
					t.Parallel()
					compareSuffixCoordinatesWithStandardTrim(t, []byte(body), cutset, window)
				})
			}
		}
	}
}

func compareSuffixCoordinatesWithStandardTrim(t *testing.T, body []byte, cutset string, window uint64) {
	t.Helper()
	data := append(append([]byte{}, body...), '\r', '\n')
	const prefix = "outside\n"
	source := strings.NewReader(prefix + string(data) + "outside tail")
	size, err := core.NewByteLength(uint64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := core.NewByteCount(window)
	if err != nil {
		t.Fatal(err)
	}
	got, err := lineio.TrimRecordSuffix(t.Context(), lineio.RecordSuffixTrimRequest{Input: lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Offset: lineio.SourceByteOffset(len(prefix)), Bytes: size, Framing: lineio.RecordFramingOpaque}, BufferBytes: buffer}, Cutset: []byte(cutset)})
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.TrimRight(data, cutset)
	if len(want) == 0 {
		if got != (lineio.RecordRange{}) {
			t.Fatalf("blank suffix = %+v, want zero range", got)
		}
	} else {
		if got.Validate() != nil || got.Offset != lineio.SourceByteOffset(len(prefix)) || got.Bytes.Uint64() != uint64(len(want)) || got.Framing != lineio.RecordFramingOpaque {
			t.Fatalf("suffix coordinates = %+v, want exact original prefix %d/%d", got, len(prefix), len(want))
		}
		physical := make([]byte, len(want))
		if _, err := source.ReadAt(physical, int64(got.Offset)); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(physical, want) {
			t.Fatalf("suffix bytes = %x, want %x", physical, want)
		}
	}
	if position, err := source.Seek(0, io.SeekCurrent); err != nil || position != 0 {
		t.Fatalf("borrowed cursor = (%d,%v), want unchanged zero", position, err)
	}
}

func TestTrimRecordSuffixWithdrawsCoordinatesOnNativeRefusal(t *testing.T) {
	t.Parallel()
	for _, selection := range []uint8{0, 1, 2, 3} {
		t.Run(fmt.Sprint(selection), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "suffix-source"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			const body = "frame\r\n"
			if _, err := io.WriteString(file, body); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
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
				cancel(fs.ErrPermission)
				want = context.Canceled
			}
			size, err := core.NewByteLength(uint64(len(body)))
			if err != nil {
				t.Fatal(err)
			}
			buffer, err := core.NewByteCount(17)
			if err != nil {
				t.Fatal(err)
			}
			got, err := lineio.TrimRecordSuffix(ctx, lineio.RecordSuffixTrimRequest{Input: lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: size, Framing: lineio.RecordFramingLF}, BufferBytes: buffer}, Cutset: []byte("\r\n")})
			if got != (lineio.RecordRange{}) || !errors.Is(err, want) {
				t.Fatalf("refused suffix = (%+v,%v), want no coordinates and %v", got, err, want)
			}
			if selection >= 2 && !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("native cancellation cause disappeared: %v", err)
			}
		})
	}
}

func FuzzTrimRecordSuffixMatchesStandardByteTrim(f *testing.F) {
	for _, body := range []string{"frame", "frame \r\n", "\r\r\n", "\xff\x00\r", "a\rb\nc"} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, body []byte) { compareSuffixCoordinatesWithStandardTrim(t, body, "\r\n", 17) })
}

func BenchmarkTrimRecordSuffixFixedWorkingWindow(b *testing.B) {
	for _, size := range []int{1024, 65537, 33554433} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			source := strings.NewReader("x" + strings.Repeat("\r", size) + "\n")
			extent, err := core.NewByteLength(uint64(size + 2))
			if err != nil {
				b.Fatal(err)
			}
			buffer, err := core.NewByteCount(4096)
			if err != nil {
				b.Fatal(err)
			}
			request := lineio.RecordSuffixTrimRequest{Input: lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: extent, Framing: lineio.RecordFramingLF}, BufferBytes: buffer}, Cutset: []byte("\r\n")}
			b.ReportAllocs()
			b.SetBytes(int64(size + 2))
			b.ResetTimer()
			for b.Loop() {
				got, err := lineio.TrimRecordSuffix(b.Context(), request)
				if err != nil || got.Bytes.Uint64() != 1 || got.Offset != 0 {
					b.Fatalf("suffix = (%+v,%v), want exact one-byte prefix", got, err)
				}
			}
		})
	}
}

func TestTrimRecordSuffixAdmitsOnlyCompleteNativeRanges(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		body    string
		framing lineio.RecordFraming
		valid   bool
	}{
		{name: "LF complete", body: "x\r\n", framing: lineio.RecordFramingLF, valid: true},
		{name: "EOF complete", body: "x", framing: lineio.RecordFramingEOF, valid: true},
		{name: "opaque embedded LF", body: "x\n y", framing: lineio.RecordFramingOpaque, valid: true},
		{name: "LF missing", body: "x", framing: lineio.RecordFramingLF},
		{name: "LF before declared end", body: "x\ny", framing: lineio.RecordFramingLF},
		{name: "EOF declared over LF", body: "x\n", framing: lineio.RecordFramingEOF},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			size, err := core.NewByteLength(uint64(len(row.body)))
			if err != nil {
				t.Fatal(err)
			}
			buffer, err := core.NewByteCount(17)
			if err != nil {
				t.Fatal(err)
			}
			r := lineio.RecordSuffixTrimRequest{Input: lineio.RecordFragmentRequest{Source: strings.NewReader(row.body), Record: lineio.RecordRange{Bytes: size, Framing: row.framing}, BufferBytes: buffer}, Cutset: []byte("\r\n")}
			if err := r.Validate(); err != nil {
				t.Fatal(err)
			}
			got, err := lineio.TrimRecordSuffix(t.Context(), r)
			if row.valid {
				if err != nil || got.Bytes.Uint64() != uint64(len(bytes.TrimRight([]byte(row.body), "\r\n"))) {
					t.Fatalf("complete range=(%+v,%v),want standard suffix coordinates", got, err)
				}
			} else if got != (lineio.RecordRange{}) || !errors.Is(err, core.ErrLineIOScan) {
				t.Fatalf("incomplete range=(%+v,%v),want zero/native scan refusal", got, err)
			}
		})
	}
	if err := (lineio.RecordSuffixTrimRequest{}).Validate(); !errors.Is(err, core.ErrLineIOContract) {
		t.Fatalf("unset request=%v,want native contract refusal", err)
	}
	deadline, stopDeadline := context.WithDeadlineCause(t.Context(), time.Time{}, io.ErrClosedPipe)
	defer stopDeadline()
	for _, row := range []struct {
		name string
		ctx  context.Context
		want error
	}{{"nil", nil, core.ErrNilContext}, {"expired deadline", deadline, context.DeadlineExceeded}} {
		t.Run("context admission/"+row.name, func(t *testing.T) {
			got, err := lineio.TrimRecordSuffix(row.ctx, lineio.RecordSuffixTrimRequest{})
			if got != (lineio.RecordRange{}) || !errors.Is(err, row.want) {
				t.Fatalf("unusable context=(%+v,%v),want zero coordinates and native refusal", got, err)
			}
			if row.want == context.DeadlineExceeded && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("native context cause lost: %v", err)
			}
		})
	}
}
