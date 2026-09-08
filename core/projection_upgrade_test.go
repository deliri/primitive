package core

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// The producer deliberately claims its projection is valid. Core must enforce
// its own structural boundary even when the product's semantic proof lies.
type untrustedIssueProjection struct {
	wire  []byte
	calls *int
}

func (untrustedIssueProjection) Validate() error                { return nil }
func (p untrustedIssueProjection) MarshalJSON() ([]byte, error) { return bytes.Clone(p.wire), nil }
func (p untrustedIssueProjection) ValidateJSONProjection([]byte, StrictJSONLimits) error {
	*p.calls++
	return nil
}

var _ ValidatedJSONProjection = untrustedIssueProjection{}

func TestIssueProjectionCannotWaiveCoreBounds(t *testing.T) {
	t.Parallel()
	defaults := DefaultStrictJSONLimits()
	cases := []struct {
		name    string
		wire    []byte
		limits  StrictJSONLimits
		wantErr error
	}{
		{name: "positive/empty object is still an exact projection", wire: []byte(`{}`), limits: defaults},
		{name: "positive/empty array retains its kind", wire: []byte(`[]`), limits: defaults},
		{name: "positive/empty string differs from empty document", wire: []byte(`""`), limits: defaults},
		{name: "positive/zero scalar remains present", wire: []byte(`0`), limits: defaults},
		{name: "positive/false scalar remains present", wire: []byte(`false`), limits: defaults},
		{name: "positive/object key order is preserved", wire: []byte(`{"z":1,"a":2}`), limits: defaults},
		{name: "positive/whitespace is preserved exactly", wire: []byte(" \n{\"a\":1}\t"), limits: defaults},
		{name: "positive/paired surrogate carries one scalar", wire: []byte(`"\ud83d\ude42"`), limits: defaults},
		{name: "positive/escaped key remains exact", wire: []byte(`{"\u0061":1}`), limits: defaults},
		{name: "positive/repeated names in separate objects are independent", wire: []byte(`[{"a":1},{"a":2}]`), limits: defaults},
		{name: "negative/empty output cannot be published", limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/whitespace is not a document", wire: []byte(" \t\n"), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/truncated object cannot reach semantic proof", wire: []byte(`{"a":`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/trailing document cannot reach semantic proof", wire: []byte(`{}[]`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/mismatched delimiter cannot reach semantic proof", wire: []byte(`{"a":]`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/exact duplicate cannot reach semantic proof", wire: []byte(`{"a":1,"a":2}`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/folded duplicate cannot reach semantic proof", wire: []byte(`{"a":1,"A":2}`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/nested folded duplicate cannot hide", wire: []byte(`{"outer":{"a":1,"A":2}}`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/unicode simple-fold collision cannot hide", wire: []byte(`{"K":1,"K":2}`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/escaped duplicate cannot hide", wire: []byte(`{"a":1,"\u0041":2}`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/invalid UTF8 cannot reach semantic proof", wire: []byte{'"', 0xff, '"'}, limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/unpaired surrogate cannot reach semantic proof", wire: []byte(`"\ud800"`), limits: defaults, wantErr: ErrJSONContract},
		{name: "negative/null cannot stand in for a projection", wire: []byte(`null`), limits: defaults, wantErr: ErrJSONContract},
	}
	// Each axis has independent caller and global thresholds. The fixtures
	// change the actual encoded structure, not an unrelated invalid setting.
	boundaries := []struct {
		name   string
		limit  int
		wire   func(int) []byte
		limits StrictJSONLimits
	}{
		{name: "caller document bytes", limit: 16, wire: func(n int) []byte { return []byte(`"` + strings.Repeat("x", n-2) + `"`) }, limits: StrictJSONLimits{DocumentMaximumBytes: ByteCount{value: 16}, NestingDepthMaximum: defaults.NestingDepthMaximum, ObjectFieldMaximum: defaults.ObjectFieldMaximum, ArrayItemMaximum: defaults.ArrayItemMaximum}},
		{name: "global document bytes", limit: JSONDocumentMaximumBytes, wire: func(n int) []byte { return []byte(`"` + strings.Repeat("x", n-2) + `"`) }, limits: defaults},
		{name: "caller nesting", limit: 3, wire: func(n int) []byte { return []byte(strings.Repeat("[", n) + "0" + strings.Repeat("]", n)) }, limits: StrictJSONLimits{DocumentMaximumBytes: defaults.DocumentMaximumBytes, NestingDepthMaximum: 3, ObjectFieldMaximum: defaults.ObjectFieldMaximum, ArrayItemMaximum: defaults.ArrayItemMaximum}},
		{name: "global nesting", limit: int(JSONNestingDepthMaximum), wire: func(n int) []byte { return []byte(strings.Repeat("[", n) + "0" + strings.Repeat("]", n)) }, limits: defaults},
		{name: "caller object fields", limit: 3, wire: strictJSONObject, limits: StrictJSONLimits{DocumentMaximumBytes: defaults.DocumentMaximumBytes, NestingDepthMaximum: defaults.NestingDepthMaximum, ObjectFieldMaximum: 3, ArrayItemMaximum: defaults.ArrayItemMaximum}},
		{name: "global object fields", limit: int(JSONObjectFieldCountMaximum), wire: strictJSONObject, limits: defaults},
		{name: "caller array items", limit: 3, wire: func(n int) []byte { return strictJSONArrayOfRepeatedValue([]byte("0"), n) }, limits: StrictJSONLimits{DocumentMaximumBytes: defaults.DocumentMaximumBytes, NestingDepthMaximum: defaults.NestingDepthMaximum, ObjectFieldMaximum: defaults.ObjectFieldMaximum, ArrayItemMaximum: 3}},
		{name: "global array items", limit: int(jsonArrayItemCountMaximum), wire: func(n int) []byte { return strictJSONArrayOfRepeatedValue([]byte("0"), n) }, limits: defaults},
	}
	for _, boundary := range boundaries {
		for _, edge := range []struct {
			name    string
			offset  int
			wantErr error
		}{
			{name: "one below", offset: -1}, {name: "exact", offset: 0}, {name: "one above", offset: 1, wantErr: ErrJSONContract},
		} {
			cases = append(cases, struct {
				name    string
				wire    []byte
				limits  StrictJSONLimits
				wantErr error
			}{name: "boundary/" + boundary.name + "/" + edge.name, wire: boundary.wire(boundary.limit + edge.offset), limits: boundary.limits, wantErr: edge.wantErr})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			original := bytes.Clone(tc.wire)
			got, err := EncodeValidatedJSON(untrustedIssueProjection{wire: tc.wire, calls: &calls}, tc.limits)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("encoding=%d bytes, %v; want error %v", len(got), err, tc.wantErr)
			}
			wantCalls := 1
			if tc.wantErr != nil {
				wantCalls = 0
				if got != nil {
					t.Fatalf("refused projection=%d bytes; want nil", len(got))
				}
			} else if !bytes.Equal(got, original) {
				t.Fatalf("projection=%d bytes; want exact %d input bytes", len(got), len(original))
			}
			if calls != wantCalls || !bytes.Equal(tc.wire, original) {
				t.Fatalf("semantic calls=%d, input unchanged=%t; want %d and true", calls, bytes.Equal(tc.wire, original), wantCalls)
			}
		})
	}
}
