package timeproof

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSignedAttributeScanLayerTriad(t *testing.T) {
	t.Parallel()
	value := rawValueFromDER(t, encodeSequence())
	known := cmsAttribute{Type: oidContentType(), Values: []asn1.RawValue{value}}
	foreign := cmsAttribute{Type: asn1.ObjectIdentifier{1, 2, 3, 99}, Values: []asn1.RawValue{value}}
	many := make([]cmsAttribute, 4096)
	for i := range many {
		many[i] = foreign
	}
	many[len(many)-1] = known
	cases := []struct {
		wantErr    error
		name       string
		attributes []cmsAttribute
		wantCount  int
	}{
		{name: "required attribute retains exact value", attributes: []cmsAttribute{known}, wantCount: 1},
		{name: "foreign attribute supplies no required fact", attributes: []cmsAttribute{foreign}},
		{name: "required attribute after former count ceiling", attributes: many, wantCount: 1},
		{name: "duplicate required identity cannot select a winner", attributes: []cmsAttribute{known, foreign, known}, wantCount: 2},
		{name: "missing required value refuses", attributes: []cmsAttribute{{Type: oidContentType()}}, wantErr: core.ErrTimeProofInvalid},
		{name: "multiple required values refuse", attributes: []cmsAttribute{{Type: oidContentType(), Values: []asn1.RawValue{value, value}}}, wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := encodedSignedAttributes(t, tc.attributes)
			got, err := parseSignedAttributes(input)
			if err != nil || len(got.Bytes) == 0 || &got.Bytes[0] != &input.Bytes[0] {
				t.Fatalf("parseSignedAttributes() = (%d bytes, %v), want borrowed nonempty input", len(got.Bytes), err)
			}
			attribute, count, err := countAttribute(got, oidContentType())
			if !errors.Is(err, tc.wantErr) || count != tc.wantCount {
				t.Fatalf("countAttribute() = (%d, %v), want (%d, %v)", count, err, tc.wantCount, tc.wantErr)
			}
			if count == 1 && (len(attribute.Values) != 1 || !bytes.Equal(attribute.Values[0].FullBytes, value.FullBytes)) {
				t.Fatalf("retained attribute = %+v, want exact canonical sequence value", attribute)
			}
			if count != 1 && (attribute.Type != nil || attribute.Values != nil) {
				t.Fatalf("nonunique attribute = %+v, want zero", attribute)
			}
		})
	}
}

func TestSignedAttributeScanRejectsMalformedTail(t *testing.T) {
	t.Parallel()
	typed := encodedSignedAttributes(t, []cmsAttribute{{Type: oidContentType(), Values: []asn1.RawValue{rawValueFromDER(t, encodeSequence())}}})
	input := typed
	input.Bytes = append(bytes.Clone(typed.Bytes), 0x30)
	got, err := parseSignedAttributes(input)
	if !errors.Is(err, core.ErrTimeProofInvalid) || got.Bytes != nil || got.FullBytes != nil {
		t.Fatalf("parseSignedAttributes(truncated tail) = (%+v, %v), want zero and %v", got, err, core.ErrTimeProofInvalid)
	}
}

func FuzzSignedAttributeScanSemanticClosure(f *testing.F) {
	known := cmsAttribute{Type: oidContentType(), Values: []asn1.RawValue{rawValueFromDER(f, encodeSequence())}}
	seed := encodedSignedAttributes(f, []cmsAttribute{known})
	f.Add(seed.Bytes)
	f.Add([]byte{})
	f.Add([]byte{0x30})
	f.Fuzz(func(t *testing.T, data []byte) {
		raw := asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: data}
		got, err := parseSignedAttributes(raw)
		if err != nil {
			if !errors.Is(err, core.ErrTimeProofInvalid) || got.Bytes != nil {
				t.Fatalf("parseSignedAttributes() = (%+v, %v), want zero and typed rejection", got, err)
			}
			return
		}
		if len(data) == 0 || &got.Bytes[0] != &data[0] || !bytes.Equal(got.Bytes, data) {
			t.Fatalf("admitted attribute span = %d bytes, want exact nonempty borrowed input", len(got.Bytes))
		}
		// Decode framing with a separate standard-library struct representation.
		// Raw OIDs avoid imposing encoding/asn1's int-backed arc width.
		wantOID, marshalErr := asn1.Marshal(oidContentType())
		if marshalErr != nil {
			t.Fatalf("Marshal(content type OID) error = %v, want nil", marshalErr)
		}
		count := 0
		firstHasOneValue := false
		for fields := data; len(fields) != 0; {
			var attribute struct {
				Type   asn1.RawValue
				Values asn1.RawValue
			}
			rest, decodeErr := asn1.Unmarshal(fields, &attribute)
			if decodeErr != nil {
				t.Fatalf("accepted attribute framing error = %v, want nil", decodeErr)
			}
			var oid x509.OID
			if oidErr := oid.UnmarshalBinary(attribute.Type.Bytes); oidErr != nil {
				t.Fatalf("accepted attribute OID error = %v, want nil", oidErr)
			}
			if bytes.Equal(attribute.Type.FullBytes, wantOID) {
				count++
				if count == 1 {
					var value asn1.RawValue
					trailing, valueErr := asn1.Unmarshal(attribute.Values.Bytes, &value)
					firstHasOneValue = valueErr == nil && len(trailing) == 0
				}
			}
			fields = rest
		}
		_, gotCount, lookupErr := countAttribute(got, oidContentType())
		wantRefusal := count > 0 && !firstHasOneValue
		if errors.Is(lookupErr, core.ErrTimeProofInvalid) != wantRefusal {
			t.Fatalf("attribute lookup error = %v, want refusal %t", lookupErr, wantRefusal)
		}
		if lookupErr == nil && gotCount != min(count, 2) {
			t.Fatalf("required attribute count = %d, want %d", gotCount, min(count, 2))
		}

	})
}
