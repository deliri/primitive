package runnercontrol_test

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
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
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoEventStreamElapsedSecondsNativeFloatBits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, number string }{
		{name: "zero", number: "0"}, {name: "signed zero", number: "-0"},
		{name: "positive seconds", number: "1.25"}, {name: "negative scalar policy belongs to consumer", number: "-1.25"},
		{name: "smallest subnormal", number: strconv.FormatFloat(math.SmallestNonzeroFloat64, 'g', -1, 64)},
		{name: "negative smallest subnormal", number: strconv.FormatFloat(-math.SmallestNonzeroFloat64, 'g', -1, 64)},
		{name: "largest finite", number: strconv.FormatFloat(math.MaxFloat64, 'g', -1, 64)},
		{name: "negative largest finite", number: strconv.FormatFloat(-math.MaxFloat64, 'g', -1, 64)},
		{name: "overflow remains refused", number: "1.7976931348623159e308"},
		{name: "negative overflow remains refused", number: "-1e309"},
		{name: "underflow retains signed zero", number: "-1e-99999999999999999999999999999"},
		{name: "zero tolerates wide positive exponent", number: "0e99999999999999999999999999999"},
		{name: "negative zero tolerates wide positive exponent", number: "-0e99999999999999999999999999999"},
		{name: "negative zero tolerates wide negative exponent", number: "-0e-99999999999999999999999999999"},
		{name: "fraction wider than numeric working window", number: "0." + strings.Repeat("0", 4096) + "125e4097"},
		{name: "integer wider than numeric working window", number: "125" + strings.Repeat("0", 4096) + "e-4098"},
		{name: "sticky digit beyond numeric window", number: "1." + strings.Repeat("0", 4096) + "1"},
		{name: "sticky digit breaks exact halfway tie", number: "1.00000000000000011102230246251565404236316680908203125" + strings.Repeat("0", 4096) + "1"},
		{name: "missing exponent digits refused", number: "1e"},
		{name: "leading zero refused", number: "01"},
		{name: "missing fraction digits refused", number: "1."},
		{name: "non numeric field refused", number: "\"1\""},
		{name: "null numeric field refused", number: "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var native float64
			nativeErr := json.Unmarshal([]byte(tc.number), &native)
			if tc.number == "null" || strings.HasPrefix(tc.number, "\"") {
				nativeErr = core.ErrJSONContract
			}
			source := `{"Action":"pass","Elapsed":` + tc.number + `}`
			frames := 0
			err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader(source), OnString: func(fragment runnercontrol.GoEventStringFragment) error { return fragment.Validate() }, OnEvent: func(frame runnercontrol.GoEventFrame) error {
				frames++
				if nativeErr != nil || frame.Action != runnercontrol.GoEventActionPass || math.Float64bits(frame.ElapsedSeconds) != math.Float64bits(native) {
					t.Fatalf("elapsed frame=%+v (%x), want native %x (%v)", frame, math.Float64bits(frame.ElapsedSeconds), math.Float64bits(native), nativeErr)
				}
				// The public typed frame is also an actual producer. Its canonical wire
				// must preserve the exact scalar, including negative zero.
				encoded, err := json.Marshal(frame)
				if err != nil {
					t.Fatal(err)
				}
				var again runnercontrol.GoEventFrame
				if err := json.Unmarshal(encoded, &again); err != nil || math.Float64bits(again.ElapsedSeconds) != math.Float64bits(native) {
					t.Fatalf("typed elapsed closure=%+v, %v", again, err)
				}
				return frame.Validate()
			}})
			if nativeErr == nil {
				if err != nil || frames != 1 {
					t.Fatalf("finite native elapsed=%d frames, %v", frames, err)
				}
			} else if frames != 0 || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("refused native elapsed=%d frames, %v", frames, err)
			}
		})
	}
}

func TestGoEventStreamElapsedNeverLeaksBetweenFrames(t *testing.T) {
	t.Parallel()
	first := runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionPass, ElapsedSeconds: 0.125}
	last := runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionPass, ElapsedSeconds: math.Copysign(0, -1)}
	absent := struct {
		Action runnercontrol.GoEventAction `json:"Action"`
	}{Action: runnercontrol.GoEventActionStart}
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(absent)
	if err != nil {
		t.Fatal(err)
	}
	c, err := json.Marshal(last)
	if err != nil {
		t.Fatal(err)
	}
	source := io.MultiReader(bytes.NewReader(a), strings.NewReader("\n"), bytes.NewReader(b), strings.NewReader("\n"), bytes.NewReader(c))
	count := 0
	err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: source, OnString: func(fragment runnercontrol.GoEventStringFragment) error { return fragment.Validate() }, OnEvent: func(frame runnercontrol.GoEventFrame) error {
		want := [...]float64{first.ElapsedSeconds, 0, last.ElapsedSeconds}[count]
		if math.Float64bits(frame.ElapsedSeconds) != math.Float64bits(want) {
			t.Fatalf("elapsed row %d=%x, want %x", count, math.Float64bits(frame.ElapsedSeconds), math.Float64bits(want))
		}
		count++
		return nil
	}})
	if err != nil || count != 3 {
		t.Fatalf("elapsed source reset=%d frames, %v", count, err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := (runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionPass, ElapsedSeconds: value}).Validate(); !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("nonfinite frame refusal=%v", err)
		}
	}
}

func TestGoEventStreamElapsedPreservesNativeCancellationCause(t *testing.T) {
	t.Parallel()
	for _, during := range []bool{false, true} {
		t.Run(strconv.FormatBool(during), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if !during {
				cancel(io.ErrClosedPipe)
			}
			count := 0
			seed := runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionPass, ElapsedSeconds: 0.125}
			encoded, err := json.Marshal(seed)
			if err != nil {
				t.Fatal(err)
			}
			err = runnercontrol.ReadGoEventStream(ctx, runnercontrol.GoEventStreamRequest{Source: bytes.NewReader(encoded), OnString: func(fragment runnercontrol.GoEventStringFragment) error { cancel(io.ErrClosedPipe); return nil }, OnEvent: func(frame runnercontrol.GoEventFrame) error { count++; return nil }})
			if count != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("native elapsed cancellation=%d frames, %v", count, err)
			}
		})
	}
}

func TestGoEventStreamElapsedMatchesActualGoProducer(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	for _, file := range []struct{ name, source string }{{name: "go.mod", source: "module elapsedfixture\n\ngo 1.27.1\n"}, {name: "elapsed_test.go", source: "package elapsedfixture\nimport \"testing\"\nfunc TestElapsed(t *testing.T){t.Parallel();for i:=range 32{if i<0{t.Fatal(i)}}}\n"}} {
		if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	output, err := os.Create(filepath.Join(directory, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := output.Close(); err != nil {
			t.Error(err)
		}
	})
	command := exec.CommandContext(t.Context(), "go", "test", "-json", "-count=1", ".")
	command.Dir = directory
	command.Stdout = output
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("Native Go producer=%v: %s", err, stderr.String())
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	oracleFile, err := os.Open(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := oracleFile.Close(); err != nil {
			t.Error(err)
		}
	})
	oracle := jsonv1.NewDecoder(oracleFile)
	count, elapsedFields := 0, 0
	err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: output, OnString: func(fragment runnercontrol.GoEventStringFragment) error { return fragment.Validate() }, OnEvent: func(frame runnercontrol.GoEventFrame) error {
		var native struct {
			Action  string
			Elapsed *float64
		}
		if err := oracle.Decode(&native); err != nil {
			return err
		}
		want := 0.0
		if native.Elapsed != nil {
			want = *native.Elapsed
			elapsedFields++
		}
		if frame.Action.String() != native.Action || math.Float64bits(frame.ElapsedSeconds) != math.Float64bits(want) {
			t.Fatalf("actual Go elapsed=%+v, want action %q value %x", frame, native.Action, math.Float64bits(want))
		}
		count++
		return nil
	}})
	var extra struct{ Action string }
	if end := oracle.Decode(&extra); end != io.EOF {
		t.Fatalf("native oracle remainder=%v", end)
	}
	if err != nil || count == 0 || elapsedFields < 2 {
		t.Fatalf("actual Go elapsed source=%d frames, %d numeric fields, %v", count, elapsedFields, err)
	}
}
