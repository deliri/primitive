package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestGoBuildHeaderNativePrefixCoordinates(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"# example.invalid/pkg\n", "\u0085\u2003# ./pkg\r\n", "# \n", "## pkg\n", "x# pkg\n", "\xff# pkg\n", strings.Repeat(" ", 16*1024*1024+1) + "# example.invalid/pkg\n"} {
		t.Run(text[:min(len(text), 30)], func(t *testing.T) {
			t.Parallel()
			count := 0
			for got, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: strings.NewReader(text)}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				trimmed := strings.TrimLeftFunc(text, unicode.IsSpace)
				want := strings.HasPrefix(trimmed, "# ")
				if want {
					if got.BuildHeader.Bytes.Uint64() != 2 || int64(got.BuildHeader.Offset) != int64(len(text)-len(trimmed)) {
						t.Fatalf("header=%+v,want exact original fixed-prefix coordinate", got.BuildHeader)
					}
				} else if got.BuildHeader != (lineio.RecordRange{}) {
					t.Fatalf("header=%+v,want absent", got.BuildHeader)
				}
			}
			if count != 1 {
				t.Fatalf("records=%d,want1", count)
			}
		})
	}
}

func FuzzGoBuildHeaderCoordinatesMatchIndependentPrefix(f *testing.F) {
	f.Add([]byte("\u0085# example.invalid/pkg\n"))
	f.Add([]byte("## native\n\xff# bad\n# "))
	f.Fuzz(func(t *testing.T, source []byte) {
		lines := bytes.SplitAfter(source, []byte{'\n'})
		ordinal, offset := 0, 0
		for got, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: bytes.NewReader(source)}) {
			if err != nil {
				t.Fatal(err)
			}
			if ordinal >= len(lines) {
				t.Fatalf("record ordinal = %d, want less than %d", ordinal, len(lines))
			}
			line := lines[ordinal]
			ordinal++
			trimmed := bytes.TrimLeftFunc(line, unicode.IsSpace)
			if bytes.HasPrefix(trimmed, []byte("# ")) {
				if int64(got.BuildHeader.Offset) != int64(offset+len(line)-len(trimmed)) || got.BuildHeader.Bytes.Uint64() != 2 {
					t.Fatalf("header=%+v,want original byte prefix", got.BuildHeader)
				}
			} else if got.BuildHeader != (lineio.RecordRange{}) {
				t.Fatalf("build header = %+v, want zero", got.BuildHeader)
			}
			offset += len(line)
		}
		want := len(lines)
		if len(lines[len(lines)-1]) == 0 {
			want--
		}
		if ordinal != want {
			t.Fatalf("records=%d,want%d", ordinal, want)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		for got, err := range runprotocol.GoFuzzSignalRecords(ctx, runprotocol.GoFuzzSignalSourceRequest{Source: bytes.NewReader(source)}) {
			if !errors.Is(err, context.Canceled) || got != (runprotocol.GoFuzzSignalRecord{}) {
				t.Fatalf("canceled source observation = %+v/%v, want zero and Canceled", got, err)
			}
		}
	})
}

func TestGoBuildHeaderNativeRefusalWithdrawsCoordinates(t *testing.T) {
	t.Parallel()
	source := signalRefusingSource{Reader: bytes.NewReader([]byte("# example.invalid/pkg\n")), cause: io.ErrClosedPipe}
	count := 0
	for got, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
		count++
		if !errors.Is(err, io.ErrClosedPipe) || got != (runprotocol.GoFuzzSignalRecord{}) {
			t.Fatalf("record/error=%+v/%v,wantzero/nativecause", got, err)
		}
	}
	if count != 1 {
		t.Fatalf("refusals=%d,want1", count)
	}
}
