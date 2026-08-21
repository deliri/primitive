package timeproof

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/asn1"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

//go:embed testdata/freetsa_2026_response.b64 testdata/digicert_2026_response.b64
var authenticFixtureFiles embed.FS

type authenticFixture struct {
	response []byte
	evidence AuthorityEvidence
	request  Request
	digest   core.SHA256Digest
}

func loadAuthenticFixture(t testing.TB) authenticFixture {
	t.Helper()

	encoded, err := authenticFixtureFiles.ReadFile(
		"testdata/freetsa_2026_response.b64",
	)
	if err != nil {
		t.Fatalf("read authentic response fixture error = %v, want nil", err)
	}
	response := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	count, err := base64.StdEncoding.Decode(response, encoded)
	if err != nil {
		t.Fatalf("decode authentic response fixture error = %v, want nil", err)
	}
	response = response[:count]
	nonce, err := parseNonce("000000000000000035eac9aacdfde4e5")
	if err != nil {
		t.Fatalf("parseNonce(authentic) error = %v, want nil", err)
	}
	digest := core.NewSHA256Digest(sha256.Sum256(nil))
	request, err := newRequest(digest, nonce, AuthorityFreeTSA)
	if err != nil {
		t.Fatalf("newRequest(authentic) error = %v, want nil", err)
	}
	evidence, err := newAuthorityEvidence(authorityEvidenceInput{
		Request: request, Response: response,
	})
	if err != nil {
		t.Fatalf("newAuthorityEvidence(authentic) error = %v, want nil", err)
	}
	return authenticFixture{
		response: response, evidence: evidence, request: request, digest: digest,
	}
}

func TestTimeProofVerifierLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	t.Run("positive authentic FreeTSA response projects one timestamp", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Verify(VerifyRequest{
			Response: fixture.response, Request: fixture.request,
			ExpectedDigest: fixture.digest,
		})
		if gotErr != nil {
			t.Fatalf("Verify(authentic) error = %v, want nil", gotErr)
		}
		if got.Policy() != TimestampPolicyFreeTSA ||
			got.Evidence().Authority() != AuthorityFreeTSA {
			t.Fatalf(
				"Verify(authentic) policy/authority = (%v, %v), want (%v, %v)",
				got.Policy(),
				got.Evidence().Authority(),
				TimestampPolicyFreeTSA,
				AuthorityFreeTSA,
			)
		}
		gotNanoseconds, gotInstantErr := got.Instant().Nanoseconds()
		const wantNanoseconds = int64(1784942623 * 1_000_000_000)
		if gotInstantErr != nil || gotNanoseconds != wantNanoseconds {
			t.Fatalf(
				"Verify(authentic) instant = (%d, %v), want (%d, nil)",
				gotNanoseconds,
				gotInstantErr,
				wantNanoseconds,
			)
		}
	})

	t.Run("negative signature mutation projects no timestamp", func(t *testing.T) {
		t.Parallel()

		response := append([]byte(nil), fixture.response...)
		response[len(response)-1] ^= 1
		got, gotErr := Verify(VerifyRequest{
			Response: response, Request: fixture.request,
			ExpectedDigest: fixture.digest,
		})
		if !errors.Is(gotErr, core.ErrTimeProofInvalid) || !got.isZero() {
			t.Fatalf(
				"Verify(mutated) = (%v, %v), want zero and %v",
				got,
				gotErr,
				core.ErrTimeProofInvalid,
			)
		}
	})

	t.Run("neutral absent request and response create no timestamp", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Verify(VerifyRequest{})
		if !errors.Is(gotErr, core.ErrTimeProofContract) || !got.isZero() {
			t.Fatalf(
				"Verify(zero) = (%v, %v), want zero and %v",
				got,
				gotErr,
				core.ErrTimeProofContract,
			)
		}
	})
}

func TestVerifierPreservesStandardLibraryDERFailure(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	got, gotErr := Verify(VerifyRequest{
		Response: []byte{0x30, 0x80}, Request: fixture.request,
		ExpectedDigest: fixture.digest,
	})
	var syntaxError asn1.SyntaxError
	if !errors.Is(gotErr, core.ErrTimeProofInvalid) ||
		!errors.As(gotErr, &syntaxError) || !got.isZero() {
		t.Fatalf(
			"Verify(indefinite DER) timestamp/error = (%v, %v), want zero and Timeproof invalid carrying asn1.SyntaxError",
			got,
			gotErr,
		)
	}
}

func TestAuthenticResponseMutationTable(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	tokenDER, _, err := parseTimestampResponse(fixture.response)
	if err != nil {
		t.Fatalf("parseTimestampResponse(authentic) error = %v, want nil", err)
	}
	token, err := parseTimestampToken(tokenDER)
	if err != nil {
		t.Fatalf("parseTimestampToken(authentic) error = %v, want nil", err)
	}
	info, err := parseTSTInfo(token.TSTDER)
	if err != nil {
		t.Fatalf("parseTSTInfo(authentic) error = %v, want nil", err)
	}
	// Every offset below is located from the parsed structure it names. A
	// fraction of the response length would drift silently if the fixture
	// changed and would not prove the structure the case claims to attack.
	root := token.Certificates[len(token.Certificates)-1]
	if root.Equal(token.Signer) {
		t.Fatalf(
			"authentic certificate signer/last subject = (%q, %q), want a distinct issuer certificate",
			token.Signer.Subject.String(),
			root.Subject.String(),
		)
	}
	structures := []struct {
		name  string
		bytes []byte
		at    int
	}{
		{name: "TSTInfo signed content", bytes: token.TSTDER, at: len(token.TSTDER) / 2},
		{name: "TSTInfo message imprint", bytes: info.MessageImprint.HashedMessage},
		{name: "TSTInfo nonce integer", bytes: info.Nonce.Bytes()},
		{name: "TSTInfo serial integer", bytes: info.Serial.Bytes()},
		{name: "included signer certificate", bytes: token.Signer.Raw, at: len(token.Signer.Raw) / 2},
		{name: "included signer public key", bytes: token.Signer.RawSubjectPublicKeyInfo},
		{name: "included signer certificate signature", bytes: token.Signer.Signature},
		{name: "included issuer certificate", bytes: root.Raw, at: len(root.Raw) / 2},
		{name: "CMS signed attributes", bytes: token.SignerInfo.SignedAttributes.Bytes, at: len(token.SignerInfo.SignedAttributes.Bytes) / 2},
		{name: "CMS signature body", bytes: token.SignerInfo.Signature, at: len(token.SignerInfo.Signature) / 2},
	}
	offsets := []struct {
		name string
		at   int
	}{
		{name: "outer DER sequence tag", at: 0},
		{name: "outer DER length", at: 2},
		{name: "CMS signature tail", at: len(fixture.response) - 1},
	}
	for _, structure := range structures {
		if len(structure.bytes) == 0 {
			t.Fatalf("authentic %s = empty, want located bytes", structure.name)
		}
		start := bytes.Index(fixture.response, structure.bytes)
		if start < 0 {
			t.Fatalf(
				"authentic %s offset = -1, want located inside the response",
				structure.name,
			)
		}
		if bytes.Contains(fixture.response[start+1:], structure.bytes) {
			t.Fatalf(
				"authentic %s appears more than once, want one unambiguous location",
				structure.name,
			)
		}
		offsets = append(offsets, struct {
			name string
			at   int
		}{name: structure.name, at: start + structure.at})
	}
	for _, tc := range offsets {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response := append([]byte(nil), fixture.response...)
			response[tc.at] ^= 1
			got, gotErr := Verify(VerifyRequest{
				Response: response, Request: fixture.request,
				ExpectedDigest: fixture.digest,
			})
			if !errors.Is(gotErr, core.ErrTimeProofInvalid) || !got.isZero() {
				t.Fatalf(
					"Verify(mutated at %d) = (%v, %v), want zero and %v",
					tc.at,
					got,
					gotErr,
					core.ErrTimeProofInvalid,
				)
			}
		})
	}
}

func TestPreparedRequestWireOrderAndBinding(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	encoded := fixture.request.Bytes()
	outer, gotErr := requireSequence(encoded)
	if gotErr != nil {
		t.Fatalf("requireSequence(TimeStampReq) error = %v, want nil", gotErr)
	}
	var gotVersion int
	fields, gotErr := asn1.Unmarshal(outer.Bytes, &gotVersion)
	if gotErr != nil {
		t.Fatalf("decode TimeStampReq version error = %v, want nil", gotErr)
	}
	var gotImprint messageImprint
	fields, gotErr = asn1.Unmarshal(fields, &gotImprint)
	if gotErr != nil {
		t.Fatalf("decode TimeStampReq imprint error = %v, want nil", gotErr)
	}
	var gotNonce *big.Int
	fields, gotErr = asn1.Unmarshal(fields, &gotNonce)
	if gotErr != nil {
		t.Fatalf("decode TimeStampReq nonce error = %v, want nil", gotErr)
	}
	var gotCertReq bool
	trailing, gotErr := asn1.Unmarshal(fields, &gotCertReq)
	if gotErr != nil || len(trailing) != 0 {
		t.Fatalf(
			"decode TimeStampReq certReq trailing/error = (%x, %v), want (empty, nil)",
			trailing,
			gotErr,
		)
	}
	wantDigest, _ := fixture.digest.Bytes()
	if gotVersion != 1 ||
		!gotImprint.HashAlgorithm.Algorithm.Equal(oidSHA256) ||
		!bytes.Equal(gotImprint.HashedMessage, wantDigest[:]) ||
		gotNonce == nil ||
		!fixture.request.Nonce().matches(gotNonce) ||
		!gotCertReq {
		t.Fatalf(
			"decoded TimeStampReq version/imprint/nonce/certReq = (%d, %+v, %v, %t), want version 1, SHA-256 imprint, exact nonce, and true",
			gotVersion,
			gotImprint,
			gotNonce,
			gotCertReq,
		)
	}
	if len(encoded) > RequestMaximumBytes {
		t.Fatalf(
			"TimeStampReq bytes = %d, want <= %d",
			len(encoded),
			RequestMaximumBytes,
		)
	}
}

func TestPrepareRequestLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	t.Run("positive production entropy prepares a closed request", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Prepare(PrepareRequest{
			Digest: fixture.digest, Authority: AuthorityFreeTSA,
		})
		if gotErr != nil || got.Validate() != nil ||
			got.Digest() != fixture.digest ||
			got.Authority() != AuthorityFreeTSA ||
			len(got.Bytes()) == 0 {
			t.Fatalf(
				"Prepare(valid) request/error = (%v, %v), want closed FreeTSA request and nil",
				got,
				gotErr,
			)
		}
	})

	t.Run("negative zero digest is rejected before a request exists", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Prepare(PrepareRequest{})
		if !errors.Is(gotErr, core.ErrTimeProofContract) ||
			len(got.Bytes()) != 0 {
			t.Fatalf(
				"Prepare(zero) = (%v, %v), want zero and %v",
				got,
				gotErr,
				core.ErrTimeProofContract,
			)
		}
	})

	t.Run("neutral zero request cannot serialize a claim", func(t *testing.T) {
		t.Parallel()

		got, gotErr := (Request{}).MarshalJSON()
		if !errors.Is(gotErr, core.ErrTimeProofContract) || got != nil {
			t.Fatalf(
				"Request{}.MarshalJSON() = (%q, %v), want (nil, %v)",
				got,
				gotErr,
				core.ErrTimeProofContract,
			)
		}
	})
}

func TestVerifyRequestHostileResponseBoundaryTable(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	foreignDigest := core.NewSHA256Digest(sha256.Sum256([]byte("foreign")))
	cases := []struct {
		wantErr error
		name    string
		request VerifyRequest
	}{
		{
			name: "authentic response within ceiling is accepted",
			request: VerifyRequest{
				Response: fixture.response, Request: fixture.request,
				ExpectedDigest: fixture.digest,
			},
		},
		{
			name: "nil response is absent",
			request: VerifyRequest{
				Request: fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofContract,
		},
		{
			name: "empty response is absent",
			request: VerifyRequest{
				Response: []byte{}, Request: fixture.request,
				ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofContract,
		},
		{
			name: "one byte response is malformed",
			request: VerifyRequest{
				Response: []byte{0x30}, Request: fixture.request,
				ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name: "truncated authentic response is malformed",
			request: VerifyRequest{
				Response: fixture.response[:len(fixture.response)-1],
				Request:  fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name: "trailing authentic response byte is noncanonical",
			request: VerifyRequest{
				Response: append(
					append([]byte(nil), fixture.response...),
					0,
				),
				Request: fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name: "one below response ceiling remains bounded parser input",
			request: VerifyRequest{
				Response: bytes.Repeat(
					[]byte{0},
					ResponseMaximumBytes-1,
				),
				Request: fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name: "exact response ceiling remains bounded parser input",
			request: VerifyRequest{
				Response: bytes.Repeat([]byte{0}, ResponseMaximumBytes),
				Request:  fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name: "one above response ceiling is rejected before parsing",
			request: VerifyRequest{
				Response: bytes.Repeat(
					[]byte{0},
					ResponseMaximumBytes+1,
				),
				Request: fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofContract,
		},
		{
			name: "far above response ceiling is rejected before parsing",
			request: VerifyRequest{
				Response: bytes.Repeat(
					[]byte{0},
					4*ResponseMaximumBytes,
				),
				Request: fixture.request, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofContract,
		},
		{
			name: "foreign expected digest cannot select another subject",
			request: VerifyRequest{
				Response: fixture.response, Request: fixture.request,
				ExpectedDigest: foreignDigest,
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name: "zero expected digest is not a subject",
			request: VerifyRequest{
				Response: fixture.response, Request: fixture.request,
			},
			wantErr: core.ErrTimeProofContract,
		},
		{
			name: "zero prepared request carries no binding",
			request: VerifyRequest{
				Response: fixture.response, ExpectedDigest: fixture.digest,
			},
			wantErr: core.ErrTimeProofContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := Verify(tc.request)
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf(
						"Verify(boundary) timestamp/error = (%v, %v), want validated timestamp and nil",
						got,
						gotErr,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !got.isZero() {
				t.Fatalf(
					"Verify(boundary) timestamp/error = (%v, %v), want zero and %v",
					got,
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestAuthorityEvidencePersistenceLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	t.Run("positive canonical evidence round trip re-verifies", func(t *testing.T) {
		t.Parallel()

		encoded, gotErr := fixture.evidence.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("AuthorityEvidence.MarshalJSON() error = %v, want nil", gotErr)
		}
		var decoded AuthorityEvidence
		if gotErr = decoded.UnmarshalJSON(encoded); gotErr != nil {
			t.Fatalf("AuthorityEvidence.UnmarshalJSON() error = %v, want nil", gotErr)
		}
		got, gotErr := Verify(VerifyRequest{
			Response: decoded.ResponseBytes(), Request: decoded.Request(),
			ExpectedDigest: fixture.digest,
		})
		if gotErr != nil || got.Policy() != TimestampPolicyFreeTSA {
			t.Fatalf(
				"Verify(round trip) policy/error = (%v, %v), want (%v, nil)",
				got.Policy(),
				gotErr,
				TimestampPolicyFreeTSA,
			)
		}
	})

	t.Run("negative noncanonical trailing JSON mutates no receiver", func(t *testing.T) {
		t.Parallel()

		encoded, gotErr := fixture.evidence.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("AuthorityEvidence.MarshalJSON() error = %v, want nil", gotErr)
		}
		original := fixture.evidence
		receiver := original
		gotErr = receiver.UnmarshalJSON(append(encoded, '\n'))
		if !errors.Is(gotErr, core.ErrJSONContract) ||
			!bytes.Equal(receiver.ResponseBytes(), original.ResponseBytes()) {
			t.Fatalf(
				"AuthorityEvidence.UnmarshalJSON(trailing) receiver/error = (%v, %v), want unchanged and %v",
				receiver,
				gotErr,
				core.ErrJSONContract,
			)
		}
	})

	t.Run("neutral zero evidence cannot serialize a claim", func(t *testing.T) {
		t.Parallel()

		got, gotErr := (AuthorityEvidence{}).MarshalJSON()
		if !errors.Is(gotErr, core.ErrTimeProofContract) || got != nil {
			t.Fatalf(
				"AuthorityEvidence{}.MarshalJSON() = (%q, %v), want (nil, %v)",
				got,
				gotErr,
				core.ErrTimeProofContract,
			)
		}
	})
}

func TestAuthoritativeTimestampPersistenceLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	verified, err := Verify(VerifyRequest{
		Response: fixture.response, Request: fixture.request,
		ExpectedDigest: fixture.digest,
	})
	if err != nil {
		t.Fatalf("Verify(authentic) setup error = %v, want nil", err)
	}
	encoded, err := verified.MarshalJSON()
	if err != nil {
		t.Fatalf("AuthoritativeTimestamp.MarshalJSON() setup error = %v, want nil", err)
	}

	t.Run("positive canonical timestamp round trip repeats verification", func(t *testing.T) {
		t.Parallel()

		var got AuthoritativeTimestamp
		gotErr := got.UnmarshalJSON(encoded)
		gotEncoded, gotMarshalErr := got.MarshalJSON()
		if gotErr != nil || gotMarshalErr != nil ||
			!bytes.Equal(gotEncoded, encoded) {
			t.Fatalf(
				"AuthoritativeTimestamp round trip bytes/errors = (%q, %v, %v), want exact bytes and nil",
				gotEncoded,
				gotErr,
				gotMarshalErr,
			)
		}
	})

	t.Run("negative forged derived time mutates no receiver", func(t *testing.T) {
		t.Parallel()

		var wire authoritativeTimestampWire
		if gotErr := json.Unmarshal(encoded, &wire); gotErr != nil {
			t.Fatalf("json.Unmarshal(timestamp wire) error = %v, want nil", gotErr)
		}
		forged, gotErr := temporal.NewInstant(time.Unix(1784942624, 0))
		if gotErr != nil {
			t.Fatalf("temporal.NewInstant(forged) error = %v, want nil", gotErr)
		}
		wire.Generation = forged
		forgedJSON, gotErr := json.Marshal(wire)
		if gotErr != nil {
			t.Fatalf("json.Marshal(forged wire) error = %v, want nil", gotErr)
		}
		receiver := verified
		gotErr = receiver.UnmarshalJSON(forgedJSON)
		receiverJSON, receiverMarshalErr := receiver.MarshalJSON()
		if !errors.Is(gotErr, core.ErrTimeProofInvalid) ||
			receiverMarshalErr != nil ||
			!bytes.Equal(receiverJSON, encoded) {
			t.Fatalf(
				"AuthoritativeTimestamp.UnmarshalJSON(forged) receiver/error = (%q, %v), want unchanged and %v",
				receiverJSON,
				gotErr,
				core.ErrTimeProofInvalid,
			)
		}
	})

	t.Run("neutral zero timestamp cannot serialize a claim", func(t *testing.T) {
		t.Parallel()

		got, gotErr := (AuthoritativeTimestamp{}).MarshalJSON()
		if !errors.Is(gotErr, core.ErrTimeProofContract) || got != nil {
			t.Fatalf(
				"AuthoritativeTimestamp{}.MarshalJSON() = (%q, %v), want (nil, %v)",
				got,
				gotErr,
				core.ErrTimeProofContract,
			)
		}
	})
}

func BenchmarkPrepareRequest(b *testing.B) {
	fixture := loadAuthenticFixture(b)
	input := PrepareRequest{Digest: fixture.digest, Authority: AuthorityFreeTSA}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := Prepare(input); err != nil {
			b.Fatalf("Prepare(valid) error = %v, want nil", err)
		}
	}
}

func BenchmarkVerifyAuthenticFreeTSA(b *testing.B) {
	fixture := loadAuthenticFixture(b)
	request := VerifyRequest{
		Response: fixture.response, Request: fixture.request,
		ExpectedDigest: fixture.digest,
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := Verify(request); err != nil {
			b.Fatalf("Verify(authentic) error = %v, want nil", err)
		}
	}
}

func BenchmarkRejectOversizedResponse(b *testing.B) {
	fixture := loadAuthenticFixture(b)
	request := VerifyRequest{
		Response: bytes.Repeat([]byte{0}, ResponseMaximumBytes+1),
		Request:  fixture.request, ExpectedDigest: fixture.digest,
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := Verify(request); !errors.Is(
			err,
			core.ErrTimeProofContract,
		) {
			b.Fatalf(
				"Verify(oversized response) error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
	}
}

func BenchmarkReplayCanonicalEvidence(b *testing.B) {
	fixture := loadAuthenticFixture(b)
	encoded, err := fixture.evidence.MarshalJSON()
	if err != nil {
		b.Fatalf("AuthorityEvidence.MarshalJSON() setup error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		var evidence AuthorityEvidence
		if err := evidence.UnmarshalJSON(encoded); err != nil {
			b.Fatalf(
				"AuthorityEvidence.UnmarshalJSON() error = %v, want nil",
				err,
			)
		}
	}
}
