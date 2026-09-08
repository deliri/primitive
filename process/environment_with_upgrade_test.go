package process_test

import (
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

type environmentWithCase struct {
	name    string
	source  []string
	key     string
	value   string
	damage  func(*process.Environment, *process.EnvironmentVariable)
	wantErr error
}

func environmentWithCases() []environmentWithCase {
	extraName, nameErr := process.NewEnvironmentName("EXTRA")
	extraValue, valueErr := process.NewEnvironmentValue("")
	if err := errors.Join(nameErr, valueErr); err != nil {
		panic(err)
	}
	extra := process.EnvironmentVariable{Name: extraName, Value: extraValue}
	base := []string{"SYSTEMROOT=fixture", "A=old", "B=kept", "C=tail"}
	maximum := environmentWithCardinality(int(process.EnvironmentVariableCountMaximum))
	// A= plus its NUL consumes three bytes, besides the fixed critical pair.
	maximumValue := int(process.EnvironmentProjectionMaximumBytes) - len("SYSTEMROOT=fixture") - 1 - 3
	return []environmentWithCase{
		{name: "positive/insert retains every existing value", source: base, key: "D", value: "new"},
		{name: "positive/replace first ordinary value", source: base, key: "A", value: "new"},
		{name: "positive/replace middle value", source: base, key: "B", value: "new"},
		{name: "positive/replace final value", source: base, key: "C", value: "new"},
		{name: "positive/replace critical value without inheriting", source: base, key: "SYSTEMROOT", value: "owned"},
		{name: "positive/equal sign remains value content", source: base, key: "A", value: "x=y=z"},
		{name: "positive/non-UTF8 bytes remain exact", source: base, key: "A", value: "\xff\xc0\x80"},
		{name: "positive/unicode name remains distinct", source: base, key: "温度", value: "雪"},
		{name: "positive/case identity follows Go", source: base, key: "a", value: "lower"},
		{name: "positive/replacing same value still follows Go order", source: base, key: "A", value: "old"},
		{name: "negative/unknown receiver mode refuses", source: base, key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) { e.Mode = process.EnvironmentModeUnknown }, wantErr: core.ErrProcessContract},
		{name: "negative/ambient receiver cannot fabricate an exact snapshot", key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) { e.Mode = process.EnvironmentModeInherit }, wantErr: core.ErrProcessContract},
		{name: "negative/contradictory inheritance refuses", source: base, key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) { e.Mode = process.EnvironmentModeInherit }, wantErr: core.ErrProcessContract},
		{name: "negative/duplicate receiver cannot be repaired by replacement", source: base, key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) {
			e.Variables = append(e.Variables, e.Variables[1])
		}, wantErr: core.ErrProcessContract},
		{name: "negative/unset replacement name refuses", source: base, key: "A", damage: func(_ *process.Environment, v *process.EnvironmentVariable) { v.Name = process.EnvironmentName{} }, wantErr: core.ErrProcessContract},
		{name: "negative/unset replacement value refuses", source: base, key: "A", damage: func(_ *process.Environment, v *process.EnvironmentVariable) { v.Value = process.EnvironmentValue{} }, wantErr: core.ErrProcessContract},
		{name: "negative/unset receiver name cannot be overwritten", source: base, key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) {
			e.Variables[1].Name = process.EnvironmentName{}
		}, wantErr: core.ErrProcessContract},
		{name: "negative/unset receiver value cannot be overwritten", source: base, key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) {
			e.Variables[1].Value = process.EnvironmentValue{}
		}, wantErr: core.ErrProcessContract},
		{name: "negative/overfull receiver cannot be repaired", source: maximum, key: "EXTRA", damage: func(e *process.Environment, v *process.EnvironmentVariable) { e.Variables = append(e.Variables, *v) }, wantErr: core.ErrProcessContract},
		{name: "negative/overflowing receiver cannot be repaired", source: []string{"SYSTEMROOT=fixture", "A=" + strings.Repeat("x", maximumValue)}, key: "A", damage: func(e *process.Environment, _ *process.EnvironmentVariable) {
			e.Variables = append(e.Variables, extra)
		}, wantErr: core.ErrProcessContract},
		{name: "neutral/empty exact receiver becomes one owned value", key: "A", value: "first"},
		{name: "boundary/empty replacement remains present", source: base, key: "A"},
		{name: "boundary/empty inserted value remains present", source: base, key: "D"},
		{name: "boundary/empty critical value never borrows ambient value", source: base, key: "SYSTEMROOT"},
		{name: "boundary/shorter name is not a prefix match", source: []string{"SYSTEMROOT=fixture", "AA=kept"}, key: "A", value: "new"},
		{name: "boundary/longer name is not a prefix match", source: base, key: "AA", value: "new"},
		{name: "boundary/insert leaves cardinality below cap", source: environmentWithCardinality(int(process.EnvironmentVariableCountMaximum) - 2), key: "EXTRA"},
		{name: "boundary/insert fills cardinality cap", source: environmentWithCardinality(int(process.EnvironmentVariableCountMaximum) - 1), key: "EXTRA"},
		{name: "boundary/insert exceeds cardinality cap", source: maximum, key: "EXTRA", wantErr: core.ErrProcessContract},
		{name: "boundary/replacement at cardinality cap remains legal", source: maximum, key: "V0", value: "replaced"},
		{name: "boundary/maximum name stays exact", source: base, key: strings.Repeat("N", int(process.EnvironmentNameMaximumBytes)), value: "long name"},
		{name: "boundary/name one below cap stays exact", source: base, key: strings.Repeat("N", int(process.EnvironmentNameMaximumBytes)-1), value: "long name"},
		{name: "boundary/value atom cap cannot bypass aggregate cap", source: base, key: "A", value: strings.Repeat("x", int(process.EnvironmentValueMaximumBytes)), wantErr: core.ErrProcessContract},
		{name: "boundary/replacement projection below cap", source: []string{"SYSTEMROOT=fixture", "A=old"}, key: "A", value: strings.Repeat("x", maximumValue-1)},
		{name: "boundary/replacement projection at cap", source: []string{"SYSTEMROOT=fixture", "A=old"}, key: "A", value: strings.Repeat("x", maximumValue)},
		{name: "boundary/replacement projection above cap", source: []string{"SYSTEMROOT=fixture", "A=old"}, key: "A", value: strings.Repeat("x", maximumValue+1), wantErr: core.ErrProcessContract},
		{name: "boundary/insertion projection at cap", source: []string{"SYSTEMROOT=fixture"}, key: "A", value: strings.Repeat("x", maximumValue)},
		{name: "boundary/insertion projection above cap", source: []string{"SYSTEMROOT=fixture"}, key: "A", value: strings.Repeat("x", maximumValue+1), wantErr: core.ErrProcessContract},
		{name: "boundary/replacing a full value releases old extent", source: []string{"SYSTEMROOT=fixture", "A=" + strings.Repeat("x", maximumValue)}, key: "A", value: "small"},
		{name: "boundary/adding even empty value to full projection refuses", source: []string{"SYSTEMROOT=fixture", "A=" + strings.Repeat("x", maximumValue)}, key: "B", wantErr: core.ErrProcessContract},
	}
}

func TestEnvironmentWithCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range environmentWithCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source, sourceErr := process.ParseExactEnvironment(tc.source)
			name, nameErr := process.NewEnvironmentName(tc.key)
			value, valueErr := process.NewEnvironmentValue(tc.value)
			if err := errors.Join(sourceErr, nameErr, valueErr); err != nil {
				t.Fatal(err)
			}
			variable := process.EnvironmentVariable{Name: name, Value: value}
			if tc.damage != nil {
				tc.damage(&source, &variable)
			}
			before := process.Environment{Mode: source.Mode, Variables: slices.Clone(source.Variables)}
			variableBefore := variable
			got, err := source.With(variable)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("With error=%v, want %v", err, tc.wantErr)
			}
			if source.Mode != before.Mode || !slices.Equal(source.Variables, before.Variables) || variable != variableBefore {
				t.Fatalf("replacement changed caller input: environment=%+v want=%+v variable=%+v want=%+v", source, before, variable, variableBefore)
			}
			if tc.wantErr != nil {
				if got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
					t.Fatalf("refusal exposed partial environment: %+v", got)
				}
				return
			}
			command := exec.Cmd{Env: append(slices.Clone(tc.source), tc.key+"="+tc.value)}
			want := command.Environ()
			projected, err := got.Strings()
			if err != nil || got.Validate() != nil || got.Mode != process.EnvironmentModeExact || !slices.Equal(projected, want) {
				t.Fatalf("replacement lost Go's exact projection: error=%v, count=%d want=%d", err, len(projected), len(want))
			}
			got.Variables[0] = process.EnvironmentVariable{}
			if source.Mode != before.Mode || !slices.Equal(source.Variables, before.Variables) {
				t.Fatalf("returned environment aliases source: got=%+v want=%+v", source, before)
			}
		})
	}
}

func environmentWithCardinality(count int) []string {
	values := make([]string, count)
	values[0] = "SYSTEMROOT=fixture"
	for i := 1; i < count; i++ {
		values[i] = "V" + strconv.Itoa(i-1) + "="
	}
	return values
}
