package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func fuzzCompletionTarget(t testing.TB, name string) runprotocol.GoFuzzTarget {
	t.Helper()
	target, err := runprotocol.ParseGoFuzzTarget(name)
	if err != nil {
		t.Fatalf("ParseGoFuzzTarget(%q) error = %v, want nil", name, err)
	}
	return target
}

func TestGoFuzzCompletionNativeProducerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, selected string
		want                 runprotocol.GoFuzzCompletion
		wantExit             int
	}{
		{name: "selected native target completes", body: `f.Add("seed"); f.Fuzz(func(t *testing.T, value string) {})`, selected: "FuzzChosen", want: runprotocol.GoFuzzCompletionPassed},
		{name: "native target without seeds still completes", body: `f.Fuzz(func(t *testing.T, value string) {})`, selected: "FuzzChosen", want: runprotocol.GoFuzzCompletionPassed},
		{name: "native target skip is not completion", body: `f.Skip("disabled by fixture")`, selected: "FuzzChosen", want: runprotocol.GoFuzzCompletionSkipped},
		{name: "missing selected target exits zero without target completion", body: `f.Add("seed"); f.Fuzz(func(t *testing.T, value string) {})`, selected: "FuzzMissing", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "completed sibling cannot certify requested target", body: `f.Add("seed"); f.Fuzz(func(t *testing.T, value string) {})`, selected: "FuzzSibling", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "failed native seed retains failure completion", body: `f.Add("seed"); f.Fuzz(func(t *testing.T, value string) { t.Fatal("fixture failure") })`, selected: "FuzzChosen", want: runprotocol.GoFuzzCompletionFailed, wantExit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, file := range [...]struct{ name, body string }{
				{"go.mod", "module example.invalid/fuzzcompletion\n\ngo 1.27.2\n"},
				{"scope_test.go", "package scope\nimport \"testing\"\nfunc FuzzChosen(f *testing.F) {" + tc.body + "}\nfunc FuzzSibling(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, value int) {}) }\n"},
			} {
				if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			stdout, err := os.Create(filepath.Join(dir, "stdout"))
			if err != nil {
				t.Fatal(err)
			}
			defer stdout.Close()
			stderr, err := os.Create(filepath.Join(dir, "stderr"))
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close()
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=readonly", "-count=1", "-vet=off", "-v", "-run=^$", "-fuzz=^"+tc.selected+"$", "-fuzztime=3s", "-fuzzminimizetime=3s", "-parallel=1", ".")
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, stdout, stderr
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOMAXPROCS=1")
			runErr := cmd.Run()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.wantExit {
				t.Fatalf("native producer exit/error = %v/%v, want %d", cmd.ProcessState, runErr, tc.wantExit)
			}
			if _, err := stdout.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			got, err := runprotocol.ObserveGoFuzzCompletion(t.Context(), runprotocol.GoFuzzCompletionRequest{Source: stdout, Target: fuzzCompletionTarget(t, "FuzzChosen")})
			if err != nil || got.Validate() != nil || got.Completion() != tc.want {
				t.Fatalf("native target observation = %v/%v, want %v", got.Completion(), err, tc.want)
			}
			if got.Completion() != runprotocol.GoFuzzCompletionAbsent {
				record := got.Record()
				length, err := record.Bytes.Int64()
				if err != nil {
					t.Fatal(err)
				}
				frame, err := io.ReadAll(io.NewSectionReader(stdout, int64(record.Offset), length))
				if err != nil || !bytes.Contains(frame, []byte(": FuzzChosen (")) {
					t.Fatalf("selected native frame = %q/%v, want exact target identity", frame, err)
				}
			}
		})
	}
}

func TestGoFuzzCompletionRecordBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	const frame = "--- PASS: FuzzChosen (0.00s)\n"
	for _, tc := range []struct {
		name, source string
		want         runprotocol.GoFuzzCompletion
		wantErr      error
	}{
		{name: "selected complete frame", source: frame, want: runprotocol.GoFuzzCompletionPassed},
		{name: "package PASS has no selected target", source: "PASS\nok\texample.invalid/pkg\t0.01s\n", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "empty source has no completion", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "target prefix does not admit sibling", source: "--- PASS: FuzzChosenSibling (0.00s)\n", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "subtarget pass does not complete top-level fuzz", source: "    --- PASS: FuzzChosen/seed (0.00s)\n", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "partial terminal record has no authority", source: strings.TrimSuffix(frame, "\n"), want: runprotocol.GoFuzzCompletionAbsent},
		{name: "skip retains its distinct native disposition", source: "--- SKIP: FuzzChosen (0.00s)\n", want: runprotocol.GoFuzzCompletionSkipped},
		{name: "failure retains its distinct native disposition", source: "--- FAIL: FuzzChosen (0.00s)\n", want: runprotocol.GoFuzzCompletionFailed},
		{name: "embedded log prefix is not a terminal frame", source: "log " + frame, want: runprotocol.GoFuzzCompletionAbsent},
		{name: "duplicate target completion refuses", source: frame + frame, wantErr: core.ErrGoToolchainOutput},
		{name: "conflicting target completion refuses", source: frame + "--- FAIL: FuzzChosen (0.00s)\n", wantErr: core.ErrGoToolchainOutput},
		{name: "failure before pass cannot be reconciled", source: "--- FAIL: FuzzChosen (0.00s)\n" + frame, wantErr: core.ErrGoToolchainOutput},
		{name: "different target frames do not conflict", source: "--- PASS: FuzzSibling (0.00s)\n" + frame, want: runprotocol.GoFuzzCompletionPassed},
		{name: "trailing diagnostic cannot become another completion", source: frame + "noise\n", want: runprotocol.GoFuzzCompletionPassed},
		{name: "malformed duration lacks native frame grammar", source: "--- PASS: FuzzChosen (NaNs)\n", want: runprotocol.GoFuzzCompletionAbsent},
		{name: "native large duration has no quota", source: "--- PASS: FuzzChosen (" + strings.Repeat("1", 65537) + ".00s)\n", want: runprotocol.GoFuzzCompletionPassed},
		{name: "long diagnostic cannot hide later terminal frame", source: strings.Repeat("x", 8*1024*1024+1) + "\n" + frame, want: runprotocol.GoFuzzCompletionPassed},
		{name: "long incomplete diagnostic creates no completion", source: strings.Repeat("x", 8*1024*1024+1), want: runprotocol.GoFuzzCompletionAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := runprotocol.ObserveGoFuzzCompletion(t.Context(), runprotocol.GoFuzzCompletionRequest{Source: strings.NewReader(tc.source), Target: fuzzCompletionTarget(t, "FuzzChosen")})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (runprotocol.GoFuzzCompletionObservation{}) {
					t.Fatalf("refusal = %v/%v, want zero/%v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.Completion() != tc.want {
				t.Fatalf("completion/error = %v/%v, want %v/nil", got.Completion(), err, tc.want)
			}
		})
	}
}

func TestGoFuzzCompletionClosedDomainAndNativeRefusal(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			t.Parallel()
			completion := runprotocol.GoFuzzCompletion(value)
			err := completion.Validate()
			wantValid := value >= int(runprotocol.GoFuzzCompletionAbsent) && value <= int(runprotocol.GoFuzzCompletionFailed)
			if (err == nil) != wantValid || err != nil && !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("completion %d error = %v, want validity %t", value, err, wantValid)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := runprotocol.ObserveGoFuzzCompletion(ctx, runprotocol.GoFuzzCompletionRequest{Source: strings.NewReader("--- PASS: FuzzChosen (0.00s)\n"), Target: fuzzCompletionTarget(t, "FuzzChosen")})
	if !errors.Is(err, context.Canceled) || got != (runprotocol.GoFuzzCompletionObservation{}) {
		t.Fatalf("canceled observation = %v/%v, want zero/canceled", got, err)
	}
	if err := (runprotocol.GoFuzzCompletionObservation{}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("absent observation error = %v, want contract refusal", err)
	}
}

type fuzzCompletionRefusingSource struct {
	*strings.Reader
	readCause, positionalCause error
}

func (s fuzzCompletionRefusingSource) Read(p []byte) (int, error) {
	n, err := s.Reader.Read(p)
	return n, errors.Join(err, s.readCause)
}

func (s fuzzCompletionRefusingSource) ReadAt(p []byte, offset int64) (int, error) {
	n, err := s.Reader.ReadAt(p, offset)
	return n, errors.Join(err, s.positionalCause)
}

func TestGoFuzzCompletionNeverExposesPartialAuthorityOnNativeSourceFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                       string
		readCause, positionalCause error
	}{
		{name: "sequential bytes plus closed-pipe refusal", readCause: io.ErrClosedPipe},
		{name: "positional bytes plus closed-pipe refusal", positionalCause: io.ErrClosedPipe},
		{name: "positional joined EOF retains refusal", positionalCause: errors.Join(io.EOF, io.ErrClosedPipe)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := fuzzCompletionRefusingSource{Reader: strings.NewReader("--- PASS: FuzzChosen (0.00s)\n"), readCause: tc.readCause, positionalCause: tc.positionalCause}
			got, err := runprotocol.ObserveGoFuzzCompletion(t.Context(), runprotocol.GoFuzzCompletionRequest{Source: source, Target: fuzzCompletionTarget(t, "FuzzChosen")})
			if !errors.Is(err, io.ErrClosedPipe) || got != (runprotocol.GoFuzzCompletionObservation{}) {
				t.Fatalf("interrupted source observation/error = %v/%v, want zero/native refusal", got, err)
			}
		})
	}
}

func FuzzGoFuzzCompletionMatchesIndependentPhysicalRecordOracle(f *testing.F) {
	dir := f.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.invalid/fuzzcompletionseed\n\ngo 1.27.2\n"), 0600); err != nil {
		f.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seed_test.go"), []byte("package seed\nimport \"testing\"\nfunc FuzzChosen(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, value int) {}) }\n"), 0600); err != nil {
		f.Fatal(err)
	}
	// The canonical valid seed is emitted by Go's actual testing.F report path.
	cmd := exec.CommandContext(f.Context(), "go", "test", "-mod=readonly", "-vet=off", "-v", "-run=^$", "-fuzz=^FuzzChosen$", "-fuzztime=1x", "-parallel=1", ".")
	cmd.Dir, cmd.Env = dir, append(os.Environ(), "GOWORK=off", "GOMAXPROCS=1")
	canonical, err := cmd.Output()
	if err != nil {
		f.Fatalf("native canonical seed error = %v, want nil", err)
	}
	f.Add(canonical)
	for _, hostile := range [...]string{"", "PASS\n", "--- PASS: FuzzChosen (0.00s)", "--- SKIP: FuzzChosen (0.00s)\n", "--- FAIL: FuzzChosen (0.00s)\n", "--- PASS: FuzzChosen (0.00s)\n--- PASS: FuzzChosen (0.00s)\n"} {
		f.Add([]byte(hostile))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		got, err := runprotocol.ObserveGoFuzzCompletion(t.Context(), runprotocol.GoFuzzCompletionRequest{Source: bytes.NewReader(body), Target: fuzzCompletionTarget(t, "FuzzChosen")})
		want, count := runprotocol.GoFuzzCompletionAbsent, 0
		// A constant-memory byte oracle checks exact physical framing. It uses
		// no production helper or regular expression, and copies no input world.
		for remaining := body; len(remaining) > 0; {
			end := bytes.IndexByte(remaining, '\n')
			if end < 0 {
				break
			}
			line := remaining[:end]
			remaining = remaining[end+1:]
			for _, disposition := range [...]struct {
				prefix     string
				completion runprotocol.GoFuzzCompletion
			}{
				{"--- PASS: FuzzChosen (", runprotocol.GoFuzzCompletionPassed},
				{"--- SKIP: FuzzChosen (", runprotocol.GoFuzzCompletionSkipped},
				{"--- FAIL: FuzzChosen (", runprotocol.GoFuzzCompletionFailed},
			} {
				if !bytes.HasPrefix(line, []byte(disposition.prefix)) || !bytes.HasSuffix(line, []byte("s)")) {
					continue
				}
				number := line[len(disposition.prefix) : len(line)-2]
				if len(number) < 4 || number[len(number)-3] != '.' {
					continue
				}
				valid := true
				for index, value := range number {
					if index != len(number)-3 && (value < '0' || value > '9') {
						valid = false
					}
				}
				if valid {
					count++
					want = disposition.completion
				}
			}
		}
		if count > 1 {
			if !errors.Is(err, core.ErrGoToolchainOutput) || got != (runprotocol.GoFuzzCompletionObservation{}) {
				t.Fatalf("duplicate observation/error = %v/%v, want zero/refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Completion() != want || got.Target() != fuzzCompletionTarget(t, "FuzzChosen") {
			t.Fatalf("record observation/error = %v/%v, want %v/nil", got.Completion(), err, want)
		}
	})
}

func BenchmarkGoFuzzCompletionFixedWorkingWindow(b *testing.B) {
	for _, extent := range [...]int{65537, 64*1024*1024 + 1} {
		b.Run(strconv.Itoa(extent), func(b *testing.B) {
			body := strings.Repeat("x", extent) + "\n--- PASS: FuzzChosen (0.00s)\n"
			target := fuzzCompletionTarget(b, "FuzzChosen")
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			for b.Loop() {
				got, err := runprotocol.ObserveGoFuzzCompletion(b.Context(), runprotocol.GoFuzzCompletionRequest{Source: strings.NewReader(body), Target: target})
				if err != nil || got.Completion() != runprotocol.GoFuzzCompletionPassed {
					b.Fatalf("completion/error = %v/%v, want pass/nil", got.Completion(), err)
				}
			}
		})
	}
}

func FuzzGoFuzzTargetMatchesGoIdentifierGrammar(f *testing.F) {
	for _, name := range [...]string{"Fuzz", "FuzzChosen", "Fuzz_", "Fuzz0", "FuzzÉpreuve", "Fuzz世界"} {
		f.Add(fuzzCompletionTarget(f, name).String())
	}
	for _, name := range [...]string{"", "Fuzzlower", "Fuzzépreuve", "Fuzz-bad", "FuzzChosen/seed", "Fuzz\xff", "TestChosen"} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		want := strings.HasPrefix(name, "Fuzz")
		for offset, value := range name {
			if !(value == '_' || unicode.IsLetter(value) || offset > 0 && unicode.IsDigit(value)) {
				want = false
			}
		}
		if len(name) > 4 {
			value, _ := utf8.DecodeRuneInString(name[4:])
			want = want && !unicode.IsLower(value)
		}
		got, err := runprotocol.ParseGoFuzzTarget(name)
		if !want {
			if !errors.Is(err, core.ErrGoToolchainContract) || got != (runprotocol.GoFuzzTarget{}) {
				t.Fatalf("target refusal = %v/%v, want zero/contract refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != name {
			t.Fatalf("target closure = %q/%v, want exact %q/nil", got.String(), err, name)
		}
		roundTrip, err := runprotocol.ParseGoFuzzTarget(got.String())
		if err != nil || roundTrip != got {
			t.Fatalf("target round trip = %v/%v, want %v/nil", roundTrip, err, got)
		}
	})
}
