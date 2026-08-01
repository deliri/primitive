package process_test

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestArgumentStringProjectsOnlyValidatedExactValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "empty constructed argument remains exact", value: ""},
		{name: "spaces remain one argument", value: "two words"},
		{name: "shell syntax remains inert bytes", value: "$(id); *"},
		{name: "control bytes remain exact", value: "\n\t\x1f"},
		{name: "invalid utf8 remains exact", value: "\xff\xfe"},
		{name: "NUL remains rejected", value: "before\x00after", wantErr: core.ErrProcessContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			argument, constructionErr := process.NewArgument(testCase.value)
			if !errors.Is(constructionErr, testCase.wantErr) {
				t.Fatalf("NewArgument() error = %v, want %v", constructionErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if _, gotErr := argument.String(); !errors.Is(gotErr, core.ErrProcessContract) {
					t.Fatalf("rejected Argument.String() error = %v, want %v", gotErr, core.ErrProcessContract)
				}
				return
			}
			got, gotErr := argument.String()
			if gotErr != nil || got != testCase.value {
				t.Fatalf("Argument.String() = (%q, %v), want (%q, nil)", got, gotErr, testCase.value)
			}
		})
	}
}

func TestParseArgumentsPreservesEveryArgumentBoundary(t *testing.T) {
	t.Parallel()

	want := []string{"", "two words", "$(id); *", "line\nfeed"}
	arguments, err := process.ParseArguments(want)
	if err != nil {
		t.Fatalf("ParseArguments() error = %v, want nil", err)
	}
	got := make([]string, len(arguments))
	for index, argument := range arguments {
		got[index], err = argument.String()
		if err != nil {
			t.Fatalf("Argument[%d].String() error = %v, want nil", index, err)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseArguments() projection = %q, want %q", got, want)
	}
	if rejected, gotErr := process.ParseArguments([]string{"ok", "bad\x00argument", "unreached"}); rejected != nil || !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("ParseArguments(NUL) = (%v, %v), want (nil, %v)", rejected, gotErr, core.ErrProcessContract)
	}
}

func TestEnvironmentStringsPreservesInheritanceAndExactOrdering(t *testing.T) {
	t.Parallel()

	nameA, err := process.NewEnvironmentName("A")
	if err != nil {
		t.Fatalf("NewEnvironmentName(A) error = %v, want nil", err)
	}
	nameB, err := process.NewEnvironmentName("B")
	if err != nil {
		t.Fatalf("NewEnvironmentName(B) error = %v, want nil", err)
	}
	valueEmpty, err := process.NewEnvironmentValue("")
	if err != nil {
		t.Fatalf("NewEnvironmentValue(empty) error = %v, want nil", err)
	}
	valueExact, err := process.NewEnvironmentValue("two=parts")
	if err != nil {
		t.Fatalf("NewEnvironmentValue(exact) error = %v, want nil", err)
	}
	cases := []struct {
		wantErr error
		name    string
		want    []string
		value   process.Environment
		wantNil bool
	}{
		{name: "inherit projects nil", value: process.Environment{Mode: process.EnvironmentModeInherit}, wantNil: true},
		{name: "exact empty projects nonnil empty", value: process.Environment{Mode: process.EnvironmentModeExact}, want: []string{}},
		{name: "exact variables preserve order values and empty", value: process.Environment{Mode: process.EnvironmentModeExact, Variables: []process.EnvironmentVariable{{Name: nameB, Value: valueExact}, {Name: nameA, Value: valueEmpty}}}, want: []string{"B=two=parts", "A="}},
		{name: "duplicate exact name is rejected", value: process.Environment{Mode: process.EnvironmentModeExact, Variables: []process.EnvironmentVariable{{Name: nameA, Value: valueExact}, {Name: nameA, Value: valueEmpty}}}, wantErr: core.ErrProcessContract},
		{name: "inherit with variables is rejected", value: process.Environment{Mode: process.EnvironmentModeInherit, Variables: []process.EnvironmentVariable{{Name: nameA, Value: valueExact}}}, wantErr: core.ErrProcessContract},
		{name: "unknown mode is rejected", value: process.Environment{}, wantErr: core.ErrProcessContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := testCase.value.Strings()
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("Environment.Strings() error = %v, want %v", gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if got != nil {
					t.Fatalf("rejected Environment.Strings() = %q, want nil", got)
				}
				return
			}
			if (got == nil) != testCase.wantNil || !slices.Equal(got, testCase.want) {
				t.Fatalf("Environment.Strings() = %#v, want %#v with nil=%t", got, testCase.want, testCase.wantNil)
			}
		})
	}
}

func TestParseExactEnvironmentRaisesOnlyUnambiguousProjections(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		values  []string
		want    []string
	}{
		{name: "empty remains exact empty", values: []string{}, want: []string{}},
		{name: "order empty and separator-rich values survive", values: []string{"B=two=parts", "A="}, want: []string{"B=two=parts", "A="}},
		{name: "missing separator rejected", values: []string{"A"}, wantErr: core.ErrProcessContract},
		{name: "empty name rejected", values: []string{"=value"}, wantErr: core.ErrProcessContract},
		{name: "duplicate name rejected", values: []string{"A=one", "A=two"}, wantErr: core.ErrProcessContract},
		{name: "nul rejected", values: []string{"A=one\x00two"}, wantErr: core.ErrProcessContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := process.ParseExactEnvironment(testCase.values)
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("ParseExactEnvironment() error = %v, want errors.Is %v", gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ParseExactEnvironment() error = %v, want nil", gotErr)
			}
			projected, projectedErr := got.Strings()
			if projectedErr != nil || !slices.Equal(projected, testCase.want) {
				t.Fatalf("parsed environment Strings() = (%v, %v), want (%v, nil)", projected, projectedErr, testCase.want)
			}
		})
	}
}

func TestParseEffectiveEnvironmentCanonicalizesOSExecLastValueWins(t *testing.T) {
	t.Parallel()

	environment, err := process.ParseEffectiveEnvironment([]string{"A=old", "B=kept", "A=new", "C=", "B=last"})
	if err != nil {
		t.Fatalf("ParseEffectiveEnvironment() error = %v, want nil", err)
	}
	got, err := environment.Strings()
	if err != nil {
		t.Fatalf("effective Environment.Strings() error = %v, want nil", err)
	}
	want := []string{"A=new", "C=", "B=last"}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseEffectiveEnvironment() projection = %q, want %q", got, want)
	}
	if _, gotErr := process.ParseEffectiveEnvironment([]string{"A=old", "malformed", "A=new"}); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("ParseEffectiveEnvironment(malformed) error = %v, want %v", gotErr, core.ErrProcessContract)
	}
}

func TestTruncatingWriterRetainsPrefixAndCountsDroppedBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		want        string
		writes      []string
		limit       uint64
		wantDropped uint64
	}{
		{name: "under limit retains everything", limit: 10, writes: []string{"abc"}, want: "abc"},
		{name: "exact limit retains everything", limit: 3, writes: []string{"abc"}, want: "abc"},
		{name: "one over retains prefix", limit: 3, writes: []string{"abcd"}, want: "abc", wantDropped: 1},
		{name: "write straddles limit", limit: 4, writes: []string{"ab", "cdef"}, want: "abcd", wantDropped: 2},
		{name: "writes after saturation are consumed", limit: 2, writes: []string{"ab", "cd", "ef"}, want: "ab", wantDropped: 4},
		{name: "empty writes are neutral", limit: 2, writes: []string{"", ""}, want: ""},
		{name: "large write stays at caller bound", limit: 8, writes: []string{strings.Repeat("x", 1<<20)}, want: strings.Repeat("x", 8), wantDropped: 1<<20 - 8},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var destination bytes.Buffer
			writer, err := process.NewTruncatingWriter(&destination, byteCount(t, testCase.limit))
			if err != nil {
				t.Fatalf("NewTruncatingWriter() error = %v, want nil", err)
			}
			for _, value := range testCase.writes {
				gotCount, gotErr := writer.Write([]byte(value))
				if gotErr != nil || gotCount != len(value) {
					t.Fatalf("TruncatingWriter.Write(%d bytes) = (%d, %v), want (%d, nil)", len(value), gotCount, gotErr, len(value))
				}
			}
			retained, retainedErr := writer.RetainedBytes()
			dropped, droppedErr := writer.DroppedBytes()
			if retainedErr != nil || droppedErr != nil || retained.Uint64() != uint64(len(testCase.want)) || dropped.Uint64() != testCase.wantDropped {
				t.Fatalf("TruncatingWriter counts = retained %d/%v dropped %d/%v, want %d/nil %d/nil", retained.Uint64(), retainedErr, dropped.Uint64(), droppedErr, len(testCase.want), testCase.wantDropped)
			}
			if destination.String() != testCase.want {
				t.Fatalf("TruncatingWriter destination = %q, want %q", destination.String(), testCase.want)
			}
		})
	}
}

func TestTruncatingWriterPreservesDestinationFailures(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("destination failed")
	writer, err := process.NewTruncatingWriter(
		writerFunc(func([]byte) (int, error) { return 0, wantErr }),
		byteCount(t, 8),
	)
	if err != nil {
		t.Fatalf("NewTruncatingWriter() error = %v, want nil", err)
	}
	gotCount, gotErr := writer.Write([]byte("payload"))
	if gotCount != 0 || !errors.Is(gotErr, wantErr) {
		t.Fatalf("TruncatingWriter.Write() = (%d, %v), want (0, %v)", gotCount, gotErr, wantErr)
	}
	retained, retainedErr := writer.RetainedBytes()
	dropped, droppedErr := writer.DroppedBytes()
	if retainedErr != nil || droppedErr != nil || retained.Uint64() != 0 || dropped.Uint64() != 0 {
		t.Fatalf("failed write counts = retained %d/%v dropped %d/%v, want zeros/nil", retained.Uint64(), retainedErr, dropped.Uint64(), droppedErr)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(value []byte) (int, error) { return f(value) }

var _ io.Writer = writerFunc(nil)
