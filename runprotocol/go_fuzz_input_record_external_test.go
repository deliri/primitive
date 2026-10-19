package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestGoFuzzInputRecordRetainedMemoryDoesNotFollowSourceExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, tc := range []struct {
		name    string
		extent  int64
		records int
	}{
		{"single multiwindow path", 64 * 1024, 1},
		{"single path beyond former line limit", 64*1024*1024 + 1, 1},
		{"every reported input beyond former count limit", 8, 65537},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			dir := t.TempDir()
			file, err := os.Create(filepath.Join(dir, "native-input"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			for range tc.records {
				if _, err := io.Copy(file, io.MultiReader(strings.NewReader("failing input written to "), &fuzzMetricFillReader{remaining: tc.extent, value: 'x'}, strings.NewReader("\n"))); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count := 0
			for record, err := range runprotocol.GoFuzzInputRecords(t.Context(), runprotocol.GoFuzzInputSourceRequest{Source: file}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				size, _ := record.Path().Bytes.Int64()
				if size != tc.extent {
					t.Fatalf("path extent = %d, want %d", size, tc.extent)
				}
				if count == tc.records {
					runtime.GC()
					runtime.ReadMemStats(&after)
					runtime.KeepAlive(record)
					if after.HeapAlloc > before.HeapAlloc+4*1024*1024 {
						t.Fatalf("live heap growth = %d, want fixed native window below 4 MiB", after.HeapAlloc-before.HeapAlloc)
					}
				}
			}
			if count != tc.records {
				t.Fatalf("reported inputs = %d, want %d", count, tc.records)
			}
			runtime.KeepAlive(file)
		})
	}
}

func TestGoFuzzInputRecordsActualNativeFailureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		wantExit   int
		wantInput  bool
	}{
		{"actual minimized input", `f.Add(0);f.Fuzz(func(t *testing.T,value int){if value!=0 {t.Fatal("native finding")}})`, 1, true},
		{"failed seed has no new input", `f.Add(0);f.Fuzz(func(t *testing.T,value int){t.Fatal("failed baseline")})`, 1, false},
		{"skipped target has no input", `f.Skip("disabled")`, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, file := range []struct{ name, body string }{{"go.mod", "module example.invalid/nativeinput\n\ngo 1.27.2\n"}, {"input_test.go", "package nativeinput\nimport \"testing\"\nfunc FuzzInput(f *testing.F){" + tc.body + "}\n"}} {
				if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output, err := os.Create(filepath.Join(dir, "native-output"))
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=readonly", "-vet=off", "-count=1", "-v", "-run=^$", "-fuzz=^FuzzInput$", "-fuzztime=3s", "-fuzzminimizetime=3s", "-parallel=1", ".")
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, output, output
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOMAXPROCS=1")
			runErr := cmd.Run()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.wantExit {
				t.Fatalf("native state/error = %v/%v, want exit %d", cmd.ProcessState, runErr, tc.wantExit)
			}
			if _, err := output.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			count := 0
			for record, err := range runprotocol.GoFuzzInputRecords(t.Context(), runprotocol.GoFuzzInputSourceRequest{Source: output}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				span := record.Path()
				size, _ := span.Bytes.Int64()
				path, err := io.ReadAll(io.NewSectionReader(output, int64(span.Offset), size))
				if err != nil {
					t.Fatal(err)
				}
				stat, err := os.Stat(filepath.Join(dir, string(path)))
				if err != nil || !stat.Mode().IsRegular() || stat.Size() == 0 {
					t.Fatalf("actual native candidate %q stat/error = %v/%v, want nonempty regular corpus", path, stat, err)
				}
			}
			if (count > 0) != tc.wantInput {
				t.Fatalf("actual candidate count = %d, want presence %t", count, tc.wantInput)
			}
		})
	}
}

func BenchmarkGoFuzzInputRecordNativeWorkingMemory(b *testing.B) {
	for _, extent := range []int64{64 * 1024, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			dir := b.TempDir()
			file, err := os.Create(filepath.Join(dir, "native-input"))
			if err != nil {
				b.Fatal(err)
			}
			defer file.Close()
			prefix := "failing input written to "
			written, err := io.Copy(file, io.MultiReader(strings.NewReader(prefix), &fuzzMetricFillReader{remaining: extent, value: 'x'}, strings.NewReader("\n")))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(written)
			for b.Loop() {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					b.Fatal(err)
				}
				count := 0
				for record, err := range runprotocol.GoFuzzInputRecords(b.Context(), runprotocol.GoFuzzInputSourceRequest{Source: file}) {
					if err != nil {
						b.Fatal(err)
					}
					count++
					size, _ := record.Path().Bytes.Int64()
					if size != extent || int64(record.Path().Offset) != int64(len(prefix)) {
						b.Fatalf("path offset/extent = %d/%d, want %d/%d", record.Path().Offset, size, len(prefix), extent)
					}
				}
				if count != 1 {
					b.Fatalf("candidate count = %d, want one", count)
				}
			}
		})
	}
}

func TestGoFuzzInputRecordGrammarLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body, want string }{
		{"native written marker", "failing input written to testdata/fuzz/FuzzX/abc\n", "testdata/fuzz/FuzzX/abc"},
		{"direct source", "failing input seed", "seed"},
		{"mixed ASCII marker", "FaIlInG InPuT WrItTeN To seed", "seed"},
		{"unicode whitespace", "failing input\u2003written to\u2003目录/seed\u2003\n", "目录/seed"},
		{"carriage return framing", "failing input written to seed\r\n", "seed"},
		{"opaque spaces inside path", "failing input written to a b/c", "a b/c"},
		{"invalid UTF8 prefix keeps source coordinates", "\xff failing input written to seed", "seed"},
		{"invalid UTF8 path stays native bytes", "failing input written to \xffseed", "\xffseed"},
		{"invalid UTF8 byte repeats inside leading whitespace", "fAiling input\u2003\x83", "\x83"},
		{"unicode case is not ASCII marker", "faİling input written to seed", ""},
		{"embedded marker", "note: failing input written to seed", "seed"},
		{"first marker owns suffix", "failing input one failing input two", "one failing input two"},
		{"first written marker owns suffix", "failing input written to one written to two", "one written to two"},
		{"empty source", "", ""},
		{"neutral unrelated log", "ordinary output\n", ""},
		{"marker without path", "failing input", ""},
		{"written marker without path", "failing input written to\t\r\n", ""},
		{"marker cannot cross physical record", "failing\ninput written to seed", ""},
		{"empty suffix after earlier path", "failing input seed written to", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := strings.NewReader(tc.body)
			count := 0
			for record, err := range runprotocol.GoFuzzInputRecords(t.Context(), runprotocol.GoFuzzInputSourceRequest{Source: source}) {
				if err != nil || record.Validate() != nil {
					t.Fatalf("record/error = %+v/%v, want validated native span", record, err)
				}
				count++
				span := record.Path()
				size, err := span.Bytes.Int64()
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(io.NewSectionReader(source, int64(span.Offset), size))
				if err != nil || string(got) != tc.want {
					t.Fatalf("path/error = %q/%v, want %q/nil", got, err, tc.want)
				}
			}
			wantCount := 0
			if tc.want != "" {
				wantCount = 1
			}
			if count != wantCount {
				t.Fatalf("path count = %d, want %d", count, wantCount)
			}
		})
	}
}

func TestGoFuzzInputRecordRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name    string
		request runprotocol.GoFuzzInputSourceRequest
		ctx     context.Context
		want    error
	}{
		{"nil source", runprotocol.GoFuzzInputSourceRequest{}, t.Context(), core.ErrGoToolchainContract},
		{"cancelled source", runprotocol.GoFuzzInputSourceRequest{Source: strings.NewReader("failing input written to seed\n")}, ctx, context.Canceled},
		{"sequential native refusal", runprotocol.GoFuzzInputSourceRequest{Source: fuzzCompletionRefusingSource{Reader: strings.NewReader("failing input seed\n"), readCause: os.ErrPermission}}, t.Context(), os.ErrPermission},
		{"positional native refusal", runprotocol.GoFuzzInputSourceRequest{Source: fuzzCompletionRefusingSource{Reader: strings.NewReader("failing input seed\n"), positionalCause: os.ErrPermission}}, t.Context(), os.ErrPermission},
		{"joined EOF native refusal", runprotocol.GoFuzzInputSourceRequest{Source: fuzzCompletionRefusingSource{Reader: strings.NewReader("failing input seed\n"), positionalCause: errors.Join(io.EOF, os.ErrPermission)}}, t.Context(), os.ErrPermission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			count := 0
			for record, err := range runprotocol.GoFuzzInputRecords(tc.ctx, tc.request) {
				count++
				if !errors.Is(err, tc.want) || record != (runprotocol.GoFuzzInputRecord{}) {
					t.Fatalf("refused record/error = %+v/%v, want zero/%v", record, err, tc.want)
				}
			}
			if count != 1 {
				t.Fatalf("refusal count = %d, want one", count)
			}
		})
	}
	if (runprotocol.GoFuzzInputRecord{}).Validate() == nil {
		t.Fatal("zero span accepted, want refusal")
	}
}

func TestGoFuzzInputRecordsStreamNativeCoordinatesBeyondFormerLimits(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{4095, 4096, 4097, 16*1024*1024 - 1, 16 * 1024 * 1024, 16*1024*1024 + 1} {
		t.Run(strconv.FormatInt(extent, 10), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file, err := os.Create(filepath.Join(dir, "native-input"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			prefix := "failing input written to "
			written, err := io.Copy(file, io.MultiReader(strings.NewReader(prefix), &fuzzMetricFillReader{remaining: extent, value: 'x'}, strings.NewReader("\r\n")))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			count := 0
			for record, err := range runprotocol.GoFuzzInputRecords(t.Context(), runprotocol.GoFuzzInputSourceRequest{Source: file}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				size, _ := record.Path().Bytes.Int64()
				whole, _ := record.Record().Bytes.Int64()
				if int64(record.Path().Offset) != int64(len(prefix)) || size != extent || whole != written {
					t.Fatalf("path offset/extent/record = %d/%d/%d, want %d/%d/%d", record.Path().Offset, size, whole, len(prefix), extent, written)
				}
			}
			if count != 1 {
				t.Fatalf("path count = %d, want one", count)
			}
		})
	}
}

// The oracle compares byte spans with stdlib byte searching and trimming; it
// never uses the production RuneReader regular expressions or trim machinery.
func FuzzGoFuzzInputRecordsMatchIndependentByteCoordinates(f *testing.F) {
	for _, body := range []string{"failing input written to seed\n", "\xfffailing input\u2003written to\u2003a b\r\n", "ordinary\nfailing input seed", "failing input\n", "fAiling input\u2003\x83", ""} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		source := bytes.NewReader(body)
		oracleOffset := 0
		for record, err := range runprotocol.GoFuzzInputRecords(t.Context(), runprotocol.GoFuzzInputSourceRequest{Source: source}) {
			if err != nil || record.Validate() != nil {
				t.Fatalf("record/error = %+v/%v, want native span", record, err)
			}
			found := false
			for oracleOffset < len(body) {
				end := len(body)
				if next := bytes.IndexByte(body[oracleOffset:], '\n'); next >= 0 {
					end = oracleOffset + next + 1
				}
				line := body[oracleOffset:end]
				fold := append([]byte(nil), line...)
				for i, c := range fold {
					if c >= 'A' && c <= 'Z' {
						fold[i] = c + ('a' - 'A')
					}
				}
				marker := bytes.Index(fold, []byte("failing input"))
				start := 0
				if marker >= 0 {
					start = marker + len("failing input")
					if written := bytes.Index(fold[start:], []byte("written to")); written >= 0 {
						start += written + len("written to")
					}
					trimmed := bytes.TrimSpace(line[start:])
					if len(trimmed) > 0 {
						start += len(line[start:]) - len(bytes.TrimLeftFunc(line[start:], unicode.IsSpace))
						size, _ := record.Path().Bytes.Int64()
						whole, _ := record.Record().Bytes.Int64()
						if int64(record.Path().Offset) != int64(oracleOffset+start) || size != int64(len(trimmed)) || int64(record.Record().Offset) != int64(oracleOffset) || whole != int64(end-oracleOffset) {
							t.Fatalf("native coordinates = %+v, want physical %d/%d path %d/%d", record, oracleOffset, end-oracleOffset, oracleOffset+start, len(trimmed))
						}
						found = true
					}
				}
				oracleOffset = end
				if found {
					break
				}
			}
			if !found {
				t.Fatalf("unexpected path span %+v", record)
			}
		}
		// Require the producer to emit every remaining nonempty candidate.
		for oracleOffset < len(body) {
			end := len(body)
			if next := bytes.IndexByte(body[oracleOffset:], '\n'); next >= 0 {
				end = oracleOffset + next + 1
			}
			line := body[oracleOffset:end]
			fold := append([]byte(nil), line...)
			for i, c := range fold {
				if c >= 'A' && c <= 'Z' {
					fold[i] = c + ('a' - 'A')
				}
			}
			if marker := bytes.Index(fold, []byte("failing input")); marker >= 0 {
				start := marker + len("failing input")
				if written := bytes.Index(fold[start:], []byte("written to")); written >= 0 {
					start += written + len("written to")
				}
				if len(bytes.TrimSpace(line[start:])) > 0 {
					t.Fatalf("native producer omitted candidate at %d", oracleOffset)
				}
			}
			oracleOffset = end
		}
	})
}
