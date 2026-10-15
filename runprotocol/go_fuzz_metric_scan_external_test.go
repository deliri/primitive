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

	"github.com/deliri/primitive/v2026/runprotocol"
)

// Counts actual native read bytes without substituting the file's behavior.
type fuzzMetricObservedFile struct {
	*os.File
	readBytes int64
}

func (source *fuzzMetricObservedFile) Read(data []byte) (int, error) {
	n, err := source.File.Read(data)
	source.readBytes += int64(n)
	return n, err
}

func (source *fuzzMetricObservedFile) ReadAt(data []byte, offset int64) (int, error) {
	n, err := source.File.ReadAt(data, offset)
	source.readBytes += int64(n)
	return n, err
}

func TestGoFuzzMetricNeutralRecordDoesNotRepeatEveryGrammarSearch(t *testing.T) {
	t.Parallel()
	file, err := os.Create(filepath.Join(t.TempDir(), "native-neutral-record"))
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
	for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: source}) {
		if err != nil {
			t.Fatal(err)
		}
		seen++
		if record.InterestingCumulative() || record.FailingInput() {
			t.Fatal("neutral native bytes invented a transcript observation")
		}
		for kind := runprotocol.GoFuzzMetricExecutions; kind <= runprotocol.GoFuzzMetricInteresting; kind++ {
			field, err := record.Field(kind)
			if err != nil || field.Presence() != runprotocol.GoFuzzMetricAbsent {
				t.Fatalf("neutral counter %d=%v/%v,want absent", kind, field, err)
			}
		}
	}
	if seen != 1 {
		t.Fatalf("native record count=%d,want one complete record", seen)
	}
	if source.readBytes > 3*extent {
		t.Fatalf("native source read %d bytes for %d neutral bytes,want at most framing plus two forward observation passes", source.readBytes, extent)
	}
	t.Logf("native read bytes=%d, source bytes=%d", source.readBytes, extent)
}

func TestGoFuzzMetricHintsPreserveEveryLiteralAcrossNativeWindows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ marker, body string }{
		{"executions:", "executions: 37"},
		{"execs:", "execs: 37"},
		{"crashers:", "crashers: 37"},
		{"new interesting", "7 new interesting inputs found"},
		{"new interesting:", "new interesting: 37"},
		{"(total:", "(total: 37)"},
		{"FaIlInG InPuT", "FaIlInG InPuT written to native-seed"},
		{"execs:", "unanchored text says execs: 37"},
	} {
		for split := 0; split <= len(tc.marker); split++ {
			t.Run(tc.marker+"/split_"+strconv.Itoa(split), func(t *testing.T) {
				t.Parallel()
				// Span the first fixed read at every byte of each spelling.
				padding := 32783 - split - strings.Index(tc.body, tc.marker)
				body := strings.Repeat(" ", padding) + tc.body + "\r\n"
				file, err := os.Create(filepath.Join(t.TempDir(), "native-window"))
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if _, err := io.WriteString(file, body); err != nil {
					t.Fatal(err)
				}
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				want, cumulative, failing := fuzzMetricOracleRecord([]byte(body))
				seen := 0
				for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: file}) {
					if err != nil || record.Validate() != nil {
						t.Fatalf("native window record/error=%+v/%v,want complete source observation", record, err)
					}
					seen++
					if record.InterestingCumulative() != cumulative || record.FailingInput() != failing {
						t.Fatalf("window flags=%t/%t,want independent byte grammar %t/%t", record.InterestingCumulative(), record.FailingInput(), cumulative, failing)
					}
					for index, expected := range want {
						field, err := record.Field(runprotocol.GoFuzzMetricKind(index + 1))
						value, present := field.Value()
						if err != nil || field.Presence() != expected.presence || value != expected.value || present != (expected.presence == runprotocol.GoFuzzMetricPresent) {
							t.Fatalf("window field %d=%d/%v/%v,want independent byte grammar %d/%v", index, value, field.Presence(), err, expected.value, expected.presence)
						}
					}
				}
				if seen != 1 {
					t.Fatalf("native window records=%d,want one", seen)
				}
			})
		}
	}
}

type fuzzMetricClosingPositionalFile struct {
	*os.File
	after int64
}

func (source *fuzzMetricClosingPositionalFile) ReadAt(data []byte, offset int64) (int, error) {
	if offset >= source.after {
		if err := source.File.Close(); err != nil {
			return 0, err
		}
	}
	return source.File.ReadAt(data, offset)
}

func TestGoFuzzMetricFilterCannotPublishBeforeLateNativeReadCompletes(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"neutral ", "failing input ", "execs: 37 "} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "native-late-refusal"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := io.Copy(file, io.MultiReader(strings.NewReader(prefix), &fuzzMetricFillReader{remaining: 65537, value: 'x'})); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			source := &fuzzMetricClosingPositionalFile{File: file, after: 32783}
			refused := false
			for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: source}) {
				if !errors.Is(err, fs.ErrClosed) || record != (runprotocol.GoFuzzMetricRecord{}) {
					t.Fatalf("late native capture record/error=%+v/%v,want no observation and real closed-file cause", record, err)
				}
				refused = true
			}
			if !refused {
				t.Fatal("late native refusal vanished")
			}
		})
	}
}

type fuzzMetricTruncatingPositionalFile struct {
	*os.File
	truncated bool
}

func (source *fuzzMetricTruncatingPositionalFile) ReadAt(data []byte, offset int64) (int, error) {
	if !source.truncated {
		source.truncated = true
		if err := source.File.Truncate(0); err != nil {
			return 0, err
		}
	}
	return source.File.ReadAt(data, offset)
}

func TestGoFuzzMetricFilterRefusesNativeSourceTruncatedAfterFraming(t *testing.T) {
	t.Parallel()
	file, err := os.Create(filepath.Join(t.TempDir(), "native-truncated-replay"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := io.Copy(file, &fuzzMetricFillReader{remaining: 65537, value: 'x'}); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	source := &fuzzMetricTruncatingPositionalFile{File: file}
	refused := false
	for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: source}) {
		if !errors.Is(err, io.ErrUnexpectedEOF) || record != (runprotocol.GoFuzzMetricRecord{}) {
			t.Fatalf("truncated native source record/error=%+v/%v,want withdrawn observation and native short-source refusal", record, err)
		}
		refused = true
	}
	if !refused || !source.truncated {
		t.Fatalf("source refusal/truncation=%t/%t,want actual native truncation and typed refusal", refused, source.truncated)
	}
}

type fuzzMetricCancelingPositionalFile struct {
	*os.File
	cancel context.CancelCauseFunc
}

func (source *fuzzMetricCancelingPositionalFile) ReadAt(data []byte, offset int64) (int, error) {
	n, err := source.File.ReadAt(data, offset)
	source.cancel(os.ErrPermission)
	return n, err
}

func TestGoFuzzMetricNativeReadPreservesCustomCancellationCause(t *testing.T) {
	t.Parallel()
	for _, beforeRead := range []bool{true, false} {
		t.Run(strconv.FormatBool(beforeRead), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "native-cancellation"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := io.Copy(file, &fuzzMetricFillReader{remaining: 65537, value: 'x'}); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(context.Canceled)
			if beforeRead {
				cancel(os.ErrPermission)
			}
			source := &fuzzMetricCancelingPositionalFile{File: file, cancel: cancel}
			refused := false
			for record, err := range runprotocol.GoFuzzMetricRecords(ctx, runprotocol.GoFuzzMetricSourceRequest{Source: source}) {
				if !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrPermission) || record != (runprotocol.GoFuzzMetricRecord{}) {
					t.Fatalf("native cancellation record/error=%+v/%v,want withdrawn observation and both context identities", record, err)
				}
				refused = true
			}
			if !refused {
				t.Fatal("native cancellation disappeared")
			}
		})
	}
}
