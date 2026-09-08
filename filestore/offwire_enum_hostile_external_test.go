package filestore_test

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// These are exhaustive backing-domain proofs, not 256 distinct quota cases.
// Each concrete type retains direct assertions and compiler-owned valid arms.

func TestInstallModeExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.InstallMode{filestore.InstallCreate, filestore.InstallReplace}
	cases := make([]struct {
		name      string
		value     filestore.InstallMode
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.InstallMode(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.InstallMode
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("InstallMode encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("InstallMode decodes JSON = %t, want false", got)
	}
}

func TestAppendModeExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.AppendMode{filestore.AppendCreate, filestore.AppendExisting, filestore.AppendCreateOrOpen}
	cases := make([]struct {
		name      string
		value     filestore.AppendMode
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.AppendMode(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.AppendMode
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("AppendMode encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("AppendMode decodes JSON = %t, want false", got)
	}
}

func TestWalkDirectiveExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.WalkDirective{filestore.WalkContinue, filestore.WalkSkipDirectory}
	cases := make([]struct {
		name      string
		value     filestore.WalkDirective
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.WalkDirective(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.WalkDirective
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("WalkDirective encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("WalkDirective decodes JSON = %t, want false", got)
	}
}

func TestWalkOrderExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.WalkOrder{filestore.WalkOrderNative, filestore.WalkOrderLexical}
	cases := make([]struct {
		name      string
		value     filestore.WalkOrder
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.WalkOrder(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.WalkOrder
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("WalkOrder encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("WalkOrder decodes JSON = %t, want false", got)
	}
}

func TestHeldStandingExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.HeldStanding{filestore.HeldStandingSame, filestore.HeldStandingReplaced, filestore.HeldStandingAbsent}
	cases := make([]struct {
		name      string
		value     filestore.HeldStanding
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.HeldStanding(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.HeldStanding
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("HeldStanding encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("HeldStanding decodes JSON = %t, want false", got)
	}
}

func TestSharingExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.Sharing{filestore.SharingAvailable, filestore.SharingHeld}
	cases := make([]struct {
		name      string
		value     filestore.Sharing
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.Sharing(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.Sharing
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("Sharing encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("Sharing decodes JSON = %t, want false", got)
	}
}

func TestPathKindExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	admitted := [...]filestore.PathKind{filestore.PathKindAbsent, filestore.PathKindDirectory, filestore.PathKindRegularFile, filestore.PathKindSymbolicLink, filestore.PathKindOther, filestore.PathKindUnreachable}
	cases := make([]struct {
		name      string
		value     filestore.PathKind
		wantValid bool
	}, math.MaxUint8+1)
	for raw := range cases {
		value := filestore.PathKind(raw)
		cases[raw].name = fmt.Sprintf("backing value %d cannot change domain membership", raw)
		cases[raw].value = value
		cases[raw].wantValid = slices.Contains(admitted[:], value)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value
			gotErr := got.Validate()
			if got != tc.value || got.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid {
				t.Fatalf("value/IsValid/Validate = (%v,%t,%v), want (%v,%t)", got, got.IsValid(), gotErr, tc.value, tc.wantValid)
			}
			if !tc.wantValid {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || got.String() != "" {
					t.Fatalf("refusal = (%v,%q), want pure Contract and unknown diagnostic", gotErr, got.String())
				}
				return
			}
			label := got.String()
			if label == "" || label == core.UnknownEnumDiagnostic {
				t.Fatalf("admitted label = %q, want nonempty named diagnostic", label)
			}
			for _, other := range admitted {
				if other != got && other.String() == label {
					t.Fatalf("distinct arms %d/%d share %q, want unique labels", got, other, label)
				}
			}
		})
	}
	var value filestore.PathKind
	var _ core.OffWireEnum = value
	if _, got := any(value).(json.Marshaler); got {
		t.Fatalf("PathKind encodes JSON = %t, want false", got)
	}
	if _, got := any(&value).(json.Unmarshaler); got {
		t.Fatalf("PathKind decodes JSON = %t, want false", got)
	}
}
