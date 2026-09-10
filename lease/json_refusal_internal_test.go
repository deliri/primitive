package lease

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestDecisionJSONCrossFieldRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	header := fixtureInternalHeader(t)
	grant := fixtureInternalGrant()
	seed, err := NewGrantDecision(GrantDecisionRequest{Header: header, Grant: grant})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		outcome Outcome
		issued  int64
		wantErr error
	}{
		{name: "grant at contact boundary", outcome: OutcomeGrant, issued: 3000},
		{name: "grant one past contact is JSON refusal", outcome: OutcomeGrant, issued: 3001, wantErr: core.ErrJSONContract},
		{name: "refusal at contact boundary", outcome: OutcomeRefusal, issued: 6000},
		{name: "refusal one past contact is JSON refusal", outcome: OutcomeRefusal, issued: 6001, wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := grant.MarshalJSON()
			if tc.outcome == OutcomeRefusal {
				body, err = (Refusal{ContactAfter: fixtureInternalInstant(6000)}).MarshalJSON()
			}
			if err != nil {
				t.Fatal(err)
			}
			issued := fixtureInternalInstant(tc.issued)
			raw := jsontext.Value(body)
			wire := decisionWire{Revision: &header.Revision, Subject: &header.Subject, Generation: &header.Generation, IssuedAt: &issued, Outcome: &tc.outcome, Body: &raw}
			data, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			got := seed
			err = got.UnmarshalJSON(data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decision JSON error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if !errors.Is(err, core.ErrLeaseContract) || got != seed {
					t.Fatalf("refusal/equality = %v/%v, want lease refusal and preserved seed", err, got == seed)
				}
				return
			}
			resultHeader, headerErr := got.Header()
			if headerErr != nil || resultHeader.IssuedAt != issued || got.Outcome() != tc.outcome {
				t.Fatalf("accepted header/outcome/error = %v/%v/%v, want exact wire facts", resultHeader, got.Outcome(), headerErr)
			}
		})
	}
}
