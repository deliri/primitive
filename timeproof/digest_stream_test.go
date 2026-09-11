package timeproof

import (
	"bytes"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This fixture changes only the unsigned digest declaration set. The authentic
// signed content, signature, signer, and request binding remain byte-identical.
func responseWithDigestSet(t testing.TB, set []byte) []byte {
	t.Helper()
	token := authenticTokenDER(t)
	sequence := rawValueFromDER(t, authenticSignedData(t, token))
	fields := splitElements(t, sequence.Bytes)
	fields[1] = set
	rebuilt := derTagged(byte(asn1.TagSequence)|derConstructed, bytes.Join(fields, nil))
	return rebuildResponse(t, rebuildToken(t, token, rebuilt))
}

func digestSetFixture(t testing.TB, foreignCount int, copies int) []byte {
	t.Helper()
	token, err := parseTimestampToken(authenticTokenDER(t))
	if err != nil {
		t.Fatalf("parseTimestampToken(seed) error = %v, want nil", err)
	}
	algorithms := make([]pkix.AlgorithmIdentifier, 0, foreignCount+copies)
	for i := range foreignCount {
		algorithms = append(algorithms, pkix.AlgorithmIdentifier{Algorithm: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 99999, i}})
	}
	for range copies {
		algorithms = append(algorithms, token.SignerInfo.DigestAlgorithm)
	}
	encoded, err := asn1.MarshalWithParams(algorithms, "set")
	if err != nil {
		t.Fatalf("asn1.MarshalWithParams(digest set) error = %v, want nil", err)
	}
	return encoded
}

func TestDigestDeclarationLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	want, err := Verify(VerifyRequest{Response: fixture.response, Request: fixture.request, ExpectedDigest: fixture.digest})
	if err != nil {
		t.Fatalf("Verify(authentic) error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		foreign int
		copies  int
		wantErr error
	}{
		{name: "one authentic digest declaration", copies: 1},
		{name: "four foreign declarations do not hide the signer digest", foreign: 4, copies: 1},
		{name: "thousand distinct foreign declarations do not impose a quota", foreign: 1024, copies: 1},
		{name: "empty set cannot manufacture a signer declaration", wantErr: core.ErrTimeProofInvalid},
		{name: "foreign declarations alone cannot bind the signer", foreign: 5, wantErr: core.ErrTimeProofInvalid},
		{name: "duplicate signer digest remains ambiguous", copies: 2, wantErr: core.ErrTimeProofInvalid},
		{name: "foreign declarations cannot hide duplicate signer digest", foreign: 5, copies: 2, wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := responseWithDigestSet(t, digestSetFixture(t, tc.foreign, tc.copies))
			got, err := Verify(VerifyRequest{Response: response, Request: fixture.request, ExpectedDigest: fixture.digest})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Verify(declarations) error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !timestampHasNoProof(got) {
					t.Fatalf("Verify(rejected) proof = %+v, want zero", got)
				}
				return
			}
			if got.Time() != want.Time() || got.Signer() != want.Signer() || got.Serial() != want.Serial() || got.Policy() != want.Policy() {
				t.Fatalf("Verify(declarations) proof = %+v, want authentic signed facts %+v", got, want)
			}
		})
	}
}

func addDigestDeclarationSeeds(f *testing.F) {
	fixture := loadAuthenticFixture(f)
	for _, count := range []int{4, 1024} {
		response := responseWithDigestSet(f, digestSetFixture(f, count, 1))
		got, err := Verify(VerifyRequest{Response: response, Request: fixture.request, ExpectedDigest: fixture.digest})
		if err != nil || got.Validate() != nil {
			f.Fatalf("Verify(digest seed) error = %v, want nil", err)
		}
		f.Add(response)
	}
	f.Add(responseWithDigestSet(f, digestSetFixture(f, 5, 2)))
}
