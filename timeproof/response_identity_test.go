package timeproof

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	json "encoding/json/v2"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestLargeResponseRetainsOnlyIdentity(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	response := responseWithDigestSet(t, digestSetFixture(t, 16384, 1))
	// This is a regression witness for the removed 128 KiB transfer quota,
	// not a new input limit.
	if len(response) <= 128*1024 {
		t.Fatalf("response fixture size = %d, want beyond the removed quota", len(response))
	}
	got, err := Verify(VerifyRequest{Response: response, Request: fixture.request, ExpectedDigest: fixture.digest})
	wantDigest := core.NewSHA256Digest(sha256.Sum256(response))
	if err != nil || got.Evidence().ResponseDigest() != wantDigest || got.Evidence().ResponseSize() != uint64(len(response)) {
		t.Fatalf("Verify(large response) = (%+v, %v), want exact digest/extent and nil", got, err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || bytes.Contains(encoded, []byte("response_base64")) || len(encoded) >= len(response) {
		t.Fatalf("metadata encoding = (%d bytes, %v), want compact identity without response material", len(encoded), err)
	}
	var restored AuthoritativeTimestamp
	if err := restored.Restore(RestoreRequest{Document: encoded, Response: response, ExpectedDigest: fixture.digest}); err != nil || !sameTimeproofEvidence(restored.Evidence(), got.Evidence()) {
		t.Fatalf("Restore(large response) = (%+v, %v), want same authenticated identity", restored, err)
	}
}

func TestRestoreResponseBindingLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	verified, err := Verify(VerifyRequest{Response: fixture.response, Request: fixture.request, ExpectedDigest: fixture.digest})
	if err != nil {
		t.Fatalf("Verify(seed) error = %v, want nil", err)
	}
	document, err := verified.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		mutate  func(*testing.T, *RestoreRequest)
		wantErr error
	}{
		{name: "matching document and source reverify"},
		{name: "metadata alone cannot restore proof", mutate: func(t *testing.T, r *RestoreRequest) { r.Response = nil }, wantErr: core.ErrJSONContract},
		{name: "source alone cannot restore metadata", mutate: func(t *testing.T, r *RestoreRequest) { r.Document = nil }, wantErr: core.ErrJSONContract},
		{name: "changed source signature yields no restored proof", mutate: func(t *testing.T, r *RestoreRequest) {
			r.Response = bytes.Clone(r.Response)
			r.Response[len(r.Response)-1] ^= 1
		}, wantErr: core.ErrTimeProofInvalid},
		{name: "independent subject cannot be replaced by persisted request", mutate: func(t *testing.T, r *RestoreRequest) {
			r.ExpectedDigest = core.NewSHA256Digest(sha256.Sum256([]byte("foreign subject")))
		}, wantErr: core.ErrTimeProofInvalid},
		{name: "forged response extent cannot pass authentic verification", mutate: func(t *testing.T, r *RestoreRequest) {
			var wire authoritativeTimestampWire
			if err := json.Unmarshal(r.Document, &wire); err != nil {
				t.Fatalf("decode fixture = %v, want nil", err)
			}
			wire.Evidence.responseBytes++
			encoded, encodeErr := wire.MarshalJSON()
			if encodeErr != nil {
				t.Fatalf("encode fixture = %v, want nil", encodeErr)
			}
			r.Document = encoded
		}, wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := RestoreRequest{Document: document, Response: fixture.response, ExpectedDigest: fixture.digest}
			if tc.mutate != nil {
				tc.mutate(t, &request)
			}
			got := verified
			gotErr := got.Restore(request)
			if !errors.Is(gotErr, tc.wantErr) || !sameTimeproofEvidence(got.Evidence(), verified.Evidence()) || got.Time() != verified.Time() || got.Signer() != verified.Signer() {
				t.Fatalf("Restore() = (%+v, %v), want preserved authentic facts and %v", got, gotErr, tc.wantErr)
			}
			var empty AuthoritativeTimestamp
			emptyErr := empty.Restore(request)
			if !errors.Is(emptyErr, tc.wantErr) || (emptyErr != nil && !timestampHasNoProof(empty)) {
				t.Fatalf("Restore(zero receiver) = (%+v, %v), want typed result without partial proof", empty, emptyErr)
			}
		})
	}
}

func responseWithForeignCertificates(t testing.TB, count int, duplicate bool) []byte {
	t.Helper()
	token := authenticTokenDER(t)
	sequence := rawValueFromDER(t, authenticSignedData(t, token))
	fields := splitElements(t, sequence.Bytes)
	certificates := rawValueFromDER(t, fields[3])
	material := bytes.Clone(certificates.Bytes)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	var last []byte
	for i := range count {
		template := x509.Certificate{
			SerialNumber:          big.NewInt(int64(i + 1)),
			Subject:               pkix.Name{CommonName: "unrelated certificate"},
			NotBefore:             time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			NotAfter:              time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageDigitalSignature,
		}
		der, err := x509.CreateCertificate(bytes.NewReader(nil), &template, &template, key.Public(), key)
		if err != nil {
			t.Fatalf("CreateCertificate(foreign) error = %v, want nil", err)
		}
		material = append(material, der...)
		last = der
	}
	if duplicate {
		if len(last) == 0 {
			t.Fatal("duplicate fixture certificate = absent, want present")
		}
		material = append(material, last...)
	}
	fields[3] = derTagged(0xa0, material)
	rebuilt := derTagged(byte(asn1.TagSequence)|derConstructed, bytes.Join(fields, nil))
	return rebuildResponse(t, rebuildToken(t, token, rebuilt))
}

func TestCertificateSetScanLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	for _, tc := range []struct {
		name      string
		count     int
		duplicate bool
		wantErr   error
	}{
		{name: "authentic certificate set preserves signer"},
		{name: "seventeen unrelated certificates do not impose a quota", count: 17},
		{name: "duplicate foreign certificate remains invalid", count: 17, duplicate: true, wantErr: core.ErrTimeProofInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := responseWithForeignCertificates(t, tc.count, tc.duplicate)
			got, err := Verify(VerifyRequest{Response: response, Request: fixture.request, ExpectedDigest: fixture.digest})
			if !errors.Is(err, tc.wantErr) || (err != nil && !timestampHasNoProof(got)) {
				t.Fatalf("Verify(certificate set) = (%+v, %v), want typed result %v", got, err, tc.wantErr)
			}
			if err == nil && (got.Evidence().ResponseSize() != uint64(len(response)) || got.Evidence().ResponseDigest() != core.NewSHA256Digest(sha256.Sum256(response))) {
				t.Fatalf("certificate response identity = %+v, want exact source digest and extent", got.Evidence())
			}
		})
	}
}
