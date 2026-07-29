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
		{name: "future domain rejects before signing", makeBody: literalBodyFixture(testDomain(255), []byte("x")), wantErr: core.ErrAttestContract},
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

			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
				Body: tc.makeBody(),
				Key:  deterministicPrivateKey(t, "canonical-body"),
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
		{name: "empty private key rejects", makeKey: emptyPrivateKeyFixture, wantErr: core.ErrAttestContract},
		{name: "seed length rejects", makeKey: sizedPrivateKeyFixture(ed25519.SeedSize), wantErr: core.ErrAttestContract},
		{name: "one byte short rejects", makeKey: sizedPrivateKeyFixture(ed25519.PrivateKeySize - 1), wantErr: core.ErrAttestContract},
		{name: "one byte long rejects", makeKey: sizedPrivateKeyFixture(ed25519.PrivateKeySize + 1), wantErr: core.ErrAttestContract},
		{name: "corrupt first public-half byte rejects", makeKey: corruptPrivateKeyFixture(ed25519.SeedSize), wantErr: core.ErrAttestContract},
		{name: "corrupt final public-half byte rejects", makeKey: corruptPrivateKeyFixture(ed25519.PrivateKeySize - 1), wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
				Body: literalBody{domain: testDomainPrimary, value: []byte("key-boundary")},
				Key:  tc.makeKey(),
			})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Sign() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && gotEnvelope != (attest.Envelope[testDomain]{}) {
				t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
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
		{name: "numeric interior accepted", text: "a-2026-z"},
		{name: "mixed long token accepted", text: "release-manifest-2026-v1"},
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

			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[textDomain]{
				Body: textDomainBody{domain: textDomain{text: tc.text, mode: tc.mode}},
				Key:  deterministicPrivateKey(t, "domain-text"),
			})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Sign() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("attest.Sign() native error = %v, want %v", gotErr, tc.wantNative)
			}
			if tc.wantErr != nil && gotEnvelope != (attest.Envelope[textDomain]{}) {
				t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
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
		{name: "three trusted keys accepted", makeKeys: keySliceFixture(allKeys[:3])},
		{name: "four trusted keys accepted", makeKeys: keySliceFixture(allKeys[:4])},
		{name: "eight trusted keys accepted", makeKeys: keySliceFixture(allKeys[:8])},
		{name: "maximum minus two accepted", makeKeys: keySliceFixture(allKeys[:attest.TrustedKeyMaximumCount-2])},
		{name: "maximum minus one accepted", makeKeys: keySliceFixture(allKeys[:attest.TrustedKeyMaximumCount-1])},
		{name: "exact maximum accepted", makeKeys: keySliceFixture(allKeys[:attest.TrustedKeyMaximumCount])},
		{name: "reverse order accepted", makeKeys: reversedKeySliceFixture(allKeys[:4])},
		{name: "interior subset accepted", makeKeys: keySliceFixture(allKeys[2:6])},
		{name: "nil set rejected", makeKeys: nilKeySliceFixture, wantErr: core.ErrAttestContract},
		{name: "empty set rejected", makeKeys: emptyKeySliceFixture, wantErr: core.ErrAttestContract},
		{name: "maximum plus one rejected", makeKeys: keySliceFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "zero key first rejected", makeKeys: zeroKeyFirstFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "zero key middle rejected", makeKeys: zeroKeyMiddleFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "zero key last rejected", makeKeys: zeroKeyLastFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "adjacent duplicate rejected", makeKeys: adjacentDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "nonadjacent duplicate rejected", makeKeys: nonadjacentDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "duplicate at maximum frontier rejected", makeKeys: maximumDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
		{name: "three identical keys reject every duplicate", makeKeys: tripleDuplicateKeyFixture(allKeys), wantErr: core.ErrAttestContract},
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
			originalFirst := input[0]
			input[0] = allKeys[len(allKeys)-1]
			if gotErr := gotTrusted.Validate(); gotErr != nil {
				t.Fatalf("TrustedKeys.Validate() after caller mutation error = %v, want nil", gotErr)
			}
			body := literalBody{domain: testDomainPrimary, value: []byte("trust isolation")}
			originalSigner := privateKeyForTrustedPublicKey(t, originalFirst, allKeys)
			if mustPublicKey(t, originalSigner) != originalFirst {
				t.Fatalf(
					"original signer = %v, want first input %v",
					mustPublicKey(t, originalSigner),
					originalFirst,
				)
			}
			originalEnvelope := mustEnvelope(t, body, originalSigner)
			gotVerified, gotVerifyErr := attest.Verify(attest.VerifyRequest[testDomain]{
				Body:        body,
				Envelope:    originalEnvelope,
				TrustedKeys: gotTrusted,
			})
			if gotVerifyErr != nil {
				t.Fatalf("attest.Verify(original signer after mutation) error = %v, want nil", gotVerifyErr)
			}
			if gotErr := gotVerified.Validate(); gotErr != nil {
				t.Fatalf("Verified.Validate() error = %v, want nil", gotErr)
			}
			replacementSigner := deterministicPrivateKey(t, "trusted-key-17")
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
		Body: body,
		Key:  privateKey,
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

func TestCanonicalWriterRetainedCapabilityIsClosed(t *testing.T) {
	t.Parallel()

	var retained io.Writer
	gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
		Body: retainingBody{retained: &retained},
		Key:  deterministicPrivateKey(t, "retained-writer"),
	})
	if gotErr != nil {
		t.Fatalf("attest.Sign() error = %v, want nil", gotErr)
	}
	if retained == nil {
		t.Fatal("WriteCanonical() retained writer = nil, want closed capability")
	}
	gotWritten, gotWriteErr := retained.Write([]byte("late mutation"))
	if gotWritten != 0 || !errors.Is(gotWriteErr, core.ErrAttestContract) {
		t.Fatalf(
			"retained Write() = (%d, %v), want (0, %v)",
			gotWritten,
			gotWriteErr,
			core.ErrAttestContract,
		)
	}
	if gotErr := gotEnvelope.Validate(); gotErr != nil {
		t.Fatalf("Envelope.Validate() after retained write error = %v, want nil", gotErr)
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
		{name: "empty body shape accepts without execution", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, nil), fixedPrivateKeyFixture("validate-empty"))},
		{name: "zero sized body shape accepts without execution", makeRequest: signValidationRequestFixture(sizedBodyFixture(0, 1, testDomainPrimary, false), fixedPrivateKeyFixture("validate-zero"))},
		{name: "oversized body shape accepts without execution", makeRequest: signValidationRequestFixture(sizedBodyFixture(attest.CanonicalBodyMaximumBytes+1, 1, testDomainPrimary, false), fixedPrivateKeyFixture("validate-oversized"))},
		{name: "writer error body shape accepts without execution", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyWriteError), fixedPrivateKeyFixture("validate-write-error"))},
		{name: "writer panic body shape accepts without execution", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyWritePanic), fixedPrivateKeyFixture("validate-write-panic"))},
		{name: "zero writer body shape accepts without execution", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyZeroWrite), fixedPrivateKeyFixture("validate-zero-write"))},
		{name: "alternate domain body shape accepts", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainAlternate, []byte("x")), fixedPrivateKeyFixture("validate-alternate"))},
		{name: "maximum body shape accepts without execution", makeRequest: signValidationRequestFixture(sizedBodyFixture(attest.CanonicalBodyMaximumBytes, 8192, testDomainPrimary, false), fixedPrivateKeyFixture("validate-maximum"))},
		{name: "prime body shape accepts without execution", makeRequest: signValidationRequestFixture(sizedBodyFixture(7919, 113, testDomainPrimary, false), fixedPrivateKeyFixture("validate-prime"))},
		{name: "nil body rejects", makeRequest: signValidationRequestFixture(nilBodyFixture, fixedPrivateKeyFixture("validate-nil-body")), wantErr: core.ErrAttestContract},
		{name: "unknown body domain rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainUnknown, []byte("x")), fixedPrivateKeyFixture("validate-unknown-domain")), wantErr: core.ErrAttestContract},
		{name: "body validation error remains reachable", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyValidationError), fixedPrivateKeyFixture("validate-body-error")), wantErr: core.ErrAttestContract, wantNative: fixtureErrorValidation},
		{name: "body validation panic is contained", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyValidationPanic), fixedPrivateKeyFixture("validate-body-panic")), wantErr: core.ErrAttestContract},
		{name: "body domain panic is contained", makeRequest: signValidationRequestFixture(hostileBodyFixture(hostileBodyDomainPanic), fixedPrivateKeyFixture("validate-domain-panic")), wantErr: core.ErrAttestContract},
		{name: "nil private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), nilPrivateKeyFixture), wantErr: core.ErrAttestContract},
		{name: "empty private key rejects", makeRequest: signValidationRequestFixture(literalBodyFixture(testDomainPrimary, []byte("x")), emptyPrivateKeyFixture), wantErr: core.ErrAttestContract},
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
		wantErr    error
		wantNative error
		name       string
		mutation   verifyValidationMutation
	}{
		{name: "matching structural request accepts"},
		{name: "changed body bytes remain a verification concern", mutation: verifyValidationBodyBytes},
		{name: "changed body extent remains a verification concern", mutation: verifyValidationBodyExtent},
		{name: "changed body domain remains a verification concern", mutation: verifyValidationBodyDomain},
		{name: "changed signature remains a verification concern", mutation: verifyValidationSignature},
		{name: "untrusted structurally valid signer remains a verification concern", mutation: verifyValidationSigner},
		{name: "writer error remains an execution concern", mutation: verifyValidationWriterError},
		{name: "writer panic remains an execution concern", mutation: verifyValidationWriterPanic},
		{name: "zero write remains an execution concern", mutation: verifyValidationZeroWrite},
		{name: "alternate valid body shape accepts", mutation: verifyValidationAlternateBody},
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
		return attest.SignRequest[testDomain]{Body: makeBody(), Key: makeKey()}
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

func emptyPrivateKeyFixture() ed25519.PrivateKey {
	return ed25519.PrivateKey{}
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

func emptyKeySliceFixture() []core.Ed25519PublicKey {
	return []core.Ed25519PublicKey{}
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

func tripleDuplicateKeyFixture(keys []core.Ed25519PublicKey) func() []core.Ed25519PublicKey {
	return keySliceFixture([]core.Ed25519PublicKey{keys[0], keys[0], keys[0]})
}
