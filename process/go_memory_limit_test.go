package process_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestParseGoMemoryLimitNativeGrammarBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		text    string
		want    uint64
		wantErr error
	}{
		{name: "empty default", text: "", want: math.MaxInt64},
		{name: "off default", text: "off", want: math.MaxInt64},
		{name: "zero is a limit", text: "0", want: 0},
		{name: "native positive sign", text: "+1", want: 1},
		{name: "native negative zero", text: "-0", want: 0},
		{name: "leading zero bytes", text: "0001", want: 1},
		{name: "byte suffix", text: "64B", want: 64},
		{name: "kibibyte suffix", text: "1KiB", want: 1 << 10},
		{name: "caller soft budget", text: "64MiB", want: 64 << 20},
		{name: "gibibyte suffix", text: "1GiB", want: 1 << 30},
		{name: "tebibyte suffix", text: "1TiB", want: 1 << 40},
		{name: "zero scaled", text: "-0TiB", want: 0},
		{name: "maximum bytes", text: "9223372036854775807", want: math.MaxInt64},
		{name: "maximum whole kibibytes", text: "9007199254740991KiB", want: math.MaxInt64 - 1023},
		{name: "maximum whole mebibytes", text: "8796093022207MiB", want: math.MaxInt64 - (1 << 20) + 1},
		{name: "maximum whole gibibytes", text: "8589934591GiB", want: math.MaxInt64 - (1 << 30) + 1},
		{name: "maximum whole tebibytes", text: "8388607TiB", want: math.MaxInt64 - (1 << 40) + 1},
		{name: "decimal overflow", text: "9223372036854775808", wantErr: strconv.ErrRange},
		{name: "kibibyte product overflow", text: "9007199254740992KiB", wantErr: core.ErrNumericOverflow},
		{name: "mebibyte product overflow", text: "8796093022208MiB", wantErr: core.ErrNumericOverflow},
		{name: "gibibyte product overflow", text: "8589934592GiB", wantErr: core.ErrNumericOverflow},
		{name: "tebibyte product overflow", text: "8388608TiB", wantErr: core.ErrNumericOverflow},
		{name: "negative bytes", text: "-1", wantErr: core.ErrProcessContract},
		{name: "negative scaled", text: "-1MiB", wantErr: core.ErrProcessContract},
		{name: "fraction", text: "1.5GiB", wantErr: core.ErrProcessContract},
		{name: "decimal SI suffix", text: "1MB", wantErr: core.ErrProcessContract},
		{name: "wrong case", text: "1kiB", wantErr: core.ErrProcessContract},
		{name: "missing numeric prefix", text: "MiB", wantErr: core.ErrProcessContract},
		{name: "missing IEC prefix", text: "iB", wantErr: core.ErrProcessContract},
		{name: "bare byte suffix", text: "B", wantErr: core.ErrProcessContract},
		{name: "unsupported IEC prefix", text: "1PiB", wantErr: core.ErrProcessContract},
		{name: "surrounding whitespace", text: " 64MiB", wantErr: core.ErrProcessContract},
		{name: "trailing newline", text: "64MiB\n", wantErr: core.ErrProcessContract},
		{name: "embedded NUL", text: "64\x00MiB", wantErr: core.ErrProcessContract},
		{name: "non UTF8", text: "\xff", wantErr: core.ErrProcessContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := process.ParseGoMemoryLimit(tc.text)
			if tc.wantErr != nil {
				if !errors.Is(err, core.ErrProcessContract) || !errors.Is(err, tc.wantErr) || got != (core.ByteLength{}) {
					t.Fatalf("ParseGoMemoryLimit(%q)=(%v,%v), want zero and process contract/%v", tc.text, got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.Uint64() != tc.want {
				t.Fatalf("ParseGoMemoryLimit(%q)=(%d,%v), want %d/nil", tc.text, got.Uint64(), err, tc.want)
			}
		})
	}
}

// Only the spawned test executable sees the selected environment. Its runtime
// decodes GOMEMLIMIT before this reporter starts; no large allocation is made.
func TestGoMemoryLimitNativeReporter(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessOutput,
		Scope:  core.TestIsolationScopePackageProcess,
	})
	if !slices.Contains(os.Args, "--primitive-go-memory-limit-report") {
		return
	}
	if _, err := fmt.Fprint(os.Stdout, debug.SetMemoryLimit(-1)); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestParseGoMemoryLimitAgreesWithNativeRuntime(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "off", "0", "+1", "-0", "64B", "1KiB", "64MiB", "1GiB", "1TiB", "9223372036854775807", "-1", "8388608TiB", "9223372036854775808", "1MB"} {
		t.Run(strconv.Quote(text), func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			request := processRequest(t, "copy", process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
			request.Arguments = arguments(t, "-test.run=^TestGoMemoryLimitNativeReporter$", "--", "--primitive-go-memory-limit-report")
			environment, err := hostfacts.AmbientEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			name, err := process.NewEnvironmentName("GOMEMLIMIT")
			if err != nil {
				t.Fatal(err)
			}
			value, err := process.NewEnvironmentValue(text)
			if err != nil {
				t.Fatal(err)
			}
			request.Environment, err = environment.With(process.EnvironmentVariable{Name: name, Value: value})
			if err != nil {
				t.Fatal(err)
			}
			got, parseErr := process.ParseGoMemoryLimit(text)
			result, err := process.Run(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			observed, err := result.Observation()
			if err != nil {
				t.Fatal(err)
			}
			if parseErr != nil {
				if !errors.Is(parseErr, core.ErrProcessContract) || observed.ExitCode != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "malformed GOMEMLIMIT") {
					t.Fatalf("native refusal=%+v stdout=%q stderr=%q parser=%v, want exit2/empty/Go malformed refusal and typed error", observed, stdout.String(), stderr.String(), parseErr)
				}
				return
			}
			want := strconv.FormatUint(got.Uint64(), 10)
			if observed.ExitCode != 0 || stdout.String() != want || stderr.Len() != 0 {
				t.Fatalf("native limit exit=%d stdout=%q stderr=%q, want 0/%q/empty", observed.ExitCode, stdout.String(), stderr.String(), want)
			}
		})
	}
}
