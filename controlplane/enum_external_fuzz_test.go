package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

type enumExternalDoor[T comparable] struct {
	Parse     func(string) (T, error)
	Validate  func(T) error
	String    func(T) string
	Marshal   func(T) ([]byte, error)
	Unmarshal func(*T, []byte) error
	Values    []T
	WantError core.ErrorIdentity
}

func FuzzSigningDomainExternalDecoders(f *testing.F) {
	fuzzEnumExternalDoor(f, enumExternalDoor[controlplane.SigningDomain]{
		Values: fuzzValidSigningDomains(), Parse: controlplane.ParseSigningDomain,
		Validate: func(value controlplane.SigningDomain) error { return value.Validate() },
		String:   func(value controlplane.SigningDomain) string { return value.String() },
		Marshal:  func(value controlplane.SigningDomain) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.SigningDomain, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		WantError: core.ErrControlPlaneSigningDomain,
	})
}

func FuzzProductStatusExternalDecoders(f *testing.F) {
	fuzzEnumExternalDoor(f, enumExternalDoor[controlplane.ProductStatus]{
		Values: fuzzValidProductStatuses(), Parse: controlplane.ParseProductStatus,
		Validate: func(value controlplane.ProductStatus) error { return value.Validate() },
		String:   func(value controlplane.ProductStatus) string { return value.String() },
		Marshal:  func(value controlplane.ProductStatus) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.ProductStatus, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		WantError: core.ErrControlPlaneProductStatus,
	})
}

func FuzzResponseHeaderFieldExternalDecoders(f *testing.F) {
	fuzzEnumExternalDoor(f, enumExternalDoor[controlplane.ResponseHeaderField]{
		Values: fuzzValidResponseHeaderFields(), Parse: controlplane.ParseResponseHeaderField,
		Validate: func(value controlplane.ResponseHeaderField) error { return value.Validate() },
		String:   func(value controlplane.ResponseHeaderField) string { return value.String() },
		Marshal:  func(value controlplane.ResponseHeaderField) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.ResponseHeaderField, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		WantError: core.ErrControlPlaneResponseHeader,
	})
}

func FuzzUsageDispositionExternalDecoders(f *testing.F) {
	fuzzEnumExternalDoor(f, enumExternalDoor[controlplane.UsageDisposition]{
		Values: fuzzValidUsageDispositions(), Parse: controlplane.ParseUsageDisposition,
		Validate: func(value controlplane.UsageDisposition) error { return value.Validate() },
		String:   func(value controlplane.UsageDisposition) string { return value.String() },
		Marshal:  func(value controlplane.UsageDisposition) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.UsageDisposition, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		WantError: core.ErrControlPlaneCheckInResponse,
	})
}

func FuzzUsageClassExternalDecoders(f *testing.F) {
	fuzzEnumExternalDoor(f, enumExternalDoor[controlplane.UsageClass]{
		Values:   fuzzValidUsageClasses(),
		Validate: func(value controlplane.UsageClass) error { return value.Validate() },
		String:   func(value controlplane.UsageClass) string { return value.String() },
		Marshal:  func(value controlplane.UsageClass) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.UsageClass, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		WantError: core.ErrControlPlaneUsageWindow,
	})
}

func FuzzOutcomeClassExternalDecoders(f *testing.F) {
	fuzzEnumExternalDoor(f, enumExternalDoor[controlplane.OutcomeClass]{
		Values:   fuzzValidOutcomeClasses(),
		Validate: func(value controlplane.OutcomeClass) error { return value.Validate() },
		String:   func(value controlplane.OutcomeClass) string { return value.String() },
		Marshal:  func(value controlplane.OutcomeClass) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.OutcomeClass, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		WantError: core.ErrControlPlaneUsageWindow,
	})
}

func fuzzEnumExternalDoor[T comparable](f *testing.F, door enumExternalDoor[T]) {
	f.Helper()
	if len(door.Values) == 0 {
		f.Fatalf("external enum valid values = 0, want a non-empty closed domain")
	}
	for _, value := range door.Values {
		encoded, err := door.Marshal(value)
		if err != nil {
			f.Fatalf("external enum canonical seed MarshalJSON() error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	for _, data := range [][]byte{
		nil, {}, []byte("null"), []byte(`""`), []byte("0"), []byte("true"),
		[]byte(`"unknown"`), bytes.Repeat([]byte{'x'}, 1<<10),
	} {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if door.Parse != nil {
			requireEnumParseOracle(t, door, string(data))
		}

		before := door.Values[0]
		candidate := before
		decodeErr := door.Unmarshal(&candidate, data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) ||
				!errors.Is(decodeErr, core.ErrControlPlaneContract) ||
				!errors.Is(decodeErr, door.WantError) || candidate != before {
				t.Fatalf(
					"external enum UnmarshalJSON(%q) = (%v, %v), want preserved %v and %v/%v/%v",
					data, candidate, decodeErr, before, core.ErrJSONContract,
					core.ErrControlPlaneContract, door.WantError,
				)
			}
			return
		}
		requireEnumAcceptedClosure(t, door, candidate)
	})
}

func requireEnumParseOracle[T comparable](t *testing.T, door enumExternalDoor[T], text string) {
	t.Helper()
	want, present := enumForText(door, text)
	got, err := door.Parse(text)
	if !present {
		var zero T
		if !errors.Is(err, core.ErrControlPlaneContract) || !errors.Is(err, door.WantError) || got != zero {
			t.Fatalf("external enum Parse(%q) = (%v, %v), want zero and %v/%v", text, got, err, core.ErrControlPlaneContract, door.WantError)
		}
		return
	}
	if err != nil || got != want {
		t.Fatalf("external enum Parse(%q) = (%v, %v), want (%v, nil)", text, got, err, want)
	}
}

func requireEnumAcceptedClosure[T comparable](t *testing.T, door enumExternalDoor[T], candidate T) {
	t.Helper()
	if err := door.Validate(candidate); err != nil {
		t.Fatalf("accepted external enum Validate() error = %v, want nil", err)
	}
	want, present := enumForText(door, door.String(candidate))
	if !present || want != candidate {
		t.Fatalf("accepted external enum token %q resolves to (%v, %t), want (%v, true)", door.String(candidate), want, present, candidate)
	}
	encoded, marshalErr := door.Marshal(candidate)
	var roundTrip T
	decodeErr := door.Unmarshal(&roundTrip, encoded)
	second, secondErr := door.Marshal(roundTrip)
	if marshalErr != nil || decodeErr != nil || secondErr != nil || roundTrip != candidate || !bytes.Equal(second, encoded) {
		t.Fatalf(
			"accepted external enum canonical closure = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
			roundTrip, second, marshalErr, decodeErr, secondErr, candidate, encoded,
		)
	}
}

func enumForText[T comparable](door enumExternalDoor[T], text string) (T, bool) {
	for _, value := range door.Values {
		if door.String(value) == text {
			return value, true
		}
	}
	var zero T
	return zero, false
}

func fuzzValidSigningDomains() []controlplane.SigningDomain {
	values := make([]controlplane.SigningDomain, 0, 4)
	for raw := 0; raw <= 255; raw++ {
		value := controlplane.SigningDomain(raw)
		if value.Validate() == nil {
			values = append(values, value)
		}
	}
	return values
}

func fuzzValidProductStatuses() []controlplane.ProductStatus {
	values := make([]controlplane.ProductStatus, 0, 6)
	for raw := 0; raw <= 255; raw++ {
		value := controlplane.ProductStatus(raw)
		if value.Validate() == nil {
			values = append(values, value)
		}
	}
	return values
}

func fuzzValidResponseHeaderFields() []controlplane.ResponseHeaderField {
	values := make([]controlplane.ResponseHeaderField, 0, 5)
	for raw := 0; raw <= 255; raw++ {
		value := controlplane.ResponseHeaderField(raw)
		if value.Validate() == nil {
			values = append(values, value)
		}
	}
	return values
}

func fuzzValidUsageDispositions() []controlplane.UsageDisposition {
	values := make([]controlplane.UsageDisposition, 0, 3)
	for raw := 0; raw <= 255; raw++ {
		value := controlplane.UsageDisposition(raw)
		if value.Validate() == nil {
			values = append(values, value)
		}
	}
	return values
}

func fuzzValidUsageClasses() []controlplane.UsageClass {
	values := make([]controlplane.UsageClass, 0, 15)
	for raw := 0; raw <= 255; raw++ {
		value := controlplane.UsageClass(raw)
		if value.Validate() == nil {
			values = append(values, value)
		}
	}
	return values
}

func fuzzValidOutcomeClasses() []controlplane.OutcomeClass {
	values := make([]controlplane.OutcomeClass, 0, 15)
	for raw := 0; raw <= 255; raw++ {
		value := controlplane.OutcomeClass(raw)
		if value.Validate() == nil {
			values = append(values, value)
		}
	}
	return values
}
