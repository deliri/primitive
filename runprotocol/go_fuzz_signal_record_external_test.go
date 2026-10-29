package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/runprotocol"
	"io"
	"strings"
	"testing"
	"unicode"
)

func TestGoFuzzSignalNativeCoordinatesLayerTriad(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"ordinary output\n", "panic: native\n", "xx context deadline exceeded\n", "\u0085--- FAIL: FuzzSelected\n", "fAiLiNg InPuT written to seed\n", " \tTo re-run:\n", "\u2003go test -run=FuzzSelected/seed\n", strings.Repeat("x", 16*1024*1024+1) + "context deadline exceeded\n", "panic: context deadline exceeded failing input\n", "\xffpanic:\n", ""} {
		t.Run(strings.TrimSpace(text[:min(len(text), 35)]), func(t *testing.T) {
			t.Parallel()
			source := strings.NewReader(text)
			count := 0
			for observed, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				if err := observed.Validate(); err != nil {
					t.Fatal(err)
				}
				markers := [...]struct {
					span    lineio.RecordRange
					literal string
				}{{observed.Panic, "panic:"}, {observed.Deadline, "context deadline exceeded"}, {observed.FailureHeader, "--- FAIL:"}, {observed.FailingInput, "failing input"}, {observed.RerunHeader, "to re-run:"}, {observed.ReplayCommand, "go test "}}
				for _, marker := range markers {
					folded := []byte(text)
					for i, b := range folded {
						if b >= 'A' && b <= 'Z' {
							folded[i] = b + 32
						}
					}
					want := bytes.Index(folded, []byte(marker.literal))
					if marker.literal == "panic:" || marker.literal == "--- FAIL:" || marker.literal == "go test " {
						want = strings.Index(text, marker.literal)
					}
					if want < 0 {
						if marker.span != (lineio.RecordRange{}) {
							t.Fatalf("absent %q span=%+v", marker.literal, marker.span)
						}
						continue
					}
					size, err := marker.span.Bytes.Int64()
					if err != nil {
						t.Fatal(err)
					}
					if int64(marker.span.Offset) != int64(want) || size != int64(len(marker.literal)) {
						t.Fatalf("%q span=%+v,want offset %d size %d", marker.literal, marker.span, want, len(marker.literal))
					}
				}
			}
			if (text == "" && count != 0) || (text != "" && count != 1) {
				t.Fatalf("records=%d,want exact physical source count", count)
			}
		})
	}
}

func FuzzGoFuzzSignalNativeMatchesIndependentBytes(f *testing.F) {
	for _, text := range []string{"panic: x\n", "context deadline exceeded", "\xff failing input", "ordinary", "\u0085--- FAIL: X\n"} {
		f.Add([]byte(text))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		source := bytes.NewReader(body)
		ordinal := 0
		lines := bytes.SplitAfter(body, []byte{'\n'})
		for got, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
			if err != nil {
				t.Fatal(err)
			}
			if ordinal >= len(lines) {
				t.Fatalf("native record ordinal = %d, want less than %d", ordinal, len(lines))
			}
			line := lines[ordinal]
			ordinal++
			start := int(got.Record.Offset)
			if got.Record.Bytes.Uint64() != uint64(len(line)) {
				t.Fatalf("physical record extent = %d, want %d", got.Record.Bytes.Uint64(), len(line))
			}
			folded := append([]byte{}, line...)
			for i, b := range folded {
				if b >= 'A' && b <= 'Z' {
					folded[i] = b + 32
				}
			}
			trimmed := bytes.TrimLeftFunc(line, unicode.IsSpace)
			prefixOffset := len(line) - len(trimmed)
			for _, pair := range []struct {
				span         lineio.RecordRange
				literal      string
				fold, prefix bool
			}{
				{got.Panic, "panic:", false, false}, {got.Deadline, "context deadline exceeded", true, false},
				{got.FailingInput, "failing input", true, false}, {got.FailureHeader, "--- FAIL:", false, true},
				{got.RerunHeader, "to re-run:", true, true}, {got.ReplayCommand, "go test ", false, true},
			} {
				haystack := line
				if pair.fold {
					haystack = folded
				}
				want := bytes.Index(haystack, []byte(pair.literal))
				if pair.prefix {
					want = -1
					if bytes.HasPrefix(haystack[prefixOffset:], []byte(pair.literal)) {
						want = prefixOffset
					}
				}
				if want < 0 {
					if pair.span != (lineio.RecordRange{}) {
						t.Fatalf("invented %s coordinate", pair.literal)
					}
				} else if int(pair.span.Offset) != start+want || pair.span.Bytes.Uint64() != uint64(len(pair.literal)) {
					t.Fatalf("%s coordinates differ from original bytes", pair.literal)
				}
			}
		}
		wantCount := len(lines)
		if len(lines[len(lines)-1]) == 0 {
			wantCount--
		}
		if ordinal != wantCount {
			t.Fatalf("count=%d,want %d", ordinal, wantCount)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		source.Seek(0, io.SeekStart)
		for got, err := range runprotocol.GoFuzzSignalRecords(ctx, runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
			if !errors.Is(err, context.Canceled) || got != (runprotocol.GoFuzzSignalRecord{}) {
				t.Fatalf("canceled source observation = %+v/%v, want zero and Canceled", got, err)
			}
		}
	})
}

type signalRefusingSource struct {
	*bytes.Reader
	cause error
}

func (s signalRefusingSource) ReadAt(p []byte, offset int64) (int, error) {
	n, err := s.Reader.ReadAt(p, offset)
	return n, errors.Join(err, s.cause)
}

func TestGoFuzzSignalNativeRefusalWithdrawsCoordinates(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{io.ErrClosedPipe, io.ErrUnexpectedEOF, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			source := signalRefusingSource{Reader: bytes.NewReader([]byte("panic: already matched\n")), cause: errors.Join(io.EOF, cause)}
			count := 0
			for got, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
				count++
				if !errors.Is(err, cause) || got != (runprotocol.GoFuzzSignalRecord{}) {
					t.Fatalf("native observation/error=%+v/%v,want zero and original refusal", got, err)
				}
			}
			if count != 1 {
				t.Fatalf("observations=%d,want one refusal", count)
			}
		})
	}
}
