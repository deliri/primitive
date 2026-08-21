package lease_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// payloadJSONContract drives one bounded strict payload decoder through the
// same hostile grammar. Every payload in this package decodes through the same
// Core strict-JSON gate, so one shared executor keeps the pressure identical
// and makes a payload that quietly opts out of a rule visible as a diff.
type payloadJSONContract[T comparable] struct {
	decode    func(*T, []byte) error
	name      string
	seed      T
	canonical T
	fields    []string
	maximum   int
}

func TestSubjectStrictJSONPressure(t *testing.T) {
	t.Parallel()

	runPayloadJSONPressure(t, payloadJSONContract[lease.Subject]{
		name:      "subject",
		seed:      fixtureSubject(t, 31),
		canonical: fixtureSubject(t, 33),
		fields:    []string{"offering", "entitlement_id", "device_id"},
		maximum:   lease.SubjectJSONMaximumBytes,
		decode: func(value *lease.Subject, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
}

func TestGrantStrictJSONPressure(t *testing.T) {
	t.Parallel()

	other := fixtureGrant()
	other.GoodUntil = fixtureInstant(6_000)
	runPayloadJSONPressure(t, payloadJSONContract[lease.Grant]{
		name:      "grant",
		seed:      fixtureGrant(),
		canonical: other,
		fields:    []string{"not_before", "contact_after", "not_after", "good_until"},
		maximum:   lease.GrantJSONMaximumBytes,
		decode: func(value *lease.Grant, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
}

func TestRefusalStrictJSONPressure(t *testing.T) {
	t.Parallel()

	runPayloadJSONPressure(t, payloadJSONContract[lease.Refusal]{
		name: "refusal",
		seed: lease.Refusal{
			ContactAfter: fixtureInstant(6_000),
		},
		canonical: lease.Refusal{
			ContactAfter: fixtureInstant(7_000),
		},
		fields:  []string{"contact_after"},
		maximum: lease.RefusalJSONMaximumBytes,
		decode: func(value *lease.Refusal, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
}

func TestRevocationStrictJSONPressure(t *testing.T) {
	t.Parallel()

	runPayloadJSONPressure(t, payloadJSONContract[lease.Revocation]{
		name:      "revocation",
		seed:      lease.Revocation{Reason: lease.RevocationReasonLicenceBreach},
		canonical: lease.Revocation{Reason: lease.RevocationReasonInsolvency},
		fields:    []string{"reason"},
		maximum:   lease.RevocationJSONMaximumBytes,
		decode: func(value *lease.Revocation, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
}

func runPayloadJSONPressure[T comparable](
	t *testing.T,
	contract payloadJSONContract[T],
) {
	t.Helper()

	canonical, err := json.Marshal(contract.canonical)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v, want nil", contract.name, err)
	}
	cases := payloadJSONCases(t, contract, canonical)
	for _, tc := range cases {
		t.Run(contract.name+"/"+tc.name, func(t *testing.T) {
			t.Parallel()

			got := contract.seed
			err := contract.decode(&got, tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("%s UnmarshalJSON() error = %v, want %v", contract.name, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != contract.seed {
					t.Fatalf("rejected %s mutated its receiver", contract.name)
				}
				return
			}
			if got != contract.canonical {
				t.Fatalf("decoded %s = %v, want %v", contract.name, got, contract.canonical)
			}
			reencoded, marshalErr := json.Marshal(got)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(%s) error = %v, want nil", contract.name, marshalErr)
			}
			if !bytes.Equal(reencoded, canonical) {
				t.Fatalf("%s re-encoding = %s, want %s", contract.name, reencoded, canonical)
			}
		})
	}
}

type payloadJSONCase struct {
	wantErr error
	name    string
	data    []byte
}

func payloadJSONCases[T comparable](
	t *testing.T,
	contract payloadJSONContract[T],
	canonical []byte,
) []payloadJSONCase {
	t.Helper()

	cases := []payloadJSONCase{
		{name: "canonical bytes decode to the exact value", data: canonical},
		{
			name: "surrounding whitespace is equivalent",
			data: append(append([]byte(" \n\t"), canonical...), ' ', '\n'),
		},
		{name: "empty input", wantErr: core.ErrJSONContract},
		{name: "null document", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "array document", data: []byte("[]"), wantErr: core.ErrJSONContract},
		{name: "string document", data: []byte(`"x"`), wantErr: core.ErrJSONContract},
		{name: "number document", data: []byte("1"), wantErr: core.ErrJSONContract},
		{name: "empty object omits every field", data: []byte("{}"), wantErr: core.ErrLeaseContract},
		{
			name:    "truncated final byte",
			data:    canonical[:len(canonical)-1],
			wantErr: core.ErrJSONContract,
		},
		{
			name:    "trailing value after the object",
			data:    append(append([]byte(nil), canonical...), []byte("true")...),
			wantErr: core.ErrJSONContract,
		},
		{
			name:    "unknown extra field",
			data:    appendObjectField(canonical, `"future_field":1`),
			wantErr: core.ErrJSONContract,
		},
		{
			name:    "invalid utf8 byte",
			data:    []byte{0xff},
			wantErr: core.ErrJSONContract,
		},
		{
			name:    "one byte over the document bound",
			data:    append(make([]byte, contract.maximum+1), canonical...),
			wantErr: core.ErrJSONContract,
		},
	}
	for _, field := range contract.fields {
		cases = append(cases,
			payloadJSONCase{
				name:    "duplicate " + field + " field",
				data:    appendObjectField(canonical, duplicatedField(t, canonical, field)),
				wantErr: core.ErrJSONContract,
			},
			payloadJSONCase{
				name:    "case insensitive duplicate " + field + " field",
				data:    appendObjectField(canonical, caseVariantField(t, canonical, field)),
				wantErr: core.ErrJSONContract,
			},
			// A lone alternate casing is rejected. Go's decoder matches object
			// keys case-insensitively, so without this rule `Contact_After`
			// would silently populate `contact_after`. Core owns one grammar
			// for every consumer and cannot know whether a caller re-encodes
			// what it decoded, so the exact declared casing is required at the
			// wire boundary rather than trusted to each caller's projection.
			payloadJSONCase{
				name:    "lone case variant " + field + " field is rejected",
				data:    appendObjectField(removeField(t, canonical, field), caseVariantField(t, canonical, field)),
				wantErr: core.ErrJSONContract,
			},
			payloadJSONCase{
				name:    "missing " + field + " field",
				data:    removeField(t, canonical, field),
				wantErr: core.ErrLeaseContract,
			},
			payloadJSONCase{
				name:    "type wrong " + field + " field",
				data:    appendObjectField(removeField(t, canonical, field), `"`+field+`":[]`),
				wantErr: core.ErrJSONContract,
			},
		)
	}
	return cases
}

// objectFields splits one flat canonical JSON object into its raw
// `"name":value` members. Every lease payload object is flat and its member
// values contain no unescaped braces or commas, so a structural split over the
// canonical encoder's own output is exact for these payloads.
func objectFields(t *testing.T, canonical []byte) map[string]string {
	t.Helper()

	var members map[string]jsontext.Value
	if err := json.Unmarshal(canonical, &members); err != nil {
		t.Fatalf("json.Unmarshal(canonical object) error = %v, want nil", err)
	}
	fields := make(map[string]string, len(members))
	for name, value := range members {
		encodedName, err := json.Marshal(name)
		if err != nil {
			t.Fatalf("json.Marshal(field name) error = %v, want nil", err)
		}
		fields[name] = string(encodedName) + ":" + string(value)
	}
	return fields
}

func duplicatedField(t *testing.T, canonical []byte, field string) string {
	t.Helper()

	member, ok := objectFields(t, canonical)[field]
	if !ok {
		t.Fatalf("canonical object has no %q field", field)
	}
	return member
}

func caseVariantField(t *testing.T, canonical []byte, field string) string {
	t.Helper()

	member := duplicatedField(t, canonical, field)
	upper := []byte(member)
	if upper[1] >= 'a' && upper[1] <= 'z' {
		upper[1] -= 'a' - 'A'
	}
	return string(upper)
}

func removeField(t *testing.T, canonical []byte, field string) []byte {
	t.Helper()

	fields := objectFields(t, canonical)
	if _, ok := fields[field]; !ok {
		t.Fatalf("canonical object has no %q field", field)
	}
	delete(fields, field)
	result := []byte("{")
	for _, member := range fields {
		if len(result) > 1 {
			result = append(result, ',')
		}
		result = append(result, member...)
	}
	return append(result, '}')
}

func appendObjectField(object []byte, field string) []byte {
	if len(object) < 2 || object[len(object)-1] != '}' {
		return nil
	}
	result := make([]byte, 0, len(object)+len(field)+1)
	result = append(result, object[:len(object)-1]...)
	if len(object) > 2 {
		result = append(result, ',')
	}
	result = append(result, field...)
	return append(result, '}')
}

// TestPayloadUnmarshalRejectsNilReceivers proves every bounded payload decoder
// refuses a nil receiver with a typed contract error instead of panicking, so
// a reflective or generic caller cannot turn a wire boundary into a crash.
func TestPayloadUnmarshalRejectsNilReceivers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		decode func([]byte) error
		name   string
	}{
		{name: "entitlement id", decode: func(data []byte) error { return (*lease.EntitlementID)(nil).UnmarshalJSON(data) }},
		{name: "device id", decode: func(data []byte) error { return (*lease.DeviceID)(nil).UnmarshalJSON(data) }},
		{name: "generation", decode: func(data []byte) error { return (*lease.Generation)(nil).UnmarshalJSON(data) }},
		{name: "revision", decode: func(data []byte) error { return (*lease.Revision)(nil).UnmarshalJSON(data) }},
		{name: "outcome", decode: func(data []byte) error { return (*lease.Outcome)(nil).UnmarshalJSON(data) }},
		{name: "revocation reason", decode: func(data []byte) error { return (*lease.RevocationReason)(nil).UnmarshalJSON(data) }},
		{name: "subject", decode: func(data []byte) error { return (*lease.Subject)(nil).UnmarshalJSON(data) }},
		{name: "grant", decode: func(data []byte) error { return (*lease.Grant)(nil).UnmarshalJSON(data) }},
		{name: "refusal", decode: func(data []byte) error { return (*lease.Refusal)(nil).UnmarshalJSON(data) }},
		{name: "revocation", decode: func(data []byte) error { return (*lease.Revocation)(nil).UnmarshalJSON(data) }},
		{name: "decision", decode: func(data []byte) error { return (*lease.Decision)(nil).UnmarshalJSON(data) }},
		{name: "document", decode: func(data []byte) error { return (*lease.Document)(nil).UnmarshalJSON(data) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.decode([]byte("{}"))
			if !errors.Is(err, core.ErrLeaseContract) ||
				!errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("%s nil receiver error = %v, want lease and JSON contract identities", tc.name, err)
			}
		})
	}
}

// TestZeroPayloadsRefuseToMarshal proves no unset payload can reach a wire or
// a signature. A zero value that encoded successfully would let an unset
// timeline or an unknown reason be signed as if OGS had chosen it.
func TestZeroPayloadsRefuseToMarshal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		marshal func() ([]byte, error)
		name    string
	}{
		{name: "entitlement id", marshal: lease.EntitlementID{}.MarshalJSON},
		{name: "device id", marshal: lease.DeviceID{}.MarshalJSON},
		{name: "generation", marshal: lease.Generation{}.MarshalJSON},
		{name: "revision", marshal: lease.Revision(0).MarshalJSON},
		{name: "outcome", marshal: lease.Outcome(0).MarshalJSON},
		{name: "revocation reason", marshal: lease.RevocationReason(0).MarshalJSON},
		{name: "subject", marshal: lease.Subject{}.MarshalJSON},
		{name: "grant", marshal: lease.Grant{}.MarshalJSON},
		{name: "refusal", marshal: lease.Refusal{}.MarshalJSON},
		{name: "revocation", marshal: lease.Revocation{}.MarshalJSON},
		{name: "decision", marshal: lease.Decision{}.MarshalJSON},
		{name: "document", marshal: lease.Document{}.MarshalJSON},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.marshal()
			if !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("zero %s MarshalJSON() error = %v, want %v", tc.name, err, core.ErrLeaseContract)
			}
			if got != nil {
				t.Fatalf("zero %s MarshalJSON() = %s, want no bytes", tc.name, got)
			}
		})
	}
}
