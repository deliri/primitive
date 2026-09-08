package process_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

type exactEnvironmentAdmissionCase struct {
	name    string
	values  []string
	wantErr error
}

// Typed cases are shared with the semantic fuzz seeds; verdicts stay in the
// test and fuzz callbacks, never in this fixture constructor.
func exactEnvironmentAdmissionCases() []exactEnvironmentAdmissionCase {
	maximumNames := make([]string, process.EnvironmentVariableCountMaximum)
	for index := range maximumNames {
		maximumNames[index] = "V" + strconv.Itoa(index) + "="
	}
	maximumProjection := "V=" + strings.Repeat("x", int(process.EnvironmentProjectionMaximumBytes)-3)
	maximumName := strings.Repeat("N", int(process.EnvironmentNameMaximumBytes))
	return []exactEnvironmentAdmissionCase{
		{name: "positive/one variable without critical variables", values: []string{"V=value"}},
		{name: "positive/two independent variables without critical variables", values: []string{"V=one", "W=two"}},
		{name: "positive/explicit critical variable stays in its position", values: []string{"V=one", "SYSTEMROOT=fixture", "W=two"}},
		{name: "positive/lowercase critical variable remains exact", values: []string{"systemroot=fixture", "V=one"}},
		{name: "positive/empty critical value is not ambient inheritance", values: []string{"SYSTEMROOT="}},
		{name: "positive/reverse lexical order is not sorted", values: []string{"Z=last", "A=first"}},
		{name: "positive/embedded equals remains value data", values: []string{"V=a=b=c"}},
		{name: "positive/shell syntax remains inert", values: []string{"V=$(id);*"}},
		{name: "positive/unicode names and values survive", values: []string{"温度=雪", "V=é"}},
		{name: "positive/control bytes remain non-NUL value data", values: []string{"V=\t\r\n\x1e"}},
		{name: "negative/one duplicate cannot be hidden by a critical addition", values: []string{"V=old", "V=new"}, wantErr: core.ErrProcessContract},
		{name: "negative/identical duplicates are still ambiguous", values: []string{"V=same", "V=same"}, wantErr: core.ErrProcessContract},
		{name: "negative/separated duplicate is refused", values: []string{"V=old", "W=kept", "V=new"}, wantErr: core.ErrProcessContract},
		{name: "negative/two collisions cannot cancel an addition", values: []string{"V=old", "W=old", "V=new", "W=new"}, wantErr: core.ErrProcessContract},
		{name: "negative/explicit critical variable does not excuse a duplicate", values: []string{"SYSTEMROOT=fixture", "V=old", "V=new"}, wantErr: core.ErrProcessContract},
		{name: "negative/critical name itself cannot repeat", values: []string{"SYSTEMROOT=old", "SYSTEMROOT=new"}, wantErr: core.ErrProcessContract},
		{name: "negative/empty duplicate cannot erase earlier value", values: []string{"V=old", "V="}, wantErr: core.ErrProcessContract},
		{name: "negative/NUL name is refused before projection", values: []string{"V\x00W=bad"}, wantErr: core.ErrProcessContract},
		{name: "negative/NUL value is refused before projection", values: []string{"V=bad\x00value"}, wantErr: core.ErrProcessContract},
		{name: "negative/missing equals is not silently dropped", values: []string{"V=kept", "malformed"}, wantErr: core.ErrProcessContract},
		{name: "neutral/nil input is exact empty", values: nil},
		{name: "boundary/non-UTF8 bytes remain exact OS data", values: []string{"V=\xff\xc0\x80"}},
		{name: "boundary/empty ordinary value stays present", values: []string{"V="}},
		{name: "boundary/empty name is refused", values: []string{"=value"}, wantErr: core.ErrProcessContract},
		{name: "boundary/empty entry is refused", values: []string{""}, wantErr: core.ErrProcessContract},
		{name: "boundary/leading equals is not a typed name", values: []string{"=C:=fixture"}, wantErr: core.ErrProcessContract},
		{name: "boundary/name exactly at its byte bound", values: []string{maximumName + "="}},
		{name: "boundary/name one byte beyond its bound", values: []string{maximumName + "N="}, wantErr: core.ErrProcessContract},
		{name: "boundary/projection includes final NUL at exact bound", values: []string{maximumProjection}},
		{name: "boundary/projection one byte beyond its bound", values: []string{maximumProjection + "x"}, wantErr: core.ErrProcessContract},
		{name: "boundary/value bound does not exempt name equals and NUL", values: []string{"V=" + strings.Repeat("x", int(process.EnvironmentValueMaximumBytes))}, wantErr: core.ErrProcessContract},
		{name: "boundary/value one beyond its own bound", values: []string{"V=" + strings.Repeat("x", int(process.EnvironmentValueMaximumBytes)+1)}, wantErr: core.ErrProcessContract},
		{name: "boundary/cardinality exactly at bound", values: maximumNames},
		{name: "boundary/cardinality one beyond bound", values: append(slices.Clone(maximumNames), "EXTRA="), wantErr: core.ErrProcessContract},
		{name: "boundary/last pair collides at maximum cardinality", values: append(slices.Clone(maximumNames[:len(maximumNames)-1]), maximumNames[0]), wantErr: core.ErrProcessContract},
		{name: "boundary/aggregate overflow across otherwise admitted pairs", values: []string{maximumProjection, "W="}, wantErr: core.ErrProcessContract},
		{name: "boundary/name prefix is not duplicate identity", values: []string{"V=x", "VV=y"}},
		{name: "boundary/value resembling critical assignment cannot mask a collision", values: []string{"V=SYSTEMROOT=fixture", "V=new"}, wantErr: core.ErrProcessContract},
		{name: "boundary/malformed overwritten value cannot be repaired", values: []string{"V=bad\x00value", "V=new"}, wantErr: core.ErrProcessContract},
		{name: "boundary/invalid tail cannot expose a valid prefix", values: []string{"V=kept", "W=also kept", "=bad"}, wantErr: core.ErrProcessContract},
	}
}

// The exact projection must survive admission unchanged. In particular, Go's
// critical-variable additions must neither reject unique input nor conceal a
// duplicate. This table also pins refusal atomicity at every bounded ingress.
func TestExactEnvironmentAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range exactEnvironmentAdmissionCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := slices.Clone(tc.values)
			got, err := process.ParseExactEnvironment(tc.values)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("admission error = %v, want %v", err, tc.wantErr)
			}
			if !slices.Equal(tc.values, before) {
				t.Fatalf("admission changed caller projection: got=%q want=%q", tc.values, before)
			}
			if tc.wantErr != nil {
				if got.Mode != process.EnvironmentModeUnknown || got.Variables != nil || !errors.Is(got.Validate(), core.ErrProcessContract) {
					t.Fatalf("refusal exposed an environment: mode=%v, variables=%d", got.Mode, len(got.Variables))
				}
				return
			}
			projected, projectionErr := got.Strings()
			if projectionErr != nil || got.Mode != process.EnvironmentModeExact || projected == nil || !slices.Equal(projected, before) {
				t.Fatalf("admitted projection changed: mode=%v count=%d want count=%d error=%v", got.Mode, len(projected), len(before), projectionErr)
			}
			if len(projected) != 0 {
				projected[0] = ""
				isolated, isolationErr := got.Strings()
				if isolationErr != nil || !slices.Equal(isolated, before) {
					t.Fatalf("mutating returned projection changed admitted environment: %v", isolationErr)
				}
			}
		})
	}
}

// Each iteration includes bounded parsing, typed admission, and exact lowering.
// It excludes fixture construction; no ambient environment is measured.
func BenchmarkExactEnvironmentAdmission(b *testing.B) {
	b.ReportAllocs()
	for _, size := range []int{1, 64, int(process.EnvironmentVariableCountMaximum)} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			values := make([]string, size)
			for index := range values {
				values[index] = "V" + strconv.Itoa(index) + "=payload"
			}
			b.ReportAllocs()
			for b.Loop() {
				got, err := process.ParseExactEnvironment(values)
				if err != nil {
					b.Fatal(err)
				}
				projected, err := got.Strings()
				if err != nil || projected == nil || !slices.Equal(projected, values) {
					b.Fatalf("exact environment failed closure: count=%d error=%v", len(projected), err)
				}
			}
		})
	}
}
