package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestGoFuzzMetricRecordNativeProducerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		wantExit   int
		wantWork   bool
	}{
		{name: "actual fuzz coordinator emits measured counters", body: `f.Add(1);f.Fuzz(func(t *testing.T,value int){})`, wantWork: true},
		{name: "failed native seed does not invent executions", body: `f.Add(1);f.Fuzz(func(t *testing.T,value int){t.Fatal("native refusal")})`, wantExit: 1},
		{name: "skipped native target has no metric authority", body: `f.Skip("native disabled scope")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, file := range []struct{ name, body string }{{"go.mod", "module example.invalid/fuzzmetrics\n\ngo 1.27.2\n"}, {"metric_test.go", "package metric\nimport \"testing\"\nfunc FuzzMetric(f *testing.F){" + tc.body + "}\n"}} {
				if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output, err := os.Create(filepath.Join(dir, "native-output"))
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=readonly", "-vet=off", "-count=1", "-v", "-run=^$", "-fuzz=^FuzzMetric$", "-fuzztime=3s", "-fuzzminimizetime=3s", "-parallel=1", ".")
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, output, output
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOMAXPROCS=1")
			runErr := cmd.Run()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.wantExit {
				t.Fatalf("native producer state/error = %v/%v, want exit %d", cmd.ProcessState, runErr, tc.wantExit)
			}
			if _, err := output.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			var measured bool
			var priorEnd int64
			for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: output}) {
				if err != nil || record.Validate() != nil {
					t.Fatalf("native metric record/error = %+v/%v, want validated record", record, err)
				}
				span := record.Record()
				size, err := span.Bytes.Int64()
				if err != nil {
					t.Fatal(err)
				}
				if int64(span.Offset) != priorEnd {
					t.Fatalf("native record offset = %d, want contiguous %d", span.Offset, priorEnd)
				}
				priorEnd += size
				field, err := record.Field(runprotocol.GoFuzzMetricExecs)
				if err != nil {
					t.Fatal(err)
				}
				if value, ok := field.Value(); ok && value > 0 {
					measured = true
				}
			}
			if measured != tc.wantWork {
				t.Fatalf("native measured work = %t, want %t", measured, tc.wantWork)
			}
		})
	}
}

func TestGoFuzzMetricRecordGrammarLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body          string
		kind                runprotocol.GoFuzzMetricKind
		presence            runprotocol.GoFuzzMetricPresence
		value               int64
		cumulative, failing bool
	}{
		{name: "ordinary executions", body: "executions: 37", kind: runprotocol.GoFuzzMetricExecutions, presence: runprotocol.GoFuzzMetricPresent, value: 37},
		{name: "coordinator comma space", body: "fuzz: elapsed: 3s, execs: 71 (23/sec)", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: 71},
		{name: "coordinator comma without space", body: "fuzz: elapsed: 3s,execs: 71", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: 71},
		{name: "zero remains observed", body: "execs: 0", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent},
		{name: "signed negative remains native data", body: "execs: -37", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: -37},
		{name: "positive maximum", body: "execs: 9223372036854775807", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: math.MaxInt64},
		{name: "negative minimum", body: "execs: -9223372036854775808", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: math.MinInt64},
		{name: "overflow remains malformed", body: "execs: 9223372036854775808", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "negative overflow remains malformed", body: "execs: -9223372036854775809", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "empty counter remains malformed", body: "execs:", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "sign only remains malformed", body: "execs: +", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "decimal syntax remains malformed", body: "execs: 1.5", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "interior tab remains numeric data", body: "execs: 37\tx", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "trailing tab trims before parse", body: "execs: 37\t", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: 37},
		{name: "trailing commas are formatting", body: "execs: 37,,,", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: 37},
		{name: "leading comma is malformed", body: "execs: ,37", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricMalformed},
		{name: "unanchored log cannot become executions", body: "some log says execs: 37", kind: runprotocol.GoFuzzMetricExecs},
		{name: "uppercase prefix is irrelevant", body: "EXECS: 37", kind: runprotocol.GoFuzzMetricExecs},
		{name: "broken prefix is irrelevant", body: "exec: 37", kind: runprotocol.GoFuzzMetricExecs},
		{name: "Unicode leading whitespace flows", body: "\u2003execs: 37\u2003", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: 37},
		{name: "negative zero remains numeric zero", body: "execs: -000", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent},
		{name: "explicit positive sign", body: "execs: +37", kind: runprotocol.GoFuzzMetricExecs, presence: runprotocol.GoFuzzMetricPresent, value: 37},
		{name: "delta interesting counter", body: "7 new interesting inputs found", kind: runprotocol.GoFuzzMetricInteresting, presence: runprotocol.GoFuzzMetricPresent, value: 7},
		{name: "coordinator cumulative counter", body: "fuzz: elapsed: 3s, new interesting: 7 (total: 9)", kind: runprotocol.GoFuzzMetricInteresting, presence: runprotocol.GoFuzzMetricPresent, value: 7, cumulative: true},
		{name: "total spelling declares native running total", body: "7 new interesting (total: 9)", kind: runprotocol.GoFuzzMetricInteresting, presence: runprotocol.GoFuzzMetricPresent, value: 7, cumulative: true},
		{name: "anchored malformed interesting count", body: "new interesting: garbage", kind: runprotocol.GoFuzzMetricInteresting, presence: runprotocol.GoFuzzMetricMalformed, cumulative: true},
		{name: "unanchored interesting chatter has no count", body: "note says new interesting", kind: runprotocol.GoFuzzMetricInteresting},
		{name: "crasher count is independent", body: "crashers: 2", kind: runprotocol.GoFuzzMetricCrashers, presence: runprotocol.GoFuzzMetricPresent, value: 2},
		{name: "failing input marker is a record observation", body: "failing input written to native-seed", kind: runprotocol.GoFuzzMetricCrashers, failing: true},
		{name: "ASCII folded failing input marker", body: "FAILING INPUT written to native-seed", kind: runprotocol.GoFuzzMetricCrashers, failing: true},
		{name: "near marker remains neutral", body: "failing inpu written to native-seed", kind: runprotocol.GoFuzzMetricCrashers},
		{name: "ordinary physical record remains neutral", body: "ordinary", kind: runprotocol.GoFuzzMetricExecutions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			count := 0
			for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: strings.NewReader(tc.body)}) {
				if err != nil || record.Validate() != nil {
					t.Fatalf("record/error = %+v/%v, want complete validated record", record, err)
				}
				count++
				field, err := record.Field(tc.kind)
				if err != nil {
					t.Fatal(err)
				}
				value, present := field.Value()
				if field.Presence() != tc.presence || value != tc.value || present != (tc.presence == runprotocol.GoFuzzMetricPresent) || record.InterestingCumulative() != tc.cumulative || record.FailingInput() != tc.failing {
					t.Fatalf("field/presence/cumulative/failing = %+v/%v/%t/%t, want %d/%v/%t/%t", field, field.Presence(), record.InterestingCumulative(), record.FailingInput(), tc.value, tc.presence, tc.cumulative, tc.failing)
				}
				if tc.presence == runprotocol.GoFuzzMetricMalformed && !errors.Is(field.Refusal(), core.ErrGoToolchainOutput) {
					t.Fatalf("malformed refusal = %v, want native output identity", field.Refusal())
				}
			}
			if count != 1 {
				t.Fatalf("physical record count = %d, want one", count)
			}
		})
	}
}

func FuzzGoFuzzMetricIntegerAgreesWithNativeStrconv(f *testing.F) {
	for _, value := range []int64{0, 37, -37, math.MinInt64, math.MaxInt64} {
		f.Add(strconv.FormatInt(value, 10), uint16(0))
	}
	f.Add("9223372036854775808", uint16(17))
	f.Add("", uint16(0))
	f.Add("garbage", uint16(1))
	f.Add("1\tx", uint16(0))
	f.Fuzz(func(t *testing.T, token string, padding uint16) {
		// Mutated token bytes reach the real native record reader; the oracle
		// uses independent stdlib byte tokenization and strconv admission.
		body := strings.Repeat(" ", int(padding)) + "execs: " + token + "\n"
		oracleToken := token
		if index := strings.IndexByte(oracleToken, '\n'); index >= 0 {
			oracleToken = oracleToken[:index]
		}
		wantToken := strings.TrimSpace(oracleToken)
		if index := strings.IndexByte(wantToken, ' '); index >= 0 {
			wantToken = wantToken[:index]
		}
		wantToken = strings.TrimRight(wantToken, ",")
		sign, digits := "", wantToken
		if strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
			sign, digits = digits[:1], digits[1:]
		}
		var want int64
		var parseErr error
		if digits == "" || strings.IndexFunc(digits, func(value rune) bool { return value < '0' || value > '9' }) >= 0 {
			parseErr = strconv.ErrSyntax
		} else {
			compact := strings.TrimLeft(digits, "0")
			if compact == "" {
				compact = "0"
			}
			if len(compact) > 19 {
				parseErr = strconv.ErrRange
			} else {
				want, parseErr = strconv.ParseInt(sign+compact, 10, 64)
			}
		}

		first := true
		for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: strings.NewReader(body)}) {
			if err != nil || record.Validate() != nil {
				t.Fatalf("mutated record/error = %+v/%v, want complete validated physical record", record, err)
			}
			if !first {
				continue
			}
			first = false
			field, err := record.Field(runprotocol.GoFuzzMetricExecs)
			if err != nil {
				t.Fatal(err)
			}
			got, present := field.Value()
			if present != (parseErr == nil) || (present && got != want) {
				t.Fatalf("native integer/presence = %d/%t, want strconv %d/%t for %q", got, present, want, parseErr == nil, wantToken)
			}
			if !present && !errors.Is(field.Refusal(), core.ErrGoToolchainOutput) {
				t.Fatalf("refusal = %v, want native output identity", field.Refusal())
			}
		}
	})
}

func TestGoFuzzMetricRequestRefusesTerminalContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for record, err := range runprotocol.GoFuzzMetricRecords(ctx, runprotocol.GoFuzzMetricSourceRequest{Source: strings.NewReader("execs: 1\n")}) {
		if !errors.Is(err, context.Canceled) || record != (runprotocol.GoFuzzMetricRecord{}) {
			t.Fatalf("cancelled record/error = %+v/%v, want zero and native cancellation", record, err)
		}
	}
}

func TestGoFuzzMetricClosedDomainsAndSourceRefusal(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			t.Parallel()
			kind := runprotocol.GoFuzzMetricKind(value)
			presence := runprotocol.GoFuzzMetricPresence(value)
			if err := kind.Validate(); (err == nil) != (value >= int(runprotocol.GoFuzzMetricExecutions) && value <= int(runprotocol.GoFuzzMetricInteresting)) {
				t.Fatalf("kind %d error = %v, want closed published domain", value, err)
			}
			if err := presence.Validate(); (err == nil) != (value <= int(runprotocol.GoFuzzMetricMalformed)) {
				t.Fatalf("presence %d error = %v, want closed published domain", value, err)
			}
		})
	}
	for _, tc := range []struct {
		name             string
		read, positional error
	}{
		{name: "sequential bytes plus permission refusal", read: os.ErrPermission},
		{name: "positional bytes plus permission refusal", positional: os.ErrPermission},
		{name: "positional joined EOF plus permission refusal", positional: errors.Join(io.EOF, os.ErrPermission)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen := false
			source := fuzzCompletionRefusingSource{Reader: strings.NewReader("execs: 37\n"), readCause: tc.read, positionalCause: tc.positional}
			for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: source}) {
				seen = true
				if !errors.Is(err, os.ErrPermission) || record != (runprotocol.GoFuzzMetricRecord{}) {
					t.Fatalf("refused record/error = %+v/%v, want zero and native permission cause", record, err)
				}
			}
			if !seen {
				t.Fatal("source refusal silently disappeared, want error observation")
			}
		})
	}
	if _, err := (runprotocol.GoFuzzMetricRecord{}).Field(runprotocol.GoFuzzMetricExecs); err == nil {
		t.Fatal("zero record admitted a field, want source contract refusal")
	}
}

type fuzzMetricFillReader struct {
	remaining int64
	value     byte
}

func (r *fuzzMetricFillReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count := min(int64(len(p)), r.remaining)
	for index := range count {
		p[index] = r.value
	}
	r.remaining -= count
	return int(count), nil
}

func TestGoFuzzMetricRecordStreamsNativeFieldsBeyondFormerWindows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, prefix, suffix string
		extent               int64
		fill                 byte
		want                 int64
	}{
		{name: "prefix straddles native working window", extent: 4093, fill: ' ', suffix: "execs: 37\n", want: 37},
		{name: "leading whitespace exceeds former Witness line limit", extent: 16*1024*1024 + 1, fill: ' ', suffix: "execs: 37\n", want: 37},
		{name: "counter whitespace is unbounded", prefix: "execs:", extent: 16*1024*1024 + 1, fill: ' ', suffix: "37\n", want: 37},
		{name: "leading numeric zeroes are unbounded", prefix: "execs: ", extent: 16*1024*1024 + 1, fill: '0', suffix: "37\n", want: 37},
		{name: "negative counter retains sign beyond zeroes", prefix: "execs: -", extent: 16*1024*1024 + 1, fill: '0', suffix: "37\n", want: -37},
		{name: "zero entire numeric field still completes", prefix: "execs: ", extent: 16*1024*1024 + 1, fill: '0', suffix: "\n"},
		{name: "trailing whitespace is not a token quota", prefix: "execs: 37", extent: 16*1024*1024 + 1, fill: ' ', suffix: "\n", want: 37},
		{name: "trailing formatting commas are not a token quota", prefix: "execs: 37", extent: 16*1024*1024 + 1, fill: ',', suffix: "\n", want: 37},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file, err := os.Create(filepath.Join(dir, "native-record"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			written, err := io.Copy(file, io.MultiReader(strings.NewReader(tc.prefix), &fuzzMetricFillReader{remaining: tc.extent, value: tc.fill}, strings.NewReader(tc.suffix)))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			count := 0
			for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: file}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				field, err := record.Field(runprotocol.GoFuzzMetricExecs)
				if err != nil {
					t.Fatal(err)
				}
				value, present := field.Value()
				extent, err := record.Record().Bytes.Int64()
				if err != nil || !present || value != tc.want || extent != written {
					t.Fatalf("native value/presence/extent/error = %d/%t/%d/%v, want %d/true/%d/nil", value, present, extent, err, tc.want, written)
				}
			}
			if count != 1 {
				t.Fatalf("native record count = %d, want one", count)
			}
		})
	}
}

type fuzzMetricOracleField struct {
	value    int64
	presence runprotocol.GoFuzzMetricPresence
}

func fuzzMetricOracleInteger(token []byte) fuzzMetricOracleField {
	var sign byte
	digits := token
	if len(digits) > 0 && (digits[0] == '+' || digits[0] == '-') {
		sign = digits[0]
		digits = digits[1:]
	}
	if len(digits) == 0 {
		return fuzzMetricOracleField{presence: runprotocol.GoFuzzMetricMalformed}
	}
	for _, value := range digits {
		if value < '0' || value > '9' {
			return fuzzMetricOracleField{presence: runprotocol.GoFuzzMetricMalformed}
		}
	}
	compact := bytes.TrimLeft(digits, "0")
	if len(compact) > 19 {
		return fuzzMetricOracleField{presence: runprotocol.GoFuzzMetricMalformed}
	}
	var normalized [20]byte
	used := 0
	if sign != 0 {
		normalized[0] = sign
		used = 1
	}
	if len(compact) == 0 {
		normalized[used] = '0'
		used++
	} else {
		used += copy(normalized[used:], compact)
	}
	value, err := strconv.ParseInt(string(normalized[:used]), 10, 64)
	if err != nil {
		return fuzzMetricOracleField{presence: runprotocol.GoFuzzMetricMalformed}
	}
	return fuzzMetricOracleField{value: value, presence: runprotocol.GoFuzzMetricPresent}
}

func fuzzMetricOraclePrefix(line []byte, prefix string) (int, bool) {
	if bytes.HasPrefix(line, []byte(prefix)) {
		return 0, true
	}
	if index := bytes.Index(line, []byte(","+prefix)); index >= 0 {
		return index + 1, true
	}
	if index := bytes.Index(line, []byte(", "+prefix)); index >= 0 {
		return index + 2, true
	}
	return 0, false
}

func fuzzMetricOracleRecord(line []byte) ([4]fuzzMetricOracleField, bool, bool) {
	line = bytes.TrimSpace(line)
	var fields [4]fuzzMetricOracleField
	for index, prefix := range [...]string{"executions:", "execs:", "crashers:", "new interesting:"} {
		start, found := fuzzMetricOraclePrefix(line, prefix)
		if !found {
			continue
		}
		token := bytes.TrimSpace(line[start+len(prefix):])
		if end := bytes.IndexByte(token, ' '); end >= 0 {
			token = token[:end]
		}
		token = bytes.TrimRight(token, ",")
		fields[index] = fuzzMetricOracleInteger(token)
	}
	if fields[3].presence != runprotocol.GoFuzzMetricPresent && bytes.Contains(line, []byte("new interesting")) {
		for token := range bytes.FieldsSeq(line) {
			candidate := fuzzMetricOracleInteger(bytes.Trim(token, ",:"))
			if candidate.presence == runprotocol.GoFuzzMetricPresent {
				fields[3] = candidate
				break
			}
		}
		if fields[3].presence != runprotocol.GoFuzzMetricPresent {
			if _, found := fuzzMetricOraclePrefix(line, "new interesting"); found {
				fields[3] = fuzzMetricOracleField{presence: runprotocol.GoFuzzMetricMalformed}
			}
		}
	}
	cumulative := bytes.Contains(line, []byte("new interesting:")) || bytes.Contains(line, []byte("(total:"))
	failing := false
	// Independent fixed byte windows fold only ASCII, matching the provider
	// literal without allocating a lowercased copy of the source record.
	const marker = "failing input"
	for index := 0; index+len(marker) <= len(line); index++ {
		match := true
		for position := range len(marker) {
			value := line[index+position]
			if value >= 'A' && value <= 'Z' {
				value += 'a' - 'A'
			}
			match = match && value == marker[position]
		}
		if match {
			failing = true
			break
		}
	}
	return fields, cumulative, failing
}

func FuzzGoFuzzMetricRecordsMatchIndependentByteOracle(f *testing.F) {
	for _, body := range []string{
		"fuzz: elapsed: 3s, execs: 37 (12/sec), new interesting: 7 (total: 8)\n",
		"executions: 1, execs: 37,crashers: 2\n",
		"7 new interesting inputs found\n",
		"new interesting: malformed\n",
		"note says executions: garbage\nFAILING INPUT written to seed\n",
		"execs: -9223372036854775808\r\nordinary\n",
		"\u2003execs: +00037\u2003",
		"execs: 37\tx\n",
		"",
	} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		offset := 0
		for record, err := range runprotocol.GoFuzzMetricRecords(t.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: bytes.NewReader(body)}) {
			if err != nil || record.Validate() != nil {
				t.Fatalf("record/error = %+v/%v, want completed native record", record, err)
			}
			length := len(body) - offset
			if end := bytes.IndexByte(body[offset:], '\n'); end >= 0 {
				length = end + 1
			}
			fields, cumulative, failing := fuzzMetricOracleRecord(body[offset : offset+length])
			span := record.Record()
			size, err := span.Bytes.Int64()
			if err != nil || int64(span.Offset) != int64(offset) || size != int64(length) || record.InterestingCumulative() != cumulative || record.FailingInput() != failing {
				t.Fatalf("record/extent/error = %+v/%d/%v, want physical %d/%d and flags %t/%t", record, size, err, offset, length, cumulative, failing)
			}
			for index, want := range fields {
				field, err := record.Field(runprotocol.GoFuzzMetricKind(index + 1))
				if err != nil {
					t.Fatal(err)
				}
				value, present := field.Value()
				if field.Presence() != want.presence || value != want.value || present != (want.presence == runprotocol.GoFuzzMetricPresent) {
					t.Fatalf("field %d value/presence = %d/%v, want %d/%v at %d/%d", index, value, field.Presence(), want.value, want.presence, offset, length)
				}
				if want.presence == runprotocol.GoFuzzMetricMalformed && !errors.Is(field.Refusal(), core.ErrGoToolchainOutput) {
					t.Fatalf("malformed refusal = %v, want native output identity", field.Refusal())
				}
			}
			offset += length
		}
		if offset != len(body) {
			t.Fatalf("consumed physical bytes = %d, want %d", offset, len(body))
		}
	})
}

func BenchmarkGoFuzzMetricRecordNativeWorkingMemory(b *testing.B) {
	for _, extent := range []int64{64 * 1024, 16*1024*1024 + 1, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			dir := b.TempDir()
			file, err := os.Create(filepath.Join(dir, "native-counter"))
			if err != nil {
				b.Fatal(err)
			}
			defer file.Close()
			written, err := io.Copy(file, io.MultiReader(&fuzzMetricFillReader{remaining: extent, value: ' '}, strings.NewReader("execs: 37\n")))
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
				for record, err := range runprotocol.GoFuzzMetricRecords(b.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: file}) {
					if err != nil {
						b.Fatal(err)
					}
					count++
					field, err := record.Field(runprotocol.GoFuzzMetricExecs)
					if err != nil {
						b.Fatal(err)
					}
					value, present := field.Value()
					if value != 37 || !present {
						b.Fatalf("native counter value/presence = %d/%t, want 37/true", value, present)
					}
				}
				if count != 1 {
					b.Fatalf("native record count = %d, want one", count)
				}
			}
		})
	}
}
