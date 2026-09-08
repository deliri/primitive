package process_test

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestParseArgumentsClosesCountAndAggregateBoundaries(t *testing.T) {
	t.Parallel()

	countCases := []struct {
		wantErr error
		name    string
		count   uint32
	}{
		{name: "one below argument count maximum", count: process.ArgumentCountMaximum - 1},
		{name: "exact argument count maximum", count: process.ArgumentCountMaximum},
		{name: "one above argument count maximum", count: process.ArgumentCountMaximum + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range countCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := process.ParseArguments(make([]string, tc.count))
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseArguments(count %d) error = %v, want %v", tc.count, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != nil {
				t.Fatalf("ParseArguments(count %d) result length = %d, want nil on refusal", tc.count, len(got))
			}
			for _, argument := range got {
				if value, err := argument.Value(); err != nil || value != "" {
					t.Fatalf("empty argument projection=%q, %v", value, err)
				}
			}
			if tc.wantErr == nil && len(got) != int(tc.count) {
				t.Fatalf("ParseArguments(count %d) result length = %d, want %d", tc.count, len(got), tc.count)
			}
		})
	}

	aggregateCases := []struct {
		wantErr error
		name    string
		extent  uint64
	}{
		{name: "one below argument projection maximum", extent: process.ArgumentProjectionMaximumBytes - 1},
		{name: "exact argument projection maximum", extent: process.ArgumentProjectionMaximumBytes},
		{name: "one above argument projection maximum", extent: process.ArgumentProjectionMaximumBytes + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range aggregateCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := strings.Repeat("a", int(tc.extent)-1)
			got, gotErr := process.ParseArguments([]string{value})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseArguments(projected extent %d) error = %v, want %v", tc.extent, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != nil {
				t.Fatalf("ParseArguments(projected extent %d) result length = %d, want nil on refusal", tc.extent, len(got))
			}
			if tc.wantErr == nil {
				if len(got) != 1 {
					t.Fatalf("argument count=%d, want one", len(got))
				}
				if projection, err := got[0].Value(); err != nil || projection != value {
					t.Fatalf("argument projection changed at extent %d: %v", tc.extent, err)
				}
			}
		})
	}
}

func TestArgumentClosesIndividualExtentBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		extent  uint64
	}{
		{name: "one below argument maximum", extent: process.ArgumentMaximumBytes - 1},
		{name: "exact argument maximum", extent: process.ArgumentMaximumBytes},
		{name: "one above argument maximum", extent: process.ArgumentMaximumBytes + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := strings.Repeat("a", int(tc.extent))
			got, gotErr := process.NewArgument(input)
			if tc.wantErr == nil {
				if projection, err := got.Value(); err != nil || projection != input {
					t.Fatalf("Argument projection changed at extent %d: %v", tc.extent, err)
				}
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewArgument(extent %d) error = %v, want %v", tc.extent, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (process.Argument{}) {
				t.Fatalf("NewArgument(extent %d) result = %v, want zero on refusal", tc.extent, got)
			}
		})
	}
}

func TestEnvironmentAtomsCloseIndividualExtentBoundaries(t *testing.T) {
	t.Parallel()

	nameCases := []struct {
		wantErr error
		name    string
		extent  uint64
	}{
		{name: "one below environment name maximum", extent: process.EnvironmentNameMaximumBytes - 1},
		{name: "exact environment name maximum", extent: process.EnvironmentNameMaximumBytes},
		{name: "one above environment name maximum", extent: process.EnvironmentNameMaximumBytes + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range nameCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := strings.Repeat("N", int(tc.extent))
			got, gotErr := process.NewEnvironmentName(input)
			if tc.wantErr == nil {
				if projection, err := got.Value(); err != nil || projection != input {
					t.Fatalf("EnvironmentName projection changed at extent %d: %v", tc.extent, err)
				}
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewEnvironmentName(extent %d) error = %v, want %v", tc.extent, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (process.EnvironmentName{}) {
				t.Fatalf("NewEnvironmentName(extent %d) result = %v, want zero on refusal", tc.extent, got)
			}
		})
	}

	valueCases := []struct {
		wantErr error
		name    string
		extent  uint64
	}{
		{name: "one below environment value maximum", extent: process.EnvironmentValueMaximumBytes - 1},
		{name: "exact environment value maximum", extent: process.EnvironmentValueMaximumBytes},
		{name: "one above environment value maximum", extent: process.EnvironmentValueMaximumBytes + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range valueCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := strings.Repeat("v", int(tc.extent))
			got, gotErr := process.NewEnvironmentValue(input)
			if tc.wantErr == nil {
				if projection, err := got.Value(); err != nil || projection != input {
					t.Fatalf("EnvironmentValue projection changed at extent %d: %v", tc.extent, err)
				}
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewEnvironmentValue(extent %d) error = %v, want %v", tc.extent, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (process.EnvironmentValue{}) {
				t.Fatalf("NewEnvironmentValue(extent %d) result = %v, want zero on refusal", tc.extent, got)
			}
		})
	}
}

func TestParseExactEnvironmentClosesCountAndAggregateBoundaries(t *testing.T) {
	t.Parallel()

	countCases := []struct {
		wantErr error
		name    string
		count   uint32
	}{
		{name: "one below environment count maximum", count: process.EnvironmentVariableCountMaximum - 1},
		{name: "exact environment count maximum", count: process.EnvironmentVariableCountMaximum},
		{name: "one above environment count maximum", count: process.EnvironmentVariableCountMaximum + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range countCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			values := make([]string, tc.count)
			for index := range values {
				values[index] = environmentProjectionName(index) + "="
			}
			got, gotErr := process.ParseExactEnvironment(values)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseExactEnvironment(count %d) error = %v, want %v", tc.count, gotErr, tc.wantErr)
			}
			projected, projectErr := got.Strings()
			if tc.wantErr != nil {
				if got.Mode != process.EnvironmentModeUnknown || got.Variables != nil || !errors.Is(projectErr, core.ErrProcessContract) {
					t.Fatalf("ParseExactEnvironment(count %d) refusal result/project error = (%v, %v), want zero and ErrProcessContract", tc.count, got, projectErr)
				}
				return
			}
			if projectErr != nil || got.Mode != process.EnvironmentModeExact || !slices.Equal(projected, values) {
				t.Fatalf("ParseExactEnvironment(count %d) projected length/error = (%d, %v), want (%d, nil)", tc.count, len(projected), projectErr, tc.count)
			}
		})
	}

	aggregateCases := []struct {
		wantErr error
		name    string
		extent  uint64
	}{
		{name: "one below environment projection maximum", extent: process.EnvironmentProjectionMaximumBytes - 1},
		{name: "exact environment projection maximum", extent: process.EnvironmentProjectionMaximumBytes},
		{name: "one above environment projection maximum", extent: process.EnvironmentProjectionMaximumBytes + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range aggregateCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := strings.Repeat("v", int(tc.extent)-3)
			got, gotErr := process.ParseExactEnvironment([]string{"A=" + value})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseExactEnvironment(projected extent %d) error = %v, want %v", tc.extent, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && (got.Mode != process.EnvironmentModeUnknown || got.Variables != nil) {
				t.Fatalf("ParseExactEnvironment(projected extent %d) result = %v, want zero on refusal", tc.extent, got)
			}
			if tc.wantErr == nil {
				projected, err := got.Strings()
				if err != nil || got.Mode != process.EnvironmentModeExact || !slices.Equal(projected, []string{"A=" + value}) {
					t.Fatalf("environment projection changed at extent %d: %v", tc.extent, err)
				}
			}
		})
	}
}

func TestParseEffectiveEnvironmentRefusesOversizeBeforeOSProjection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		count  uint32
		extent uint64
	}{
		{name: "negative/unique names exceed vector cap", count: process.EnvironmentVariableCountMaximum + 1},
		{name: "negative/one pair exceeds aggregate cap", extent: process.EnvironmentProjectionMaximumBytes + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			values := make([]string, tc.count)
			for i := range values {
				values[i] = environmentProjectionName(i) + "=value"
			}
			if tc.extent != 0 {
				values = []string{"A=" + strings.Repeat("v", int(tc.extent)-3)}
			}
			got, err := process.ParseEffectiveEnvironment(values)
			if !errors.Is(err, core.ErrProcessContract) || got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
				t.Fatalf("oversize effective environment=%v, %v; want zero and contract refusal", got, err)
			}
		})
	}
}

func TestRequestValidationCannotBypassProjectionBounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		argumentCount uint32
		variableCount uint32
		wantErr       error
	}{
		{name: "neutral/no argv or explicit environment variables"},
		{name: "positive/exact argument count is usable", argumentCount: process.ArgumentCountMaximum},
		{name: "negative/argument vector exceeds count", argumentCount: process.ArgumentCountMaximum + 1, wantErr: core.ErrProcessContract},
		{name: "positive/unique environment at exact count", variableCount: process.EnvironmentVariableCountMaximum},
		{name: "negative/unique environment exceeds count", variableCount: process.EnvironmentVariableCountMaximum + 1, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
			request.Arguments = make([]process.Argument, tc.argumentCount)
			argument, err := process.NewArgument("")
			if err != nil {
				t.Fatal(err)
			}
			for i := range request.Arguments {
				request.Arguments[i] = argument
			}
			request.Environment = process.Environment{Mode: process.EnvironmentModeExact, Variables: make([]process.EnvironmentVariable, tc.variableCount)}
			value, err := process.NewEnvironmentValue("")
			if err != nil {
				t.Fatal(err)
			}
			for i := range request.Environment.Variables {
				name, err := process.NewEnvironmentName(environmentProjectionName(i))
				if err != nil {
					t.Fatal(err)
				}
				request.Environment.Variables[i] = process.EnvironmentVariable{Name: name, Value: value}
			}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("request admission=%v; want %v", err, tc.wantErr)
			}
		})
	}
}

func BenchmarkParseExactEnvironmentAtMaximumCount(b *testing.B) {
	values := make([]string, process.EnvironmentVariableCountMaximum)
	for index := range values {
		values[index] = environmentProjectionName(index) + "=value"
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := process.ParseExactEnvironment(values)
		if err != nil || got.Mode != process.EnvironmentModeExact || len(got.Variables) != len(values) {
			b.Fatalf("ParseExactEnvironment(maximum count) = (variables %d, %v), want (%d, nil)", len(got.Variables), err, len(values))
		}
		for i, variable := range got.Variables {
			name, value, found := strings.Cut(values[i], "=")
			gotName, nameErr := variable.Name.Value()
			gotValue, valueErr := variable.Value.Value()
			if !found || nameErr != nil || valueErr != nil || gotName != name || gotValue != value {
				b.Fatalf("environment pair %d changed: name=%q value=%q errors=%v/%v", i, gotName, gotValue, nameErr, valueErr)
			}
		}
	}
}

func environmentProjectionName(index int) string {
	return "BOUND_" + strconv.Itoa(index)
}
