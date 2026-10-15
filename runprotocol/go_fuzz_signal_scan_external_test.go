package runprotocol_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestGoFuzzSignalNeutralRecordDoesNotRepeatEveryMarkerSearch(t *testing.T) {
	t.Parallel()
	file, err := os.Create(filepath.Join(t.TempDir(), "native-neutral-signal-record"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	const extent = 65537
	if _, err := io.Copy(file, &fuzzMetricFillReader{remaining: extent, value: 'x'}); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	source := &fuzzMetricObservedFile{File: file}
	seen := 0
	for record, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
		if err != nil || record.Validate() != nil {
			t.Fatalf("native signal record/error=%+v/%v,want complete observation", record, err)
		}
		seen++
		for _, marker := range [...]lineio.RecordRange{record.Panic, record.Deadline, record.FailureHeader, record.FailingInput, record.RerunHeader, record.ReplayCommand, record.BuildHeader} {
			if marker != (lineio.RecordRange{}) {
				t.Fatalf("neutral native source invented a marker: %+v", marker)
			}
		}
	}
	if seen != 1 || source.readBytes > 3*extent {
		t.Fatalf("native records/read bytes=%d/%d for %d source bytes,want one record with at most three forward passes", seen, source.readBytes, extent)
	}
	t.Logf("native signal read bytes=%d, source bytes=%d", source.readBytes, extent)
}

func TestGoFuzzSignalFixedScanPreservesMarkerWindowCoordinates(t *testing.T) {
	t.Parallel()
	for _, literal := range []string{"panic:", "CoNtExT dEaDlInE eXcEeDeD", "FaIlInG InPuT"} {
		for split := 0; split <= len(literal); split++ {
			t.Run(literal+"/split_"+strconv.Itoa(split), func(t *testing.T) {
				t.Parallel()
				// ReadRune/Peek fill the first native buffer before the larger
				// reads begin. Cross each possible boundary of that first read.
				padding := 4096 - split
				body := strings.Repeat("x", padding) + literal + " tail\r\n"
				seen := 0
				for record, err := range runprotocol.GoFuzzSignalRecords(t.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: strings.NewReader(body)}) {
					if err != nil || record.Validate() != nil {
						t.Fatalf("window record/error=%+v/%v,want exact native marker", record, err)
					}
					seen++
					marker := record.Panic
					switch literal {
					case "panic:":
					case "CoNtExT dEaDlInE eXcEeDeD":
						marker = record.Deadline
					case "FaIlInG InPuT":
						marker = record.FailingInput
					}
					if int64(marker.Offset) != int64(padding) || marker.Bytes.Uint64() != uint64(len(literal)) {
						t.Fatalf("window marker=%+v,want original position %d/%d", marker, padding, len(literal))
					}
				}
				if seen != 1 {
					t.Fatalf("window records=%d,want one", seen)
				}
			})
		}
	}
}

func TestGoFuzzSignalFixedScanConservesNativeRefusals(t *testing.T) {
	t.Parallel()
	for _, selection := range []uint8{0, 1, 2, 3} {
		t.Run(strconv.Itoa(int(selection)), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "native-signal-refusal"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := io.Copy(file, io.MultiReader(strings.NewReader("panic: native "), &fuzzMetricFillReader{remaining: 65537, value: 'x'})); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(context.Canceled)
			var source lineio.RecordSource
			var want error
			switch selection {
			case 0:
				source = &fuzzMetricClosingPositionalFile{File: file, after: 4096}
				want = fs.ErrClosed
			case 1:
				source = &fuzzMetricTruncatingPositionalFile{File: file}
				want = io.ErrUnexpectedEOF
			case 2:
				source = file
				cancel(os.ErrPermission)
				want = context.Canceled
			case 3:
				source = &fuzzMetricCancelingPositionalFile{File: file, cancel: cancel}
				want = context.Canceled
			}
			refused := false
			for record, err := range runprotocol.GoFuzzSignalRecords(ctx, runprotocol.GoFuzzSignalSourceRequest{Source: source}) {
				if !errors.Is(err, want) || record != (runprotocol.GoFuzzSignalRecord{}) {
					t.Fatalf("native refusal record/error=%+v/%v,want withdrawn coordinates and %v", record, err, want)
				}
				if selection >= 2 && !errors.Is(err, os.ErrPermission) {
					t.Fatalf("native cancellation=%v,want custom cause", err)
				}
				refused = true
			}
			if !refused {
				t.Fatal("native refusal disappeared")
			}
		})
	}
}

func BenchmarkGoFuzzSignalFixedNativeWorkingMemory(b *testing.B) {
	for _, extent := range []int64{65536, 16*1024*1024 + 1, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			file, err := os.Create(filepath.Join(b.TempDir(), "native-signal-benchmark"))
			if err != nil {
				b.Fatal(err)
			}
			defer file.Close()
			written, err := io.Copy(file, io.MultiReader(&fuzzMetricFillReader{remaining: extent, value: 'x'}, strings.NewReader(" context deadline exceeded\n")))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(written)
			for b.Loop() {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					b.Fatal(err)
				}
				seen := 0
				for record, err := range runprotocol.GoFuzzSignalRecords(b.Context(), runprotocol.GoFuzzSignalSourceRequest{Source: file}) {
					if err != nil || int64(record.Deadline.Offset) != extent+1 || record.Deadline.Bytes.Uint64() != 25 {
						b.Fatalf("native benchmark marker/error=%+v/%v,want exact source coordinate", record.Deadline, err)
					}
					seen++
				}
				if seen != 1 {
					b.Fatalf("native benchmark records=%d,want one", seen)
				}
			}
		})
	}
}
