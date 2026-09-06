package attest_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

func TestSignPublicCanonicalBodyProductionPathMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeBody   func() attest.CanonicalBody[testDomain]
		wantErr    error
		wantNative error
		name       string
		wantBytes  uint64
	}{
		{name: "one byte in one write seals", makeBody: sizedBodyFixture(1, 1, testDomainPrimary, false), wantBytes: 1},
		{name: "two bytes one at a time seal", makeBody: sizedBodyFixture(2, 1, testDomainPrimary, false), wantBytes: 2},
		{name: "prime extent irregular chunks seal", makeBody: sizedBodyFixture(7919, 113, testDomainPrimary, false), wantBytes: 7919},
		{name: "one page in one write seals", makeBody: sizedBodyFixture(4096, 4096, testDomainPrimary, false), wantBytes: 4096},
		{name: "sixty four kibibytes in pages seals", makeBody: sizedBodyFixture(64<<10, 4096, testDomainPrimary, false), wantBytes: 64 << 10},
		{name: "maximum minus one seals", makeBody: sizedBodyFixture(attest.CanonicalBodyMaximumBytes-1, 8191, testDomainPrimary, false), wantBytes: attest.CanonicalBodyMaximumBytes - 1},
		{name: "exact maximum seals", makeBody: sizedBodyFixture(attest.CanonicalBodyMaximumBytes, 8192, testDomainPrimary, false), wantBytes: attest.CanonicalBodyMaximumBytes},
		{name: "alternate domain seals", makeBody: sizedBodyFixture(1, 1, testDomainAlternate, false), wantBytes: 1},
		{name: "embedded zero bytes seal exactly", makeBody: literalBodyFixture(testDomainPrimary, []byte{0, 1, 0, 2}), wantBytes: 4},
		{name: "utf8 bytes seal without interpretation", makeBody: literalBodyFixture(testDomainPrimary, []byte("世界")), wantBytes: 6},
		{name: "zero canonical extent rejects", makeBody: sizedBodyFixture(0, 1, testDomainPrimary, false), wantErr: core.ErrAttestContract},
		{name: "maximum plus one rejects", makeBody: sizedBodyFixture(attest.CanonicalBodyMaximumBytes+1, 8192, testDomainPrimary, false), wantErr: core.ErrAttestContract},
		{name: "ignored limit error remains terminal", makeBody: sizedBodyFixture(attest.CanonicalBodyMaximumBytes+1, 8192, testDomainPrimary, true), wantErr: core.ErrAttestContract},
		{name: "unknown domain rejects before signing", makeBody: literalBodyFixture(testDomainUnknown, []byte("x")), wantErr: core.ErrAttestContract},
		{name: "validation error remains reachable", makeBody: hostileBodyFixture(hostileBodyValidationError), wantErr: core.ErrAttestContract, wantNative: fixtureErrorValidation},
		{name: "validation panic is contained", makeBody: hostileBodyFixture(hostileBodyValidationPanic), wantErr: core.ErrAttestContract},
		{name: "domain panic is contained", makeBody: hostileBodyFixture(hostileBodyDomainPanic), wantErr: core.ErrAttestContract},
		{name: "writer error remains reachable", makeBody: hostileBodyFixture(hostileBodyWriteError), wantErr: core.ErrAttestContract, wantNative: fixtureErrorWrite},
		{name: "writer panic is contained", makeBody: hostileBodyFixture(hostileBodyWritePanic), wantErr: core.ErrAttestContract},
		{name: "zero length write cannot fake body", makeBody: hostileBodyFixture(hostileBodyZeroWrite), wantErr: core.ErrAttestContract},
		{name: "nil body rejects", makeBody: nilBodyFixture, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := tc.makeBody()
			key := deterministicPrivateKey(t, "canonical-body")
			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
				Body:   body,
				Signer: key,
			})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Sign() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("attest.Sign() native error = %v, want %v", gotErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if gotEnvelope != (attest.Envelope[testDomain]{}) {
					t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
				}
				return
			}
			digest := sha256.New()
			if err := body.WriteCanonical(digest); err != nil {
				t.Fatalf("independent body hash error = %v, want nil", err)
			}
			var rawDigest [sha256.Size]byte
			copy(rawDigest[:], digest.Sum(nil))
			wantDigest := core.NewSHA256Digest(rawDigest)
			if gotEnvelope.BodySHA256 != wantDigest {
				t.Fatalf("signed SHA-256 = %v, want %v", gotEnvelope.BodySHA256, wantDigest)
			}
			if gotEnvelope.Domain != body.AttestationDomain() || gotEnvelope.Signer != mustPublicKey(t, key) {
				t.Fatalf("signed identity = (%v, %v), want caller domain and signer", gotEnvelope.Domain, gotEnvelope.Signer)
			}
			signature, err := gotEnvelope.Signature.Bytes()
			if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), independentAttestationFrame(t, gotEnvelope), signature[:]) {
				t.Fatalf("signed body independent verification error = %v, want authentic frame", err)
			}
			gotBytes, gotBytesErr := gotEnvelope.BodyLength.Uint64()
			if gotBytesErr != nil || gotBytes != tc.wantBytes {
				t.Fatalf(
					"Envelope.BodyLength.Uint64() = (%d, %v), want (%d, nil)",
					gotBytes,
					gotBytesErr,
					tc.wantBytes,
				)
			}
		})
	}
}

func TestSignPublicPrivateKeyBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeKey func() ed25519.PrivateKey
		wantErr error
		name    string
	}{
		{name: "exact standard private key signs", makeKey: fixedPrivateKeyFixture("valid-private-key")},
		{name: "nil private key rejects", makeKey: nilPrivateKeyFixture, wantErr: core.ErrAttestContract},
		{name: "seed length rejects", makeKey: sizedPrivateKeyFixture(ed25519.SeedSize), wantErr: core.ErrAttestContract},
		{name: "one byte short rejects", makeKey: sizedPrivateKeyFixture(ed25519.PrivateKeySize - 1), wantErr: core.ErrAttestContract},
		{name: "one byte long rejects", makeKey: sizedPrivateKeyFixture(ed25519.PrivateKeySize + 1), wantErr: core.ErrAttestContract},
		{name: "corrupt first public-half byte rejects", makeKey: corruptPrivateKeyFixture(ed25519.SeedSize), wantErr: core.ErrAttestContract},
		{name: "corrupt final public-half byte rejects", makeKey: corruptPrivateKeyFixture(ed25519.PrivateKeySize - 1), wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := tc.makeKey()
			body := literalBody{domain: testDomainPrimary, value: []byte("key-boundary")}
			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: key})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Sign() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && gotEnvelope != (attest.Envelope[testDomain]{}) {
				t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
			}
			if tc.wantErr == nil {
				proof, err := attest.Verify(attest.VerifyRequest[testDomain]{Body: body, Envelope: gotEnvelope, TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key))})
				if err != nil {
					t.Fatalf("Verify(admitted private key) error = %v, want nil", err)
				}
				retained, err := proof.Envelope()
				if err != nil || retained != gotEnvelope {
					t.Fatalf("admitted key proof = (%+v, %v), want %+v", retained, err, gotEnvelope)
				}
			}
		})
	}
}

func TestSigningDomainPublicCanonicalTextBoundaryMatrix(t *testing.T) {
	t.Parallel()

	maximumLetters := strings.Repeat("a", attest.SigningDomainMaximumBytes)
	cases := []struct {
		text       string
		wantErr    error
		wantNative error
		name       string
		mode       textDomainMode
	}{
		{name: "single lowercase letter accepted", text: "a"},
		{name: "single digit accepted", text: "0"},
		{name: "letter digit accepted", text: "a0"},
		{name: "internal hyphen accepted", text: "a-b"},
		{name: "two separated hyphens accepted", text: "a-b-c"},
		{name: "upper lowercase alphabet endpoint accepted", text: "z"},
		{name: "upper digit alphabet endpoint accepted", text: "9"},
		{name: "maximum minus one letters accepted", text: strings.Repeat("a", attest.SigningDomainMaximumBytes-1)},
		{name: "exact maximum letters accepted", text: maximumLetters},
		{name: "exact maximum alternating accepted", text: alternatingDomainText(attest.SigningDomainMaximumBytes)},
		{name: "maximum ending digit accepted", text: strings.Repeat("a", attest.SigningDomainMaximumBytes-1) + "9"},
		{name: "empty rejected", wantErr: core.ErrAttestContract},
		{name: "one above maximum rejected", text: maximumLetters + "a", wantErr: core.ErrAttestContract},
		{name: "leading hyphen rejected", text: "-a", wantErr: core.ErrAttestContract},
		{name: "trailing hyphen rejected", text: "a-", wantErr: core.ErrAttestContract},
		{name: "adjacent hyphens rejected", text: "a--b", wantErr: core.ErrAttestContract},
		{name: "uppercase rejected", text: "A", wantErr: core.ErrAttestContract},
		{name: "one below lowercase alphabet rejected", text: "`", wantErr: core.ErrAttestContract},
		{name: "one above lowercase alphabet rejected", text: "{", wantErr: core.ErrAttestContract},
		{name: "one above digit alphabet rejected", text: ":", wantErr: core.ErrAttestContract},
		{name: "underscore rejected", text: "a_b", wantErr: core.ErrAttestContract},
		{name: "slash rejected", text: "a/b", wantErr: core.ErrAttestContract},
		{name: "space rejected", text: "a b", wantErr: core.ErrAttestContract},
		{name: "leading whitespace rejected", text: " a", wantErr: core.ErrAttestContract},
		{name: "trailing whitespace rejected", text: "a ", wantErr: core.ErrAttestContract},
		{name: "newline rejected", text: "a\nb", wantErr: core.ErrAttestContract},
		{name: "nul rejected", text: "a\x00b", wantErr: core.ErrAttestContract},
		{name: "multibyte rejected", text: "é", wantErr: core.ErrAttestContract},
		{name: "invalid utf8 rejected", text: string([]byte{0xff}), wantErr: core.ErrAttestContract},
		{name: "domain validation error remains reachable", mode: textDomainValidationError, wantErr: core.ErrAttestContract, wantNative: fixtureErrorValidation},
		{name: "domain validation panic is contained", mode: textDomainValidationPanic, wantErr: core.ErrAttestContract},
		{name: "domain marshal error remains reachable", mode: textDomainMarshalError, wantErr: core.ErrAttestContract, wantNative: fixtureErrorMarshal},
		{name: "domain marshal panic is contained", mode: textDomainMarshalPanic, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := textDomainBody{domain: textDomain{text: tc.text, mode: tc.mode}}
			key := deterministicPrivateKey(t, "domain-text")
			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[textDomain]{Body: body, Signer: key})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Sign() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("attest.Sign() native error = %v, want %v", gotErr, tc.wantNative)
			}
			if tc.wantErr != nil && gotEnvelope != (attest.Envelope[textDomain]{}) {
				t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
			}
			if tc.wantErr == nil {
				if gotEnvelope.Domain != body.domain {
					t.Fatalf("signed domain = %+v, want %+v", gotEnvelope.Domain, body.domain)
				}
				signature, err := gotEnvelope.Signature.Bytes()
				if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), independentAttestationFrame(t, gotEnvelope), signature[:]) {
					t.Fatalf("signed domain independent verification error = %v, want authentic frame", err)
				}
				proof, err := attest.Verify(attest.VerifyRequest[textDomain]{Body: body, Envelope: gotEnvelope, TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key))})
				if err != nil {
					t.Fatalf("Verify(admitted domain) error = %v, want nil", err)
				}
				retained, err := proof.Envelope()
				if err != nil || retained != gotEnvelope {
					t.Fatalf("admitted domain proof = (%+v, %v), want %+v", retained, err, gotEnvelope)
				}
			}
		})
	}
}

func TestTrustedKeysPublicCardinalityAndIsolationMatrix(t *testing.T) {
	t.Parallel()

	allKeys := deterministicPublicKeys(t, attest.TrustedKeyMaximumCount+1)
	cases := []struct {
		makeKeys func() []core.Ed25519PublicKey
		wantErr  error
		name     string
	}{
		{name: "one trusted key accepted", makeKeys: keySliceFixture(allKeys[:1])},
		{name: "two trusted keys accepted", makeKeys: keySliceFixture(allKeys[:2])},
		{name: "maximum minus one accepted", makeKeys: keySliceFixture(allKeys[:attest.TrustedKeyMaximumCount-1])},
		{name: "exact maximum accepted", makeKeys: keySliceFixture(allKeys[:attest.TrustedKeyMaximumCount])},
		{name: "reverse order accepted", makeKeys: reversedKeySliceFixture(allKeys[:4])},
		{name: "nil set rejected", makeKeys: nilKeySliceFixture, wantErr: core.ErrAttestContract},
		{name: "maximum plus one rejected", makeKeys: keySliceFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "zero key first rejected", makeKeys: zeroKeyFirstFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "zero key middle rejected", makeKeys: zeroKeyMiddleFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "zero key last rejected", makeKeys: zeroKeyLastFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "adjacent duplicate rejected", makeKeys: adjacentDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "nonadjacent duplicate rejected", makeKeys: nonadjacentDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "duplicate at maximum frontier rejected", makeKeys: maximumDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := tc.makeKeys()
			gotTrusted, gotErr := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: input})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.NewTrustedKeys() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if gotTrusted != (attest.TrustedKeys{}) {
					t.Fatalf("attest.NewTrustedKeys() = %+v, want zero", gotTrusted)
				}
				return
			}
			if gotErr := gotTrusted.Validate(); gotErr != nil {
				t.Fatalf("TrustedKeys.Validate() error = %v, want nil", gotErr)
			}
			original := slices.Clone(input)
			for index := range input {
				input[index] = allKeys[len(allKeys)-1]
			}
			if gotErr := gotTrusted.Validate(); gotErr != nil {
				t.Fatalf("TrustedKeys.Validate() after caller mutation error = %v, want nil", gotErr)
			}
			body := literalBody{domain: testDomainPrimary, value: []byte("trust isolation")}
			for index, key := range original {
				originalSigner := privateKeyForTrustedPublicKey(t, key, allKeys)
				if got := mustPublicKey(t, originalSigner); got != key {
					t.Fatalf("original signer at %d = %v, want %v", index, got, key)
				}
				originalEnvelope := mustEnvelope(t, body, originalSigner)
				proof, err := attest.Verify(attest.VerifyRequest[testDomain]{Body: body, Envelope: originalEnvelope, TrustedKeys: gotTrusted})
				if err != nil {
					t.Fatalf("Verify(retained signer at %d) error = %v, want nil", index, err)
				}
				retained, err := proof.Envelope()
				if err != nil || retained != originalEnvelope {
					t.Fatalf("retained signer proof at %d = (%+v, %v), want %+v", index, retained, err, originalEnvelope)
				}
			}
			replacementSigner := privateKeyForTrustedPublicKey(t, allKeys[len(allKeys)-1], allKeys)
			replacementEnvelope := mustEnvelope(t, body, replacementSigner)
			gotReplacement, gotReplacementErr := attest.Verify(attest.VerifyRequest[testDomain]{
				Body:        body,
				Envelope:    replacementEnvelope,
				TrustedKeys: gotTrusted,
			})
			if !errors.Is(gotReplacementErr, core.ErrAttestVerification) {
				t.Fatalf(
					"attest.Verify(replacement signer after mutation) error = %v, want %v",
					gotReplacementErr,
					core.ErrAttestVerification,
				)
			}
			if gotReplacement != (attest.Verified[testDomain]{}) {
				t.Fatalf("attest.Verify(replacement signer) proof = %+v, want zero", gotReplacement)
			}
		})
	}
}

func TestSignCopiesPrivateKeyBeforeCallingConsumerBody(t *testing.T) {
	t.Parallel()

	privateKey := deterministicPrivateKey(t, "private-key-isolation")
	wantSigner := mustPublicKey(t, privateKey)
	body := keyMutatingBody{key: privateKey}
	gotEnvelope, gotSignErr := attest.Sign(attest.SignRequest[testDomain]{
		Body:   body,
		Signer: privateKey,
	})
	if gotSignErr != nil {
		t.Fatalf("attest.Sign() error = %v, want nil", gotSignErr)
	}
	if gotEnvelope.Signer != wantSigner {
		t.Fatalf("Envelope.Signer after body key mutation = %v, want %v", gotEnvelope.Signer, wantSigner)
	}
	gotVerified, gotErr := attest.Verify(attest.VerifyRequest[testDomain]{
		Body:        literalBody{domain: testDomainPrimary, value: []byte("x")},
		Envelope:    gotEnvelope,
		TrustedKeys: mustTrustedKeys(t, wantSigner),
	})
	if gotErr != nil {
		t.Fatalf("attest.Verify() after body key mutation error = %v, want nil", gotErr)
	}
	if gotErr := gotVerified.Validate(); gotErr != nil {
		t.Fatalf("Verified.Validate() error = %v, want nil", gotErr)
	}
}

func TestSignRequestValidatePublicBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeRequest func(testing.TB) attest.SignRequest[testDomain]
		wantErr     error
		wantNative  error
		name        string
	}{
		{name: "one byte body shape accepts", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), fixedPrivateKeyFixture("validate-one"))},
		{name: "alternate domain body shape accepts", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainAlternate, []byte("x")), fixedPrivateKeyFixture("validate-alternate"))},
		{name: "nil body rejects", makeRequest: signValidationRequestFixture(nilBodyFixture, fixedPrivateKeyFixture("validate-nil-body")), wantErr: core.ErrAttestContract},
		{name: "unknown body domain rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainUnknown, []byte("x")), fixedPrivateKeyFixture("validate-unknown-domain")), wantErr: core.ErrAttestContract},
		{name: "body validation error remains reachable", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyValidationError), fixedPrivateKeyFixture("validate-body-error")), wantErr: core.ErrAttestContract, wantNative: fixtureErrorValidation},
		{name: "body validation panic is contained", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyValidationPanic), fixedPrivateKeyFixture("validate-body-panic")), wantErr: core.ErrAttestContract},
		{name: "body domain panic is contained", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyDomainPanic), fixedPrivateKeyFixture("validate-domain-panic")), wantErr: core.ErrAttestContract},
		{name: "nil private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), nilPrivateKeyFixture), wantErr: core.ErrAttestContract},
		{name: "seed sized private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), sizedPrivateKeyFixture(ed25519.SeedSize)), wantErr: core.ErrAttestContract},
		{name: "one byte short private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), sizedPrivateKeyFixture(ed25519.PrivateKeySize-1)), wantErr: core.ErrAttestContract},
		{name: "one byte long private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), sizedPrivateKeyFixture(ed25519.PrivateKeySize+1)), wantErr: core.ErrAttestContract},
		{name: "inconsistent private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), corruptPrivateKeyFixture(ed25519.SeedSize)), wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.makeRequest(t).Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("SignRequest.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("SignRequest.Validate() native error = %v, want %v", gotErr, tc.wantNative)
			}
		})
	}
}

func TestVerifyRequestValidatePublicBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr       error
		wantNative    error
		wantVerifyErr error
		name          string
		mutation      verifyValidationMutation
	}{
		{name: "matching structural request accepts"},
		{name: "changed body bytes remain a verification concern", mutation: verifyValidationBodyBytes, wantVerifyErr: core.ErrAttestVerification},
		{name: "changed body extent remains a verification concern", mutation: verifyValidationBodyExtent, wantVerifyErr: core.ErrAttestVerification},
		{name: "changed body domain remains a verification concern", mutation: verifyValidationBodyDomain, wantVerifyErr: core.ErrAttestVerification},
		{name: "changed signature remains a verification concern", mutation: verifyValidationSignature, wantVerifyErr: core.ErrAttestVerification},
		{name: "untrusted structurally valid signer remains a verification concern", mutation: verifyValidationSigner, wantVerifyErr: core.ErrAttestVerification},
		{name: "writer error remains an execution concern", mutation: verifyValidationWriterError, wantVerifyErr: core.ErrAttestContract},
		{name: "writer panic remains an execution concern", mutation: verifyValidationWriterPanic, wantVerifyErr: core.ErrAttestContract},
		{name: "zero write remains an execution concern", mutation: verifyValidationZeroWrite, wantVerifyErr: core.ErrAttestContract},
		{name: "maximum body shape defers mismatched extent to execution", mutation: verifyValidationAlternateBody, wantVerifyErr: core.ErrAttestVerification},
		{name: "zero envelope rejects", mutation: verifyValidationZeroEnvelope, wantErr: core.ErrAttestContract},
		{name: "zero trust rejects", mutation: verifyValidationZeroTrust, wantErr: core.ErrAttestContract},
		{name: "nil body rejects", mutation: verifyValidationNilBody, wantErr: core.ErrAttestContract},
		{name: "zero signer rejects", mutation: verifyValidationZeroSigner, wantErr: core.ErrAttestContract},
		{name: "zero body length rejects", mutation: verifyValidationZeroLength, wantErr: core.ErrAttestContract},
		{name: "zero body digest rejects", mutation: verifyValidationZeroDigest, wantErr: core.ErrAttestContract},
		{name: "zero signature rejects", mutation: verifyValidationZeroSignature, wantErr: core.ErrAttestContract},
		{name: "unknown body domain rejects", mutation: verifyValidationUnknownBodyDomain, wantErr: core.ErrAttestContract},
		{name: "body validation error remains reachable", mutation: verifyValidationBodyError, wantErr: core.ErrAttestContract, wantNative: fixtureErrorValidation},
		{name: "body validation panic is contained", mutation: verifyValidationBodyPanic, wantErr: core.ErrAttestContract},
		{name: "body domain panic is contained", mutation: verifyValidationDomainPanic, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := verifyValidationRequestFixture(t, tc.mutation)
			gotErr := request.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("VerifyRequest.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("VerifyRequest.Validate() native error = %v, want %v", gotErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				return
			}
			proof, err := attest.Verify(request)
			if !errors.Is(err, tc.wantVerifyErr) {
				t.Fatalf("Verify(admitted shape) error = %v, want %v", err, tc.wantVerifyErr)
			}
			if tc.wantVerifyErr != nil {
				if proof != (attest.Verified[testDomain]{}) {
					t.Fatalf("failed execution proof = %+v, want zero", proof)
				}
				return
			}
			retained, err := proof.Envelope()
			if err != nil || retained != request.Envelope {
				t.Fatalf("matching execution proof = (%+v, %v), want %+v", retained, err, request.Envelope)
			}
		})
	}
}

type verifyValidationMutation uint8

const (
	verifyValidationNone verifyValidationMutation = iota
	verifyValidationBodyBytes
	verifyValidationBodyExtent
	verifyValidationBodyDomain
	verifyValidationSignature
	verifyValidationSigner
	verifyValidationWriterError
	verifyValidationWriterPanic
	verifyValidationZeroWrite
	verifyValidationAlternateBody
	verifyValidationZeroEnvelope
	verifyValidationZeroTrust
	verifyValidationNilBody
	verifyValidationZeroSigner
	verifyValidationZeroLength
	verifyValidationZeroDigest
	verifyValidationZeroSignature
	verifyValidationUnknownBodyDomain
	verifyValidationBodyError
	verifyValidationBodyPanic
	verifyValidationDomainPanic
)

func signValidationRequestFixture(
	makeBody func() attest.CanonicalBody[testDomain],
	makeKey func() ed25519.PrivateKey,
) func(testing.TB) attest.SignRequest[testDomain] {
	return func(testing.TB) attest.SignRequest[testDomain] {
		return attest.SignRequest[testDomain]{Body: makeBody(), Signer: makeKey()}
	}
}

func verifyValidationRequestFixture(
	t testing.TB,
	mutation verifyValidationMutation,
) attest.VerifyRequest[testDomain] {
	t.Helper()
	privateKey := deterministicPrivateKey(t, "verify-request-validation")
	body := literalBody{domain: testDomainPrimary, value: []byte("x")}
	request := attest.VerifyRequest[testDomain]{
		Body:        body,
		Envelope:    mustEnvelope(t, body, privateKey),
		TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, privateKey)),
	}
	switch mutation {
	case verifyValidationNone:
	case verifyValidationBodyBytes:
		request.Body = literalBody{domain: testDomainPrimary, value: []byte("y")}
	case verifyValidationBodyExtent:
		request.Body = literalBody{domain: testDomainPrimary, value: []byte("xy")}
	case verifyValidationBodyDomain:
		request.Body = literalBody{domain: testDomainAlternate, value: []byte("x")}
	case verifyValidationSignature:
		request.Envelope.Signature = mutateSignature(t, request.Envelope.Signature)
	case verifyValidationSigner:
		request.Envelope.Signer = mustPublicKey(t, deterministicPrivateKey(t, "verify-request-other"))
	case verifyValidationWriterError:
		request.Body = hostileBody{mode: hostileBodyWriteError}
	case verifyValidationWriterPanic:
		request.Body = hostileBody{mode: hostileBodyWritePanic}
	case verifyValidationZeroWrite:
		request.Body = hostileBody{mode: hostileBodyZeroWrite}
	case verifyValidationAlternateBody:
		request.Body = sizedBody{size: attest.CanonicalBodyMaximumBytes, chunkSize: 8192, domain: testDomainPrimary}
	case verifyValidationZeroEnvelope:
		request.Envelope = attest.Envelope[testDomain]{}
	case verifyValidationZeroTrust:
		request.TrustedKeys = attest.TrustedKeys{}
	case verifyValidationNilBody:
		request.Body = nil
	case verifyValidationZeroSigner:
		request.Envelope.Signer = core.Ed25519PublicKey{}
	case verifyValidationZeroLength:
		request.Envelope.BodyLength = core.ByteCount{}
	case verifyValidationZeroDigest:
		request.Envelope.BodySHA256 = core.SHA256Digest{}
	case verifyValidationZeroSignature:
		request.Envelope.Signature = attest.Signature{}
	case verifyValidationUnknownBodyDomain:
		request.Body = literalBody{domain: testDomainUnknown, value: []byte("x")}
	case verifyValidationBodyError:
		request.Body = hostileBody{mode: hostileBodyValidationError}
	case verifyValidationBodyPanic:
		request.Body = hostileBody{mode: hostileBodyValidationPanic}
	case verifyValidationDomainPanic:
		request.Body = hostileBody{mode: hostileBodyDomainPanic}
	default:
		t.Fatalf("verify validation mutation = %d, want admitted mutation", mutation)
	}
	return request
}

type textDomain struct {
	text string
	mode textDomainMode
}

type textDomainMode uint8

const (
	textDomainNormal textDomainMode = iota
	textDomainValidationError
	textDomainValidationPanic
	textDomainMarshalError
	textDomainMarshalPanic
)

func (d textDomain) Validate() error {
	switch d.mode {
	case textDomainNormal, textDomainMarshalError, textDomainMarshalPanic:
		return nil
	case textDomainValidationError:
		return fixtureErrorValidation
	case textDomainValidationPanic:
		panic(fixtureErrorValidation)
	default:
		return core.ErrAttestContract
	}
}

func (d textDomain) MarshalText() ([]byte, error) {
	switch d.mode {
	case textDomainNormal:
		return []byte(d.text), nil
	case textDomainMarshalError:
		return nil, fixtureErrorMarshal
	case textDomainMarshalPanic:
		panic(fixtureErrorMarshal)
	default:
		return nil, core.ErrAttestContract
	}
}

func (textDomain) ParseCanonicalText(text []byte) (textDomain, error) {
	return textDomain{text: string(text)}, nil
}

type textDomainBody struct {
	domain textDomain
}

func (textDomainBody) Validate() error {
	return nil
}

func (b textDomainBody) AttestationDomain() textDomain {
	return b.domain
}

func (textDomainBody) WriteCanonical(destination io.Writer) error {
	_, err := io.WriteString(destination, "x")
	return err
}

func sizedBodyFixture(
	size int,
	chunkSize int,
	domain testDomain,
	ignoreErr bool,
) func() attest.CanonicalBody[testDomain] {
	return func() attest.CanonicalBody[testDomain] {
		return sizedBody{
			size:      size,
			chunkSize: chunkSize,
			domain:    domain,
			ignoreErr: ignoreErr,
		}
	}
}

func literalBodyFixture(
	domain testDomain,
	value []byte,
) func() attest.CanonicalBody[testDomain] {
	return func() attest.CanonicalBody[testDomain] {
		return literalBody{domain: domain, value: slices.Clone(value)}
	}
}

func hostileBodyFixture(mode hostileBodyMode) func() attest.CanonicalBody[testDomain] {
	return func() attest.CanonicalBody[testDomain] {
		return hostileBody{mode: mode}
	}
}

func nilBodyFixture() attest.CanonicalBody[testDomain] {
	return nil
}

func fixedPrivateKeyFixture(label string) func() ed25519.PrivateKey {
	return func() ed25519.PrivateKey {
		seed := sha256.Sum256([]byte(label))
		return ed25519.NewKeyFromSeed(seed[:])
	}
}

func nilPrivateKeyFixture() ed25519.PrivateKey {
	return nil
}

func sizedPrivateKeyFixture(size int) func() ed25519.PrivateKey {
	return func() ed25519.PrivateKey {
		return make(ed25519.PrivateKey, size)
	}
}

func corruptPrivateKeyFixture(index int) func() ed25519.PrivateKey {
	return func() ed25519.PrivateKey {
		privateKey := fixedPrivateKeyFixture("corrupt-private-key")()
		privateKey[index] ^= 1
		return privateKey
	}
}

func alternatingDomainText(size int) string {
	value := make([]byte, size)
	for index := range value {
		if index%2 == 0 {
			value[index] = 'a'
			continue
		}
		value[index] = '-'
	}
	value[len(value)-1] = 'z'
	return string(value)
}

func deterministicPublicKeys(t testing.TB, count int) []core.Ed25519PublicKey {
	t.Helper()
	keys := make([]core.Ed25519PublicKey, count)
	for index := range keys {
		label := "trusted-key-" + strconv.Itoa(index+1)
		keys[index] = mustPublicKey(t, deterministicPrivateKey(t, label))
	}
	return keys
}

func privateKeyForTrustedPublicKey(
	t testing.TB,
	target core.Ed25519PublicKey,
	keys []core.Ed25519PublicKey,
) ed25519.PrivateKey {
	t.Helper()
	for index, key := range keys {
		if key == target {
			return deterministicPrivateKey(t, "trusted-key-"+strconv.Itoa(index+1))
		}
	}
	t.Fatalf("trusted public key = %v, want one of %v", target, keys)
	return nil
}

func keySliceFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return func() []core.Ed25519PublicKey {
		return slices.Clone(keys)
	}
}

func reversedKeySliceFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return func() []core.Ed25519PublicKey {
		result := slices.Clone(keys)
		slices.Reverse(result)
		return result
	}
}

func nilKeySliceFixture() []core.Ed25519PublicKey {
	return nil
}

func zeroKeyFirstFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return keySliceFixture([]core.Ed25519PublicKey{{}, keys[0]})
}

func zeroKeyMiddleFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return keySliceFixture([]core.Ed25519PublicKey{keys[0], {}, keys[1]})
}

func zeroKeyLastFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return keySliceFixture([]core.Ed25519PublicKey{keys[0], {}})
}

func adjacentDuplicateKeyFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return keySliceFixture([]core.Ed25519PublicKey{keys[0], keys[0]})
}

func nonadjacentDuplicateKeyFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return keySliceFixture([]core.Ed25519PublicKey{keys[0], keys[1], keys[0]})
}

func maximumDuplicateKeyFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return func() []core.Ed25519PublicKey {
		result := slices.Clone(keys[:attest.TrustedKeyMaximumCount])
		result[len(result)-1] = result[0]
		return result
	}
}
