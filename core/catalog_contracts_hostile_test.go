package core

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"strconv"
	"testing"
)

func TestCatalogPageLimitHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   uint16
	}{
		{name: "zero is below the positive domain", value: 0, wantErr: ErrPrimitiveContract},
		{name: "one is the narrowest page", value: 1},
		{name: "shared maximum is admitted", value: CatalogPageMaximumEntries},
		{name: "one above shared maximum is refused", value: CatalogPageMaximumEntries + 1, wantErr: ErrPrimitiveContract},
		{name: "uint16 maximum is refused", value: math.MaxUint16, wantErr: ErrPrimitiveContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewCatalogPageLimit(testCase.value)
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) || got != (CatalogPageLimit{}) {
					t.Fatalf("NewCatalogPageLimit(%d) = (%v, %v), want zero and errors.Is %v",
						testCase.value, got, gotErr, ErrPrimitiveContract)
				}
				return
			}
			if gotErr != nil || got.Uint16() != testCase.value {
				t.Fatalf("NewCatalogPageLimit(%d) = (%d, %v), want (%d, nil)",
					testCase.value, got.Uint16(), gotErr, testCase.value)
			}
		})
	}
}

func TestCatalogEnumsExhaustivelyCloseEveryUint8Value(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		selection := CatalogSelectionKind(raw)
		wantSelection := selection == CatalogSelectionAll || selection == CatalogSelectionSpecific
		proveCatalogEnumValue(t, selection, wantSelection)

		position := CatalogPositionKind(raw)
		wantPosition := position == CatalogPositionStart || position == CatalogPositionAfter
		proveCatalogEnumValue(t, position, wantPosition)

		continuation := CatalogContinuationState(raw)
		wantContinuation := continuation == CatalogContinuationEnd || continuation == CatalogContinuationMore
		proveCatalogEnumValue(t, continuation, wantContinuation)
	}
}

type catalogJSONEnum[T any] interface {
	comparable
	Validate() error
	String() string
	MarshalJSON() ([]byte, error)
}

func proveCatalogEnumValue[T catalogJSONEnum[T]](t *testing.T, value T, wantValid bool) {
	t.Helper()

	gotErr := value.Validate()
	encoded, marshalErr := value.MarshalJSON()
	if !wantValid {
		if !errors.Is(gotErr, ErrPrimitiveContract) || !errors.Is(marshalErr, ErrJSONContract) || encoded != nil {
			t.Fatalf("catalog enum %T(%v) = validate %v, marshal (%q, %v), want contract/JSON refusal",
				value, value, gotErr, encoded, marshalErr)
		}
		return
	}
	if gotErr != nil || marshalErr != nil || value.String() == "" {
		t.Fatalf("catalog enum %T(%v) = validate %v, text %q, marshal %v, want admitted",
			value, value, gotErr, value.String(), marshalErr)
	}
}

func TestCatalogPageLimitJSONRefusesNonCanonicalFormsWithoutMutation(t *testing.T) {
	t.Parallel()

	before, err := NewCatalogPageLimit(7)
	if err != nil {
		t.Fatalf("NewCatalogPageLimit(7) error = %v, want nil", err)
	}
	canonical, err := json.Marshal(before)
	if err != nil {
		t.Fatalf("json.Marshal(CatalogPageLimit) error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "leading zero", data: []byte{'0', canonical[0]}},
		{name: "leading whitespace", data: append([]byte{' '}, canonical...)},
		{name: "trailing whitespace", data: append(append([]byte(nil), canonical...), ' ')},
		{name: "negative", data: []byte{'-', '1'}},
		{name: "fraction", data: []byte{'1', '.', '0'}},
		{name: "string", data: []byte{'"', canonical[0], '"'}},
		{
			name: "one above maximum",
			data: strconv.AppendUint(nil, CatalogPageMaximumEntries+1, 10),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := before
			gotErr := got.UnmarshalJSON(testCase.data)
			if !errors.Is(gotErr, ErrJSONContract) || got != before {
				t.Fatalf("CatalogPageLimit.UnmarshalJSON(%q) = (%v, %v), want preserved %v and errors.Is %v",
					testCase.data, got, gotErr, before, ErrJSONContract)
			}
		})
	}
}
