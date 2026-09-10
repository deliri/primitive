package submission

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const decisionWhitespaceFixtureBytes = 1 << 20

func TestReceiptReuseDecisionExtentLayerTriad(t *testing.T) {
	t.Parallel()
	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{Request: grant.request, KeyByte: 0x41, ScopeByte: 0x64})
	projection := mustReuseDecision(t, reuse)
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	before := decodeDecisionProjection(t, projection)
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{name: "canonical_reuse", data: encoded},
		{name: "large_valid_prefix", data: append(bytes.Repeat([]byte(" "), decisionWhitespaceFixtureBytes), encoded...)},
		{name: "large_valid_inside_object", data: append(append([]byte{'{'}, bytes.Repeat([]byte(" "), decisionWhitespaceFixtureBytes)...), encoded[1:]...)},
		{name: "neutral_whitespace_is_not_a_decision", data: bytes.Repeat([]byte(" "), decisionWhitespaceFixtureBytes), wantErr: core.ErrJSONContract},
		{name: "trailing_object_after_large_gap", data: append(append(append([]byte{}, encoded...), bytes.Repeat([]byte(" "), decisionWhitespaceFixtureBytes)...), []byte("{}")...), wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := before
			err := got.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) || got.Kind != DecisionReuse || got.Grant != nil || got.Evidence == nil || *got.Evidence != reuse.evidence {
				t.Fatalf("decision=%v error=%v, want exact reuse evidence and %v", got, err, tc.wantErr)
			}
		})
	}
}
