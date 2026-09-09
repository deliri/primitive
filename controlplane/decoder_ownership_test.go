package controlplane_test

import (
	"bytes"
	json "encoding/json/v2"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// This consumer-owned fixture deliberately erases the bytes passed to its
// decoder. Primitive must keep its signed wire commitment independently owned.
type responseCount struct {
	Count uint64 `json:"count"`
}
type erasingResponseBody responseCount

// The producing side uses ordinary Go JSON; only the receiving side erases.
// Both types derive their wire fields from one compiler-owned fixture struct.
type responseCountProjection responseCount

func (p responseCountProjection) Validate() error { return erasingResponseBody(p).Validate() }
func (p responseCountProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(responseCount(p))
}

type erasingResponseWire erasingResponseBody

func (b erasingResponseBody) Validate() error {
	if b.Count == 0 {
		return core.ErrJSONContract
	}
	return nil
}
func (b erasingResponseBody) MarshalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(erasingResponseWire(b))
}
func (b *erasingResponseBody) UnmarshalJSON(data []byte) error {
	defer clear(data)
	var wire erasingResponseWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	candidate := erasingResponseBody(wire)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*b = candidate
	return nil
}

func TestResponseDecoderCannotRewriteItsRetainedWireProof(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		count uint64
	}{{"smallest admitted body", 1}, {"largest admitted count", ^uint64(0)}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := authenticatedResponseForTest(t, 30)
			want := erasingResponseBody{Count: tc.count}
			projection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[responseCountProjection]{Server: fixture.server, Signer: fixture.signer, Header: fixture.header, Body: responseCountProjection(want), Assessment: acceptedProtocolAssessment(t, fixture.header)})
			if err != nil {
				t.Fatalf("IssueResponse() error = %v, want nil", err)
			}
			encoded, err := projection.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v, want nil", err)
			}
			original := bytes.Clone(encoded)
			var document controlplane.ResponseDocument[erasingResponseBody, *erasingResponseBody]
			if err := document.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("UnmarshalJSON(erasing consumer) error = %v, want nil", err)
			}
			if !bytes.Equal(encoded, original) {
				t.Fatalf("caller wire after decoder = %d bytes, want unchanged %d bytes", len(encoded), len(original))
			}
			proof, err := controlplane.VerifyResponse(controlplane.ResponseVerification[erasingResponseBody, *erasingResponseBody]{Client: fixture.client, Document: document, Expected: fixture.expected})
			if err != nil {
				t.Fatalf("VerifyResponse() error = %v, want nil", err)
			}
			for range 2 {
				got, err := proof.Body()
				if err != nil || got != want {
					t.Fatalf("Body() = (%v, %v), want (%v, nil)", got, err, want)
				}
			}
			if err := document.Validate(); err != nil {
				t.Fatalf("document after accessor Validate() error = %v, want nil", err)
			}
		})
	}
}
