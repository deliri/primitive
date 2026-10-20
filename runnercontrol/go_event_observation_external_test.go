package runnercontrol_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/testserial"
)

// Native wire fixture, independently decoded with Go's time.Time JSON owner.
type goEventObservationWire struct {
	Time        *time.Time                      `json:"Time,omitzero"`
	FailedBuild string                          `json:"FailedBuild,omitzero"`
	ImportPath  string                          `json:"ImportPath,omitzero"`
	Package     string                          `json:"Package,omitzero"`
	Test        string                          `json:"Test,omitzero"`
	Output      string                          `json:"Output,omitzero"`
	Action      runnercontrol.GoEventAction     `json:"Action"`
	OutputType  runnercontrol.GoEventOutputKind `json:"OutputType,omitzero"`
	Elapsed     float64                         `json:"Elapsed"`
}

func goEventObservationFixture(t testing.TB, fixture goEventObservationWire) []byte {
	t.Helper()
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestGoEventObservationMetadataConservation(t *testing.T) {
	t.Parallel()
	timestamp := time.Date(2026, 10, 19, 1, 2, 3, 987654321, time.FixedZone("", -3*3600-30*60))
	for _, tc := range []struct {
		name    string
		fixture goEventObservationWire
	}{
		{name: "empty metadata is neutral", fixture: goEventObservationWire{Action: runnercontrol.GoEventActionStart}},
		{name: "all atoms have distinct authority", fixture: goEventObservationWire{Time: &timestamp, FailedBuild: "failed/build", ImportPath: "import/path", Package: "package/path", Test: "TestName/sub", Output: "stdout\né界😀\"\\", Action: runnercontrol.GoEventActionFail, OutputType: runnercontrol.GoEventOutputError, Elapsed: 0.125}},
		{name: "early native year remains representable", fixture: goEventObservationWire{Time: new(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)), Action: runnercontrol.GoEventActionPass}},
		{name: "long labels are admitted in full", fixture: goEventObservationWire{Package: strings.Repeat("é", 32769), Test: strings.Repeat("界", 32769), Action: runnercontrol.GoEventActionRun}},
		{name: "negative elapsed remains native data", fixture: goEventObservationWire{Action: runnercontrol.GoEventActionPass, Elapsed: -0.5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoded := goEventObservationFixture(t, tc.fixture)
			// Follow populated data with an empty event: a reset must not lend prior atoms.
			neutral := goEventObservationWire{Action: runnercontrol.GoEventActionStart}
			source := io.MultiReader(bytes.NewReader(encoded), strings.NewReader("\n"), bytes.NewReader(goEventObservationFixture(t, neutral)))
			var output strings.Builder
			count, closed := 0, 0
			err := runnercontrol.ReadGoEventObservations(t.Context(), runnercontrol.GoEventObservationRequest{
				Source: source,
				OnOutput: func(fragment runnercontrol.GoEventStringFragment) error {
					if err := fragment.Validate(); err != nil {
						return err
					}
					if fragment.Field != runnercontrol.GoEventFieldOutput {
						t.Fatalf("output callback field=%v, want Output", fragment.Field)
					}
					if fragment.Final {
						closed++
					}
					_, err := output.Write(fragment.Data)
					return err
				},
				OnEvent: func(got runnercontrol.GoEventObservation) error {
					want := tc.fixture
					if count == 1 {
						want = neutral
					}
					var wantTime time.Time
					if want.Time != nil {
						wantTime = *want.Time
					}
					if count > 1 || !got.Time.Equal(wantTime) || got.Time.Nanosecond() != wantTime.Nanosecond() || got.FailedBuild != want.FailedBuild || got.ImportPath != want.ImportPath || got.Package != want.Package || got.Test != want.Test || output.String() != want.Output || got.Frame.Action != want.Action || got.Frame.OutputKind != want.OutputType || math.Float64bits(got.Frame.ElapsedSeconds) != math.Float64bits(want.Elapsed) {
						t.Fatalf("event%d=%+v output%q, want %+v", count, got, output.String(), want)
					}
					if count == 0 && want.Output != "" && closed != 1 {
						t.Fatalf("output closes=%d, want1", closed)
					}
					count++
					output.Reset()
					return got.Validate()
				},
			})
			if err != nil || count != 2 {
				t.Fatalf("projection=(%d,%v), want2,nil", count, err)
			}
		})
	}
}

func TestGoEventObservationRefusalsAndPrefix(t *testing.T) {
	t.Parallel()
	seed := goEventObservationFixture(t, goEventObservationWire{Action: runnercontrol.GoEventActionOutput, Output: "observable", Package: "prefix"})
	for _, tc := range []struct {
		name                                            string
		data                                            string
		sourceError, errorOutput, errorEvent, errorWant error
		wantEvents                                      int
		cancelBefore, cancelOutput                      bool
	}{
		{name: "empty stream is neutral"},
		{name: "source refusal", sourceError: io.ErrClosedPipe, errorWant: io.ErrClosedPipe},
		{name: "late source refusal preserves only prefix", data: string(seed), sourceError: io.ErrClosedPipe, errorWant: io.ErrClosedPipe, wantEvents: 1},
		{name: "output refusal cannot emit event", data: string(seed), errorOutput: io.ErrShortWrite, errorWant: io.ErrShortWrite},
		{name: "event refusal is retained", data: string(seed), errorEvent: io.ErrShortWrite, errorWant: io.ErrShortWrite, wantEvents: 1},
		{name: "pre cancellation", data: string(seed), cancelBefore: true, errorWant: context.Canceled},
		{name: "output cancellation cannot emit event", data: string(seed), cancelOutput: true, errorWant: context.Canceled},
		{name: "malformed trailing data cannot seal prefix", data: string(seed) + "\n{", errorWant: core.ErrJSONContract, wantEvents: 1},
		{name: "missing action", data: `{"Package":"x"}`, errorWant: core.ErrJSONContract},
		{name: "future action", data: `{"Action":"future"}`, errorWant: core.ErrJSONContract},
		{name: "duplicate atom", data: `{"Action":"start","Package":"a","Package":"b"}`, errorWant: core.ErrJSONContract},
		{name: "empty timestamp is not absent", data: `{"Action":"start","Time":""}`, errorWant: core.ErrJSONContract},
		{name: "malformed timestamp", data: `{"Action":"start","Time":"2026-13-19T00:00:00Z"}`, errorWant: core.ErrJSONContract},
		{name: "timestamp must have native JSON grammar", data: `{"Action":"start","Time":"2026-10-19T00:00:00,5Z"}`, errorWant: core.ErrJSONContract},
		{name: "timestamp type refusal", data: `{"Action":"start","Time":12}`, errorWant: core.ErrJSONContract},
		{name: "nonfinite elapsed", data: `{"Action":"pass","Elapsed":1e999}`, errorWant: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if tc.cancelBefore {
				cancel(io.ErrClosedPipe)
			}
			var source io.Reader = strings.NewReader(tc.data)
			if tc.sourceError != nil {
				source = io.MultiReader(source, goEventFailureReader{cause: tc.sourceError})
			}
			count := 0
			err := runnercontrol.ReadGoEventObservations(ctx, runnercontrol.GoEventObservationRequest{Source: source, OnOutput: func(runnercontrol.GoEventStringFragment) error {
				if tc.cancelOutput {
					cancel(io.ErrClosedPipe)
				}
				return tc.errorOutput
			}, OnEvent: func(got runnercontrol.GoEventObservation) error {
				count++
				if got.Package != "prefix" {
					t.Fatalf("prefix=%+v, want admitted metadata", got)
				}
				return tc.errorEvent
			}})
			if count != tc.wantEvents || (tc.errorWant == nil && err != nil) || (tc.errorWant != nil && !errors.Is(err, tc.errorWant)) {
				t.Fatalf("projection=(%d,%v), want%d,%v", count, err, tc.wantEvents, tc.errorWant)
			}
			if (tc.cancelBefore || tc.cancelOutput) && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("cancellation=%v, want native cause", err)
			}
		})
	}
}

func TestGoEventObservationContracts(t *testing.T) {
	t.Parallel()
	good := runnercontrol.GoEventObservation{Frame: runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionStart}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, got := range []runnercontrol.GoEventObservation{{}, {Frame: good.Frame, Package: string([]byte{0xff})}, {Frame: good.Frame, Test: string([]byte{0xff})}, {Frame: good.Frame, FailedBuild: string([]byte{0xff})}, {Frame: good.Frame, ImportPath: string([]byte{0xff})}} {
		if err := got.Validate(); err == nil {
			t.Fatalf("Validate(%+v)=nil, want refusal", got)
		}
	}
	for _, request := range []runnercontrol.GoEventObservationRequest{{}, {Source: (*strings.Reader)(nil), OnOutput: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(runnercontrol.GoEventObservation) error { return nil }}, {Source: strings.NewReader("")}} {
		if err := runnercontrol.ReadGoEventObservations(t.Context(), request); !errors.Is(err, core.ErrPrimitiveContract) {
			t.Fatalf("missing contract=%v, want PrimitiveContract", err)
		}
	}
}

func TestGoEventObservationOutputHasNoMetadataAllocation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int64{32767, 32768, 32769, 1048577} {
		t.Run(strconv.FormatInt(extent, 10), func(t *testing.T) {
			encoded := goEventObservationFixture(t, goEventObservationWire{Action: runnercontrol.GoEventActionOutput, Output: "UNIQUE_OUTPUT_MARKER", Package: "native"})
			prefix, suffix, found := bytes.Cut(encoded, []byte("UNIQUE_OUTPUT_MARKER"))
			if !found {
				t.Fatal("typed seed missing marker")
			}
			source := io.MultiReader(bytes.NewReader(prefix), io.LimitReader(goEventPadding{}, extent), bytes.NewReader(suffix))
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			var consumed int64
			events, closes := 0, 0
			err := runnercontrol.ReadGoEventObservations(t.Context(), runnercontrol.GoEventObservationRequest{Source: source, OnOutput: func(fragment runnercontrol.GoEventStringFragment) error {
				for _, b := range fragment.Data {
					if b != 'x' {
						t.Fatalf("output byte=%d, want x", b)
					}
				}
				consumed += int64(len(fragment.Data))
				if fragment.Final {
					closes++
				}
				return fragment.Validate()
			}, OnEvent: func(got runnercontrol.GoEventObservation) error {
				events++
				if got.Package != "native" || got.Frame.Action != runnercontrol.GoEventActionOutput || consumed != extent || closes != 1 {
					t.Fatalf("event=%+v bytes%d closes%d, want exact native output", got, consumed, closes)
				}
				runtime.GC()
				runtime.ReadMemStats(&after)
				return nil
			}})
			retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
			t.Logf("output%d retained%d bytes", extent, retained)
			if err != nil || events != 1 || retained > 128<<10 {
				t.Fatalf("output=(%d,%v,%d retained), want1,nil,bounded", events, err, retained)
			}
		})
	}
}

func TestGoEventObservationPopulationWorkingMemory(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{64, 16385} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			seed := goEventObservationFixture(t, goEventObservationWire{Package: "one/package", Test: "one/test", Action: runnercontrol.GoEventActionPass})
			reader := &goObservationRepeatReader{record: append(seed, '\n'), remaining: population}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count := 0
			err := runnercontrol.ReadGoEventObservations(t.Context(), runnercontrol.GoEventObservationRequest{Source: reader, OnOutput: func(runnercontrol.GoEventStringFragment) error { return nil }, OnEvent: func(got runnercontrol.GoEventObservation) error {
				count++
				if got.Package != "one/package" || got.Test != "one/test" {
					t.Fatalf("event=%+v, want exact atoms", got)
				}
				if count == population {
					runtime.GC()
					runtime.ReadMemStats(&after)
				}
				return nil
			}})
			runtime.KeepAlive(reader)
			retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
			t.Logf("events%d retained%d bytes", population, retained)
			if err != nil || count != population || retained > 128<<10 {
				t.Fatalf("stream=(%d,%v,%d retained), want%d,nil,bounded", count, err, retained, population)
			}
		})
	}
}

type goObservationRepeatReader struct {
	record            []byte
	remaining, offset int
}

func (r *goObservationRepeatReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.record[r.offset:])
	r.offset += n
	if r.offset == len(r.record) {
		r.offset = 0
		r.remaining--
	}
	return n, nil
}

func FuzzGoEventObservationNativeMetadata(f *testing.F) {
	f.Add([]byte("é界😀\n\t\"\\"), uint8(0))
	f.Add([]byte{}, uint8(1))
	f.Add([]byte{0xff, 0, 0xc0}, uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, mode uint8) {
		for offset := 0; ; {
			end := min(offset+4096, len(data))
			text := strings.ToValidUTF8(string(data[offset:end]), "�")
			timestamp := time.Date(2026, 10, 19, 0, 0, 0, int(mode)*1234567, time.UTC)
			fixture := goEventObservationWire{Time: &timestamp, Action: runnercontrol.GoEventActionOutput, Package: text, Test: text, FailedBuild: text, ImportPath: text, Output: text, Elapsed: float64(mode) / 8}
			encoded := goEventObservationFixture(t, fixture)
			var native goEventObservationWire
			if err := json.Unmarshal(encoded, &native); err != nil {
				t.Fatal(err)
			}
			count := 0
			var output strings.Builder
			err := runnercontrol.ReadGoEventObservations(t.Context(), runnercontrol.GoEventObservationRequest{Source: bytes.NewReader(encoded), OnOutput: func(fragment runnercontrol.GoEventStringFragment) error {
				_, err := output.Write(fragment.Data)
				return err
			}, OnEvent: func(got runnercontrol.GoEventObservation) error {
				count++
				if got.Validate() != nil || native.Time == nil || !got.Time.Equal(*native.Time) || got.Package != native.Package || got.Test != native.Test || got.ImportPath != native.ImportPath || got.FailedBuild != native.FailedBuild || got.Frame.Action != native.Action || got.Frame.OutputKind != native.OutputType || got.Frame.ElapsedSeconds != native.Elapsed || output.String() != native.Output {
					t.Fatalf("projection=%+v output%q, want Native%+v", got, output.String(), native)
				}
				return nil
			}})
			if err != nil || count != 1 {
				t.Fatalf("projection=(%d,%v), want1,nil", count, err)
			}
			if end == len(data) {
				break
			}
			offset = end
		}
	})
}
