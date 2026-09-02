package timeproof

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func rawValueFromDER(t testing.TB, encoded []byte) asn1.RawValue {
	t.Helper()

	var raw asn1.RawValue
	trailing, err := asn1.Unmarshal(encoded, &raw)
	if err != nil || len(trailing) != 0 {
		t.Fatalf(
			"asn1.Unmarshal(raw value) trailing/error = (%x, %v), want (empty, nil)",
			trailing,
			err,
		)
	}
	return raw
}

// splitElements returns the exact DER of every top-level element inside one
// already-parsed value body.
func splitElements(t testing.TB, der []byte) [][]byte {
	t.Helper()

	elements := make([][]byte, 0, 8)
	for len(der) != 0 {
		raw, remaining, err := consumeRaw(der)
		if err != nil {
			t.Fatalf("consumeRaw(element) error = %v, want nil", err)
		}
		elements = append(elements, append([]byte(nil), raw.FullBytes...))
		der = remaining
	}
	return elements
}

// replaceLeadingInteger rebuilds one SEQUENCE with its first INTEGER element
// replaced, leaving every other element byte-identical.
func replaceLeadingInteger(t testing.TB, sequenceDER []byte, value int) []byte {
	t.Helper()

	sequence, err := requireSequence(sequenceDER)
	if err != nil {
		t.Fatalf("requireSequence(surgery target) error = %v, want nil", err)
	}
	elements := splitElements(t, sequence.Bytes)
	if len(elements) == 0 {
		t.Fatal("surgery target elements = 0, want at least the version integer")
	}
	replacement, err := asn1.Marshal(value)
	if err != nil {
		t.Fatalf("asn1.Marshal(version %d) error = %v, want nil", value, err)
	}
	elements[0] = replacement
	return derTagged(byte(asn1.TagSequence)|derConstructed, bytes.Join(elements, nil))
}

// rebuildToken reassembles a ContentInfo around one replacement SignedData.
func rebuildToken(t testing.TB, tokenDER []byte, signedData []byte) []byte {
	t.Helper()

	outer, err := requireSequence(tokenDER)
	if err != nil {
		t.Fatalf("requireSequence(token) error = %v, want nil", err)
	}
	oidRaw, remaining, err := consumeRaw(outer.Bytes)
	if err != nil {
		t.Fatalf("consumeRaw(token content type) error = %v, want nil", err)
	}
	content, trailing, err := consumeRaw(remaining)
	if err != nil || len(trailing) != 0 {
		t.Fatalf(
			"consumeRaw(token content) trailing/error = (%d, %v), want (0, nil)",
			len(trailing),
			err,
		)
	}
	explicit := derTagged(content.FullBytes[0], signedData)
	body := append(append([]byte(nil), oidRaw.FullBytes...), explicit...)
	return derTagged(byte(asn1.TagSequence)|derConstructed, body)
}

func authenticSignedData(t testing.TB, tokenDER []byte) []byte {
	t.Helper()

	_, explicit, err := parseContentInfo(tokenDER)
	if err != nil {
		t.Fatalf("parseContentInfo(authentic) error = %v, want nil", err)
	}
	sequence, err := requireSequence(explicit.Bytes)
	if err != nil {
		t.Fatalf("requireSequence(SignedData) error = %v, want nil", err)
	}
	return append([]byte(nil), sequence.FullBytes...)
}

// rebuildResponse wraps one token in the authentic granted PKIStatusInfo.
func rebuildResponse(t testing.TB, tokenDER []byte) []byte {
	t.Helper()

	fixture := loadAuthenticFixture(t)
	outer, err := requireSequence(fixture.response)
	if err != nil {
		t.Fatalf("requireSequence(authentic response) error = %v, want nil", err)
	}
	statusRaw, _, err := consumeRaw(outer.Bytes)
	if err != nil {
		t.Fatalf("consumeRaw(authentic status info) error = %v, want nil", err)
	}
	body := append(append([]byte(nil), statusRaw.FullBytes...), tokenDER...)
	return derTagged(byte(asn1.TagSequence)|derConstructed, body)
}

// signedDataWithVersions rebuilds the authentic token with replacement CMS
// SignedData and SignerInfo versions. Neither field is covered by the CMS
// signature, so only an explicit gate can reject a rewritten value.
func signedDataWithVersions(
	t testing.TB,
	signedVersion int,
	signerVersion int,
) []byte {
	t.Helper()

	tokenDER := authenticTokenDER(t)
	signedData := authenticSignedData(t, tokenDER)
	sequence, err := requireSequence(signedData)
	if err != nil {
		t.Fatalf("requireSequence(SignedData) error = %v, want nil", err)
	}
	elements := splitElements(t, sequence.Bytes)
	if len(elements) < 2 {
		t.Fatalf("SignedData elements = %d, want at least 2", len(elements))
	}
	signers, remaining, err := consumeRaw(elements[len(elements)-1])
	if err != nil || len(remaining) != 0 ||
		!isUniversal(signers, asn1.TagSet, true) {
		t.Fatalf("SignedData final element is not a signer SET: %v", err)
	}
	signerElements := splitElements(t, signers.Bytes)
	if len(signerElements) != signerMaximumCount {
		t.Fatalf(
			"authentic signer count = %d, want %d",
			len(signerElements),
			signerMaximumCount,
		)
	}
	signerElements[0] = replaceLeadingInteger(t, signerElements[0], signerVersion)
	elements[len(elements)-1] = derTagged(
		byte(asn1.TagSet)|derConstructed,
		bytes.Join(signerElements, nil),
	)
	rebuilt := derTagged(
		byte(asn1.TagSequence)|derConstructed,
		bytes.Join(elements, nil),
	)
	return rebuildResponse(t, rebuildToken(
		t, tokenDER, replaceLeadingInteger(t, rebuilt, signedVersion),
	))
}

func TestCMSUnsignedVersionSurgeryTable(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticFixture(t)
	t.Run("faithful rebuild reproduces the authentic response exactly", func(t *testing.T) {
		t.Parallel()

		got := signedDataWithVersions(t, cmsSignedDataVersion, cmsSignerInfoVersion)
		if !bytes.Equal(got, fixture.response) {
			t.Fatalf(
				"faithful DER rebuild length = %d, want byte-identical authentic response length %d",
				len(got),
				len(fixture.response),
			)
		}
		if _, err := Verify(VerifyRequest{
			Response: got, Request: fixture.request,
			ExpectedDigest: fixture.digest,
		}); err != nil {
			t.Fatalf("Verify(faithful rebuild) error = %v, want nil", err)
		}
	})

	cases := []struct {
		name          string
		signedVersion int
		signerVersion int
	}{
		{name: "SignedData version one below the required 3", signedVersion: 2, signerVersion: cmsSignerInfoVersion},
		{name: "SignedData version one above the required 3", signedVersion: 4, signerVersion: cmsSignerInfoVersion},
		{name: "SignedData version claiming id-data shape", signedVersion: 1, signerVersion: cmsSignerInfoVersion},
		{name: "SignedData version zero", signedVersion: 0, signerVersion: cmsSignerInfoVersion},
		{name: "SignedData version far above the closed set", signedVersion: 4096, signerVersion: cmsSignerInfoVersion},
		{name: "SignerInfo version claiming subjectKeyIdentifier", signedVersion: cmsSignedDataVersion, signerVersion: 3},
		{name: "SignerInfo version zero", signedVersion: cmsSignedDataVersion, signerVersion: 0},
		{name: "SignerInfo version one above the required 1", signedVersion: cmsSignedDataVersion, signerVersion: 2},
		{name: "SignerInfo version far above the closed set", signedVersion: cmsSignedDataVersion, signerVersion: 4096},
		{name: "both versions rewritten", signedVersion: 1, signerVersion: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response := signedDataWithVersions(t, tc.signedVersion, tc.signerVersion)
			got, gotErr := Verify(VerifyRequest{
				Response: response, Request: fixture.request,
				ExpectedDigest: fixture.digest,
			})
			if !errors.Is(gotErr, core.ErrTimeProofInvalid) || !got.isZero() {
				t.Fatalf(
					"Verify(SignedData %d / SignerInfo %d) = (%v, %v), want zero and %v",
					tc.signedVersion,
					tc.signerVersion,
					got,
					gotErr,
					core.ErrTimeProofInvalid,
				)
			}
		})
	}
}

// signingTimeAttribute builds one signed signing-time attribute. The CMS
// signature binds this attribute together with the TSTInfo digest, so a
// contradiction can only be produced by the signing authority itself. These
// cases are therefore a direct unit ratchet on the helper, not an end-to-end
// proof; the authentic fixture proves the agreeing path through Verify.
func signingTimeAttribute(t testing.TB, values ...time.Time) cmsAttribute {
	t.Helper()

	raws := make([]asn1.RawValue, 0, len(values))
	for _, value := range values {
		encoded, err := asn1.Marshal(value)
		if err != nil {
			t.Fatalf("asn1.Marshal(signing time) error = %v, want nil", err)
		}
		raws = append(raws, rawValueFromDER(t, encoded))
	}
	return cmsAttribute{Type: oidSigningTime(), Values: raws}
}

func TestSigningTimeAttributeShapeTable(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, time.July, 25, 1, 23, 43, 0, time.UTC)
	contentType := cmsAttribute{Type: oidContentType()}
	notATime := cmsAttribute{
		Type:   oidSigningTime(),
		Values: []asn1.RawValue{{Class: asn1.ClassUniversal, Tag: asn1.TagInteger, FullBytes: []byte{0x02, 0x01, 0x01}}},
	}
	cases := []struct {
		wantErr    error
		name       string
		attributes []cmsAttribute
	}{
		{
			name:       "absent signing time is accepted",
			attributes: []cmsAttribute{contentType},
		},
		{
			name:       "signing time equal to the generation instant is accepted",
			attributes: []cmsAttribute{contentType, signingTimeAttribute(t, when)},
		},
		{
			name:       "empty attribute set is accepted",
			attributes: nil,
		},
		{
			name:       "signing time one second after generation",
			attributes: []cmsAttribute{signingTimeAttribute(t, when.Add(time.Second))},
		},
		{
			name:       "signing time one second before generation",
			attributes: []cmsAttribute{signingTimeAttribute(t, when.Add(-time.Second))},
		},
		{
			name:       "signing time far after generation",
			attributes: []cmsAttribute{signingTimeAttribute(t, when.AddDate(1, 0, 0))},
		},
		{
			name: "generalized signing time outside the UTC-time year range",
			attributes: []cmsAttribute{
				signingTimeAttribute(t, time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "duplicate signing time attributes",
			attributes: []cmsAttribute{
				signingTimeAttribute(t, when), signingTimeAttribute(t, when),
			},
			wantErr: core.ErrTimeProofInvalid,
		},
		{
			name:       "signing time attribute carrying no value",
			attributes: []cmsAttribute{signingTimeAttribute(t)},
			wantErr:    core.ErrTimeProofInvalid,
		},
		{
			name:       "signing time attribute carrying two values",
			attributes: []cmsAttribute{signingTimeAttribute(t, when, when)},
			wantErr:    core.ErrTimeProofInvalid,
		},
		{
			name:       "signing time value that is not a time",
			attributes: []cmsAttribute{notATime},
			wantErr:    core.ErrTimeProofInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := validateSigningTimeAttribute(tc.attributes)
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf(
						"validateSigningTimeAttribute(valid shape) error = %v, want nil",
						gotErr,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"validateSigningTimeAttribute(invalid shape) error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func accuracyRaw(t testing.TB, wire accuracyWire) asn1.RawValue {
	t.Helper()

	encoded, err := asn1.Marshal(wire)
	if err != nil {
		t.Fatalf("asn1.Marshal(accuracy) error = %v, want nil", err)
	}
	return rawValueFromDER(t, encoded)
}

func TestAccuracyBoundaryTable(t *testing.T) {
	t.Parallel()

	maximumSeconds := int64(math.MaxInt64) / int64(time.Second)
	cases := []struct {
		wantErr         error
		name            string
		wire            accuracyWire
		wantNanoseconds int64
	}{
		{name: "one second", wire: accuracyWire{Seconds: 1}, wantNanoseconds: int64(time.Second)},
		{name: "maximum whole seconds fitting Duration", wire: accuracyWire{Seconds: int(maximumSeconds)}, wantNanoseconds: maximumSeconds * int64(time.Second)},
		{name: "one millisecond", wire: accuracyWire{Millis: 1}, wantNanoseconds: int64(time.Millisecond)},
		{name: "maximum 999 milliseconds", wire: accuracyWire{Millis: 999}, wantNanoseconds: 999 * int64(time.Millisecond)},
		{name: "one microsecond", wire: accuracyWire{Micros: 1}, wantNanoseconds: int64(time.Microsecond)},
		{name: "maximum 999 microseconds", wire: accuracyWire{Micros: 999}, wantNanoseconds: 999 * int64(time.Microsecond)},
		{name: "seconds plus milliseconds", wire: accuracyWire{Seconds: 1, Millis: 1}, wantNanoseconds: int64(time.Second + time.Millisecond)},
		{name: "seconds plus microseconds", wire: accuracyWire{Seconds: 1, Micros: 1}, wantNanoseconds: int64(time.Second + time.Microsecond)},
		{name: "both subsecond components", wire: accuracyWire{Millis: 1, Micros: 1}, wantNanoseconds: int64(time.Millisecond + time.Microsecond)},
		{name: "all components at subsecond ceilings", wire: accuracyWire{Seconds: 2, Millis: 999, Micros: 999}, wantNanoseconds: int64(2*time.Second + 999*time.Millisecond + 999*time.Microsecond)},
		{name: "empty accuracy has no meaning", wire: accuracyWire{}, wantErr: core.ErrTimeProofInvalid},
		{name: "negative seconds", wire: accuracyWire{Seconds: -1}, wantErr: core.ErrTimeProofInvalid},
		{name: "negative milliseconds", wire: accuracyWire{Millis: -1}, wantErr: core.ErrTimeProofInvalid},
		{name: "milliseconds one above 999", wire: accuracyWire{Millis: 1000}, wantErr: core.ErrTimeProofInvalid},
		{name: "milliseconds far above 999", wire: accuracyWire{Millis: math.MaxInt32}, wantErr: core.ErrTimeProofInvalid},
		{name: "negative microseconds", wire: accuracyWire{Micros: -1}, wantErr: core.ErrTimeProofInvalid},
		{name: "microseconds one above 999", wire: accuracyWire{Micros: 1000}, wantErr: core.ErrTimeProofInvalid},
		{name: "microseconds far above 999", wire: accuracyWire{Micros: math.MaxInt32}, wantErr: core.ErrTimeProofInvalid},
		{name: "whole seconds one above Duration", wire: accuracyWire{Seconds: int(maximumSeconds + 1)}, wantErr: core.ErrTimeProofInvalid},
	}
	if strconv.IntSize == 64 {
		const wrappingSeconds int64 = 18_446_744_074
		cases = append(cases, struct {
			wantErr         error
			name            string
			wire            accuracyWire
			wantNanoseconds int64
		}{
			name:    "huge seconds cannot wrap into a small positive duration",
			wire:    accuracyWire{Seconds: int(wrappingSeconds)},
			wantErr: core.ErrTimeProofInvalid,
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseAccuracy(accuracyRaw(t, tc.wire))
			if tc.wantErr == nil {
				if gotErr != nil || got.Nanoseconds() != tc.wantNanoseconds {
					t.Fatalf(
						"parseAccuracy(%+v) = (%d, %v), want (%d, nil)",
						tc.wire,
						got.Nanoseconds(),
						gotErr,
						tc.wantNanoseconds,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !got.IsZero() {
				t.Fatalf(
					"parseAccuracy(%+v) = (%d, %v), want (0, %v)",
					tc.wire,
					got.Nanoseconds(),
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func signingCertificateV2Attribute(
	t testing.TB,
	signer *x509.Certificate,
) cmsAttribute {
	return signingCertificateV2AttributeWithAlgorithm(t, signer, nil)
}

func signingCertificateV2AttributeWithAlgorithm(
	t testing.TB,
	signer *x509.Certificate,
	algorithm []byte,
) cmsAttribute {
	t.Helper()

	digest := sha256.Sum256(signer.Raw)
	hashDER, err := asn1.Marshal(digest[:])
	if err != nil {
		t.Fatalf("asn1.Marshal(ESSCertIDv2 hash) error = %v, want nil", err)
	}
	certificateFields := append(append([]byte(nil), algorithm...), hashDER...)
	certificateID := derTagged(
		byte(asn1.TagSequence)|derConstructed,
		certificateFields,
	)
	certificates := derTagged(
		byte(asn1.TagSequence)|derConstructed,
		certificateID,
	)
	value := derTagged(
		byte(asn1.TagSequence)|derConstructed,
		certificates,
	)
	return cmsAttribute{
		Type:   oidSigningCertificateV2(),
		Values: []asn1.RawValue{rawValueFromDER(t, value)},
	}
}

func issuerSerialFields(
	t testing.TB,
	names []asn1.RawValue,
	serial *big.Int,
) []byte {
	t.Helper()

	serialDER, err := asn1.Marshal(serial)
	if err != nil {
		t.Fatalf("asn1.Marshal(issuer serial) error = %v, want nil", err)
	}
	return issuerSerialFieldsWithSerialDER(t, names, serialDER)
}

func issuerSerialFieldsWithSerialDER(
	t testing.TB,
	names []asn1.RawValue,
	serialDER []byte,
) []byte {
	t.Helper()

	var nameFields []byte
	for _, name := range names {
		nameFields = append(nameFields, name.FullBytes...)
	}
	generalNames := derTagged(byte(asn1.TagSequence)|derConstructed, nameFields)
	return derTagged(
		byte(asn1.TagSequence)|derConstructed,
		append(generalNames, serialDER...),
	)
}

func issuerGeneralName(t testing.TB, tag byte, content []byte) asn1.RawValue {
	t.Helper()

	return rawValueFromDER(t, derTagged(0x80|tag, content))
}

func issuerDirectoryName(t testing.TB, rawIssuer []byte) asn1.RawValue {
	t.Helper()

	return rawValueFromDER(
		t,
		derTagged(0x80|derConstructed|generalNameDirectoryNameTag, rawIssuer),
	)
}

func TestOptionalIssuerSerialBindsExactSignerIdentity(t *testing.T) {
	t.Parallel()

	token, err := parseTimestampToken(authenticTokenDER(t))
	if err != nil {
		t.Fatalf("parseTimestampToken(authentic) error = %v, want nil", err)
	}
	signer := token.Signer
	exactName := issuerDirectoryName(t, signer.RawIssuer)
	foreignIssuer := append([]byte(nil), signer.RawIssuer...)
	foreignIssuer[len(foreignIssuer)-1] ^= 0x01
	foreignName := issuerDirectoryName(t, foreignIssuer)
	dnsName := issuerGeneralName(t, 2, []byte("tsa.example"))
	mailName := issuerGeneralName(t, 1, []byte("tsa@example"))
	uriName := issuerGeneralName(t, 6, []byte("https://tsa.example"))
	ipName := issuerGeneralName(t, 7, []byte{127, 0, 0, 1})
	registeredName := issuerGeneralName(t, 8, []byte{0x2a, 0x03})
	exactSerial := new(big.Int).Set(signer.SerialNumber)

	accepted := []struct {
		name  string
		names []asn1.RawValue
	}{
		{name: "exact directory name is the sole general name", names: []asn1.RawValue{exactName}},
		{name: "directory name follows DNS name", names: []asn1.RawValue{dnsName, exactName}},
		{name: "directory name precedes DNS name", names: []asn1.RawValue{exactName, dnsName}},
		{name: "duplicate exact directory names remain the same identity", names: []asn1.RawValue{exactName, exactName}},
		{name: "directory name follows mail name", names: []asn1.RawValue{mailName, exactName}},
		{name: "directory name follows URI name", names: []asn1.RawValue{uriName, exactName}},
		{name: "directory name follows IP name", names: []asn1.RawValue{ipName, exactName}},
		{name: "directory name follows registered identifier", names: []asn1.RawValue{registeredName, exactName}},
		{name: "exact directory name follows a foreign directory name", names: []asn1.RawValue{foreignName, exactName}},
		{name: "exact directory name survives a heterogeneous name set", names: []asn1.RawValue{dnsName, mailName, uriName, ipName, registeredName, foreignName, exactName}},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := verifyOptionalIssuerSerial(issuerSerialFields(t, tc.names, exactSerial), signer)
			if gotErr != nil {
				t.Fatalf("verifyOptionalIssuerSerial(accepted identity set) error = %v, want nil", gotErr)
			}
		})
	}

	oneAboveSerial := new(big.Int).Add(new(big.Int).Set(exactSerial), big.NewInt(1))
	oneBelowSerial := new(big.Int).Sub(new(big.Int).Set(exactSerial), big.NewInt(1))
	overMaximumSerial := new(big.Int).Lsh(big.NewInt(1), SerialMaximumBits)
	exactSerialDER, err := asn1.Marshal(exactSerial)
	if err != nil {
		t.Fatalf("asn1.Marshal(exact serial) error = %v, want nil", err)
	}
	rejected := []struct {
		name   string
		fields []byte
	}{
		{name: "truncated outer issuer sequence", fields: []byte{byte(asn1.TagSequence) | derConstructed}},
		{name: "integer cannot replace outer issuer sequence", fields: exactSerialDER},
		{name: "trailing field follows outer issuer sequence", fields: append(issuerSerialFields(t, []asn1.RawValue{exactName}, exactSerial), 0x05, 0x00)},
		{name: "empty outer issuer sequence", fields: derTagged(byte(asn1.TagSequence)|derConstructed, nil)},
		{name: "serial cannot replace general names", fields: derTagged(byte(asn1.TagSequence)|derConstructed, exactSerialDER)},
		{name: "general names without serial", fields: issuerSerialFieldsWithSerialDER(t, []asn1.RawValue{exactName}, nil)},
		{name: "empty general names do not identify signer", fields: issuerSerialFields(t, nil, exactSerial)},
		{name: "foreign directory name does not identify signer", fields: issuerSerialFields(t, []asn1.RawValue{foreignName}, exactSerial)},
		{name: "non-directory general name with issuer bytes does not identify signer", fields: issuerSerialFields(t, []asn1.RawValue{issuerGeneralName(t, 5, signer.RawIssuer)}, exactSerial)},
		{name: "raw issuer sequence without directory-name tag does not identify signer", fields: issuerSerialFields(t, []asn1.RawValue{rawValueFromDER(t, signer.RawIssuer)}, exactSerial)},
		{name: "serial one below signer rejects", fields: issuerSerialFields(t, []asn1.RawValue{exactName}, oneBelowSerial)},
		{name: "serial one above signer rejects", fields: issuerSerialFields(t, []asn1.RawValue{exactName}, oneAboveSerial)},
		{name: "zero serial rejects", fields: issuerSerialFields(t, []asn1.RawValue{exactName}, big.NewInt(0))},
		{name: "negative serial rejects", fields: issuerSerialFields(t, []asn1.RawValue{exactName}, big.NewInt(-1))},
		{name: "serial one bit above RFC ceiling rejects", fields: issuerSerialFields(t, []asn1.RawValue{exactName}, overMaximumSerial)},
		{name: "trailing field follows exact serial inside issuer sequence", fields: issuerSerialFieldsWithSerialDER(t, []asn1.RawValue{exactName}, append(exactSerialDER, 0x05, 0x00))},
		{name: "truncated general name rejects", fields: derTagged(byte(asn1.TagSequence)|derConstructed, append([]byte{byte(asn1.TagSequence) | derConstructed, 0x01}, exactSerialDER...))},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := verifyOptionalIssuerSerial(tc.fields, signer)
			if !errors.Is(gotErr, core.ErrTimeProofInvalid) {
				t.Fatalf("verifyOptionalIssuerSerial(hostile identity) error = %v, want %v", gotErr, core.ErrTimeProofInvalid)
			}
		})
	}
}

func TestSigningCertificateAttributeClosure(t *testing.T) {
	t.Parallel()

	token, err := parseTimestampToken(authenticTokenDER(t))
	if err != nil {
		t.Fatalf("parseTimestampToken(authentic) error = %v, want nil", err)
	}
	v1, v1Count := countAttribute(token.Attributes, oidSigningCertificate())
	if v1Count != 1 {
		t.Fatalf("authentic SigningCertificate count = %d, want 1", v1Count)
	}
	v2 := signingCertificateV2Attribute(t, token.Signer)
	sha256OID, err := asn1.Marshal(oidSHA256())
	if err != nil {
		t.Fatalf("asn1.Marshal(SHA-256 OID) error = %v, want nil", err)
	}
	explicitDefaultV2 := signingCertificateV2AttributeWithAlgorithm(
		t,
		token.Signer,
		derTagged(byte(asn1.TagSequence)|derConstructed, sha256OID),
	)
	invalidV2 := v2
	invalidV2.Values = append([]asn1.RawValue(nil), v2.Values...)
	invalidV2.Values[0].FullBytes = append(
		[]byte(nil),
		v2.Values[0].FullBytes...,
	)
	invalidV2.Values[0].FullBytes[len(invalidV2.Values[0].FullBytes)-1] ^= 1
	cases := []struct {
		wantErr    error
		signer     *x509.Certificate
		name       string
		attributes []cmsAttribute
	}{
		{name: "ESSCertID v1 alone binds the signer", attributes: []cmsAttribute{v1}, signer: token.Signer},
		{name: "ESSCertID v2 alone binds the signer", attributes: []cmsAttribute{v2}, signer: token.Signer},
		{name: "RFC 5816 permits both identifiers when both bind", attributes: []cmsAttribute{v1, v2}, signer: token.Signer},
		{name: "ESSCertID v2 rejects explicit DER default SHA-256 algorithm", attributes: []cmsAttribute{explicitDefaultV2}, signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
		{name: "no certificate identifier", signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
		{name: "absent signer", attributes: []cmsAttribute{v1}, wantErr: core.ErrTimeProofInvalid},
		{name: "duplicate v1 identifier", attributes: []cmsAttribute{v1, v1}, signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
		{name: "duplicate v2 identifier", attributes: []cmsAttribute{v2, v2}, signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
		{name: "invalid v2 identifier", attributes: []cmsAttribute{invalidV2}, signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
		{name: "valid v1 cannot hide invalid v2", attributes: []cmsAttribute{v1, invalidV2}, signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := verifySigningCertificateAttribute(tc.attributes, tc.signer)
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf(
						"verifySigningCertificateAttribute(valid) error = %v, want nil",
						gotErr,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"verifySigningCertificateAttribute(invalid) error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestTSANameMatchesVerifiedSigner(t *testing.T) {
	t.Parallel()

	token, err := parseTimestampToken(authenticTokenDER(t))
	if err != nil {
		t.Fatalf("parseTimestampToken(authentic) error = %v, want nil", err)
	}
	info, err := parseTSTInfo(token.TSTDER)
	if err != nil {
		t.Fatalf("parseTSTInfo(authentic) error = %v, want nil", err)
	}
	if len(info.TSASubject) == 0 {
		t.Fatal("authentic TSTInfo TSA subject bytes = 0, want the signed directory name")
	}
	cases := []struct {
		wantErr error
		signer  *x509.Certificate
		name    string
		subject []byte
	}{
		{name: "authentic TSA directory name matches signer", subject: info.TSASubject, signer: token.Signer},
		{name: "absent optional TSA name is accepted", signer: token.Signer},
		{name: "TSA name with absent signer", subject: info.TSASubject, wantErr: core.ErrTimeProofInvalid},
		{name: "issuer name cannot replace TSA signer name", subject: token.Signer.RawIssuer, signer: token.Signer, wantErr: core.ErrTimeProofInvalid},
		{name: "empty signer subject cannot match asserted TSA", subject: info.TSASubject, signer: &x509.Certificate{}, wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := verifyTSAName(tc.subject, tc.signer)
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("verifyTSAName(valid) error = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"verifyTSAName(invalid) error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestAuthorityRegistryClosure(t *testing.T) {
	t.Parallel()

	count := 0
	for authority := AuthorityUnknown + 1; authority < authorityLimit; authority++ {
		count++
		if err := authority.Validate(); err != nil {
			t.Fatalf("Authority(%d).Validate() error = %v, want nil", authority, err)
		}
		registry, err := authorityRegistry(authority)
		if err != nil {
			t.Fatalf(
				"authorityRegistry(%v) error = %v, want a registry contract for every closed member",
				authority,
				err,
			)
		}
		if registry.root == nil || !registry.root.IsCA {
			t.Fatalf(
				"authorityRegistry(%v) root = %v, want a certificate authority anchor",
				authority,
				registry.root,
			)
		}
		if err := registry.endpoint.Validate(); err != nil {
			t.Fatalf(
				"authorityRegistry(%v) endpoint validate error = %v, want nil",
				authority,
				err,
			)
		}
		if err := registry.policy.Validate(); err != nil {
			t.Fatalf(
				"authorityRegistry(%v) policy validate error = %v, want nil",
				authority,
				err,
			)
		}
		if registry.policyOID.String() != registry.policy.String() {
			t.Fatalf(
				"authorityRegistry(%v) policy OID/token = (%s, %s), want one identity",
				authority,
				registry.policyOID.String(),
				registry.policy.String(),
			)
		}
		if authority.String() == "" {
			t.Fatalf("Authority(%d).String() = %q, want a canonical token", authority, "")
		}
	}
	if count == 0 {
		t.Fatal("closed authority count = 0, want at least one registered authority")
	}

	for _, authority := range []Authority{
		AuthorityUnknown, authorityLimit, authorityLimit + 1, Authority(200),
	} {
		if err := authority.Validate(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"Authority(%d).Validate() error = %v, want %v",
				authority,
				err,
				core.ErrTimeProofContract,
			)
		}
		if _, err := authorityRegistry(authority); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"authorityRegistry(%d) error = %v, want %v",
				authority,
				err,
				core.ErrTimeProofContract,
			)
		}
	}

	for policy := TimestampPolicyUnknown + 1; policy < timestampPolicyLimit; policy++ {
		if err := policy.Validate(); err != nil || policy.String() == "" {
			t.Fatalf(
				"TimestampPolicy(%d) validate/token = (%v, %q), want (nil, canonical)",
				policy,
				err,
				policy.String(),
			)
		}
	}
	for _, policy := range []TimestampPolicy{
		TimestampPolicyUnknown, timestampPolicyLimit, TimestampPolicy(200),
	} {
		if err := policy.Validate(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"TimestampPolicy(%d).Validate() error = %v, want %v",
				policy,
				err,
				core.ErrTimeProofContract,
			)
		}
	}
}

func TestFreeTSATrustAnchorIsFreshlyParsedAndPinned(t *testing.T) {
	t.Parallel()

	root, err := loadFreeTSARoot()
	if err != nil {
		t.Fatalf("loadFreeTSARoot() error = %v, want nil", err)
	}
	again, err := loadFreeTSARoot()
	if err != nil || again == root || !bytes.Equal(again.Raw, root.Raw) {
		t.Fatalf(
			"loadFreeTSARoot() second call = (%p, %v), want a fresh parse of anchor %p",
			again,
			err,
			root,
		)
	}
	if !root.IsCA || !root.BasicConstraintsValid {
		t.Fatalf(
			"pinned anchor isCA/basicConstraintsValid = (%t, %t), want (true, true)",
			root.IsCA,
			root.BasicConstraintsValid,
		)
	}
	if err := verifyFreeTSARoot(root); err != nil {
		t.Fatalf("verifyFreeTSARoot(pinned anchor) error = %v, want nil", err)
	}
	if !strings.Contains(root.Subject.CommonName, "freetsa.org") {
		t.Fatalf(
			"pinned anchor subject = %q, want the reviewed FreeTSA root",
			root.Subject.CommonName,
		)
	}

	t.Run("absent anchor is not the pinned identity", func(t *testing.T) {
		t.Parallel()

		if err := verifyFreeTSARoot(nil); !errors.Is(err, core.ErrTimeProofInvalid) {
			t.Fatalf(
				"verifyFreeTSARoot(nil) error = %v, want %v",
				err,
				core.ErrTimeProofInvalid,
			)
		}
	})

	t.Run("a different real certificate is not the pinned identity", func(t *testing.T) {
		t.Parallel()

		tokenDER := authenticTokenDER(t)
		token, err := parseTimestampToken(tokenDER)
		if err != nil {
			t.Fatalf("parseTimestampToken(authentic) error = %v, want nil", err)
		}
		if err := verifyFreeTSARoot(token.Signer); !errors.Is(err, core.ErrTimeProofInvalid) {
			t.Fatalf(
				"verifyFreeTSARoot(signer certificate) error = %v, want %v",
				err,
				core.ErrTimeProofInvalid,
			)
		}
	})

	t.Run("a mutated anchor is not the pinned identity", func(t *testing.T) {
		t.Parallel()

		mutated := &x509.Certificate{}
		*mutated = *root
		mutated.Raw = append([]byte(nil), root.Raw...)
		mutated.Raw[len(mutated.Raw)-1] ^= 1
		if err := verifyFreeTSARoot(mutated); !errors.Is(err, core.ErrTimeProofInvalid) {
			t.Fatalf(
				"verifyFreeTSARoot(mutated anchor) error = %v, want %v",
				err,
				core.ErrTimeProofInvalid,
			)
		}
	})
}

func TestSerialNumberIngressTable(t *testing.T) {
	t.Parallel()

	maximum := strings.Repeat("ff", serialMaximumBytes)
	cases := []struct {
		wantErr error
		name    string
		json    string
	}{
		{name: "one byte minimum serial is accepted", json: `"01"`},
		{name: "high bit set in one byte is accepted", json: `"ff"`},
		{name: "two byte serial is accepted", json: `"0102"`},
		{name: "exact 160 bit ceiling is accepted", json: `"` + maximum + `"`},
		{name: "one byte below the ceiling is accepted", json: `"` + strings.Repeat("ff", serialMaximumBytes-1) + `"`},
		{name: "interior zero bytes are accepted", json: `"01000001"`},
		{name: "one byte above the ceiling", json: `"` + strings.Repeat("ff", serialMaximumBytes+1) + `"`, wantErr: core.ErrTimeProofContract},
		{name: "far above the ceiling", json: `"` + strings.Repeat("ff", 4*serialMaximumBytes) + `"`, wantErr: core.ErrTimeProofContract},
		{name: "empty token", json: `""`, wantErr: core.ErrTimeProofContract},
		{name: "single nibble token", json: `"1"`, wantErr: core.ErrTimeProofContract},
		{name: "odd length token", json: `"010"`, wantErr: core.ErrTimeProofContract},
		{name: "zero serial", json: `"00"`, wantErr: core.ErrTimeProofContract},
		{name: "leading zero byte is noncanonical", json: `"0001"`, wantErr: core.ErrTimeProofContract},
		{name: "many leading zero bytes are noncanonical", json: `"00000001"`, wantErr: core.ErrTimeProofContract},
		{name: "uppercase hexadecimal is noncanonical", json: `"0A"`, wantErr: core.ErrTimeProofContract},
		{name: "mixed case hexadecimal is noncanonical", json: `"aB"`, wantErr: core.ErrTimeProofContract},
		{name: "non hexadecimal token", json: `"zz"`, wantErr: core.ErrTimeProofContract},
		{name: "prefixed hexadecimal token", json: `"0x01"`, wantErr: core.ErrTimeProofContract},
		{name: "whitespace padded token", json: `" 01"`, wantErr: core.ErrTimeProofContract},
		{name: "JSON number instead of token", json: `1`, wantErr: core.ErrJSONContract},
		{name: "JSON null instead of token", json: `null`, wantErr: core.ErrJSONContract},
		{name: "JSON object instead of token", json: `{}`, wantErr: core.ErrJSONContract},
		{name: "unterminated JSON string", json: `"01`, wantErr: core.ErrJSONContract},
		{name: "empty JSON document", json: ``, wantErr: core.ErrJSONContract},
		{name: "oversized JSON document", json: `"` + strings.Repeat("a", serialJSONMaximumBytes) + `"`, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original, err := parseSerialNumber("7f")
			if err != nil {
				t.Fatalf("parseSerialNumber(control) error = %v, want nil", err)
			}
			got := original
			gotErr := got.UnmarshalJSON([]byte(tc.json))
			if tc.wantErr == nil {
				encoded, marshalErr := got.MarshalJSON()
				if gotErr != nil || marshalErr != nil ||
					!bytes.Equal(encoded, []byte(tc.json)) {
					t.Fatalf(
						"SerialNumber round trip = (%q, %v, %v), want (%s, nil, nil)",
						encoded,
						gotErr,
						marshalErr,
						tc.json,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || got != original {
				t.Fatalf(
					"SerialNumber.UnmarshalJSON(%s) receiver/error = (%q, %v), want unchanged %q and %v",
					tc.json,
					got.String(),
					gotErr,
					original.String(),
					tc.wantErr,
				)
			}
		})
	}
}

func TestNonceIngressTable(t *testing.T) {
	t.Parallel()

	canonical := "000000000000000035eac9aacdfde4e5"
	cases := []struct {
		wantErr error
		name    string
		json    string
	}{
		{name: "canonical lowercase nonce is accepted", json: `"` + canonical + `"`},
		{name: "maximum nonce is accepted", json: `"` + strings.Repeat("ff", NonceBytes) + `"`},
		{name: "minimum nonzero nonce is accepted", json: `"` + strings.Repeat("0", 2*NonceBytes-1) + `1"`},
		{name: "all zero nonce is not a nonce", json: `"` + strings.Repeat("0", 2*NonceBytes) + `"`, wantErr: core.ErrTimeProofContract},
		{name: "one nibble short", json: `"` + canonical[1:] + `"`, wantErr: core.ErrTimeProofContract},
		{name: "one nibble long", json: `"0` + canonical + `"`, wantErr: core.ErrTimeProofContract},
		{name: "uppercase hexadecimal is noncanonical", json: `"000000000000000035EAC9AACDFDE4E5"`, wantErr: core.ErrTimeProofContract},
		{name: "non hexadecimal nonce", json: `"` + strings.Repeat("z", 2*NonceBytes) + `"`, wantErr: core.ErrTimeProofContract},
		{name: "empty token", json: `""`, wantErr: core.ErrTimeProofContract},
		{name: "unquoted token", json: canonical, wantErr: core.ErrJSONContract},
		{name: "JSON null instead of token", json: `null`, wantErr: core.ErrJSONContract},
		{name: "empty JSON document", json: ``, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original, err := parseNonce(canonical)
			if err != nil {
				t.Fatalf("parseNonce(control) error = %v, want nil", err)
			}
			got := original
			gotErr := got.UnmarshalJSON([]byte(tc.json))
			if tc.wantErr == nil {
				encoded, marshalErr := got.MarshalJSON()
				if gotErr != nil || marshalErr != nil ||
					!bytes.Equal(encoded, []byte(tc.json)) {
					t.Fatalf(
						"Nonce round trip = (%q, %v, %v), want (%s, nil, nil)",
						encoded,
						gotErr,
						marshalErr,
						tc.json,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || got != original {
				t.Fatalf(
					"Nonce.UnmarshalJSON(%s) receiver/error = (%q, %v), want unchanged %q and %v",
					tc.json,
					got.String(),
					gotErr,
					original.String(),
					tc.wantErr,
				)
			}
		})
	}
}

func TestClosedEnumJSONIngressTable(t *testing.T) {
	t.Parallel()

	t.Run("authority tokens", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			wantErr error
			name    string
			json    string
		}{
			{name: "canonical FreeTSA token is accepted", json: `"freetsa"`},
			{name: "canonical DigiCert token is accepted", json: `"digicert"`},
			{name: "capitalized token is noncanonical", json: `"FreeTSA"`, wantErr: core.ErrTimeProofContract},
			{name: "uppercase token is noncanonical", json: `"FREETSA"`, wantErr: core.ErrTimeProofContract},
			{name: "empty token is not an authority", json: `""`, wantErr: core.ErrTimeProofContract},
			{name: "unknown authority token", json: `"sectigo"`, wantErr: core.ErrTimeProofContract},
			{name: "policy OID is not an authority token", json: `"1.2.3.4.1"`, wantErr: core.ErrTimeProofContract},
			{name: "whitespace padded token", json: `" freetsa"`, wantErr: core.ErrTimeProofContract},
			{name: "JSON number instead of token", json: `1`, wantErr: core.ErrJSONContract},
			{name: "JSON null instead of token", json: `null`, wantErr: core.ErrJSONContract},
			{name: "empty JSON document", json: ``, wantErr: core.ErrJSONContract},
			{name: "oversized JSON document", json: `"` + strings.Repeat("a", enumJSONMaximumBytes) + `"`, wantErr: core.ErrJSONContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := AuthorityFreeTSA
				gotErr := got.UnmarshalJSON([]byte(tc.json))
				if tc.wantErr == nil {
					encoded, marshalErr := got.MarshalJSON()
					if gotErr != nil || marshalErr != nil ||
						!bytes.Equal(encoded, []byte(tc.json)) {
						t.Fatalf(
							"Authority round trip = (%q, %v, %v), want (%s, nil, nil)",
							encoded,
							gotErr,
							marshalErr,
							tc.json,
						)
					}
					return
				}
				if !errors.Is(gotErr, tc.wantErr) || got != AuthorityFreeTSA {
					t.Fatalf(
						"Authority.UnmarshalJSON(%s) receiver/error = (%v, %v), want unchanged and %v",
						tc.json,
						got,
						gotErr,
						tc.wantErr,
					)
				}
			})
		}
	})

	t.Run("policy tokens", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			wantErr error
			name    string
			json    string
		}{
			{name: "canonical FreeTSA policy OID is accepted", json: `"1.2.3.4.1"`},
			{name: "canonical DigiCert policy OID is accepted", json: `"2.16.840.1.114412.7.1"`},
			{name: "sibling arc is not the reviewed policy", json: `"1.2.3.4.2"`, wantErr: core.ErrTimeProofContract},
			{name: "parent arc is not the reviewed policy", json: `"1.2.3.4"`, wantErr: core.ErrTimeProofContract},
			{name: "child arc is not the reviewed policy", json: `"1.2.3.4.1.1"`, wantErr: core.ErrTimeProofContract},
			{name: "trailing separator is noncanonical", json: `"1.2.3.4.1."`, wantErr: core.ErrTimeProofContract},
			{name: "leading zero arc is noncanonical", json: `"1.2.3.4.01"`, wantErr: core.ErrTimeProofContract},
			{name: "empty token is not a policy", json: `""`, wantErr: core.ErrTimeProofContract},
			{name: "authority token is not a policy", json: `"freetsa"`, wantErr: core.ErrTimeProofContract},
			{name: "JSON number instead of token", json: `1`, wantErr: core.ErrJSONContract},
			{name: "empty JSON document", json: ``, wantErr: core.ErrJSONContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := TimestampPolicyFreeTSA
				gotErr := got.UnmarshalJSON([]byte(tc.json))
				if tc.wantErr == nil {
					encoded, marshalErr := got.MarshalJSON()
					if gotErr != nil || marshalErr != nil ||
						!bytes.Equal(encoded, []byte(tc.json)) {
						t.Fatalf(
							"TimestampPolicy round trip = (%q, %v, %v), want (%s, nil, nil)",
							encoded,
							gotErr,
							marshalErr,
							tc.json,
						)
					}
					return
				}
				if !errors.Is(gotErr, tc.wantErr) || got != TimestampPolicyFreeTSA {
					t.Fatalf(
						"TimestampPolicy.UnmarshalJSON(%s) receiver/error = (%v, %v), want unchanged and %v",
						tc.json,
						got,
						gotErr,
						tc.wantErr,
					)
				}
			})
		}
	})

	t.Run("zero enums cannot serialize a claim", func(t *testing.T) {
		t.Parallel()

		if _, err := AuthorityUnknown.MarshalJSON(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"AuthorityUnknown.MarshalJSON() error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
		if _, err := TimestampPolicyUnknown.MarshalJSON(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"TimestampPolicyUnknown.MarshalJSON() error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
	})
}
