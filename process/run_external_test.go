package process_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

// processTestBackstop is the deadlock backstop for every wait in this package.
// Blocking child behaviors must outlast it by a wide margin so a wedged Run is
// distinguishable from a child that simply finished first.
const processTestBackstop = 20 * time.Second

// processTestBlock is how long a deliberately blocking child stays alive. It is
// far beyond processTestBackstop so timing never decides a test outcome.
const processTestBlock = 5 * time.Minute

// processTestLingerLifetime is how long a lingering descendant keeps an
// inherited output descriptor open after its parent exits.
const processTestLingerLifetime = 3 * time.Second

// argumentSeparator joins echoed argv values. It is not NUL, because NUL is the
// one byte the Argument contract rejects, so it cannot appear inside a value.
const argumentSeparator = "\x1e"

func TestRunStreamingLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive streams stdin stdout stderr and nonzero exit", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		request := processRequest(t, "streams", process.Streams{
			Stdin:  strings.NewReader("alpha"),
			Stdout: &stdout,
			Stderr: &stderr,
		})
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil {
			t.Fatalf("process.Run() error = %v, want nil", gotErr)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Result.Validate() error = %v, want nil", err)
		}
		if _, err := got.CPUTime(); err != nil {
			t.Fatalf("Result.CPUTime() error = %v, want nil", err)
		}
		exit := resultExitCode(t, got)
		if exit != 7 || stdout.String() != "alpha" ||
			stderr.String() != "diagnostic" {
			t.Fatalf(
				"process.Run() = exit:%d stdout:%q stderr:%q, want 7/%q/%q",
				exit,
				stdout.String(),
				stderr.String(),
				"alpha",
				"diagnostic",
			)
		}
		if resultByteLength(t, got.StdinBytes) != uint64(len("alpha")) ||
			resultByteLength(t, got.StdoutBytes) != uint64(len("alpha")) ||
			resultByteLength(t, got.StderrBytes) != uint64(len("diagnostic")) {
			t.Fatalf("process.Run() byte counts do not match the three forwarded streams")
		}
	})

	t.Run("negative one byte beyond stdout bound fails typed", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		request := processRequest(t, "output:9", process.Streams{
			Stdin:  bytes.NewReader(nil),
			Stdout: &stdout,
			Stderr: io.Discard,
		})
		request.OutputLimit = byteCount(t, 8)
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessOutputLimit) ||
			!errors.Is(gotErr, core.ErrProcessStream) {
			t.Fatalf(
				"process.Run(output over bound) error = %v, want %v and %v",
				gotErr,
				core.ErrProcessOutputLimit,
				core.ErrProcessStream,
			)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("bounded-failure Result.Validate() error = %v, want nil", err)
		}
		if stdout.Len() != 8 || resultByteLength(t, got.StdoutBytes) != 8 {
			t.Fatalf(
				"bounded stdout = len:%d count:%d, want 8/8",
				stdout.Len(),
				resultByteLength(t, got.StdoutBytes),
			)
		}
		var exceeded process.OutputLimitExceeded
		if !errors.As(gotErr, &exceeded) ||
			exceeded.Stream != process.StreamStdout ||
			exceeded.Limit != request.OutputLimit {
			t.Fatalf(
				"process.Run(output over bound) typed detail = %+v from %v, want stdout/%v",
				exceeded,
				gotErr,
				request.OutputLimit,
			)
		}
	})

	t.Run("neutral silent child forwards and fabricates no bytes", func(t *testing.T) {
		t.Parallel()

		got, gotErr := process.Run(context.Background(), processRequest(
			t,
			"silent",
			process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
			},
		))
		if gotErr != nil {
			t.Fatalf("process.Run(silent) error = %v, want nil", gotErr)
		}
		if resultExitCode(t, got) != 0 ||
			resultByteLength(t, got.StdinBytes) != 0 ||
			resultByteLength(t, got.StdoutBytes) != 0 ||
			resultByteLength(t, got.StderrBytes) != 0 {
			t.Fatalf("process.Run(silent) returned nonzero exit or fabricated stream bytes")
		}
		exit, err := got.ExitCode()
		if err != nil {
			t.Fatalf("Result.ExitCode() error = %v, want nil", err)
		}
		if successful, err := exit.Success(); err != nil || !successful {
			t.Fatalf("silent ExitCode.Success() = (%t, %v), want true/nil", successful, err)
		}
		if signaled, err := exit.Signaled(); err != nil || signaled {
			t.Fatalf("silent ExitCode.Signaled() = (%t, %v), want false/nil", signaled, err)
		}
	})
}

// TestRunLowersArgvExactlyWithoutInterpretation is the primary proof of this
// package's central claim: argv reaches the child byte for byte, and no shell,
// glob, quote, or variable expansion sits between the typed Argument values and
// the executed process.
func TestRunLowersArgvExactlyWithoutInterpretation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		argv []string
	}{
		{name: "no arguments beyond the behavior selector", argv: nil},
		{name: "single empty argument survives lowering", argv: []string{""}},
		{name: "repeated empty arguments keep their exact count", argv: []string{"", "", ""}},
		{name: "internal spaces are one argument not three", argv: []string{"two more words"}},
		{name: "leading and trailing spaces are preserved", argv: []string{"  padded  "}},
		{name: "single quotes are literal bytes", argv: []string{"'quoted'"}},
		{name: "double quotes are literal bytes", argv: []string{`"quoted"`}},
		{name: "backslashes are literal bytes", argv: []string{`back\slash\\`}},
		{name: "shell command separators are literal bytes", argv: []string{"a; b && c || d"}},
		{name: "shell substitution syntax is not expanded", argv: []string{"$(id)", "`id`", "${HOME}"}},
		{name: "glob metacharacters are not expanded", argv: []string{"*", "?", "[a-z]"}},
		{name: "redirection metacharacters are literal bytes", argv: []string{">out", "<in", "2>&1", "|"}},
		{name: "newline and tab and carriage return survive", argv: []string{"line\nnext\tcol\rback"}},
		{name: "flag-shaped arguments are not consumed as flags", argv: []string{"-x", "--flag=value", "--"}},
		{name: "multibyte text survives byte for byte", argv: []string{"日本語", "emoji", "ß"}},
		{name: "invalid utf-8 bytes survive because argv is bytes", argv: []string{"\x80\xff\xfe"}},
		{name: "argument order is preserved exactly", argv: []string{"1", "2", "3", "4", "5"}},
		{name: "four kibibyte argument survives", argv: []string{strings.Repeat("x", 1<<12)}},
		{name: "many arguments preserve count and order", argv: manyArguments(64)},
		{name: "every non-NUL byte value survives in one argument", argv: []string{allNonNULBytes()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			request := processRequest(t, "argv", process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: io.Discard,
			})
			request = withArguments(t, request, tc.argv...)
			got, gotErr := process.Run(context.Background(), request)
			if gotErr != nil {
				t.Fatalf("process.Run(argv) error = %v, want nil", gotErr)
			}
			if exit := resultExitCode(t, got); exit != 0 {
				t.Fatalf("process.Run(argv) exit = %d, want 0", exit)
			}
			want := strings.Join(tc.argv, argumentSeparator)
			if gotArgv := stdout.String(); gotArgv != want {
				t.Fatalf(
					"child argv = %q (%d values), want %q (%d values)",
					gotArgv,
					len(splitArgv(gotArgv)),
					want,
					len(tc.argv),
				)
			}
		})
	}
}

func TestRunLowersEnvironmentExactlyOrInherits(t *testing.T) {
	t.Parallel()

	ambient := os.Getenv("PATH")
	if ambient == "" {
		t.Fatal("ambient PATH = empty, want a nonempty value so inheritance is observable")
	}

	t.Run("inherit mode passes the ambient value to the child", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		request := processRequest(t, "environment:PATH", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: io.Discard,
		})
		if _, gotErr := process.Run(context.Background(), request); gotErr != nil {
			t.Fatalf("process.Run(inherit) error = %v, want nil", gotErr)
		}
		if stdout.String() != ambient {
			t.Fatalf("inherited child PATH = %q, want the ambient %q", stdout.String(), ambient)
		}
	})

	t.Run("exact mode withholds every ambient value", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		request := processRequest(t, "environment:PATH", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: io.Discard,
		})
		request.Environment = process.Environment{
			Mode: process.EnvironmentModeExact,
			Variables: []process.EnvironmentVariable{
				environmentVariable(t, "PROCESS_TEST_VALUE", "exact"),
			},
		}
		if _, gotErr := process.Run(context.Background(), request); gotErr != nil {
			t.Fatalf("process.Run(exact) error = %v, want nil", gotErr)
		}
		if stdout.String() != "" {
			t.Fatalf(
				"exact-environment child PATH = %q, want empty because ambient values are withheld",
				stdout.String(),
			)
		}
	})

	t.Run("exact mode delivers each declared value verbatim", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value string
		}{
			{name: "ordinary value", value: "exact"},
			{name: "empty value is delivered as empty not absent", value: ""},
			{name: "value containing an equals sign", value: "a=b=c"},
			{name: "value containing spaces", value: "  two words  "},
			{name: "value containing a newline", value: "first\nsecond"},
			{name: "multibyte value", value: "日本語"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var stdout bytes.Buffer
				request := processRequest(t, "environment:PROCESS_TEST_VALUE", process.Streams{
					Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: io.Discard,
				})
				request.Environment = process.Environment{
					Mode: process.EnvironmentModeExact,
					Variables: []process.EnvironmentVariable{
						environmentVariable(t, "PROCESS_TEST_VALUE", tc.value),
					},
				}
				if _, gotErr := process.Run(context.Background(), request); gotErr != nil {
					t.Fatalf("process.Run(exact %q) error = %v, want nil", tc.value, gotErr)
				}
				if stdout.String() != tc.value {
					t.Fatalf("child value = %q, want %q", stdout.String(), tc.value)
				}
			})
		}
	})

	t.Run("working directory reaches the child exactly", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		request := processRequest(t, "working-directory", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: io.Discard,
		})
		directory := t.TempDir()
		resolved, err := filepath.EvalSymlinks(directory)
		if err != nil {
			t.Fatalf("filepath.EvalSymlinks(%q) error = %v, want nil", directory, err)
		}
		parsed, err := core.ParseAbsolutePath(resolved)
		if err != nil {
			t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", resolved, err)
		}
		request.WorkingDirectory = parsed
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil || resultExitCode(t, got) != 0 ||
			stdout.String() != resolved {
			t.Fatalf(
				"working-directory run = (exit:%d stdout:%q error:%v), want 0/%q/nil",
				resultExitCode(t, got),
				stdout.String(),
				gotErr,
				resolved,
			)
		}
	})
}

func TestRunExitCodeObservationsAcrossThePlatformDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code int
	}{
		{name: "zero is the only successful code", code: 0},
		{name: "one is the ordinary failure code", code: 1},
		{name: "two is a distinct failure code", code: 2},
		{name: "forty-two is an arbitrary middle code", code: 42},
		{name: "one hundred twenty five precedes the shell reserved range", code: 125},
		{name: "one hundred twenty six is the shell non-executable code", code: 126},
		{name: "one hundred twenty seven is the shell not-found code", code: 127},
		{name: "two hundred fifty four is one below the maximum", code: 254},
		{name: "two hundred fifty five is the maximum single-byte code", code: 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := processRequest(t, "exit:"+strconv.Itoa(tc.code), process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
			})
			got, gotErr := process.Run(context.Background(), request)
			if gotErr != nil {
				t.Fatalf("process.Run(exit:%d) error = %v, want nil observation", tc.code, gotErr)
			}
			exit, err := got.ExitCode()
			if err != nil {
				t.Fatalf("Result.ExitCode() error = %v, want nil", err)
			}
			gotCode, err := exit.Int()
			if err != nil {
				t.Fatalf("ExitCode.Int() error = %v, want nil", err)
			}
			gotSuccess, err := exit.Success()
			if err != nil {
				t.Fatalf("ExitCode.Success() error = %v, want nil", err)
			}
			gotSignaled, err := exit.Signaled()
			if err != nil {
				t.Fatalf("ExitCode.Signaled() error = %v, want nil", err)
			}
			if gotCode != tc.code || gotSuccess != (tc.code == 0) || gotSignaled {
				t.Fatalf(
					"exit observation = (code:%d success:%t signaled:%t), want (%d/%t/false)",
					gotCode,
					gotSuccess,
					gotSignaled,
					tc.code,
					tc.code == 0,
				)
			}
		})
	}
}

func TestRunRejectsTerminalContextBeforeStartingAChild(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ctx     context.Context
		wantErr error
		name    string
	}{
		{
			name:    "nil context is a process ingress violation",
			ctx:     nil,
			wantErr: core.ErrNilContext,
		},
		{
			name:    "already cancelled context is refused before execution",
			ctx:     cancelledContext(),
			wantErr: context.Canceled,
		},
		{
			name:    "already expired deadline is refused before execution",
			ctx:     expiredContext(),
			wantErr: context.DeadlineExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A writing behavior makes the refusal observable: if any child ran,
			// the counter and the result would carry its bytes.
			counter := &countingWriter{}
			request := processRequest(t, "output:64", process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: counter, Stderr: io.Discard,
			})
			got, gotErr := process.Run(tc.ctx, request)
			if got != (process.Result{}) ||
				!errors.Is(gotErr, tc.wantErr) ||
				!errors.Is(gotErr, core.ErrProcessContract) {
				t.Fatalf(
					"process.Run(terminal context) = (%v, %v), want zero/%v and %v",
					got,
					gotErr,
					tc.wantErr,
					core.ErrProcessContract,
				)
			}
			if counter.total() != 0 {
				t.Fatalf(
					"refused run forwarded %d bytes, want 0 because no child may start",
					counter.total(),
				)
			}
		})
	}
}

func TestRunOutputBoundPressure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		output    uint64
		limit     uint64
		wantBytes uint64
	}{
		{name: "zero child output is below one-byte bound", output: 0, limit: 1, wantBytes: 0},
		{name: "one child byte is exactly one-byte bound", output: 1, limit: 1, wantBytes: 1},
		{name: "two child bytes are one beyond one-byte bound", output: 2, limit: 1, wantBytes: 1, wantErr: core.ErrProcessOutputLimit},
		{name: "many child bytes against the minimum bound keep one byte", output: 4096, limit: 1, wantBytes: 1, wantErr: core.ErrProcessOutputLimit},
		{name: "zero child output against a large bound is neutral", output: 0, limit: 1 << 20, wantBytes: 0},
		{name: "one child byte is one below two-byte bound", output: 1, limit: 2, wantBytes: 1},
		{name: "two child bytes are exactly two-byte bound", output: 2, limit: 2, wantBytes: 2},
		{name: "three child bytes are one beyond two-byte bound", output: 3, limit: 2, wantBytes: 2, wantErr: core.ErrProcessOutputLimit},
		{name: "seven child bytes are one below eight-byte bound", output: 7, limit: 8, wantBytes: 7},
		{name: "eight child bytes are exactly eight-byte bound", output: 8, limit: 8, wantBytes: 8},
		{name: "nine child bytes are one beyond eight-byte bound", output: 9, limit: 8, wantBytes: 8, wantErr: core.ErrProcessOutputLimit},
		{name: "4095 child bytes are one below page bound", output: 4095, limit: 4096, wantBytes: 4095},
		{name: "4096 child bytes are exactly page bound", output: 4096, limit: 4096, wantBytes: 4096},
		{name: "4097 child bytes are one beyond page bound", output: 4097, limit: 4096, wantBytes: 4096, wantErr: core.ErrProcessOutputLimit},
		{name: "65535 child bytes are one below the pipe-crossing bound", output: 65535, limit: 65536, wantBytes: 65535},
		{name: "65536 child bytes are exactly the pipe-crossing bound", output: 65536, limit: 65536, wantBytes: 65536},
		{name: "65537 child bytes are one beyond the pipe-crossing bound", output: 65537, limit: 65536, wantBytes: 65536, wantErr: core.ErrProcessOutputLimit},
		{name: "one mebibyte is exactly the mebibyte bound", output: 1 << 20, limit: 1 << 20, wantBytes: 1 << 20},
		{name: "one byte beyond a mebibyte truncates at the bound", output: (1 << 20) + 1, limit: 1 << 20, wantBytes: 1 << 20, wantErr: core.ErrProcessOutputLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			counter := &countingWriter{}
			request := processRequest(
				t,
				"output:"+strconv.FormatUint(tc.output, 10),
				process.Streams{
					Stdin: bytes.NewReader(nil), Stdout: counter, Stderr: io.Discard,
				},
			)
			request.OutputLimit = byteCount(t, tc.limit)
			got, gotErr := process.Run(context.Background(), request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("process.Run() error = %v, want %v", gotErr, tc.wantErr)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Result.Validate() error = %v, want nil", err)
			}
			if counter.total() != tc.wantBytes ||
				resultByteLength(t, got.StdoutBytes) != tc.wantBytes {
				t.Fatalf(
					"forwarded stdout = writer:%d result:%d, want %d",
					counter.total(),
					resultByteLength(t, got.StdoutBytes),
					tc.wantBytes,
				)
			}
			if resultByteLength(t, got.StderrBytes) != 0 {
				t.Fatalf(
					"stderr count = %d, want 0 because the child wrote only stdout",
					resultByteLength(t, got.StderrBytes),
				)
			}
		})
	}
}

// TestRunBoundsStdoutAndStderrIndependently proves the documented contract that
// one limit applies to each output stream separately rather than to a shared
// budget, and that two simultaneous breaches both stay reachable.
func TestRunBoundsStdoutAndStderrIndependently(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		stdout        uint64
		stderr        uint64
		limit         uint64
		wantStdout    uint64
		wantStderr    uint64
		wantStdoutErr bool
		wantStderrErr bool
	}{
		{
			name:   "both streams exactly at the bound share no budget",
			stdout: 8, stderr: 8, limit: 8, wantStdout: 8, wantStderr: 8,
		},
		{
			name:   "sum far beyond the bound is accepted while each stream is within it",
			stdout: 4096, stderr: 4096, limit: 4096, wantStdout: 4096, wantStderr: 4096,
		},
		{
			name:   "only stdout breaches its own bound",
			stdout: 9, stderr: 8, limit: 8, wantStdout: 8, wantStderr: 8, wantStdoutErr: true,
		},
		{
			name:   "only stderr breaches its own bound",
			stdout: 8, stderr: 9, limit: 8, wantStdout: 8, wantStderr: 8, wantStderrErr: true,
		},
		{
			name:   "both streams breach and both failures remain reachable",
			stdout: 9, stderr: 9, limit: 8, wantStdout: 8, wantStderr: 8,
			wantStdoutErr: true, wantStderrErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := &countingWriter{}
			stderr := &countingWriter{}
			request := processRequest(
				t,
				fmt.Sprintf("both:%d:%d", tc.stdout, tc.stderr),
				process.Streams{Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: stderr},
			)
			request.OutputLimit = byteCount(t, tc.limit)
			got, gotErr := process.Run(context.Background(), request)
			wantErr := tc.wantStdoutErr || tc.wantStderrErr
			if (gotErr != nil) != wantErr {
				t.Fatalf("process.Run(both) error = %v, want error %t", gotErr, wantErr)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Result.Validate() error = %v, want nil", err)
			}
			if resultByteLength(t, got.StdoutBytes) != tc.wantStdout ||
				resultByteLength(t, got.StderrBytes) != tc.wantStderr {
				t.Fatalf(
					"result counts = stdout:%d stderr:%d, want %d/%d",
					resultByteLength(t, got.StdoutBytes),
					resultByteLength(t, got.StderrBytes),
					tc.wantStdout,
					tc.wantStderr,
				)
			}
			if stdout.total() != tc.wantStdout || stderr.total() != tc.wantStderr {
				t.Fatalf(
					"writer counts = stdout:%d stderr:%d, want %d/%d",
					stdout.total(),
					stderr.total(),
					tc.wantStdout,
					tc.wantStderr,
				)
			}
			gotStdoutErr, gotStderrErr := breachedStreams(gotErr)
			if gotStdoutErr != tc.wantStdoutErr || gotStderrErr != tc.wantStderrErr {
				t.Fatalf(
					"breached streams from %v = stdout:%t stderr:%t, want %t/%t",
					gotErr,
					gotStdoutErr,
					gotStderrErr,
					tc.wantStdoutErr,
					tc.wantStderrErr,
				)
			}
		})
	}
}

// TestRunSerializesOneWriterSharedByBothOutputStreams proves why the two
// bounded writers share a mutex. Under the race detector an unsynchronized
// forward path would be reported, and the merged count must be exact.
func TestRunSerializesOneWriterSharedByBothOutputStreams(t *testing.T) {
	t.Parallel()

	const perStream = 1 << 16
	shared := &countingWriter{}
	request := processRequest(
		t,
		fmt.Sprintf("both:%d:%d", perStream, perStream),
		process.Streams{Stdin: bytes.NewReader(nil), Stdout: shared, Stderr: shared},
	)
	got, gotErr := process.Run(context.Background(), request)
	if gotErr != nil {
		t.Fatalf("process.Run(shared writer) error = %v, want nil", gotErr)
	}
	if resultByteLength(t, got.StdoutBytes) != perStream ||
		resultByteLength(t, got.StderrBytes) != perStream {
		t.Fatalf(
			"shared-writer result = stdout:%d stderr:%d, want %d each",
			resultByteLength(t, got.StdoutBytes),
			resultByteLength(t, got.StderrBytes),
			perStream,
		)
	}
	if shared.total() != 2*perStream {
		t.Fatalf(
			"shared writer accepted %d bytes, want %d with no interleaved loss",
			shared.total(),
			2*perStream,
		)
	}
	if gotConcurrent := shared.concurrent(); gotConcurrent {
		t.Errorf("shared writer concurrent observation = %t, want false", gotConcurrent)
	}
}

// TestRunStreamsLargeStdinWithExactAccounting proves the input path streams a
// payload far larger than any internal buffer and counts every byte once.
func TestRunStreamsLargeStdinWithExactAccounting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		bytes uint64
	}{
		{name: "empty stdin produces a zero count", bytes: 0},
		{name: "single byte stdin", bytes: 1},
		{name: "one byte below the usual pipe buffer", bytes: 65535},
		{name: "exactly the usual pipe buffer", bytes: 65536},
		{name: "one byte above the usual pipe buffer", bytes: 65537},
		{name: "one mebibyte crosses many pipe fills", bytes: 1 << 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			request := processRequest(t, "stdin-count", process.Streams{
				Stdin:  io.LimitReader(filledReader{}, int64(tc.bytes)),
				Stdout: &stdout,
				Stderr: io.Discard,
			})
			got, gotErr := process.Run(context.Background(), request)
			if gotErr != nil {
				t.Fatalf("process.Run(stdin-count) error = %v, want nil", gotErr)
			}
			want := strconv.FormatUint(tc.bytes, 10)
			if stdout.String() != want {
				t.Fatalf("child observed stdin = %q bytes, want %q", stdout.String(), want)
			}
			if gotBytes := resultByteLength(t, got.StdinBytes); gotBytes != tc.bytes {
				t.Fatalf("Result.StdinBytes() = %d, want %d", gotBytes, tc.bytes)
			}
		})
	}
}

func TestRunPreservesNativeStreamFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		failing    func(error) process.Streams
		name       string
		behavior   string
		wantStream process.Stream
	}{
		{
			name:       "stdout writer failure stops and remains reachable",
			behavior:   "output:32",
			wantStream: process.StreamStdout,
			failing: func(native error) process.Streams {
				return process.Streams{
					Stdin:  bytes.NewReader(nil),
					Stdout: failingWriter{err: native},
					Stderr: io.Discard,
				}
			},
		},
		{
			name:       "stderr writer failure stops and remains reachable",
			behavior:   "both:0:32",
			wantStream: process.StreamStderr,
			failing: func(native error) process.Streams {
				return process.Streams{
					Stdin:  bytes.NewReader(nil),
					Stdout: io.Discard,
					Stderr: failingWriter{err: native},
				}
			},
		},
		{
			name:       "stdin reader failure stops and remains reachable",
			behavior:   "copy",
			wantStream: process.StreamStdin,
			failing: func(native error) process.Streams {
				return process.Streams{
					Stdin:  failingReader{err: native},
					Stdout: io.Discard,
					Stderr: io.Discard,
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			native := errors.New("test stream failed")
			request := processRequest(t, tc.behavior, tc.failing(native))
			got, gotErr := process.Run(context.Background(), request)
			if !errors.Is(gotErr, core.ErrProcessStream) ||
				!errors.Is(gotErr, native) {
				t.Fatalf(
					"process.Run(failing %v) error = %v, want %v and the native cause",
					tc.wantStream,
					gotErr,
					core.ErrProcessStream,
				)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("stream-failure Result.Validate() error = %v, want nil", err)
			}
			var failure process.StreamFailure
			if !errors.As(gotErr, &failure) ||
				failure.Stream != tc.wantStream ||
				!errors.Is(failure.Cause, native) {
				t.Fatalf(
					"stream failure = %+v from %v, want %v with the native cause",
					failure,
					gotErr,
					tc.wantStream,
				)
			}
			if err := failure.Validate(); err != nil {
				t.Fatalf("returned StreamFailure.Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestRunAccountsPartialReaderResults pins the accounting contract for legal but
// awkward reader shapes: bytes delivered alongside io.EOF are counted and are
// not a failure, while bytes delivered alongside any other error are counted and
// are a failure.
func TestRunAccountsPartialReaderResults(t *testing.T) {
	t.Parallel()

	t.Run("bytes returned with io.EOF are counted without a failure", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		request := processRequest(t, "copy", process.Streams{
			Stdin:  &finalChunkReader{payload: "trailing", err: io.EOF},
			Stdout: &stdout,
			Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil {
			t.Fatalf("process.Run(bytes with EOF) error = %v, want nil", gotErr)
		}
		if stdout.String() != "trailing" ||
			resultByteLength(t, got.StdinBytes) != uint64(len("trailing")) {
			t.Fatalf(
				"bytes with EOF = stdout:%q count:%d, want %q/%d",
				stdout.String(),
				resultByteLength(t, got.StdinBytes),
				"trailing",
				len("trailing"),
			)
		}
	})

	t.Run("bytes returned with a non-EOF error are counted and reported", func(t *testing.T) {
		t.Parallel()

		native := errors.New("test reader failed after delivering bytes")
		request := processRequest(t, "copy", process.Streams{
			Stdin:  &finalChunkReader{payload: "partial", err: native},
			Stdout: io.Discard,
			Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) || !errors.Is(gotErr, native) {
			t.Fatalf(
				"process.Run(bytes with error) error = %v, want %v and the native cause",
				gotErr,
				core.ErrProcessStream,
			)
		}
		if gotBytes := resultByteLength(t, got.StdinBytes); gotBytes != uint64(len("partial")) {
			t.Fatalf(
				"Result.StdinBytes() = %d, want %d delivered bytes counted honestly",
				gotBytes,
				len("partial"),
			)
		}
	})
}

func TestRunRejectsMalformedStreamImplementations(t *testing.T) {
	t.Parallel()

	t.Run("writer count beyond supplied bytes is rejected without false accounting", func(t *testing.T) {
		t.Parallel()

		request := processRequest(t, "output:4", process.Streams{
			Stdin:  bytes.NewReader(nil),
			Stdout: invalidCountWriter{},
			Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) ||
			!errors.Is(gotErr, io.ErrShortWrite) {
			t.Fatalf(
				"process.Run(invalid writer count) error = %v, want %v and %v",
				gotErr,
				core.ErrProcessStream,
				io.ErrShortWrite,
			)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("invalid-writer Result.Validate() error = %v, want nil", err)
		}
		if gotBytes := resultByteLength(t, got.StdoutBytes); gotBytes != 0 {
			t.Fatalf("invalid writer stdout count = %d, want 0 trusted bytes", gotBytes)
		}
	})

	t.Run("writer short count without an error is rejected as a short write", func(t *testing.T) {
		t.Parallel()

		request := processRequest(t, "output:64", process.Streams{
			Stdin:  bytes.NewReader(nil),
			Stdout: shortCountWriter{},
			Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) ||
			!errors.Is(gotErr, io.ErrShortWrite) {
			t.Fatalf(
				"process.Run(short writer count) error = %v, want %v and %v",
				gotErr,
				core.ErrProcessStream,
				io.ErrShortWrite,
			)
		}
		if gotBytes := resultByteLength(t, got.StdoutBytes); gotBytes == 0 {
			t.Fatal("short writer stdout count = 0, want the bytes the writer did accept")
		}
	})

	t.Run("reader count beyond supplied buffer is rejected as a stream failure", func(t *testing.T) {
		t.Parallel()

		request := processRequest(t, "copy", process.Streams{
			Stdin:  invalidCountReader{},
			Stdout: io.Discard,
			Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) {
			t.Fatalf(
				"process.Run(invalid reader count) error = %v, want %v",
				gotErr,
				core.ErrProcessStream,
			)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("invalid-reader Result.Validate() error = %v, want nil", err)
		}
		if gotBytes := resultByteLength(t, got.StdinBytes); gotBytes != 0 {
			t.Fatalf("invalid reader stdin count = %d, want 0 trusted bytes", gotBytes)
		}
	})

	t.Run("negative reader count is rejected as a stream failure", func(t *testing.T) {
		t.Parallel()

		request := processRequest(t, "copy", process.Streams{
			Stdin:  negativeCountReader{},
			Stdout: io.Discard,
			Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) {
			t.Fatalf(
				"process.Run(negative reader count) error = %v, want %v",
				gotErr,
				core.ErrProcessStream,
			)
		}
		if gotBytes := resultByteLength(t, got.StdinBytes); gotBytes != 0 {
			t.Fatalf("negative reader stdin count = %d, want 0 trusted bytes", gotBytes)
		}
	})

	t.Run("panicking writer is contained as a stream failure", func(t *testing.T) {
		t.Parallel()

		request := processRequest(t, "output:4", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: panicWriter{}, Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) {
			t.Fatalf(
				"process.Run(panicking writer) error = %v, want %v",
				gotErr,
				core.ErrProcessStream,
			)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("panicking-writer Result.Validate() error = %v, want nil", err)
		}
		if gotBytes := resultByteLength(t, got.StdoutBytes); gotBytes != 0 {
			t.Fatalf("panicking writer stdout count = %d, want 0 trusted bytes", gotBytes)
		}
	})

	t.Run("panicking reader is contained as a stream failure", func(t *testing.T) {
		t.Parallel()

		request := processRequest(t, "copy", process.Streams{
			Stdin: panicReader{}, Stdout: io.Discard, Stderr: io.Discard,
		})
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) {
			t.Fatalf(
				"process.Run(panicking reader) error = %v, want %v",
				gotErr,
				core.ErrProcessStream,
			)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("panicking-reader Result.Validate() error = %v, want nil", err)
		}
		if gotBytes := resultByteLength(t, got.StdinBytes); gotBytes != 0 {
			t.Fatalf("panicking reader stdin count = %d, want 0 trusted bytes", gotBytes)
		}
	})

	t.Run("nil typed writer panic is contained as a stream failure", func(t *testing.T) {
		t.Parallel()

		// A non-nil interface holding a nil pointer passes the nil-stream gate and
		// panics on first use. Containment, not the gate, is what must hold.
		var nilBuffer *bytes.Buffer
		request := processRequest(t, "output:4", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: nilBuffer, Stderr: io.Discard,
		})
		if err := request.Validate(); err != nil {
			t.Fatalf("Request.Validate() error = %v, want nil for a non-nil interface", err)
		}
		got, gotErr := process.Run(context.Background(), request)
		if !errors.Is(gotErr, core.ErrProcessStream) {
			t.Fatalf(
				"process.Run(nil typed writer) error = %v, want %v",
				gotErr,
				core.ErrProcessStream,
			)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("nil-writer Result.Validate() error = %v, want nil", err)
		}
	})
}

func TestRunCancellationReapsTheDirectChild(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	stdout := &readyWriter{ready: ready}
	request := processRequest(t, "wait", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: io.Discard,
	})
	done := make(chan runOutcome, 1)
	go func() {
		result, err := process.Run(ctx, request)
		done <- runOutcome{result: result, err: err}
	}()

	select {
	case <-ready:
		cancel()
	case <-time.After(processTestBackstop):
		t.Fatalf(
			"child readiness wait reached %s, want readiness before the deadlock backstop",
			processTestBackstop,
		)
	}

	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) ||
			!errors.Is(got.err, core.ErrProcessWait) {
			t.Fatalf(
				"process.Run(cancelled) error = %v, want %v and %v",
				got.err,
				context.Canceled,
				core.ErrProcessWait,
			)
		}
		var failure process.Failure
		if !errors.As(got.err, &failure) || failure.Kind != process.FailureKindWait {
			t.Fatalf("cancelled failure = %+v from %v, want a wait-phase failure", failure, got.err)
		}
		if err := failure.Validate(); err != nil {
			t.Fatalf("returned Failure.Validate() error = %v, want nil", err)
		}
		// Regression proof: a cancelled run once reported only the context cause
		// and destroyed the native wait error, so a caller could not inspect how
		// the child actually terminated.
		var exitErr *exec.ExitError
		if !errors.As(got.err, &exitErr) {
			t.Fatalf(
				"cancelled error = %v, want the native *exec.ExitError to stay reachable",
				got.err,
			)
		}
		if exitErr.ProcessState == nil || exitErr.ProcessState.ExitCode() != -1 {
			t.Fatalf(
				"native exit state = %v, want the signaled marker",
				exitErr.ProcessState,
			)
		}
		if err := got.result.Validate(); err != nil {
			t.Fatalf("cancelled Result.Validate() error = %v, want nil", err)
		}
		exit, err := got.result.ExitCode()
		if err != nil {
			t.Fatalf("Result.ExitCode() error = %v, want nil", err)
		}
		signaled, err := exit.Signaled()
		if err != nil || !signaled {
			t.Fatalf("cancelled ExitCode.Signaled() = (%t, %v), want true/nil", signaled, err)
		}
		successful, err := exit.Success()
		if err != nil || successful {
			t.Fatalf("cancelled ExitCode.Success() = (%t, %v), want false/nil", successful, err)
		}
	case <-time.After(processTestBackstop):
		t.Fatalf(
			"process.Run(cancelled) wait reached %s, want direct-child reaping before the deadlock backstop",
			processTestBackstop,
		)
	}
}

// TestRunWaitDelayBoundsALingeringDescendant proves WaitDelay is honored when a
// descendant outlives the direct child while holding an inherited output
// descriptor, and that the native exec.ErrWaitDelay stays reachable.
func TestRunWaitDelayBoundsALingeringDescendant(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "linger", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	request.WaitDelay = milliseconds(t, 300)
	got, gotErr := runWithinBackstop(context.Background(), t, request)
	if !errors.Is(gotErr, core.ErrProcessWait) ||
		!errors.Is(gotErr, exec.ErrWaitDelay) {
		t.Fatalf(
			"process.Run(lingering descendant) error = %v, want %v and %v",
			gotErr,
			core.ErrProcessWait,
			exec.ErrWaitDelay,
		)
	}
	var failure process.Failure
	if !errors.As(gotErr, &failure) || failure.Kind != process.FailureKindWait {
		t.Fatalf("wait-delay failure = %+v from %v, want a wait-phase failure", failure, gotErr)
	}
	if err := failure.Validate(); err != nil {
		t.Fatalf("returned Failure.Validate() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("wait-delay Result.Validate() error = %v, want nil", err)
	}
	if exit := resultExitCode(t, got); exit != 0 {
		t.Fatalf("wait-delay exit = %d, want 0 because the direct child exited normally", exit)
	}
}

// TestRunCancellationTerminatesDespiteALingeringDescendant proves cancellation
// still reaps the direct child and returns when a descendant outlives it holding
// an inherited output descriptor. It deliberately does not assert
// exec.ErrWaitDelay: os/exec gives the process ExitError precedence over the
// copy error, so on this path the delay bounds the wait without being nameable.
func TestRunCancellationTerminatesDespiteALingeringDescendant(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	request := processRequest(t, "linger-wait", process.Streams{
		Stdin:  bytes.NewReader(nil),
		Stdout: &readyWriter{ready: ready},
		Stderr: io.Discard,
	})
	request.WaitDelay = milliseconds(t, 300)
	done := make(chan runOutcome, 1)
	go func() {
		result, err := process.Run(ctx, request)
		done <- runOutcome{result: result, err: err}
	}()

	select {
	case <-ready:
		cancel()
	case <-time.After(processTestBackstop):
		t.Fatalf(
			"child readiness wait reached %s, want readiness before the deadlock backstop",
			processTestBackstop,
		)
	}

	var got process.Result
	var gotErr error
	select {
	case outcome := <-done:
		got, gotErr = outcome.result, outcome.err
	case <-time.After(processTestBackstop):
		t.Fatalf(
			"process.Run() wait reached %s, want termination before the deadlock backstop",
			processTestBackstop,
		)
	}
	if !errors.Is(gotErr, core.ErrProcessWait) ||
		!errors.Is(gotErr, context.Canceled) {
		t.Fatalf(
			"process.Run(cancelled with lingering descendant) error = %v, want %v and %v",
			gotErr,
			core.ErrProcessWait,
			context.Canceled,
		)
	}
	var failure process.Failure
	if !errors.As(gotErr, &failure) || failure.Kind != process.FailureKindWait {
		t.Fatalf("cancelled wait failure = %v, want a typed wait-phase failure", gotErr)
	}
	if err := failure.Validate(); err != nil {
		t.Fatalf("returned Failure.Validate() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("cancelled Result.Validate() error = %v, want nil", err)
	}
	exit, err := got.ExitCode()
	if err != nil {
		t.Fatalf("Result.ExitCode() error = %v, want nil", err)
	}
	signaled, err := exit.Signaled()
	if err != nil || !signaled {
		t.Fatalf("lingering-descendant ExitCode.Signaled() = (%t, %v), want true/nil", signaled, err)
	}
}

func TestRunStartFailurePressure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		prepare    func(testing.TB, process.Request) process.Request
		wantNative error
		name       string
	}{
		{
			name:       "absent command file cannot start",
			wantNative: os.ErrNotExist,
			prepare: func(tb testing.TB, request process.Request) process.Request {
				request.Command = absolutePath(tb, filepath.Join(tb.TempDir(), "missing"))
				return request
			},
		},
		{
			name:       "command inside an absent directory cannot start",
			wantNative: os.ErrNotExist,
			prepare: func(tb testing.TB, request process.Request) process.Request {
				request.Command = absolutePath(
					tb,
					filepath.Join(tb.TempDir(), "absent", "command"),
				)
				return request
			},
		},
		{
			name:       "non-executable command file cannot start",
			wantNative: os.ErrPermission,
			prepare: func(tb testing.TB, request process.Request) process.Request {
				path := filepath.Join(tb.TempDir(), "unexecutable")
				if err := os.WriteFile(path, []byte("not executable"), 0o600); err != nil {
					tb.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
				}
				request.Command = absolutePath(tb, path)
				return request
			},
		},
		{
			name: "directory as command cannot start",
			prepare: func(tb testing.TB, request process.Request) process.Request {
				request.Command = absolutePath(tb, tb.TempDir())
				return request
			},
		},
		{
			name:       "absent working directory cannot start",
			wantNative: os.ErrNotExist,
			prepare: func(tb testing.TB, request process.Request) process.Request {
				request.WorkingDirectory = absolutePath(
					tb,
					filepath.Join(tb.TempDir(), "absent"),
				)
				return request
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			counter := &countingWriter{}
			request := tc.prepare(t, processRequest(t, "output:64", process.Streams{
				Stdin: bytes.NewReader(nil), Stdout: counter, Stderr: io.Discard,
			}))
			got, gotErr := process.Run(context.Background(), request)
			if got != (process.Result{}) || !errors.Is(gotErr, core.ErrProcessStart) {
				t.Fatalf(
					"process.Run(unstartable) = (%v, %v), want zero/%v",
					got,
					gotErr,
					core.ErrProcessStart,
				)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf(
					"process.Run(unstartable) error = %v, want the native %v",
					gotErr,
					tc.wantNative,
				)
			}
			var failure process.Failure
			if !errors.As(gotErr, &failure) ||
				failure.Kind != process.FailureKindStart ||
				failure.Command != request.Command {
				t.Fatalf(
					"start failure = %+v from %v, want the exact requested command",
					failure,
					gotErr,
				)
			}
			if err := failure.Validate(); err != nil {
				t.Fatalf("returned Failure.Validate() error = %v, want nil", err)
			}
			if counter.total() != 0 {
				t.Fatalf("unstarted run forwarded %d bytes, want 0", counter.total())
			}
		})
	}
}

// TestProcessHelper is the child-process entry point for every real-child test
// in this package. In the parent run there is no behavior argument, so it
// returns immediately and asserts nothing. In a child run it never returns: it
// performs one behavior and calls os.Exit, which is why it cannot be parallel.
func TestProcessHelper(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessOutput,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	behavior, arguments, selected := helperBehavior()
	if !selected {
		return
	}
	runHelperBehavior(behavior, arguments)
	os.Exit(0)
}

func helperBehavior() (string, []string, bool) {
	for index, argument := range os.Args {
		if argument != "--" {
			continue
		}
		if index+1 >= len(os.Args) {
			return "", nil, false
		}
		return os.Args[index+1], os.Args[index+2:], true
	}
	return "", nil, false
}

func runHelperBehavior(behavior string, arguments []string) {
	switch {
	case behavior == "silent":
	case behavior == "argv":
		_, _ = io.WriteString(os.Stdout, strings.Join(arguments, argumentSeparator))
	case behavior == "streams":
		_, _ = io.Copy(os.Stdout, os.Stdin)
		_, _ = io.WriteString(os.Stderr, "diagnostic")
		os.Exit(7)
	case behavior == "copy":
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case behavior == "stdin-count":
		count, _ := io.Copy(io.Discard, os.Stdin)
		_, _ = io.WriteString(os.Stdout, strconv.FormatInt(count, 10))
	case behavior == "wait":
		_, _ = io.WriteString(os.Stdout, "ready")
		<-time.After(processTestBlock)
	case behavior == "linger":
		helperSpawnLingeringDescendant()
	case behavior == "linger-wait":
		helperSpawnLingeringDescendant()
		_, _ = io.WriteString(os.Stdout, "ready")
		<-time.After(processTestBlock)
	case behavior == "hold-descriptor":
		<-time.After(processTestLingerLifetime)
	case behavior == "working-directory":
		directory, err := os.Getwd()
		if err != nil {
			os.Exit(91)
		}
		_, _ = io.WriteString(os.Stdout, directory)
	case strings.HasPrefix(behavior, "environment:"):
		_, _ = io.WriteString(
			os.Stdout,
			os.Getenv(strings.TrimPrefix(behavior, "environment:")),
		)
	case strings.HasPrefix(behavior, "exit:"):
		helperExit(strings.TrimPrefix(behavior, "exit:"))
	case strings.HasPrefix(behavior, "output:"):
		helperWrite(os.Stdout, strings.TrimPrefix(behavior, "output:"))
	case strings.HasPrefix(behavior, "both:"):
		helperWriteBoth(strings.TrimPrefix(behavior, "both:"))
	default:
		fmt.Fprint(os.Stderr, "unknown process test behavior")
		os.Exit(93)
	}
}

// helperSpawnLingeringDescendant leaves one grandchild holding this process's
// inherited stdout after this process exits. That is the only way to reach the
// os/exec WaitDelay path without an unresponsive caller-owned stream.
func helperSpawnLingeringDescendant() {
	descendant := exec.Command(
		os.Args[0],
		"-test.run=^TestProcessHelper$",
		"--",
		"hold-descriptor",
	)
	descendant.Stdout = os.Stdout
	descendant.Stderr = os.Stderr
	if err := descendant.Start(); err != nil {
		os.Exit(94)
	}
}

func helperExit(text string) {
	code, err := strconv.Atoi(text)
	if err != nil {
		os.Exit(92)
	}
	os.Exit(code)
}

func helperWrite(destination io.Writer, text string) {
	count, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		os.Exit(92)
	}
	if _, err := io.CopyN(destination, filledReader{}, int64(count)); err != nil {
		os.Exit(95)
	}
}

func helperWriteBoth(text string) {
	stdoutText, stderrText, found := strings.Cut(text, ":")
	if !found {
		os.Exit(92)
	}
	helperWrite(os.Stdout, stdoutText)
	helperWrite(os.Stderr, stderrText)
}

func processRequest(
	tb testing.TB,
	behavior string,
	streams process.Streams,
) process.Request {
	tb.Helper()

	executable, err := os.Executable()
	if err != nil {
		tb.Fatalf("os.Executable() error = %v, want nil", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		tb.Fatalf("os.Getwd() error = %v, want nil", err)
	}
	waitDelay, err := temporal.DurationFromSeconds(2)
	if err != nil {
		tb.Fatalf("temporal.DurationFromSeconds(2) error = %v, want nil", err)
	}
	return process.Request{
		Command:          absolutePath(tb, executable),
		Arguments:        arguments(tb, "-test.run=^TestProcessHelper$", "--", behavior),
		Environment:      process.Environment{Mode: process.EnvironmentModeInherit},
		WorkingDirectory: absolutePath(tb, workingDirectory),
		Streams:          streams,
		OutputLimit:      byteCount(tb, 1<<20),
		WaitDelay:        waitDelay,
	}
}

// withArguments appends caller argv after the behavior selector so a test can
// pressure exact argv lowering without rebuilding the whole request.
func withArguments(
	tb testing.TB,
	request process.Request,
	values ...string,
) process.Request {
	tb.Helper()

	request.Arguments = append(request.Arguments, arguments(tb, values...)...)
	return request
}

func arguments(tb testing.TB, values ...string) []process.Argument {
	tb.Helper()

	result := make([]process.Argument, len(values))
	for index, value := range values {
		argument, err := process.NewArgument(value)
		if err != nil {
			tb.Fatalf("process.NewArgument(%q) error = %v, want nil", value, err)
		}
		result[index] = argument
	}
	return result
}

func absolutePath(tb testing.TB, value string) core.AbsolutePath {
	tb.Helper()

	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		tb.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func byteCount(tb testing.TB, value uint64) core.ByteCount {
	tb.Helper()

	result, err := core.NewByteCount(value)
	if err != nil {
		tb.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return result
}

func milliseconds(tb testing.TB, value uint64) temporal.Duration {
	tb.Helper()

	duration, err := temporal.DurationFromMilliseconds(value)
	if err != nil {
		tb.Fatalf("temporal.DurationFromMilliseconds(%d) error = %v, want nil", value, err)
	}
	return duration
}

func resultExitCode(tb testing.TB, result process.Result) int {
	tb.Helper()

	exit, err := result.ExitCode()
	if err != nil {
		tb.Fatalf("Result.ExitCode() error = %v, want nil", err)
	}
	value, err := exit.Int()
	if err != nil {
		tb.Fatalf("ExitCode.Int() error = %v, want nil", err)
	}
	return value
}

func resultByteLength(
	tb testing.TB,
	project func() (core.ByteLength, error),
) uint64 {
	tb.Helper()

	length, err := project()
	if err != nil {
		tb.Fatalf("Result byte projection error = %v, want nil", err)
	}
	return length.Uint64()
}

type runOutcome struct {
	err    error
	result process.Result
}

// runWithinBackstop proves a Run that is expected to terminate on its own does
// so, instead of letting a wedged wait stall the package test binary.
func runWithinBackstop(
	ctx context.Context,
	tb testing.TB,
	request process.Request,
) (process.Result, error) {
	tb.Helper()

	done := make(chan runOutcome, 1)
	go func() {
		result, err := process.Run(ctx, request)
		done <- runOutcome{result: result, err: err}
	}()
	select {
	case got := <-done:
		return got.result, got.err
	case <-time.After(processTestBackstop):
		tb.Fatalf(
			"process.Run() wait reached %s, want termination before the deadlock backstop",
			processTestBackstop,
		)
		return process.Result{}, nil
	}
}

// breachedStreams reports which output streams reached their bound, reading the
// typed detail rather than any diagnostic text.
func breachedStreams(err error) (bool, bool) {
	var stdout, stderr bool
	for _, exceeded := range collectOutputLimits(err) {
		switch exceeded.Stream {
		case process.StreamStdout:
			stdout = true
		case process.StreamStderr:
			stderr = true
		case process.StreamUnknown, process.StreamStdin:
		}
	}
	return stdout, stderr
}

// collectOutputLimits walks the whole joined error tree, because one run can
// breach both output bounds and errors.As would stop at the first match.
func collectOutputLimits(err error) []process.OutputLimitExceeded {
	switch typed := err.(type) {
	case nil:
		return nil
	case process.OutputLimitExceeded:
		return []process.OutputLimitExceeded{typed}
	case interface{ Unwrap() []error }:
		var found []process.OutputLimitExceeded
		for _, inner := range typed.Unwrap() {
			found = append(found, collectOutputLimits(inner)...)
		}
		return found
	case interface{ Unwrap() error }:
		return collectOutputLimits(typed.Unwrap())
	default:
		return nil
	}
}

func manyArguments(count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = "argument-" + strconv.Itoa(index)
	}
	return values
}

// allNonNULBytes builds one argument holding every byte the Argument contract
// admits, proving argv lowering is byte transparent rather than text shaped.
func allNonNULBytes() string {
	raw := make([]byte, 0, 255)
	for value := 1; value <= 255; value++ {
		raw = append(raw, byte(value))
	}
	return string(raw)
}

func splitArgv(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, argumentSeparator)
}

type countingWriter struct {
	mu       sync.Mutex
	count    uint64
	active   int
	overlaps int
}

func (w *countingWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	w.active++
	overlapping := w.active > 1
	w.mu.Unlock()

	w.mu.Lock()
	if overlapping {
		w.overlaps++
	}
	w.count += uint64(len(value))
	w.active--
	w.mu.Unlock()
	return len(value), nil
}

func (w *countingWriter) total() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

func (w *countingWriter) concurrent() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.overlaps > 0
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

// finalChunkReader delivers one payload together with a terminal error, the
// legal reader shape that io.Reader documents and most implementations skip.
type finalChunkReader struct {
	err       error
	payload   string
	delivered bool
}

func (r *finalChunkReader) Read(buffer []byte) (int, error) {
	if r.delivered {
		return 0, r.err
	}
	r.delivered = true
	count := copy(buffer, r.payload)
	if count < len(r.payload) {
		return count, nil
	}
	return count, r.err
}

type readyWriter struct {
	ready chan struct{}
	once  sync.Once
}

func (w *readyWriter) Write(value []byte) (int, error) {
	if strings.Contains(string(value), "ready") {
		w.once.Do(func() { close(w.ready) })
	}
	return len(value), nil
}

// filledReader is an endless source of one non-zero byte, so a truncated or
// fabricated stream is visible in the payload as well as in the counters.
type filledReader struct{}

func (filledReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = 'x'
	}
	return len(value), nil
}

type invalidCountWriter struct{}

func (invalidCountWriter) Write(value []byte) (int, error) {
	return len(value) + 1, nil
}

type shortCountWriter struct{}

func (shortCountWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	return len(value) - 1, nil
}

type invalidCountReader struct{}

func (invalidCountReader) Read(value []byte) (int, error) {
	return len(value) + 1, nil
}

type negativeCountReader struct{}

func (negativeCountReader) Read([]byte) (int, error) {
	return -1, nil
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) {
	panic("test writer panic")
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("test reader panic")
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	cancel()
	return ctx
}
