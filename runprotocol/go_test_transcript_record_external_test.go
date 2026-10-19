package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestGoTestTranscriptNativeSyntaxMatchesByteOracle(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"", "neutral", "--- FAIL:", " --- FAIL: X", "x--- FAIL:", "FAIL\tpkg", "FAIL\t", "FAIL", "fail\tpkg",
		"panic:", "PANIC:", "xpanic:", "runtime error:", "RUNTIME ERROR:", "fatal error:", "FATAL ERROR:",
		"context deadline exceeded", "CONTEXT DEADLINE EXCEEDED", "signal: killed", "SIGNAL: killed", "xsignal: killed",
		"signal:", "signal: ", "SIGNAL:    ", "signal:\tkilled", "WARNING: DATA RACE", "xwarning: data race",
		"exit status 0", "exit status 1", "exit status ", "exit status 00", "exit status 0 ignored", "exit status  0",
		"exit status 0\t", "exit status 0\tignored", "exit status \t0", "exit status \u20030", "exit status 0\t ignored",
		"exit status 0\u2003 ignored", "exit status 0\t \u2003", "exit status 0\t 0", "\u2003panic:\u2003",
		"\ufeffsignal: killed", "\x00panic:", "\xffpanic:", "\xffSIGNAL: killed", "\u017fIGNAL: killed", "S\u0130GNAL: killed",
		"--- FAIL: panic: runtime error:", "FAIL\tpkg panic:", "panic: fatal error:", "fatal error: runtime error:",
		"signal: context deadline exceeded", "WARNING: DATA RACE runtime error:", "exit status 0 panic:", "\t\r",
		"FAIL\t\u2003", "\u2003FAIL\tpkg\u2003", "neutral\npanic:\nFAIL\tpkg", "panic:\r\nexit status 1",
	} {
		for _, ending := range []string{"", "\n"} {
			t.Run(strconv.Quote(body)+"/"+strconv.Quote(ending), func(t *testing.T) {
				t.Parallel()
				compareGoTestSyntaxWithBytes(t, []byte(body+ending))
			})
		}
	}
}

func compareGoTestSyntaxWithBytes(t *testing.T, data []byte) {
	t.Helper()
	for _, records := range [...]iter.Seq2[runprotocol.GoTestRecordSyntax, error]{
		runprotocol.GoTestTranscriptRecords(t.Context(), runprotocol.GoTestTranscriptRequest{Source: bytes.NewReader(data)}),
		runprotocol.GoTestTranscriptStreamRecords(t.Context(), runprotocol.GoTestTranscriptStreamRequest{Source: bytes.NewReader(data)}),
	} {
		var offset int64
		for observed, err := range records {
			if err != nil || observed.Validate() != nil {
				t.Fatalf("native syntax=%+v error=%v,want valid native record", observed, err)
			}
			remaining := data[offset:]
			n := bytes.IndexByte(remaining, '\n') + 1
			framing := lineio.RecordFramingLF
			if n == 0 {
				n = len(remaining)
				framing = lineio.RecordFramingEOF
			}
			want := goTestSyntaxByteOracle(remaining[:n])
			want.Record = observed.Record
			if observed != want || int64(observed.Record.Offset) != offset || observed.Record.Bytes.Uint64() != uint64(n) || observed.Record.Framing != framing {
				t.Fatalf("native syntax=%+v,want=%+v at %d/%d with framing=%v", observed, want, offset, n, framing)
			}
			offset += int64(n)
		}
		if offset != int64(len(data)) {
			t.Fatalf("consumed bytes=%d,want %d", offset, len(data))
		}
	}
}

func goTestSyntaxByteOracle(data []byte) runprotocol.GoTestRecordSyntax {
	trimmed := bytes.TrimSpace(data)
	lower := bytes.ToLower(trimmed)
	var nonzero bool
	if rest, ok := bytes.CutPrefix(lower, []byte("exit status ")); ok {
		value, _, _ := bytes.Cut(rest, []byte(" "))
		if len(value) == 0 {
			value = rest
		}
		nonzero = len(value) != 0 && !bytes.Equal(value, []byte("0"))
	}
	return runprotocol.GoTestRecordSyntax{
		FailureHeader: bytes.HasPrefix(trimmed, []byte("--- FAIL:")), PackageFailure: bytes.HasPrefix(trimmed, []byte("FAIL\t")),
		Panic: bytes.Contains(trimmed, []byte("panic:")), FoldedPanic: bytes.Contains(lower, []byte("panic:")),
		RuntimeError: bytes.Contains(lower, []byte("runtime error:")), FatalError: bytes.Contains(lower, []byte("fatal error:")),
		ContextDeadline: bytes.Contains(lower, []byte("context deadline exceeded")), Signal: bytes.HasPrefix(lower, []byte("signal: ")),
		DataRace: bytes.HasPrefix(lower, []byte("warning: data race")), NonZeroExitStatus: nonzero,
	}
}

func FuzzGoTestTranscriptSyntaxMatchesNativeBytes(f *testing.F) {
	for _, seed := range []string{"panic:", "FAIL\t", "exit status 0\t ignored", "S\u0130GNAL: killed", "\xffpanic:\nneutral\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(compareGoTestSyntaxWithBytes)
}

type goTestSyntaxClosingFile struct {
	*os.File
	read int
}

func (f *goTestSyntaxClosingFile) Read(p []byte) (int, error) {
	if f.read >= 4096 {
		if err := f.Close(); err != nil {
			return 0, err
		}
	}
	n, err := f.File.Read(p)
	f.read += n
	return n, err
}

func TestGoTestTranscriptLateNativeRefusalWithdrawsRecord(t *testing.T) {
	t.Parallel()
	file, err := os.Create(filepath.Join(t.TempDir(), "native-transcript"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := io.Copy(file, io.MultiReader(strings.NewReader("panic: "), &fuzzMetricFillReader{remaining: 65537, value: 'x'})); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for observed, err := range runprotocol.GoTestTranscriptRecords(t.Context(), runprotocol.GoTestTranscriptRequest{Source: &goTestSyntaxClosingFile{File: file}}) {
		if observed != (runprotocol.GoTestRecordSyntax{}) || !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("late refusal=%+v/%v,want withdrawn syntax with native closed file", observed, err)
		}
		seen++
	}
	if seen != 1 {
		t.Fatalf("refusals=%d,want one", seen)
	}
}

func TestGoTestTranscriptCancelCauseAndNilSource(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(os.ErrPermission)
	seen := false
	for observed, err := range runprotocol.GoTestTranscriptRecords(ctx, runprotocol.GoTestTranscriptRequest{Source: strings.NewReader("panic:")}) {
		if observed != (runprotocol.GoTestRecordSyntax{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrPermission) {
			t.Fatalf("canceled syntax=%+v/%v,want both native causes and no observation", observed, err)
		}
		seen = true
	}
	if !seen {
		t.Fatal("native cancellation disappeared")
	}
	seen = false
	for observed, err := range runprotocol.GoTestTranscriptRecords(t.Context(), runprotocol.GoTestTranscriptRequest{}) {
		if observed != (runprotocol.GoTestRecordSyntax{}) || !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("nil source=%+v/%v,want refusal", observed, err)
		}
		seen = true
	}
	if !seen {
		t.Fatal("nil source refusal disappeared")
	}
}

func TestGoTestTranscriptBorrowedExtentConservesNativeRefusals(t *testing.T) {
	t.Parallel()
	for _, selection := range []uint8{0, 1, 2} {
		t.Run(strconv.Itoa(int(selection)), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "native-positioned-record"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := io.WriteString(file, "panic: native coordinate\n"); err != nil {
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
				source = &fuzzMetricClosingPositionalFile{File: file}
				want = fs.ErrClosed
			case 1:
				source = &fuzzMetricTruncatingPositionalFile{File: file}
				want = io.ErrUnexpectedEOF
			case 2:
				source = &fuzzMetricCancelingPositionalFile{File: file, cancel: cancel}
				want = context.Canceled
			}
			seen := false
			for observed, err := range runprotocol.GoTestTranscriptRecords(ctx, runprotocol.GoTestTranscriptRequest{Source: source}) {
				if observed != (runprotocol.GoTestRecordSyntax{}) || !errors.Is(err, want) {
					t.Fatalf("borrowed native syntax=%+v/%v,want no syntax and %v", observed, err, want)
				}
				if selection == 2 && !errors.Is(err, os.ErrPermission) {
					t.Fatalf("custom native cause disappeared: %v", err)
				}
				seen = true
			}
			if !seen {
				t.Fatal("native positional refusal disappeared")
			}
		})
	}
}

func BenchmarkGoTestTranscriptNativeWorkingMemory(b *testing.B) {
	for _, extent := range []int64{65536, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			file, err := os.Create(filepath.Join(b.TempDir(), "native-transcript"))
			if err != nil {
				b.Fatal(err)
			}
			defer file.Close()
			written, err := io.Copy(file, io.MultiReader(&fuzzMetricFillReader{remaining: extent, value: 'x'}, strings.NewReader(" panic:\n")))
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
				for observed, err := range runprotocol.GoTestTranscriptRecords(b.Context(), runprotocol.GoTestTranscriptRequest{Source: file}) {
					if err != nil || !observed.Panic || !observed.FoldedPanic || observed.Record.Bytes.Uint64() != uint64(written) {
						b.Fatalf("native syntax=%+v/%v,want exact complete panic record", observed, err)
					}
					seen++
				}
				if seen != 1 {
					b.Fatalf("records=%d,want one", seen)
				}
			}
		})
	}
}
