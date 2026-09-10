package runnercontrol

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoBuildEventLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                    string
		build                                   goBuildEventWire
		terminal                                string
		executionErr                            error
		wantErr                                 error
		wantPassed, wantFailed, wantUnavailable uint32
	}{
		{name: "dependency diagnostic preserves selected pass", build: goBuildEventWire{Action: "build-output", ImportPath: "dependency", Output: "compiler diagnostic\n"}, terminal: "pass", wantPassed: 1},
		{name: "failure event and selected failure count once", build: goBuildEventWire{Action: "build-fail", ImportPath: "dependency"}, terminal: "fail", executionErr: core.ErrProcessWait, wantFailed: 1},
		{name: "diagnostic resembling benchmark manufactures no measurement", build: goBuildEventWire{Action: "build-output", ImportPath: "dependency", Output: "BenchmarkFake 1 2 ns/op 3 B/op 4 allocs/op\n"}, terminal: "pass", wantPassed: 1},
		{name: "package-free tool diagnostic preserves selected pass", build: goBuildEventWire{Action: "build-output", Output: "tool diagnostic\n"}, terminal: "pass", wantPassed: 1},
		{name: "failure plus successful exit is contradiction", build: goBuildEventWire{Action: "build-fail", ImportPath: "dependency"}, terminal: "pass", wantErr: core.ErrPrimitiveContract, wantPassed: 1},
		{name: "diagnostic alone cannot supply terminal evidence", build: goBuildEventWire{Action: "build-output", Output: "tool diagnostic\n"}, wantErr: core.ErrPrimitiveContract, wantUnavailable: 1},
		{name: "empty output event is refused", build: goBuildEventWire{Action: "build-output"}, wantErr: core.ErrJSONContract, wantUnavailable: 1},
		{name: "failure event cannot carry diagnostic output", build: goBuildEventWire{Action: "build-fail", Output: "diagnostic in wrong event"}, wantErr: core.ErrJSONContract, wantUnavailable: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, err := NewGoTestObservationCompiler(ObservationPolicy{Format: ObservationGoTestJSON, ExpectedUnits: 1})
			if err != nil {
				t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
			}
			data, err := json.Marshal(tc.build)
			if err != nil {
				t.Fatalf("Marshal(build event) error = %v, want nil", err)
			}
			data = append(data, '\n')
			if tc.terminal != "" {
				terminal, err := json.Marshal(goTestEventWire{Action: tc.terminal, Package: "selected"})
				if err != nil {
					t.Fatalf("Marshal(test event) error = %v, want nil", err)
				}
				data = append(data, terminal...)
				data = append(data, '\n')
			}
			if n, err := compiler.Write(data); n != len(data) || err != nil {
				t.Fatalf("Write() = %d/%v, want %d/nil", n, err, len(data))
			}
			got, err := compiler.Seal(tc.executionErr)
			if (tc.wantErr == nil && err != nil) || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) {
				t.Fatalf("Seal() error = %v, want %v", err, tc.wantErr)
			}
			if len(got.Accounting.Attempts) != 1 {
				t.Fatalf("Seal() attempts = %d, want 1", len(got.Accounting.Attempts))
			}
			attempt := got.Accounting.Attempts[0]
			if attempt.Passed != tc.wantPassed || attempt.Failed != tc.wantFailed || attempt.Unavailable != tc.wantUnavailable || len(got.Benchmarks) != 0 {
				t.Fatalf("Seal() = %+v, want passed %d/failed %d/unavailable %d/no benchmarks", got, tc.wantPassed, tc.wantFailed, tc.wantUnavailable)
			}
		})
	}
}

func TestGoBuildAndTestWireMembersRemainSeparate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, data string }{
		{"test package member cannot cross build wire", `{"Action":"build-output","ImportPath":"dependency","Output":"diagnostic","Package":"selected"}`},
		{"build identity cannot cross test wire", `{"Action":"pass","Package":"selected","ImportPath":"dependency"}`},
		{"duplicate discriminator cannot change wire selection", `{"Action":"build-fail","ImportPath":"dependency","Action":"pass"}`},
		{"build identity has the wrong JSON type", `{"Action":"build-fail","ImportPath":5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, err := NewGoTestObservationCompiler(ObservationPolicy{Format: ObservationGoTestJSON, ExpectedUnits: 1})
			if err != nil {
				t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
			}
			if _, err := compiler.Write([]byte(tc.data)); err != nil {
				t.Fatalf("Write() error = %v, want nil", err)
			}
			got, err := compiler.Seal(nil)
			if !errors.Is(err, core.ErrJSONContract) || len(got.Accounting.Attempts) != 1 || got.Accounting.Attempts[0].Unavailable != 1 || len(got.Benchmarks) != 0 {
				t.Fatalf("Seal(mixed wire) = %+v/%v, want unavailable/typed JSON refusal", got, err)
			}
		})
	}
}
