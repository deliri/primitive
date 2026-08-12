package process_test

import (
	"bytes"
	"errors"
	"io"
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

			got, gotErr := process.NewArgument(strings.Repeat("a", int(tc.extent)))
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

			got, gotErr := process.NewEnvironmentName(strings.Repeat("N", int(tc.extent)))
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

			got, gotErr := process.NewEnvironmentValue(strings.Repeat("v", int(tc.extent)))
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
			if projectErr != nil || len(projected) != int(tc.count) {
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
		})
	}
}

func TestParseEffectiveEnvironmentRefusesOversizeBeforeOSProjection(t *testing.T) {
	t.Parallel()

	tooMany := make([]string, process.EnvironmentVariableCountMaximum+1)
	for index := range tooMany {
		tooMany[index] = "EFFECTIVE_" + strconv.Itoa(index) + "=value"
	}
	got, gotErr := process.ParseEffectiveEnvironment(tooMany)
	if !errors.Is(gotErr, core.ErrProcessContract) || got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
		t.Fatalf("ParseEffectiveEnvironment(count %d) = (%v, %v), want zero and errors.Is(..., %v)", len(tooMany), got, gotErr, core.ErrProcessContract)
	}

	value := strings.Repeat("v", int(process.EnvironmentProjectionMaximumBytes)-2)
	got, gotErr = process.ParseEffectiveEnvironment([]string{"A=" + value})
	if !errors.Is(gotErr, core.ErrProcessContract) || got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
		t.Fatalf("ParseEffectiveEnvironment(one-above projection) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrProcessContract)
	}
}

func TestRequestValidationCannotBypassProjectionBounds(t *testing.T) {
	t.Parallel()

	base := processRequest(t, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	argument, err := process.NewArgument("")
	if err != nil {
		t.Fatalf("NewArgument(empty) error = %v, want nil", err)
	}
	tooManyArguments := make([]process.Argument, process.ArgumentCountMaximum+1)
	for index := range tooManyArguments {
		tooManyArguments[index] = argument
	}
	base.Arguments = tooManyArguments
	if gotErr := base.Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("Request.Validate(argument count %d) error = %v, want errors.Is(..., %v)", len(base.Arguments), gotErr, core.ErrProcessContract)
	}

	name, err := process.NewEnvironmentName("A")
	if err != nil {
		t.Fatalf("NewEnvironmentName(A) error = %v, want nil", err)
	}
	value, err := process.NewEnvironmentValue("")
	if err != nil {
		t.Fatalf("NewEnvironmentValue(empty) error = %v, want nil", err)
	}
	tooManyVariables := make([]process.EnvironmentVariable, process.EnvironmentVariableCountMaximum+1)
	for index := range tooManyVariables {
		tooManyVariables[index] = process.EnvironmentVariable{Name: name, Value: value}
	}
	base.Arguments = nil
	base.Environment = process.Environment{Mode: process.EnvironmentModeExact, Variables: tooManyVariables}
	if gotErr := base.Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("Request.Validate(environment count %d) error = %v, want errors.Is(..., %v)", len(tooManyVariables), gotErr, core.ErrProcessContract)
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
		if err != nil || len(got.Variables) != len(values) {
			b.Fatalf("ParseExactEnvironment(maximum count) = (variables %d, %v), want (%d, nil)", len(got.Variables), err, len(values))
		}
	}
}

func environmentProjectionName(index int) string {
	return "BOUND_" + strconv.Itoa(index)
}
