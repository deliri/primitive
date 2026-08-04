package attest_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

func TestEnvelopeJSONPublicNormalizationMatrix(t *testing.T) {
	t.Parallel()

	canonical := canonicalEnvelopeJSONFixture(t)
	cases := []struct {
		makeInput envelopeJSONFixture
		name      string
	}{
		{name: "canonical projection closes", makeInput: cloneJSONFixture},
		{name: "leading whitespace normalizes", makeInput: prefixJSONFixture(" \n\t")},
		{name: "trailing whitespace normalizes", makeInput: suffixJSONFixture("\r\n ")},
		{name: "surrounding whitespace normalizes", makeInput: surroundJSONFixture("\n ", "\t")},
		{name: "reverse member order normalizes", makeInput: reverseEnvelopeMembersFixture},
		{name: "domain member moved last normalizes", makeInput: domainLastFixture},
		{name: "spaces around separators normalize", makeInput: spacedEnvelopeFixture},
		{name: "escaped domain character normalizes", makeInput: escapedDomainFixture},
		{name: "escaped solidus-free text normalizes", makeInput: escapedLetterFixture},
		{name: "all harmless forms together normalize", makeInput: combinedNormalizationFixture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := tc.makeInput(t, canonical)
			var gotEnvelope attest.Envelope[testDomain]
			gotErr := gotEnvelope.UnmarshalJSON(input)
			if gotErr != nil {
				t.Fatalf("Envelope.UnmarshalJSON() error = %v, want nil", gotErr)
			}
			if gotErr := gotEnvelope.Validate(); gotErr != nil {
				t.Fatalf("Envelope.Validate() error = %v, want nil", gotErr)
			}
			gotCanonical, gotMarshalErr := gotEnvelope.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotMarshalErr)
			}
			if !bytes.Equal(gotCanonical, canonical) {
				t.Fatalf("Envelope.MarshalJSON() = %s, want %s", gotCanonical, canonical)
			}
		})
	}
}

func TestEnvelopeJSONPublicHostileRejectionPreservesReceiverMatrix(t *testing.T) {
	t.Parallel()

	canonical := canonicalEnvelopeJSONFixture(t)
	cases := []struct {
		makeInput envelopeJSONFixture
		name      string
	}{
		{name: "empty document rejects", makeInput: fixedJSONFixture(nil)},
		{name: "whitespace-only document rejects", makeInput: fixedJSONFixture([]byte(" \n\t"))},
		{name: "truncated opening object rejects", makeInput: fixedJSONFixture([]byte("{"))},
		{name: "truncated canonical document rejects", makeInput: truncateJSONFixture},
		{name: "oversized document rejects before parse", makeInput: oversizedJSONFixture},
		{name: "unknown member rejects", makeInput: unknownMemberFixture},
		{name: "exact duplicate domain rejects", makeInput: duplicateDomainFixture},
		{name: "case-variant duplicate domain rejects", makeInput: caseVariantDomainFixture},
		{name: "missing domain rejects", makeInput: removeDomainFixture},
		{name: "missing signer rejects", makeInput: removeSignerFixture},
		{name: "missing body length rejects", makeInput: removeBodyLengthFixture},
		{name: "missing body digest rejects", makeInput: removeBodyDigestFixture},
		{name: "missing signature rejects", makeInput: removeSignatureFixture},
		{name: "null document rejects", makeInput: fixedJSONFixture([]byte("null"))},
		{name: "array document rejects", makeInput: fixedJSONFixture([]byte("[]"))},
		{name: "string document rejects", makeInput: fixedJSONFixture([]byte(`"envelope"`))},
		{name: "number document rejects", makeInput: fixedJSONFixture([]byte("1"))},
		{name: "boolean document rejects", makeInput: fixedJSONFixture([]byte("true"))},
		{name: "domain null rejects", makeInput: replaceDomainValueFixture("null")},
		{name: "domain number rejects", makeInput: replaceDomainValueFixture("1")},
		{name: "unknown canonical domain rejects", makeInput: replaceDomainValueFixture(`"future-domain"`)},
		{name: "signer number rejects", makeInput: replaceSignerValueFixture("1")},
		{name: "signer uppercase hexadecimal rejects", makeInput: uppercaseSignerFixture},
		{name: "body length string rejects", makeInput: replaceBodyLengthValueFixture(`"4"`)},
		{name: "body length negative rejects", makeInput: replaceBodyLengthValueFixture("-1")},
		{name: "body length maximum plus one rejects", makeInput: replaceBodyLengthValueFixture(strconv.Itoa(attest.CanonicalBodyMaximumBytes + 1))},
		{name: "body digest number rejects", makeInput: replaceBodyDigestValueFixture("1")},
		{name: "body digest uppercase hexadecimal rejects", makeInput: uppercaseDigestFixture},
		{name: "signature number rejects", makeInput: replaceSignatureValueFixture("1")},
		{name: "signature one byte short rejects", makeInput: shortSignatureFixture},
		{name: "signature uppercase hexadecimal rejects", makeInput: uppercaseSignatureFixture},
		{name: "invalid utf8 rejects", makeInput: invalidUTF8Fixture},
		{name: "trailing second value rejects", makeInput: suffixJSONFixture("{}")},
		{name: "nested unknown object rejects", makeInput: nestedUnknownFixture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantEnvelope := mustEnvelope(
				t,
				literalBody{value: []byte("receiver"), domain: testDomainAlternate},
				deterministicPrivateKey(t, "receiver-preservation"),
			)
			gotEnvelope := wantEnvelope
			gotErr := gotEnvelope.UnmarshalJSON(tc.makeInput(t, canonical))
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("Envelope.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
			}
			if !errors.Is(gotErr, core.ErrAttestContract) {
				t.Fatalf("Envelope.UnmarshalJSON() attest identity = %v, want %v", gotErr, core.ErrAttestContract)
			}
			if gotEnvelope != wantEnvelope {
				t.Fatalf("Envelope after rejection = %+v, want preserved %+v", gotEnvelope, wantEnvelope)
			}
		})
	}
}

func TestEnvelopeJSONCanonicalFieldOrderAndMaximumBound(t *testing.T) {
	t.Parallel()

	privateKey := deterministicPrivateKey(t, "canonical-json-order")
	envelope := mustEnvelope(
		t,
		literalBody{value: []byte("json order"), domain: testDomainPrimary},
		privateKey,
	)
	gotJSON, gotErr := envelope.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotErr)
	}
	wantPrefix := `{"` + envelopeJSONMemberDomain.text() + `":"` +
		testDomainPrimaryText + `","` + envelopeJSONMemberSigner.text() + `":"`
	if !strings.HasPrefix(string(gotJSON), wantPrefix) {
		t.Fatalf("Envelope.MarshalJSON() prefix = %q, want %q", gotJSON, wantPrefix)
	}
	wantFieldOrder := [5]envelopeJSONMember{
		envelopeJSONMemberDomain,
		envelopeJSONMemberSigner,
		envelopeJSONMemberBodyLength,
		envelopeJSONMemberBodySHA256,
		envelopeJSONMemberSignature,
	}
	previousIndex := -1
	for _, member := range wantFieldOrder {
		marker := []byte(`"` + member.text() + `":`)
		gotIndex := bytes.Index(gotJSON, marker)
		if gotIndex <= previousIndex {
			t.Fatalf(
				"Envelope.MarshalJSON() member %s index = %d, want greater than %d",
				member.text(),
				gotIndex,
				previousIndex,
			)
		}
		previousIndex = gotIndex
	}

	maximumDomain := textDomain{text: strings.Repeat("a", attest.SigningDomainMaximumBytes)}
	maximumEnvelope, gotSignErr := attest.Sign(attest.SignRequest[textDomain]{
		Body: sizedTextDomainBody{
			domain: maximumDomain,
			size:   attest.CanonicalBodyMaximumBytes,
		},
		Signer: privateKey,
	})
	if gotSignErr != nil {
		t.Fatalf("attest.Sign(maximum) error = %v, want nil", gotSignErr)
	}
	gotMaximum, gotMaximumErr := maximumEnvelope.MarshalJSON()
	if gotMaximumErr != nil {
		t.Fatalf("Envelope.MarshalJSON(maximum) error = %v, want nil", gotMaximumErr)
	}
	if len(gotMaximum) != attest.EnvelopeCanonicalJSONMaximumBytes {
		t.Fatalf(
			"len(Envelope.MarshalJSON(maximum)) = %d, want %d",
			len(gotMaximum),
			attest.EnvelopeCanonicalJSONMaximumBytes,
		)
	}
	gotMaximumThroughMarshaler, gotMaximumThroughMarshalerErr := json.Marshal(maximumEnvelope)
	if gotMaximumThroughMarshalerErr != nil {
		t.Fatalf("json.Marshal(maximum Envelope) error = %v, want nil",
			gotMaximumThroughMarshalerErr)
	}
	if !bytes.Equal(gotMaximumThroughMarshaler, gotMaximum) {
		t.Fatalf("json.Marshal(maximum Envelope) = %q, want direct fixed point %q",
			gotMaximumThroughMarshaler, gotMaximum)
	}
	var gotMaximumEnvelope attest.Envelope[textDomain]
	gotMaximumDecodeErr := gotMaximumEnvelope.UnmarshalJSON(gotMaximum)
	if gotMaximumDecodeErr != nil || gotMaximumEnvelope != maximumEnvelope {
		t.Fatalf(
			"maximum Envelope.UnmarshalJSON() = (%+v, %v), want (%+v, nil)",
			gotMaximumEnvelope,
			gotMaximumDecodeErr,
			maximumEnvelope,
		)
	}

	belowMaximumDomain := textDomain{text: strings.Repeat("a", attest.SigningDomainMaximumBytes-1)}
	belowMaximumEnvelope, gotBelowSignErr := attest.Sign(attest.SignRequest[textDomain]{
		Body: sizedTextDomainBody{
			domain: belowMaximumDomain,
			size:   attest.CanonicalBodyMaximumBytes,
		},
		Signer: privateKey,
	})
	if gotBelowSignErr != nil {
		t.Fatalf("attest.Sign(maximum minus one JSON) error = %v, want nil", gotBelowSignErr)
	}
	gotBelowMaximum, gotBelowMarshalErr := belowMaximumEnvelope.MarshalJSON()
	if gotBelowMarshalErr != nil {
		t.Fatalf("Envelope.MarshalJSON(maximum minus one) error = %v, want nil", gotBelowMarshalErr)
	}
	wantBelowBytes := attest.EnvelopeCanonicalJSONMaximumBytes - 1
	if len(gotBelowMaximum) != wantBelowBytes {
		t.Fatalf("len(Envelope.MarshalJSON(maximum minus one)) = %d, want %d", len(gotBelowMaximum), wantBelowBytes)
	}
	var gotBelowEnvelope attest.Envelope[textDomain]
	gotBelowDecodeErr := gotBelowEnvelope.UnmarshalJSON(gotBelowMaximum)
	if gotBelowDecodeErr != nil || gotBelowEnvelope != belowMaximumEnvelope {
		t.Fatalf(
			"maximum minus one Envelope.UnmarshalJSON() = (%+v, %v), want (%+v, nil)",
			gotBelowEnvelope,
			gotBelowDecodeErr,
			belowMaximumEnvelope,
		)
	}
}

// TestEnvelopeJSONMaximumExtentAdmitsInsignificantWhitespace pressures the
// interaction the canonical-extent and normalization matrices each miss: an
// envelope whose canonical projection is exactly EnvelopeCanonicalJSONMaximumBytes
// still has to survive the insignificant whitespace that any enclosing
// pretty-printer introduces. Bounding the accepted document at the canonical
// extent makes a self-produced maximum envelope undecodable after one space.
func TestEnvelopeJSONMaximumExtentAdmitsInsignificantWhitespace(t *testing.T) {
	t.Parallel()

	envelope := maximumExtentEnvelopeFixture(t)
	canonical, gotMarshalErr := envelope.MarshalJSON()
	if gotMarshalErr != nil {
		t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotMarshalErr)
	}
	if len(canonical) != attest.EnvelopeCanonicalJSONMaximumBytes {
		t.Fatalf(
			"len(Envelope.MarshalJSON()) = %d, want the canonical maximum %d",
			len(canonical),
			attest.EnvelopeCanonicalJSONMaximumBytes,
		)
	}
	allowance := attest.EnvelopeJSONMaximumBytes - attest.EnvelopeCanonicalJSONMaximumBytes
	if allowance <= 0 {
		t.Fatalf("whitespace allowance = %d, want positive", allowance)
	}

	cases := []struct {
		makeInput envelopeJSONFixture
		name      string
	}{
		{name: "canonical maximum extent closes", makeInput: cloneJSONFixture},
		{name: "one leading space at maximum extent normalizes", makeInput: prefixJSONFixture(" ")},
		{name: "one trailing space at maximum extent normalizes", makeInput: suffixJSONFixture(" ")},
		{name: "surrounding whitespace at maximum extent normalizes", makeInput: surroundJSONFixture("\n\t", "\r\n")},
		{name: "pretty printed maximum extent normalizes", makeInput: indentJSONFixture("", "  ")},
		{name: "deeply indented maximum extent normalizes", makeInput: indentJSONFixture(strings.Repeat(" ", 24), "    ")},
		{
			name:      "one byte below whitespace allowance normalizes",
			makeInput: suffixJSONFixture(strings.Repeat(" ", allowance-1)),
		},
		{
			name:      "whitespace allowance boundary normalizes",
			makeInput: suffixJSONFixture(strings.Repeat(" ", allowance)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := tc.makeInput(t, canonical)
			if len(input) > attest.EnvelopeJSONMaximumBytes {
				t.Fatalf(
					"fixture length = %d, want at most the accepted maximum %d",
					len(input),
					attest.EnvelopeJSONMaximumBytes,
				)
			}
			var gotEnvelope attest.Envelope[textDomain]
			if gotErr := gotEnvelope.UnmarshalJSON(input); gotErr != nil {
				t.Fatalf("Envelope.UnmarshalJSON() error = %v, want nil", gotErr)
			}
			if gotEnvelope != envelope {
				t.Fatalf("Envelope.UnmarshalJSON() = %+v, want %+v", gotEnvelope, envelope)
			}
			gotCanonical, gotErr := gotEnvelope.MarshalJSON()
			if gotErr != nil {
				t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotErr)
			}
			if !bytes.Equal(gotCanonical, canonical) {
				t.Fatalf("Envelope.MarshalJSON() = %s, want %s", gotCanonical, canonical)
			}
		})
	}

	t.Run("one byte past the whitespace allowance is rejected", func(t *testing.T) {
		t.Parallel()

		input := suffixJSONFixture(strings.Repeat(" ", allowance+1))(t, canonical)
		var gotEnvelope attest.Envelope[textDomain]
		gotErr := gotEnvelope.UnmarshalJSON(input)
		if !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("Envelope.UnmarshalJSON() error = %v, want core.ErrJSONContract", gotErr)
		}
		if gotEnvelope != (attest.Envelope[textDomain]{}) {
			t.Fatalf("Envelope.UnmarshalJSON() receiver = %+v, want the zero value", gotEnvelope)
		}
	})
}

func maximumExtentEnvelopeFixture(t testing.TB) attest.Envelope[textDomain] {
	t.Helper()
	envelope, err := attest.Sign(attest.SignRequest[textDomain]{
		Body: sizedTextDomainBody{
			domain: textDomain{text: strings.Repeat("a", attest.SigningDomainMaximumBytes)},
			size:   attest.CanonicalBodyMaximumBytes,
		},
		Signer: deterministicPrivateKey(t, "maximum-extent-envelope"),
	})
	if err != nil {
		t.Fatalf("attest.Sign(maximum extent) error = %v, want nil", err)
	}
	return envelope
}

func indentJSONFixture(prefix string, indent string) envelopeJSONFixture {
	return func(t testing.TB, input []byte) []byte {
		t.Helper()
		var indented bytes.Buffer
		if err := json.Indent(&indented, input, prefix, indent); err != nil {
			t.Fatalf("json.Indent() error = %v, want nil", err)
		}
		return indented.Bytes()
	}
}

func TestSignatureJSONPublicCanonicalBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		makeInput func(testing.TB) ([]byte, string)
		name      string
	}{
		{name: "standard library signature closes", makeInput: signedSignatureJSONFixture("signature-valid-standard", []byte("x"))},
		{name: "different key signature closes", makeInput: signedSignatureJSONFixture("signature-valid-other", []byte("x"))},
		{name: "different body signature closes", makeInput: signedSignatureJSONFixture("signature-valid-standard", []byte("y"))},
		{name: "leading whitespace normalizes", makeInput: decoratedSignatureJSONFixture(" \n", "")},
		{name: "trailing whitespace normalizes", makeInput: decoratedSignatureJSONFixture("", "\t ")},
		{name: "surrounding whitespace normalizes", makeInput: decoratedSignatureJSONFixture("\n", "\r\n")},
		{name: "equivalent lowercase escape normalizes", makeInput: escapedSignatureJSONFixture},
		{name: "all zero structural signature closes", makeInput: hexadecimalSignatureJSONFixture(strings.Repeat("0", 128))},
		{name: "all maximum structural signature closes", makeInput: hexadecimalSignatureJSONFixture(strings.Repeat("f", 128))},
		{name: "alternating structural signature closes", makeInput: hexadecimalSignatureJSONFixture(strings.Repeat("01", 64))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input, wantHex := tc.makeInput(t)
			var gotSignature attest.Signature
			gotErr := gotSignature.UnmarshalJSON(input)
			if gotErr != nil {
				t.Fatalf("Signature.UnmarshalJSON() error = %v, want nil", gotErr)
			}
			gotHex, gotHexErr := gotSignature.Hex()
			if gotHexErr != nil || gotHex != wantHex {
				t.Fatalf("Signature.Hex() = (%q, %v), want (%q, nil)", gotHex, gotHexErr, wantHex)
			}
			gotCanonical, gotMarshalErr := gotSignature.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("Signature.MarshalJSON() error = %v, want nil", gotMarshalErr)
			}
			wantCanonical := strconv.AppendQuote(nil, wantHex)
			if !bytes.Equal(gotCanonical, wantCanonical) {
				t.Fatalf("Signature.MarshalJSON() = %s, want %s", gotCanonical, wantCanonical)
			}
		})
	}
}

func TestSignatureJSONPublicHostileReceiverPreservationMatrix(t *testing.T) {
	t.Parallel()

	originalEnvelope := mustEnvelope(
		t,
		literalBody{value: []byte("signature"), domain: testDomainPrimary},
		deterministicPrivateKey(t, "signature-json"),
	)
	originalJSON, gotMarshalErr := originalEnvelope.Signature.MarshalJSON()
	if gotMarshalErr != nil {
		t.Fatalf("Signature.MarshalJSON() error = %v, want nil", gotMarshalErr)
	}
	cases := []struct {
		name  string
		input []byte
	}{
		{name: "empty rejects", input: nil},
		{name: "null rejects", input: []byte("null")},
		{name: "number rejects", input: []byte("1")},
		{name: "array rejects", input: []byte("[]")},
		{name: "object rejects", input: []byte("{}")},
		{name: "empty string rejects", input: []byte(`""`)},
		{name: "one hexadecimal digit rejects", input: []byte(`"0"`)},
		{name: "one byte short rejects", input: []byte(`"` + strings.Repeat("0", 126) + `"`)},
		{name: "one hexadecimal character short rejects", input: []byte(`"` + strings.Repeat("0", 127) + `"`)},
		{name: "one hexadecimal character long rejects", input: []byte(`"` + strings.Repeat("0", 129) + `"`)},
		{name: "one byte long rejects", input: []byte(`"` + strings.Repeat("0", 130) + `"`)},
		{name: "nonhexadecimal rejects", input: []byte(`"` + strings.Repeat("g", 128) + `"`)},
		{name: "uppercase rejects", input: bytes.ToUpper(originalJSON)},
		{name: "trailing value rejects", input: append(bytes.Clone(originalJSON), []byte(" 0")...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotSignature := originalEnvelope.Signature
			gotErr := gotSignature.UnmarshalJSON(tc.input)
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("Signature.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
			}
			if gotSignature != originalEnvelope.Signature {
				t.Fatalf("Signature after rejection = %+v, want preserved %+v", gotSignature, originalEnvelope.Signature)
			}
		})
	}
}

func signedSignatureJSONFixture(
	keyLabel string,
	body []byte,
) func(testing.TB) ([]byte, string) {
	return func(t testing.TB) ([]byte, string) {
		t.Helper()
		envelope := mustEnvelope(
			t,
			literalBody{value: bytes.Clone(body), domain: testDomainPrimary},
			deterministicPrivateKey(t, keyLabel),
		)
		encoded, err := envelope.Signature.MarshalJSON()
		if err != nil {
			t.Fatalf("Signature.MarshalJSON() error = %v, want nil", err)
		}
		hexadecimal, err := envelope.Signature.Hex()
		if err != nil {
			t.Fatalf("Signature.Hex() error = %v, want nil", err)
		}
		return encoded, hexadecimal
	}
}

func decoratedSignatureJSONFixture(
	prefix string,
	suffix string,
) func(testing.TB) ([]byte, string) {
	return func(t testing.TB) ([]byte, string) {
		t.Helper()
		encoded, hexadecimal := signedSignatureJSONFixture("signature-decorated", []byte("x"))(t)
		result := append([]byte(prefix), encoded...)
		return append(result, suffix...), hexadecimal
	}
}

func escapedSignatureJSONFixture(t testing.TB) ([]byte, string) {
	t.Helper()
	_, hexadecimal := signedSignatureJSONFixture("signature-escaped", []byte("x"))(t)
	encoded := strconv.AppendQuote(nil, hexadecimal)
	replacement := []byte(`\u00` + hex.EncodeToString([]byte{hexadecimal[0]}))
	encoded = append(encoded[:1], append(replacement, encoded[2:]...)...)
	return encoded, hexadecimal
}

func hexadecimalSignatureJSONFixture(
	hexadecimal string,
) func(testing.TB) ([]byte, string) {
	return func(testing.TB) ([]byte, string) {
		return strconv.AppendQuote(nil, hexadecimal), hexadecimal
	}
}

func TestEnvelopeJSONDomainReconstructionFailureMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr    error
		wantNative error
		name       string
		domainText string
	}{
		{name: "canonical reconstruction closes", domainText: "canonical-source"},
		{name: "typed parse error rejects", domainText: "parse-error", wantErr: core.ErrJSONContract, wantNative: fixtureErrorValidation},
		{name: "parse panic is contained", domainText: "parse-panic", wantErr: core.ErrJSONContract},
		{name: "divergent reconstruction rejects", domainText: "divergent-source", wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			privateKey := deterministicPrivateKey(t, "domain-reconstruction")
			envelope, gotSignErr := attest.Sign(attest.SignRequest[reconstructDomain]{
				Body:   reconstructDomainBody{domain: reconstructDomain{text: tc.domainText}},
				Signer: privateKey,
			})
			if gotSignErr != nil {
				t.Fatalf("attest.Sign() error = %v, want nil", gotSignErr)
			}
			wire, gotMarshalErr := envelope.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotMarshalErr)
			}
			preservedEnvelope, gotPreservedErr := attest.Sign(attest.SignRequest[reconstructDomain]{
				Body:   reconstructDomainBody{domain: reconstructDomain{text: "preserved"}},
				Signer: privateKey,
			})
			if gotPreservedErr != nil {
				t.Fatalf("attest.Sign(preserved) error = %v, want nil", gotPreservedErr)
			}
			gotEnvelope := preservedEnvelope
			gotErr := gotEnvelope.UnmarshalJSON(wire)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Envelope.UnmarshalJSON() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("Envelope.UnmarshalJSON() native error = %v, want %v", gotErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if gotEnvelope != preservedEnvelope {
					t.Fatalf("Envelope after rejection = %+v, want preserved %+v", gotEnvelope, preservedEnvelope)
				}
				return
			}
			if gotEnvelope != envelope {
				t.Fatalf("Envelope.UnmarshalJSON() = %+v, want %+v", gotEnvelope, envelope)
			}
		})
	}
}

func TestJSONPublicNilReceiverBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func() error
		name string
	}{
		{
			name: "nil envelope receiver rejects",
			run: func() error {
				var receiver *attest.Envelope[testDomain]
				return receiver.UnmarshalJSON([]byte("{}"))
			},
		},
		{
			name: "nil signature receiver rejects",
			run: func() error {
				var receiver *attest.Signature
				return receiver.UnmarshalJSON([]byte(`"` + strings.Repeat("0", 128) + `"`))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run()
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
			}
			if !errors.Is(gotErr, core.ErrAttestContract) {
				t.Fatalf("UnmarshalJSON() attest identity = %v, want %v", gotErr, core.ErrAttestContract)
			}
		})
	}
}

type reconstructDomain struct {
	text string
}

func (reconstructDomain) Validate() error {
	return nil
}

func (d reconstructDomain) MarshalText() ([]byte, error) {
	return []byte(d.text), nil
}

func (reconstructDomain) ParseCanonicalText(text []byte) (reconstructDomain, error) {
	switch string(text) {
	case "parse-error":
		return reconstructDomain{}, fixtureErrorValidation
	case "parse-panic":
		panic(fixtureErrorValidation)
	case "divergent-source":
		return reconstructDomain{text: "divergent-target"}, nil
	default:
		return reconstructDomain{text: string(text)}, nil
	}
}

type reconstructDomainBody struct {
	domain reconstructDomain
}

func (reconstructDomainBody) Validate() error {
	return nil
}

func (b reconstructDomainBody) AttestationDomain() reconstructDomain {
	return b.domain
}

func (reconstructDomainBody) WriteCanonical(destination io.Writer) error {
	_, err := destination.Write([]byte("x"))
	return err
}

type sizedTextDomainBody struct {
	domain textDomain
	size   int
}

func (sizedTextDomainBody) Validate() error {
	return nil
}

func (b sizedTextDomainBody) AttestationDomain() textDomain {
	return b.domain
}

func (b sizedTextDomainBody) WriteCanonical(destination io.Writer) error {
	var block [8192]byte
	remaining := b.size
	for remaining > 0 {
		count := min(remaining, len(block))
		if _, err := destination.Write(block[:count]); err != nil {
			return err
		}
		remaining -= count
	}
	return nil
}

func canonicalEnvelopeJSONFixture(t testing.TB) []byte {
	t.Helper()
	envelope := mustEnvelope(
		t,
		literalBody{value: []byte("body"), domain: testDomainPrimary},
		deterministicPrivateKey(t, "canonical-json"),
	)
	encoded, err := envelope.MarshalJSON()
	if err != nil {
		t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

type envelopeJSONFixture func(testing.TB, []byte) []byte

type envelopeJSONMember uint8

const (
	envelopeJSONMemberUnknown envelopeJSONMember = iota
	envelopeJSONMemberDomain
	envelopeJSONMemberSigner
	envelopeJSONMemberBodyLength
	envelopeJSONMemberBodySHA256
	envelopeJSONMemberSignature
)

func (m envelopeJSONMember) text() string {
	switch m {
	case envelopeJSONMemberDomain:
		return "domain"
	case envelopeJSONMemberSigner:
		return "signer"
	case envelopeJSONMemberBodyLength:
		return "body_length_bytes"
	case envelopeJSONMemberBodySHA256:
		return "body_sha256"
	case envelopeJSONMemberSignature:
		return "signature"
	default:
		return ""
	}
}

type envelopeJSONParts struct {
	Domain     json.RawMessage `json:"domain"`
	Signer     json.RawMessage `json:"signer"`
	BodyLength json.RawMessage `json:"body_length_bytes"`
	BodySHA256 json.RawMessage `json:"body_sha256"`
	Signature  json.RawMessage `json:"signature"`
}

type optionalEnvelopeJSONParts struct {
	Domain     json.RawMessage `json:"domain,omitempty"`
	Signer     json.RawMessage `json:"signer,omitempty"`
	BodyLength json.RawMessage `json:"body_length_bytes,omitempty"`
	BodySHA256 json.RawMessage `json:"body_sha256,omitempty"`
	Signature  json.RawMessage `json:"signature,omitempty"`
}

func cloneJSONFixture(_ testing.TB, input []byte) []byte {
	return bytes.Clone(input)
}

func fixedJSONFixture(value []byte) envelopeJSONFixture {
	return func(testing.TB, []byte) []byte {
		return bytes.Clone(value)
	}
}

func prefixJSONFixture(prefix string) envelopeJSONFixture {
	return func(_ testing.TB, input []byte) []byte {
		return append([]byte(prefix), input...)
	}
}

func suffixJSONFixture(suffix string) envelopeJSONFixture {
	return func(_ testing.TB, input []byte) []byte {
		return append(bytes.Clone(input), suffix...)
	}
}

func surroundJSONFixture(prefix string, suffix string) envelopeJSONFixture {
	return func(_ testing.TB, input []byte) []byte {
		result := append([]byte(prefix), input...)
		return append(result, suffix...)
	}
}

func truncateJSONFixture(_ testing.TB, input []byte) []byte {
	return bytes.Clone(input[:len(input)-1])
}

func oversizedJSONFixture(testing.TB, []byte) []byte {
	return bytes.Repeat([]byte(" "), attest.EnvelopeJSONMaximumBytes+1)
}

func reverseEnvelopeMembersFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	wire := struct {
		Signature  json.RawMessage `json:"signature"`
		BodySHA256 json.RawMessage `json:"body_sha256"`
		BodyLength json.RawMessage `json:"body_length_bytes"`
		Signer     json.RawMessage `json:"signer"`
		Domain     json.RawMessage `json:"domain"`
	}{
		Signature:  parts.Signature,
		BodySHA256: parts.BodySHA256,
		BodyLength: parts.BodyLength,
		Signer:     parts.Signer,
		Domain:     parts.Domain,
	}
	return marshalJSONFixture(t, wire)
}

func domainLastFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	wire := struct {
		Signer     json.RawMessage `json:"signer"`
		BodyLength json.RawMessage `json:"body_length_bytes"`
		BodySHA256 json.RawMessage `json:"body_sha256"`
		Signature  json.RawMessage `json:"signature"`
		Domain     json.RawMessage `json:"domain"`
	}{
		Signer:     parts.Signer,
		BodyLength: parts.BodyLength,
		BodySHA256: parts.BodySHA256,
		Signature:  parts.Signature,
		Domain:     parts.Domain,
	}
	return marshalJSONFixture(t, wire)
}

func spacedEnvelopeFixture(_ testing.TB, input []byte) []byte {
	replacer := strings.NewReplacer(`:`, ` : `, `,`, `, `)
	return []byte(replacer.Replace(string(input)))
}

func escapedDomainFixture(_ testing.TB, input []byte) []byte {
	return bytes.Replace(input, []byte(testDomainPrimaryText), []byte(`test-primary-20\u00326`), 1)
}

func escapedLetterFixture(_ testing.TB, input []byte) []byte {
	return bytes.Replace(input, []byte(testDomainPrimaryText), []byte(`test-prim\u0061ry-2026`), 1)
}

func combinedNormalizationFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	reversed := reverseEnvelopeMembersFixture(t, input)
	escaped := escapedDomainFixture(t, reversed)
	spaced := spacedEnvelopeFixture(t, escaped)
	return surroundJSONFixture("\n", "\t")(t, spaced)
}

func unknownMemberFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	wire := struct {
		Domain     json.RawMessage `json:"domain"`
		Signer     json.RawMessage `json:"signer"`
		BodyLength json.RawMessage `json:"body_length_bytes"`
		BodySHA256 json.RawMessage `json:"body_sha256"`
		Signature  json.RawMessage `json:"signature"`
		Unknown    bool            `json:"unknown"`
	}{
		Domain:     parts.Domain,
		Signer:     parts.Signer,
		BodyLength: parts.BodyLength,
		BodySHA256: parts.BodySHA256,
		Signature:  parts.Signature,
		Unknown:    true,
	}
	return marshalJSONFixture(t, wire)
}

func duplicateDomainFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	prefix := []byte(`{"` + envelopeJSONMemberDomain.text() + `":`)
	prefix = append(prefix, parts.Domain...)
	prefix = append(prefix, ',')
	return append(prefix, input[1:]...)
}

func caseVariantDomainFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	prefix := []byte(`{"Domain":`)
	prefix = append(prefix, parts.Domain...)
	prefix = append(prefix, ',')
	return append(prefix, input[1:]...)
}

func removeDomainFixture(t testing.TB, input []byte) []byte {
	return removeNamedJSONMember(t, input, envelopeJSONMemberDomain)
}

func removeSignerFixture(t testing.TB, input []byte) []byte {
	return removeNamedJSONMember(t, input, envelopeJSONMemberSigner)
}

func removeBodyLengthFixture(t testing.TB, input []byte) []byte {
	return removeNamedJSONMember(t, input, envelopeJSONMemberBodyLength)
}

func removeBodyDigestFixture(t testing.TB, input []byte) []byte {
	return removeNamedJSONMember(t, input, envelopeJSONMemberBodySHA256)
}

func removeSignatureFixture(t testing.TB, input []byte) []byte {
	return removeNamedJSONMember(t, input, envelopeJSONMemberSignature)
}

func removeNamedJSONMember(
	t testing.TB,
	input []byte,
	member envelopeJSONMember,
) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	optional := optionalEnvelopeJSONParts(parts)
	switch member {
	case envelopeJSONMemberDomain:
		optional.Domain = nil
	case envelopeJSONMemberSigner:
		optional.Signer = nil
	case envelopeJSONMemberBodyLength:
		optional.BodyLength = nil
	case envelopeJSONMemberBodySHA256:
		optional.BodySHA256 = nil
	case envelopeJSONMemberSignature:
		optional.Signature = nil
	default:
		t.Fatalf("envelope JSON member = %d, want admitted member", member)
	}
	return marshalJSONFixture(t, optional)
}

func replaceDomainValueFixture(value string) envelopeJSONFixture {
	return replaceJSONValueFixture(envelopeJSONMemberDomain, value)
}

func replaceSignerValueFixture(value string) envelopeJSONFixture {
	return replaceJSONValueFixture(envelopeJSONMemberSigner, value)
}

func replaceBodyLengthValueFixture(value string) envelopeJSONFixture {
	return replaceJSONValueFixture(envelopeJSONMemberBodyLength, value)
}

func replaceBodyDigestValueFixture(value string) envelopeJSONFixture {
	return replaceJSONValueFixture(envelopeJSONMemberBodySHA256, value)
}

func replaceSignatureValueFixture(value string) envelopeJSONFixture {
	return replaceJSONValueFixture(envelopeJSONMemberSignature, value)
}

func replaceJSONValueFixture(
	member envelopeJSONMember,
	value string,
) envelopeJSONFixture {
	return func(t testing.TB, input []byte) []byte {
		t.Helper()
		parts := envelopeJSONPartsFixture(t, input)
		replacement := json.RawMessage(value)
		switch member {
		case envelopeJSONMemberDomain:
			parts.Domain = replacement
		case envelopeJSONMemberSigner:
			parts.Signer = replacement
		case envelopeJSONMemberBodyLength:
			parts.BodyLength = replacement
		case envelopeJSONMemberBodySHA256:
			parts.BodySHA256 = replacement
		case envelopeJSONMemberSignature:
			parts.Signature = replacement
		default:
			t.Fatalf("envelope JSON member = %d, want admitted member", member)
		}
		return marshalJSONFixture(t, parts)
	}
}

func uppercaseSignerFixture(t testing.TB, input []byte) []byte {
	return uppercaseJSONValueFixture(t, input, envelopeJSONMemberSigner)
}

func uppercaseDigestFixture(t testing.TB, input []byte) []byte {
	return uppercaseJSONValueFixture(t, input, envelopeJSONMemberBodySHA256)
}

func uppercaseSignatureFixture(t testing.TB, input []byte) []byte {
	return uppercaseJSONValueFixture(t, input, envelopeJSONMemberSignature)
}

func uppercaseJSONValueFixture(
	t testing.TB,
	input []byte,
	member envelopeJSONMember,
) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	var target *json.RawMessage
	switch member {
	case envelopeJSONMemberSigner:
		target = &parts.Signer
	case envelopeJSONMemberBodySHA256:
		target = &parts.BodySHA256
	case envelopeJSONMemberSignature:
		target = &parts.Signature
	default:
		t.Fatalf("uppercase envelope JSON member = %d, want hexadecimal member", member)
	}
	*target = bytes.ToUpper(*target)
	return marshalJSONFixture(t, parts)
}

func shortSignatureFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	if len(parts.Signature) < 4 {
		t.Fatalf("canonical envelope signature bytes = %d, want at least 4", len(parts.Signature))
	}
	parts.Signature = append(bytes.Clone(parts.Signature[:1]), parts.Signature[3:]...)
	return marshalJSONFixture(t, parts)
}

func invalidUTF8Fixture(_ testing.TB, input []byte) []byte {
	result := bytes.Clone(input)
	result[1] = 0xff
	return result
}

func nestedUnknownFixture(t testing.TB, input []byte) []byte {
	t.Helper()
	parts := envelopeJSONPartsFixture(t, input)
	wire := struct {
		Domain     json.RawMessage `json:"domain"`
		Signer     json.RawMessage `json:"signer"`
		BodyLength json.RawMessage `json:"body_length_bytes"`
		BodySHA256 json.RawMessage `json:"body_sha256"`
		Signature  json.RawMessage `json:"signature"`
		Nested     struct {
			Value int `json:"value"`
		} `json:"nested"`
	}{
		Domain:     parts.Domain,
		Signer:     parts.Signer,
		BodyLength: parts.BodyLength,
		BodySHA256: parts.BodySHA256,
		Signature:  parts.Signature,
	}
	wire.Nested.Value = 1
	return marshalJSONFixture(t, wire)
}

func envelopeJSONPartsFixture(t testing.TB, input []byte) envelopeJSONParts {
	t.Helper()
	var parts envelopeJSONParts
	if err := json.Unmarshal(input, &parts); err != nil {
		t.Fatalf("json.Unmarshal(canonical envelope) error = %v, want nil", err)
	}
	return parts
}

func marshalJSONFixture[T any](t testing.TB, value T) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(fixture) error = %v, want nil", err)
	}
	return encoded
}
