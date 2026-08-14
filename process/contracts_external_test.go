package process_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// controlBytes is every ASCII control byte except NUL, which the argv and
// environment-value contracts must admit because argv carries bytes, not text.
const controlBytes = "\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f"

func TestArgumentAdmitsExactBytesAndRejectsNUL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "empty argument is a valid exact value", value: ""},
		{name: "single character argument", value: "a"},
		{name: "bare dash is not a flag to this contract", value: "-"},
		{name: "long flag with an embedded equals sign", value: "--flag=value"},
		{name: "argument containing spaces stays one value", value: "two words"},
		{name: "argument that is only an equals sign", value: "="},
		{name: "argument that is only whitespace", value: " \t\n"},
		{name: "shell metacharacters are ordinary bytes", value: "$(id); rm -rf / | tee"},
		{name: "glob metacharacters are ordinary bytes", value: "*?[a-z]"},
		{name: "quotes and backslashes are ordinary bytes", value: `'"\\`},
		{name: "multibyte text is admitted", value: "日本語"},
		{name: "invalid utf-8 bytes are admitted because argv is bytes", value: "\x80\xff\xfe"},
		{name: "every control byte except NUL is admitted", value: controlBytes},
		{name: "single high byte is admitted", value: "\xff"},
		{name: "four kibibyte argument is admitted", value: strings.Repeat("x", 1<<12)},
		{name: "one mebibyte argument is admitted", value: strings.Repeat("y", 1<<20)},

		{
			name:    "lone NUL is rejected",
			value:   "\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "trailing NUL is rejected",
			value:   "before\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "leading NUL is rejected",
			value:   "\x00after",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "interior NUL is rejected",
			value:   "before\x00after",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "repeated NUL bytes are rejected",
			value:   "\x00\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "NUL after a long valid prefix is rejected",
			value:   strings.Repeat("x", 1<<12) + "\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "NUL beside other control bytes is rejected",
			value:   controlBytes + "\x00",
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := process.NewArgument(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"process.NewArgument(%q) error = %v, want %v",
					tc.value,
					gotErr,
					tc.wantErr,
				)
			}
			if tc.wantErr != nil {
				if got != (process.Argument{}) {
					t.Fatalf(
						"process.NewArgument(%q) = %v, want the zero argument on rejection",
						tc.value,
						got,
					)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("accepted Argument.Validate() error = %v, want nil", err)
			}
		})
	}

	if gotErr := (process.Argument{}).Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf(
			"process.Argument{}.Validate() error = %v, want %v because an unset argument is not an empty one",
			gotErr,
			core.ErrProcessContract,
		)
	}
}

func TestEnvironmentNameRejectsEveryAmbiguousForm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "single uppercase letter", value: "A"},
		{name: "conventional uppercase name", value: "PATH"},
		{name: "lower case with underscore", value: "lower_case"},
		{name: "name containing dots", value: "with.dots"},
		{name: "name containing a dash", value: "with-dash"},
		{name: "name starting with a digit is not this contract's concern", value: "1NAME"},
		{name: "name containing a space is byte exact", value: "with space"},
		{name: "multibyte name is admitted", value: "日本語"},
		{name: "invalid utf-8 name is admitted", value: "\x80\xff"},
		{name: "four kibibyte name is admitted", value: strings.Repeat("N", 1<<12)},

		{
			name:    "empty name is rejected",
			value:   "",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "lone equals sign is rejected",
			value:   "=",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "leading equals sign is rejected",
			value:   "=VALUE",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "trailing equals sign is rejected",
			value:   "NAME=",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "interior equals sign is rejected",
			value:   "NA=ME",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "lone NUL is rejected",
			value:   "\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "trailing NUL is rejected",
			value:   "NAME\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "both reserved bytes together are rejected",
			value:   "NA=ME\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "reserved byte after a long valid prefix is rejected",
			value:   strings.Repeat("N", 1<<12) + "=",
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := process.NewEnvironmentName(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"process.NewEnvironmentName(%q) error = %v, want %v",
					tc.value,
					gotErr,
					tc.wantErr,
				)
			}
			if tc.wantErr != nil {
				if got != (process.EnvironmentName{}) {
					t.Fatalf(
						"process.NewEnvironmentName(%q) = %v, want the zero name on rejection",
						tc.value,
						got,
					)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("accepted EnvironmentName.Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestEnvironmentNameAccessRejectsUnsetAndPreservesExactAdmittedBytes(t *testing.T) {
	t.Parallel()

	if got, err := (process.EnvironmentName{}).Value(); got != "" || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("zero EnvironmentName.Value() = %q, %v; want empty and errors.Is(..., ErrProcessContract)", got, err)
	}
	const exact = "EXACT_ENVIRONMENT_NAME"
	name, err := process.NewEnvironmentName(exact)
	if err != nil {
		t.Fatalf("NewEnvironmentName(%q) error = %v, want nil", exact, err)
	}
	got, err := name.Value()
	if err != nil || got != exact {
		t.Fatalf("EnvironmentName.Value() = %q, %v; want %q, nil", got, err, exact)
	}
}

func TestEnvironmentPresenceExhaustsUint8Domain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		presence := process.EnvironmentPresence(raw)
		wantValid := presence == process.EnvironmentPresenceAbsent || presence == process.EnvironmentPresencePresent
		if got := presence.IsValid(); got != wantValid {
			t.Fatalf("EnvironmentPresence(%d).IsValid() = %t, want %t", raw, got, wantValid)
		}
		gotErr := presence.Validate()
		if wantValid && gotErr != nil {
			t.Fatalf("EnvironmentPresence(%d).Validate() error = %v, want nil", raw, gotErr)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Fatalf("EnvironmentPresence(%d).Validate() error = %v, want errors.Is %v", raw, gotErr, core.ErrProcessContract)
		}
		if gotUnknown := presence.String() == core.UnknownEnumDiagnostic; gotUnknown == wantValid {
			t.Fatalf("EnvironmentPresence(%d).String() unknown = %t, want %t", raw, gotUnknown, !wantValid)
		}
	}
}

func TestEnvironmentPresenceCompilerOwnedLabels(t *testing.T) {
	t.Parallel()

	if got := process.EnvironmentPresenceAbsent.String(); got != "absent" {
		t.Fatalf("EnvironmentPresenceAbsent.String() = %q, want %q", got, "absent")
	}
	if got := process.EnvironmentPresencePresent.String(); got != "present" {
		t.Fatalf("EnvironmentPresencePresent.String() = %q, want %q", got, "present")
	}
}

func TestEnvironmentLookupRejectsContradictoryPresenceAndValue(t *testing.T) {
	t.Parallel()

	empty, err := process.NewEnvironmentValue("")
	if err != nil {
		t.Fatalf("NewEnvironmentValue(empty) error = %v, want nil", err)
	}
	value, err := process.NewEnvironmentValue("value")
	if err != nil {
		t.Fatalf("NewEnvironmentValue(value) error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		lookup  process.EnvironmentLookup
		wantErr error
	}{
		{name: "absent owns zero value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresenceAbsent}},
		{name: "present owns empty exact value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresencePresent, Value: empty}},
		{name: "present owns nonempty exact value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresencePresent, Value: value}},
		{name: "zero lookup rejects unknown presence", lookup: process.EnvironmentLookup{}, wantErr: core.ErrProcessContract},
		{name: "future presence rejects zero value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresence(255)}, wantErr: core.ErrProcessContract},
		{name: "absent rejects admitted empty value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresenceAbsent, Value: empty}, wantErr: core.ErrProcessContract},
		{name: "absent rejects admitted nonempty value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresenceAbsent, Value: value}, wantErr: core.ErrProcessContract},
		{name: "present rejects unset value", lookup: process.EnvironmentLookup{Presence: process.EnvironmentPresencePresent}, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.lookup.Validate()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("EnvironmentLookup.Validate() error = %v, want errors.Is(..., ErrProcessContract)", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnvironmentLookup.Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestEnvironmentValueAccessRejectsUnsetAndPreservesExactAdmittedBytes(t *testing.T) {
	t.Parallel()

	if got, err := (process.EnvironmentValue{}).Value(); got != "" || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("zero EnvironmentValue.Value() = %q, %v; want empty and errors.Is(..., ErrProcessContract)", got, err)
	}
	const exact = " exact value "
	value, err := process.NewEnvironmentValue(exact)
	if err != nil {
		t.Fatalf("NewEnvironmentValue(%q) error = %v, want nil", exact, err)
	}
	got, err := value.Value()
	if err != nil || got != exact {
		t.Fatalf("EnvironmentValue.Value() = %q, %v; want %q, nil", got, err, exact)
	}
}

func TestEnvironmentValueAdmitsExactBytesAndRejectsNUL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "empty value is valid and distinct from unset", value: ""},
		{name: "ordinary value", value: "value"},
		{name: "value that is only an equals sign", value: "="},
		{name: "value containing an equals sign is unambiguous", value: "a=b"},
		{name: "value containing many equals signs", value: "a=b=c=d"},
		{name: "value containing spaces", value: "two words"},
		{name: "value containing newlines", value: "first\nsecond\n"},
		{name: "multibyte value", value: "日本語"},
		{name: "invalid utf-8 value is admitted", value: "\x80\xff\xfe"},
		{name: "every control byte except NUL is admitted", value: controlBytes},
		{name: "one mebibyte value is admitted", value: strings.Repeat("z", 1<<20)},

		{
			name:    "lone NUL is rejected",
			value:   "\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "trailing NUL is rejected",
			value:   "before\x00",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "leading NUL is rejected",
			value:   "\x00after",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "interior NUL is rejected",
			value:   "before\x00after",
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "NUL beside an equals sign is rejected",
			value:   "a=b\x00",
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := process.NewEnvironmentValue(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"process.NewEnvironmentValue(%q) error = %v, want %v",
					tc.value,
					gotErr,
					tc.wantErr,
				)
			}
			if tc.wantErr != nil {
				if got != (process.EnvironmentValue{}) {
					t.Fatalf(
						"process.NewEnvironmentValue(%q) = %v, want the zero value on rejection",
						tc.value,
						got,
					)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("accepted EnvironmentValue.Validate() error = %v, want nil", err)
			}
		})
	}

	if gotErr := (process.EnvironmentValue{}).Validate(); !errors.Is(
		gotErr,
		core.ErrProcessContract,
	) {
		t.Fatalf(
			"process.EnvironmentValue{}.Validate() error = %v, want %v because unset is not empty",
			gotErr,
			core.ErrProcessContract,
		)
	}
}

func TestEnvironmentVariableRequiresBothHalves(t *testing.T) {
	t.Parallel()

	name, err := process.NewEnvironmentName("NAME")
	if err != nil {
		t.Fatalf("process.NewEnvironmentName(NAME) error = %v, want nil", err)
	}
	value, err := process.NewEnvironmentValue("value")
	if err != nil {
		t.Fatalf("process.NewEnvironmentValue(value) error = %v, want nil", err)
	}
	empty, err := process.NewEnvironmentValue("")
	if err != nil {
		t.Fatalf("process.NewEnvironmentValue(empty) error = %v, want nil", err)
	}
	cases := []struct {
		wantErr  error
		name     string
		variable process.EnvironmentVariable
	}{
		{
			name:     "name and value present is accepted",
			variable: process.EnvironmentVariable{Name: name, Value: value},
		},
		{
			name:     "name with an explicitly empty value is accepted",
			variable: process.EnvironmentVariable{Name: name, Value: empty},
		},
		{
			name:     "zero variable is rejected",
			variable: process.EnvironmentVariable{},
			wantErr:  core.ErrProcessContract,
		},
		{
			name:     "unset name with a set value is rejected",
			variable: process.EnvironmentVariable{Value: value},
			wantErr:  core.ErrProcessContract,
		},
		{
			name:     "set name with an unset value is rejected",
			variable: process.EnvironmentVariable{Name: name},
			wantErr:  core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.variable.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"EnvironmentVariable.Validate() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestEnvironmentModeAndDuplicatePressure(t *testing.T) {
	t.Parallel()

	first := environmentVariable(t, "A", "first")
	second := environmentVariable(t, "B", "second")
	third := environmentVariable(t, "C", "third")
	duplicate := environmentVariable(t, "A", "replacement")
	identical := environmentVariable(t, "A", "first")
	lowercase := environmentVariable(t, "a", "distinct case")
	cases := []struct {
		wantErr     error
		name        string
		environment process.Environment
	}{
		{
			name: "inherit with no exact variables is accepted",
			environment: process.Environment{
				Mode: process.EnvironmentModeInherit,
			},
		},
		{
			name: "exact empty environment is accepted",
			environment: process.Environment{
				Mode: process.EnvironmentModeExact,
			},
		},
		{
			name: "exact one-variable environment is accepted",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first},
			},
		},
		{
			name: "exact ordered distinct environment is accepted",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first, second},
			},
		},
		{
			name: "exact three distinct variables are accepted",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first, second, third},
			},
		},
		{
			name: "names differing only in case are distinct",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first, lowercase},
			},
		},
		{
			name: "exact environment with an empty value is accepted",
			environment: process.Environment{
				Mode: process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{
					environmentVariable(t, "EMPTY", ""),
				},
			},
		},
		{
			name:        "zero environment mode is rejected",
			environment: process.Environment{},
			wantErr:     core.ErrProcessContract,
		},
		{
			name: "first mode value above the domain is rejected",
			environment: process.Environment{
				Mode: process.EnvironmentModeExact + 1,
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "future environment mode is rejected",
			environment: process.Environment{
				Mode: process.EnvironmentMode(math.MaxUint8),
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "unknown mode with valid variables is still rejected",
			environment: process.Environment{
				Mode:      process.EnvironmentModeUnknown,
				Variables: []process.EnvironmentVariable{first},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "inherit with a variable is contradictory",
			environment: process.Environment{
				Mode:      process.EnvironmentModeInherit,
				Variables: []process.EnvironmentVariable{first},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "inherit with an unset variable is contradictory",
			environment: process.Environment{
				Mode:      process.EnvironmentModeInherit,
				Variables: []process.EnvironmentVariable{{}},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "exact adjacent duplicate name is rejected",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first, duplicate},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "exact fully identical duplicate is rejected",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first, identical},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "exact distant duplicate name is rejected",
			environment: process.Environment{
				Mode: process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{
					first, second, third, duplicate,
				},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "exact unset variable is rejected",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{{}},
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "exact trailing unset variable is rejected",
			environment: process.Environment{
				Mode:      process.EnvironmentModeExact,
				Variables: []process.EnvironmentVariable{first, second, {}},
			},
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.environment.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Environment.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestStreamsRejectEveryNilCombination(t *testing.T) {
	t.Parallel()

	reader := bytes.NewReader(nil)
	cases := []struct {
		streams process.Streams
		wantErr error
		name    string
	}{
		{
			name:    "all three streams present is accepted",
			streams: process.Streams{Stdin: reader, Stdout: io.Discard, Stderr: io.Discard},
		},
		{
			name:    "all three nil is rejected",
			streams: process.Streams{},
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "nil stdin is rejected",
			streams: process.Streams{Stdout: io.Discard, Stderr: io.Discard},
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "nil stdout is rejected",
			streams: process.Streams{Stdin: reader, Stderr: io.Discard},
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "nil stderr is rejected",
			streams: process.Streams{Stdin: reader, Stdout: io.Discard},
			wantErr: core.ErrProcessContract,
		},
		{
			name:    "only stdin present is rejected",
			streams: process.Streams{Stdin: reader},
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.streams.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Streams.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestRequestIngressPressure(t *testing.T) {
	t.Parallel()

	request := processRequest(t, "silent", process.Streams{
		Stdin:  bytes.NewReader(nil),
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	cases := []struct {
		mutate  func(testing.TB, process.Request) process.Request
		wantErr error
		name    string
	}{
		{
			name:   "complete request is accepted",
			mutate: func(_ testing.TB, value process.Request) process.Request { return value },
		},
		{
			name: "request with no arguments is accepted",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Arguments = nil
				return value
			},
		},
		{
			name: "request with an empty argument slice is accepted",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Arguments = []process.Argument{}
				return value
			},
		},
		{
			name: "request with an empty-string argument is accepted",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.Arguments = arguments(tb, "")
				return value
			},
		},
		{
			name: "request with many arguments is accepted",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.Arguments = arguments(tb, manyArguments(256)...)
				return value
			},
		},
		{
			name: "exact empty environment is accepted",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Environment = process.Environment{
					Mode: process.EnvironmentModeExact,
				}
				return value
			},
		},
		{
			name: "minimum output limit of one byte is accepted",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.OutputLimit = byteCount(tb, 1)
				return value
			},
		},
		{
			name: "maximum signed output limit is accepted",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.OutputLimit = byteCount(tb, math.MaxInt64)
				return value
			},
		},
		{
			name: "first unsigned-only output limit is rejected",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.OutputLimit = byteCount(tb, uint64(math.MaxInt64)+1)
				return value
			},
			wantErr: core.ErrNumericOverflow,
		},
		{
			name: "one nanosecond wait delay is accepted",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				delay, err := temporal.DurationFromNanoseconds(1)
				if err != nil {
					tb.Fatalf("temporal.DurationFromNanoseconds(1) error = %v, want nil", err)
				}
				value.WaitDelay = delay
				return value
			},
		},
		{
			name: "one day wait delay is accepted",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				delay, err := temporal.DurationFromHours(24)
				if err != nil {
					tb.Fatalf("temporal.DurationFromHours(24) error = %v, want nil", err)
				}
				value.WaitDelay = delay
				return value
			},
		},

		{
			name: "unset command is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Command = core.AbsolutePath{}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "unset working directory is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.WorkingDirectory = core.AbsolutePath{}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "unset trailing argument is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Arguments = append(value.Arguments, process.Argument{})
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "unset leading argument is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Arguments = append(
					[]process.Argument{{}},
					value.Arguments...,
				)
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "the only argument being unset is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Arguments = []process.Argument{{}}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "unset environment is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Environment = process.Environment{}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "contradictory inherited environment is rejected",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.Environment = process.Environment{
					Mode: process.EnvironmentModeInherit,
					Variables: []process.EnvironmentVariable{
						environmentVariable(tb, "A", "first"),
					},
				}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "duplicate exact environment name is rejected",
			mutate: func(tb testing.TB, value process.Request) process.Request {
				value.Environment = process.Environment{
					Mode: process.EnvironmentModeExact,
					Variables: []process.EnvironmentVariable{
						environmentVariable(tb, "A", "first"),
						environmentVariable(tb, "A", "second"),
					},
				}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "nil stdin is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Streams.Stdin = nil
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "nil stdout is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Streams.Stdout = nil
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "nil stderr is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Streams.Stderr = nil
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "every nil stream is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.Streams = process.Streams{}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "unset output limit is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.OutputLimit = core.ByteCount{}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "zero wait delay is rejected",
			mutate: func(_ testing.TB, value process.Request) process.Request {
				value.WaitDelay = temporal.Duration{}
				return value
			},
			wantErr: core.ErrProcessContract,
		},
		{
			name: "wholly zero request is rejected",
			mutate: func(_ testing.TB, _ process.Request) process.Request {
				return process.Request{}
			},
			wantErr: core.ErrProcessContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.mutate(t, request).Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Request.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestZeroResultAndExitCodeRefuseProjection(t *testing.T) {
	t.Parallel()

	result := process.Result{}
	if gotErr := result.Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.Result{}.Validate() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := result.ExitCode(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.Result{}.ExitCode() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := result.CPUTime(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.Result{}.CPUTime() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := result.StdinBytes(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.Result{}.StdinBytes() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := result.StdoutBytes(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.Result{}.StdoutBytes() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := result.StderrBytes(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.Result{}.StderrBytes() error = %v, want %v", gotErr, core.ErrProcessContract)
	}

	exit := process.ExitCode{}
	if gotErr := exit.Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.ExitCode{}.Validate() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := exit.Int(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.ExitCode{}.Int() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := exit.Success(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.ExitCode{}.Success() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
	if _, gotErr := exit.Signaled(); !errors.Is(gotErr, core.ErrProcessContract) {
		t.Fatalf("process.ExitCode{}.Signaled() error = %v, want %v", gotErr, core.ErrProcessContract)
	}
}

func TestClosedEnumsExhaustAllBackingValues(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		environmentMode := process.EnvironmentMode(raw)
		wantEnvironmentValid := environmentMode == process.EnvironmentModeInherit ||
			environmentMode == process.EnvironmentModeExact
		if got := environmentMode.IsValid(); got != wantEnvironmentValid {
			t.Errorf(
				"EnvironmentMode(%d).IsValid() = %t, want %t",
				raw,
				got,
				wantEnvironmentValid,
			)
		}
		if gotErr := environmentMode.Validate(); wantEnvironmentValid && gotErr != nil || !wantEnvironmentValid && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Errorf(
				"EnvironmentMode(%d).Validate() error = %v, want valid %t",
				raw,
				gotErr,
				wantEnvironmentValid,
			)
		}
		if gotUnknown := environmentMode.String() ==
			process.EnvironmentModeUnknown.String(); gotUnknown == wantEnvironmentValid {
			t.Errorf(
				"EnvironmentMode(%d).String() unknown = %t, want %t",
				raw,
				gotUnknown,
				!wantEnvironmentValid,
			)
		}

		stream := process.Stream(raw)
		wantStreamValid := stream >= process.StreamStdin &&
			stream <= process.StreamStderr
		if got := stream.IsValid(); got != wantStreamValid {
			t.Errorf("Stream(%d).IsValid() = %t, want %t", raw, got, wantStreamValid)
		}
		if gotErr := stream.Validate(); wantStreamValid && gotErr != nil || !wantStreamValid && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Errorf(
				"Stream(%d).Validate() error = %v, want valid %t",
				raw,
				gotErr,
				wantStreamValid,
			)
		}
		if gotUnknown := stream.String() ==
			process.StreamUnknown.String(); gotUnknown == wantStreamValid {
			t.Errorf(
				"Stream(%d).String() unknown = %t, want %t",
				raw,
				gotUnknown,
				!wantStreamValid,
			)
		}

		failureKind := process.FailureKind(raw)
		wantFailureValid := failureKind == process.FailureKindStart ||
			failureKind == process.FailureKindWait
		if got := failureKind.IsValid(); got != wantFailureValid {
			t.Errorf(
				"FailureKind(%d).IsValid() = %t, want %t",
				raw,
				got,
				wantFailureValid,
			)
		}
		if gotErr := failureKind.Validate(); wantFailureValid && gotErr != nil || !wantFailureValid && !errors.Is(gotErr, core.ErrProcessContract) {
			t.Errorf(
				"FailureKind(%d).Validate() error = %v, want valid %t",
				raw,
				gotErr,
				wantFailureValid,
			)
		}
		if gotUnknown := failureKind.String() ==
			process.FailureKindUnknown.String(); gotUnknown == wantFailureValid {
			t.Errorf(
				"FailureKind(%d).String() unknown = %t, want %t",
				raw,
				gotUnknown,
				!wantFailureValid,
			)
		}
	}
}

// TestClosedEnumLabelsAreDistinctAndStable pins the compiler-owned labels. The
// exhaustive scan above proves only that invalid values render as unknown; it
// would still pass if two admitted values shared one label.
func TestClosedEnumLabelsAreDistinctAndStable(t *testing.T) {
	t.Parallel()

	if got := process.EnvironmentModeInherit.String(); got != "inherit" {
		t.Errorf("EnvironmentModeInherit.String() = %q, want %q", got, "inherit")
	}
	if got := process.EnvironmentModeExact.String(); got != "exact" {
		t.Errorf("EnvironmentModeExact.String() = %q, want %q", got, "exact")
	}
	if got := process.StreamStdin.String(); got != "stdin" {
		t.Errorf("StreamStdin.String() = %q, want %q", got, "stdin")
	}
	if got := process.StreamStdout.String(); got != "stdout" {
		t.Errorf("StreamStdout.String() = %q, want %q", got, "stdout")
	}
	if got := process.StreamStderr.String(); got != "stderr" {
		t.Errorf("StreamStderr.String() = %q, want %q", got, "stderr")
	}
	if got := process.FailureKindStart.String(); got != "start" {
		t.Errorf("FailureKindStart.String() = %q, want %q", got, "start")
	}
	if got := process.FailureKindWait.String(); got != "wait" {
		t.Errorf("FailureKindWait.String() = %q, want %q", got, "wait")
	}
	if gotSame := process.FailureKindStart.String() == process.FailureKindWait.String(); gotSame {
		t.Errorf(
			"FailureKind label equality = %t for %q and %q, want false",
			gotSame,
			process.FailureKindStart.String(),
			process.FailureKindWait.String(),
		)
	}
}

// TestProcessErrorIdentityHierarchy pins the Core-owned parent chain. Callers
// decide on a whole family, so a reparented identity would silently change which
// errors.Is checks succeed for every consumer.
func TestProcessErrorIdentityHierarchy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		identity error
		parents  []error
	}{
		{
			name:     "contract is a primitive contract violation",
			identity: core.ErrProcessContract,
			parents:  []error{core.ErrPrimitiveContract},
		},
		{
			name:     "start descends from the process contract",
			identity: core.ErrProcessStart,
			parents:  []error{core.ErrProcessContract, core.ErrPrimitiveContract},
		},
		{
			name:     "wait descends from the process contract",
			identity: core.ErrProcessWait,
			parents:  []error{core.ErrProcessContract, core.ErrPrimitiveContract},
		},
		{
			name:     "stream descends from the process contract",
			identity: core.ErrProcessStream,
			parents:  []error{core.ErrProcessContract, core.ErrPrimitiveContract},
		},
		{
			name:     "output limit descends from stream not directly from contract",
			identity: core.ErrProcessOutputLimit,
			parents: []error{
				core.ErrProcessStream,
				core.ErrProcessContract,
				core.ErrPrimitiveContract,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, parent := range tc.parents {
				if !errors.Is(tc.identity, parent) {
					t.Errorf("errors.Is(%v, %v) = false, want true", tc.identity, parent)
				}
			}
			if errors.Is(core.ErrProcessStart, core.ErrProcessWait) {
				t.Errorf(
					"errors.Is(%v, %v) = true, want false",
					core.ErrProcessStart,
					core.ErrProcessWait,
				)
			}
			if errors.Is(core.ErrProcessStream, core.ErrProcessOutputLimit) {
				t.Errorf(
					"errors.Is(%v, %v) = true, want false",
					core.ErrProcessStream,
					core.ErrProcessOutputLimit,
				)
			}
			if gotText := tc.identity.Error(); gotText == "" {
				t.Errorf("identity text = %q, want a non-empty diagnostic", gotText)
			}
		})
	}
}

func environmentVariable(
	tb testing.TB,
	name string,
	value string,
) process.EnvironmentVariable {
	tb.Helper()

	parsedName, err := process.NewEnvironmentName(name)
	if err != nil {
		tb.Fatalf("process.NewEnvironmentName(%q) error = %v, want nil", name, err)
	}
	parsedValue, err := process.NewEnvironmentValue(value)
	if err != nil {
		tb.Fatalf("process.NewEnvironmentValue(%q) error = %v, want nil", value, err)
	}
	variable := process.EnvironmentVariable{Name: parsedName, Value: parsedValue}
	if err := variable.Validate(); err != nil {
		tb.Fatalf("EnvironmentVariable.Validate() error = %v, want nil", err)
	}
	return variable
}
