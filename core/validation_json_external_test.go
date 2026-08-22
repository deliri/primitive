package core_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type externalValidatedRecord struct {
	A uint64 `json:"a"`
	B uint64 `json:"b"`
}

type externalPanicUnmarshalRecord struct{}

type externalPanicValidateRecord struct {
	Value string `json:"value"`
}

func (r externalValidatedRecord) Validate() error {
	if r.A == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (externalPanicUnmarshalRecord) Validate() error {
	return nil
}

func (*externalPanicUnmarshalRecord) UnmarshalJSON([]byte) error {
	panic("consumer unmarshal panic")
}

func (externalPanicValidateRecord) Validate() error {
	panic("consumer validate panic")
}

func TestStrictJSONExternalConsumerPanicsBecomeTypedErrors(t *testing.T) {
	t.Parallel()

	limits := core.DefaultStrictJSONLimits()
	t.Run("consumer UnmarshalJSON panic returns the zero value", func(t *testing.T) {
		t.Parallel()

		got, gotErr := core.DecodeStrictJSON[externalPanicUnmarshalRecord](
			bytes.NewReader([]byte(`{}`)),
			limits,
		)
		if !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("DecodeStrictJSON(panicking unmarshaler) error = %v, want %v", gotErr, core.ErrJSONContract)
		}
		if got != (externalPanicUnmarshalRecord{}) {
			t.Fatalf("DecodeStrictJSON(panicking unmarshaler) value = %v, want zero", got)
		}
	})
	t.Run("consumer Validate panic returns the zero value", func(t *testing.T) {
		t.Parallel()

		got, gotErr := core.DecodeStrictJSON[externalPanicValidateRecord](
			bytes.NewReader([]byte(`{"value":"decoded"}`)),
			limits,
		)
		if !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("DecodeStrictJSON(panicking validator) error = %v, want %v", gotErr, core.ErrJSONContract)
		}
		if got != (externalPanicValidateRecord{}) {
			t.Fatalf("DecodeStrictJSON(panicking validator) value = %v, want zero", got)
		}
	})
}

func TestStrictJSONExternalTypedNilNeverPanics(t *testing.T) {
	t.Parallel()

	limits := core.DefaultStrictJSONLimits()
	var nilDigest *core.SHA256Digest
	if _, gotErr := core.EncodeValidatedJSON(nilDigest, limits); !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("EncodeValidatedJSON(nil *SHA256Digest) error = %v, want %v", gotErr, core.ErrJSONContract)
	}
	nullCases := []struct {
		name string
		wire []byte
	}{
		{name: "exact null literal reaches typed nil pointer path", wire: []byte("null")},
		{name: "leading space around null reaches typed nil pointer path", wire: []byte(" null")},
		{name: "trailing space around null reaches typed nil pointer path", wire: []byte("null ")},
		{name: "newline around null reaches typed nil pointer path", wire: []byte("\nnull\n")},
		{name: "tab around null reaches typed nil pointer path", wire: []byte("\tnull\t")},
	}
	for _, tc := range nullCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := core.DecodeStrictJSON[*core.SHA256Digest](bytes.NewReader(tc.wire), limits)
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("DecodeStrictJSON[*SHA256Digest](%q) error = %v, want %v", tc.wire, gotErr, core.ErrJSONContract)
			}
			if got != nil {
				t.Fatalf("DecodeStrictJSON[*SHA256Digest](%q) value = %v, want nil", tc.wire, got)
			}
		})
	}
}

func TestStrictJSONExternalErrorsAlwaysReturnZeroValue(t *testing.T) {
	t.Parallel()

	limits := core.DefaultStrictJSONLimits()
	cases := []struct {
		name string
		wire []byte
	}{
		{name: "type-wrong first field returns zero", wire: []byte(`{"a":"x","b":2}`)},
		{name: "type-wrong second field cannot leak first field", wire: []byte(`{"a":1,"b":"x"}`)},
		{name: "unknown first field returns zero", wire: []byte(`{"zz":2,"a":7}`)},
		{name: "unknown second field cannot leak first field", wire: []byte(`{"a":7,"zz":2}`)},
		{name: "trailing document cannot leak decoded record", wire: []byte(`{"a":9,"b":2}null`)},
		{name: "exact duplicate field returns zero", wire: []byte(`{"a":1,"a":2,"b":3}`)},
		{name: "case-folded duplicate field returns zero", wire: []byte(`{"a":1,"A":2,"b":3}`)},
		{name: "validation failure after complete decode returns zero", wire: []byte(`{"a":0,"b":2}`)},
		{name: "truncated object after first field returns zero", wire: []byte(`{"a":7,`)},
		{name: "empty document returns zero", wire: []byte{}},
		{name: "array instead of typed object returns zero", wire: []byte(`[1,2]`)},
		{name: "scalar instead of typed object returns zero", wire: []byte(`7`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := core.DecodeStrictJSON[externalValidatedRecord](bytes.NewReader(tc.wire), limits)
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("DecodeStrictJSON(%q) error = %v, want %v", tc.wire, gotErr, core.ErrJSONContract)
			}
			if got != (externalValidatedRecord{}) {
				t.Fatalf("DecodeStrictJSON(%q) value = %+v, want zero", tc.wire, got)
			}
		})
	}
}
