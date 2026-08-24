package taskmanager

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

type textBoundaryKind uint8

const (
	textBoundaryTitle textBoundaryKind = iota + 1
	textBoundaryDescription
)

type textBoundaryCase struct {
	wantErr error
	name    string
	value   string
	kind    textBoundaryKind
}

func TestTextParsersPressureEveryOwnedBoundary(t *testing.T) {
	t.Parallel()

	cases := []textBoundaryCase{
		{name: "title one-rune floor is accepted", value: "T", kind: textBoundaryTitle},
		{name: "title ordinary ASCII is accepted", value: "Build task manager", kind: textBoundaryTitle},
		{name: "title interior spaces are accepted", value: "Build the task manager socket", kind: textBoundaryTitle},
		{name: "title interior punctuation is accepted", value: "Task manager: client/server", kind: textBoundaryTitle},
		{name: "title Unicode is accepted", value: "修复 task manager", kind: textBoundaryTitle},
		{name: "title exact ASCII rune ceiling is accepted", value: strings.Repeat("t", TitleMaximumRunes), kind: textBoundaryTitle},
		{name: "title exact Unicode rune ceiling is accepted", value: strings.Repeat("界", TitleMaximumRunes), kind: textBoundaryTitle},
		{name: "title one below rune ceiling is accepted", value: strings.Repeat("t", TitleMaximumRunes-1), kind: textBoundaryTitle},
		{name: "title two below rune ceiling is accepted", value: strings.Repeat("界", TitleMaximumRunes-2), kind: textBoundaryTitle},
		{name: "title combining marks inside extent are accepted", value: "Cafe\u0301 task", kind: textBoundaryTitle},
		{name: "description empty optional value is accepted", value: "", kind: textBoundaryDescription},
		{name: "description one-rune floor is accepted", value: "D", kind: textBoundaryDescription},
		{name: "description ordinary ASCII is accepted", value: "Storage remains consumer-owned.", kind: textBoundaryDescription},
		{name: "description interior spaces are accepted", value: "Use direct IDs and bounded pages.", kind: textBoundaryDescription},
		{name: "description Unicode is accepted", value: "十年の履歴を一度に読み込まない", kind: textBoundaryDescription},
		{name: "description exact ASCII rune ceiling is accepted", value: strings.Repeat("d", DescriptionMaximumRunes), kind: textBoundaryDescription},
		{name: "description exact Unicode rune ceiling is accepted", value: strings.Repeat("界", DescriptionMaximumRunes), kind: textBoundaryDescription},
		{name: "description one below rune ceiling is accepted", value: strings.Repeat("d", DescriptionMaximumRunes-1), kind: textBoundaryDescription},
		{name: "description punctuation at both ends is accepted", value: "[proof] complete!", kind: textBoundaryDescription},
		{name: "description emoji inside extent is accepted", value: "Agent swarm proof 🐝", kind: textBoundaryDescription},

		{name: "title empty value is rejected", value: "", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title one space is rejected", value: " ", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title all spaces are rejected", value: "   ", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title leading space is rejected", value: " Task", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title trailing space is rejected", value: "Task ", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title leading tab is rejected", value: "\tTask", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title trailing tab is rejected", value: "Task\t", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title interior tab is rejected", value: "Task\tmanager", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title line feed is rejected", value: "Task\nmanager", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title carriage return is rejected", value: "Task\rmanager", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title nul is rejected", value: "Task\x00manager", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title delete control is rejected", value: "Task\x7fmanager", kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title one above ASCII rune ceiling is rejected", value: strings.Repeat("t", TitleMaximumRunes+1), kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title one above Unicode rune ceiling is rejected", value: strings.Repeat("界", TitleMaximumRunes+1), kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title two above rune ceiling is rejected", value: strings.Repeat("t", TitleMaximumRunes+2), kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "title invalid UTF-8 is rejected", value: string([]byte{utf8.RuneSelf, 0xff}), kind: textBoundaryTitle, wantErr: core.ErrTaskManagerContract},
		{name: "description one leading space is rejected", value: " Description", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description one trailing space is rejected", value: "Description ", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description line feed is rejected", value: "Description\nnext", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description carriage return is rejected", value: "Description\rnext", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description nul is rejected", value: "Description\x00next", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description interior tab is rejected", value: "Description\tnext", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description exact ceiling plus control is rejected", value: strings.Repeat("d", DescriptionMaximumRunes-1) + "\n", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description one above ASCII rune ceiling is rejected", value: strings.Repeat("d", DescriptionMaximumRunes+1), kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description one above Unicode rune ceiling is rejected", value: strings.Repeat("界", DescriptionMaximumRunes+1), kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
		{name: "description C1 control is rejected", value: "Description\u009fnext", kind: textBoundaryDescription, wantErr: core.ErrTaskManagerContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseTextBoundary(tc)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("text parser error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != "" {
				t.Fatalf("text parser rejected result = %q, want zero", got)
			}
			if tc.wantErr == nil && got != tc.value {
				t.Fatalf("text parser accepted result = %q, want exact %q", got, tc.value)
			}
		})
	}
}

func parseTextBoundary(tc textBoundaryCase) (string, error) {
	switch tc.kind {
	case textBoundaryTitle:
		got, err := ParseTitle(tc.value)
		if err != nil {
			return got.String(), err
		}
		return got.String(), got.Validate()
	case textBoundaryDescription:
		got, err := ParseDescription(tc.value)
		if err != nil {
			return got.String(), err
		}
		return got.String(), got.Validate()
	default:
		return "", contractError()
	}
}

func TestEnumDomainsExhaustEveryBackingByteAndJSONProjection(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		gotLifecycle := ProjectLifecycle(raw)
		wantLifecycleValid := raw == int(ProjectLifecycleActive) || raw == int(ProjectLifecycleCompleted)
		proveEnumByte(t, "ProjectLifecycle", raw, gotLifecycle.Validate(), wantLifecycleValid, gotLifecycle.MarshalJSON)

		gotOrder := PageOrder(raw)
		wantOrderValid := raw == int(PageOrderAscending) || raw == int(PageOrderDescending)
		proveEnumByte(t, "PageOrder", raw, gotOrder.Validate(), wantOrderValid, gotOrder.MarshalJSON)

		gotKind := TaskKind(raw)
		wantKindValid := raw >= int(TaskKindFeature) && raw <= int(TaskKindChore)
		proveEnumByte(t, "TaskKind", raw, gotKind.Validate(), wantKindValid, gotKind.MarshalJSON)

		gotState := TaskState(raw)
		wantStateValid := raw >= int(TaskStateBacklog) && raw <= int(TaskStateCancelled)
		proveEnumByte(t, "TaskState", raw, gotState.Validate(), wantStateValid, gotState.MarshalJSON)

		gotCollection := TaskCollection(raw)
		wantCollectionValid := raw == int(TaskCollectionActive) || raw == int(TaskCollectionCompleted)
		proveEnumByte(t, "TaskCollection", raw, gotCollection.Validate(), wantCollectionValid, gotCollection.MarshalJSON)

		gotEvidenceKind := EvidenceKind(raw)
		wantEvidenceKindValid := raw >= int(EvidenceKindNote) && raw <= int(EvidenceKindArtifact)
		proveEnumByte(t, "EvidenceKind", raw, gotEvidenceKind.Validate(), wantEvidenceKindValid, gotEvidenceKind.MarshalJSON)
	}
}

func proveEnumByte(t *testing.T, enum string, raw int, gotErr error, wantValid bool, marshal func() ([]byte, error)) {
	t.Helper()
	if wantValid && gotErr != nil {
		t.Fatalf("%s(%d).Validate() error = %v, want nil", enum, raw, gotErr)
	}
	if !wantValid && !errors.Is(gotErr, core.ErrTaskManagerContract) {
		t.Fatalf("%s(%d).Validate() error = %v, want %v", enum, raw, gotErr, core.ErrTaskManagerContract)
	}
	encoded, marshalErr := marshal()
	if wantValid && (marshalErr != nil || len(encoded) == 0) {
		t.Fatalf("%s(%d).MarshalJSON() = (%q, %v), want nonempty and nil", enum, raw, encoded, marshalErr)
	}
	if !wantValid && !errors.Is(marshalErr, core.ErrTaskManagerContract) {
		t.Fatalf("%s(%d).MarshalJSON() error = %v, want %v", enum, raw, marshalErr, core.ErrTaskManagerContract)
	}
}

func TestRevisionJSONRefusesEveryNoncanonicalBoundaryWithoutMutatingReceiver(t *testing.T) {
	t.Parallel()

	type revisionCase struct {
		wantErr error
		name    string
		wire    string
		want    uint64
	}
	cases := []revisionCase{
		{name: "one floor is accepted", wire: "1", want: 1},
		{name: "single digit is accepted", wire: "7", want: 7},
		{name: "two digits are accepted", wire: "42", want: 42},
		{name: "signed-int maximum is accepted", wire: "9223372036854775807", want: math.MaxInt64},
		{name: "one above signed-int maximum is accepted", wire: "9223372036854775808", want: uint64(math.MaxInt64) + 1},
		{name: "uint64 maximum is accepted", wire: "18446744073709551615", want: math.MaxUint64},
		{name: "zero is rejected", wire: "0", wantErr: core.ErrJSONContract},
		{name: "negative one is rejected", wire: "-1", wantErr: core.ErrJSONContract},
		{name: "leading plus is rejected", wire: "+1", wantErr: core.ErrJSONContract},
		{name: "leading zero is rejected", wire: "01", wantErr: core.ErrJSONContract},
		{name: "two leading zeros are rejected", wire: "001", wantErr: core.ErrJSONContract},
		{name: "trailing decimal point is rejected", wire: "1.", wantErr: core.ErrJSONContract},
		{name: "decimal fraction is rejected", wire: "1.0", wantErr: core.ErrJSONContract},
		{name: "exponent is rejected", wire: "1e0", wantErr: core.ErrJSONContract},
		{name: "quoted number is rejected", wire: `"1"`, wantErr: core.ErrJSONContract},
		{name: "null is rejected", wire: "null", wantErr: core.ErrJSONContract},
		{name: "empty token is rejected", wire: "", wantErr: core.ErrJSONContract},
		{name: "space-prefixed token is rejected", wire: " 1", wantErr: core.ErrJSONContract},
		{name: "space-suffixed token is rejected", wire: "1 ", wantErr: core.ErrJSONContract},
		{name: "uint64 overflow is rejected", wire: "18446744073709551616", wantErr: core.ErrJSONContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mustRevision(t, 99)
			before := got
			gotErr := got.UnmarshalJSON([]byte(tc.wire))
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Revision.UnmarshalJSON(%q) error = %v, want %v", tc.wire, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != before {
					t.Fatalf("Revision.UnmarshalJSON(%q) receiver = %v, want preserved %v", tc.wire, got, before)
				}
				return
			}
			gotValue, valueErr := got.Uint64()
			if valueErr != nil || gotValue != tc.want {
				t.Fatalf("Revision.UnmarshalJSON(%q) value = (%d, %v), want (%d, nil)", tc.wire, gotValue, valueErr, tc.want)
			}
			encoded, encodeErr := got.MarshalJSON()
			if encodeErr != nil || !bytes.Equal(encoded, []byte(tc.wire)) {
				t.Fatalf("Revision.MarshalJSON() = (%q, %v), want (%q, nil)", encoded, encodeErr, tc.wire)
			}
		})
	}
}

func mustRevision(t testing.TB, value uint64) Revision {
	t.Helper()
	parsed, err := NewRevision(value)
	if err != nil {
		t.Fatalf("NewRevision(%d) error = %v, want nil", value, err)
	}
	return parsed
}
