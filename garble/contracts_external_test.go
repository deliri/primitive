package garble_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

func TestSeedCanonicalBoundaryIncludesAllZeroValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  [garble.SeedBytes]byte
	}{
		{name: "all zero seed admitted"},
		{name: "lowest bit in first byte admitted", raw: [garble.SeedBytes]byte{1}},
		{name: "highest bit in first byte admitted", raw: [garble.SeedBytes]byte{0x80}},
		{name: "lowest bit in final byte admitted", raw: [garble.SeedBytes]byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{name: "highest bit in final byte admitted", raw: [garble.SeedBytes]byte{0, 0, 0, 0, 0, 0, 0, 0x80}},
		{name: "high bits admitted", raw: [garble.SeedBytes]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{name: "mixed seed admitted", raw: [garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8}},
		{name: "alternating low pattern admitted", raw: [garble.SeedBytes]byte{0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa}},
		{name: "alternating high pattern admitted", raw: [garble.SeedBytes]byte{0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55}},
		{name: "internal zeros admitted", raw: [garble.SeedBytes]byte{1, 0, 2, 0, 3, 0, 4, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			seed := garble.NewSeed(tc.raw)
			if gotErr := seed.Validate(); gotErr != nil {
				t.Fatalf("Seed.Validate() error = %v, want nil", gotErr)
			}
			gotBytes, gotBytesErr := seed.Bytes()
			if gotBytesErr != nil || gotBytes != tc.raw {
				t.Fatalf("Seed.Bytes() = (%v, %v), want (%v, nil)", gotBytes, gotBytesErr, tc.raw)
			}
			wantEncoded := base64.RawStdEncoding.EncodeToString(tc.raw[:])
			gotEncoded, gotEncodedErr := seed.Encoded()
			if gotEncodedErr != nil || gotEncoded != wantEncoded {
				t.Fatalf("Seed.Encoded() = (%q, %v), want (%q, nil)", gotEncoded, gotEncodedErr, wantEncoded)
			}
			gotParsed, gotParseErr := garble.ParseSeed(wantEncoded)
			if gotParseErr != nil || gotParsed != seed {
				t.Fatalf("ParseSeed(%q) = (%v, %v), want (%v, nil)", wantEncoded, gotParsed, gotParseErr, seed)
			}
		})
	}
}

func TestSeedRejectsEveryLossyGarbleCLIForm(t *testing.T) {
	t.Parallel()

	sevenBytes := base64.RawStdEncoding.EncodeToString(make([]byte, garble.SeedBytes-1))
	nineBytes := base64.RawStdEncoding.EncodeToString(make([]byte, garble.SeedBytes+1))
	canonical := base64.RawStdEncoding.EncodeToString(make([]byte, garble.SeedBytes))
	cases := []struct {
		name  string
		value string
	}{
		{name: "empty rejected"},
		{name: "seven decoded bytes rejected", value: sevenBytes},
		{name: "nine decoded bytes rejected instead of terminal truncation", value: nineBytes},
		{name: "padding rejected instead of terminal normalization", value: canonical + "="},
		{name: "random token rejected", value: "random"},
		{name: "URL alphabet rejected", value: "__________8"},
		{name: "invalid base64 rejected", value: "not_base64!"},
		{name: "leading space rejected", value: " " + canonical},
		{name: "trailing space rejected", value: canonical + " "},
		{name: "leading newline rejected", value: "\n" + canonical},
		{name: "embedded padding rejected", value: canonical[:5] + "=" + canonical[6:]},
		{name: "quoted JSON token rejected", value: `"` + canonical + `"`},
		{name: "one canonical character truncated", value: canonical[:len(canonical)-1]},
		{name: "one canonical character appended", value: canonical + "A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := garble.ParseSeed(tc.value)
			if got != (garble.Seed{}) ||
				!errors.Is(gotErr, core.ErrGarbleContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"ParseSeed(%q) = (%v, %v), want (zero, %v and %v)",
					tc.value,
					got,
					gotErr,
					core.ErrGarbleContract,
					core.ErrPrimitiveContract,
				)
			}
		})
	}

	if gotErr := (garble.Seed{}).Validate(); !errors.Is(gotErr, core.ErrGarbleContract) {
		t.Fatalf("Seed{}.Validate() error = %v, want %v", gotErr, core.ErrGarbleContract)
	}
}

func TestSeedJSONBoundaryAndReceiverPreservation(t *testing.T) {
	t.Parallel()

	seed := garble.NewSeed([garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8})
	wire, gotMarshalErr := json.Marshal(seed)
	if gotMarshalErr != nil {
		t.Fatalf("json.Marshal(Seed) error = %v, want nil", gotMarshalErr)
	}
	if len(wire) != garble.SeedCanonicalJSONBytes {
		t.Fatalf("len(json.Marshal(Seed)) = %d, want %d", len(wire), garble.SeedCanonicalJSONBytes)
	}

	validCases := []struct {
		name string
		data []byte
	}{
		{name: "canonical compact document", data: append([]byte(nil), wire...)},
		{name: "one leading space", data: append([]byte{' '}, wire...)},
		{name: "one trailing space", data: append(append([]byte(nil), wire...), ' ')},
		{name: "leading and trailing spaces", data: append(append([]byte{' '}, wire...), ' ')},
		{name: "leading tab", data: append([]byte{'\t'}, wire...)},
		{name: "trailing newline", data: append(append([]byte(nil), wire...), '\n')},
		{name: "leading CRLF", data: append([]byte{'\r', '\n'}, wire...)},
		{name: "equivalent JSON unicode escape", data: []byte(`"\u0041QIDBAUGBwg"`)},
		{
			name: "one below whitespace allowance",
			data: append(
				append([]byte(nil), wire...),
				bytes.Repeat([]byte{' '}, garble.SeedJSONWhitespaceAllowanceBytes-1)...,
			),
		},
		{
			name: "exact whitespace allowance",
			data: append(
				append([]byte(nil), wire...),
				bytes.Repeat([]byte{' '}, garble.SeedJSONWhitespaceAllowanceBytes)...,
			),
		},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got garble.Seed
			gotErr := got.UnmarshalJSON(tc.data)
			if gotErr != nil || got != seed {
				t.Fatalf("Seed.UnmarshalJSON(%q) = (%v, %v), want (%v, nil)", tc.data, got, gotErr, seed)
			}
		})
	}

	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty"},
		{name: "null", data: []byte("null")},
		{name: "boolean", data: []byte("true")},
		{name: "array", data: []byte("[]")},
		{name: "object", data: []byte("{}")},
		{name: "padded", data: []byte(`"AQIDBAUGBwg="`)},
		{name: "URL alphabet", data: []byte(`"__________8"`)},
		{name: "numeric", data: []byte("1")},
		{name: "unterminated string", data: []byte(`"AQIDBAUGBwg`)},
		{name: "invalid UTF-8", data: []byte{'"', 0xff, '"'}},
		{name: "trailing document", data: append(append([]byte(nil), wire...), wire...)},
		{name: "one above bound", data: append(append([]byte(nil), wire...), bytes.Repeat([]byte{' '}, garble.SeedJSONWhitespaceAllowanceBytes+1)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := seed
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrGarbleContract) {
				t.Fatalf("Seed.UnmarshalJSON(%q) error = %v, want %v and %v", tc.data, gotErr, core.ErrJSONContract, core.ErrGarbleContract)
			}
			if got != seed {
				t.Fatalf("rejected Seed JSON mutated receiver: got %v, want %v", got, seed)
			}
		})
	}

}

func TestCustodyRequiresExactWideCoreMaterial(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		size    int
	}{
		{name: "Core minimum rejected", size: core.SecretMaterialMinimumBytes, wantErr: core.ErrGarbleContract},
		{name: "one above Core minimum rejected", size: core.SecretMaterialMinimumBytes + 1, wantErr: core.ErrGarbleContract},
		{name: "middle extent rejected", size: garble.CustodyBytes / 2, wantErr: core.ErrGarbleContract},
		{name: "two below rejected", size: garble.CustodyBytes - 2, wantErr: core.ErrGarbleContract},
		{name: "one below rejected", size: garble.CustodyBytes - 1, wantErr: core.ErrGarbleContract},
		{name: "exact admitted", size: garble.CustodyBytes},
		{name: "one above rejected by Core", size: garble.CustodyBytes + 1, wantErr: core.ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			material, gotMaterialErr := core.NewSecretMaterial(bytes.Repeat([]byte{1}, tc.size))
			if tc.size > core.SecretMaterialMaximumBytes {
				if !errors.Is(gotMaterialErr, tc.wantErr) {
					t.Fatalf("NewSecretMaterial(%d bytes) error = %v, want %v", tc.size, gotMaterialErr, tc.wantErr)
				}
				return
			}
			if gotMaterialErr != nil {
				t.Fatalf("NewSecretMaterial(%d bytes) error = %v, want nil", tc.size, gotMaterialErr)
			}
			got, gotErr := garble.NewCustody(material)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewCustody(%d bytes) = (%v, %v), want error %v", tc.size, got, gotErr, tc.wantErr)
			}
			if gotErr == nil {
				if gotValidateErr := got.Validate(); gotValidateErr != nil {
					t.Fatalf("Custody.Validate() error = %v, want nil", gotValidateErr)
				}
			}
		})
	}
}

func TestCustodyAndSeedFormattingAlwaysRedacts(t *testing.T) {
	t.Parallel()

	material, gotMaterialErr := core.NewSecretMaterial(bytes.Repeat([]byte{1}, garble.CustodyBytes))
	if gotMaterialErr != nil {
		t.Fatalf("NewSecretMaterial() error = %v, want nil", gotMaterialErr)
	}
	custody, gotCustodyErr := garble.NewCustody(material)
	if gotCustodyErr != nil {
		t.Fatalf("NewCustody() error = %v, want nil", gotCustodyErr)
	}
	seed := garble.NewSeed([garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8})
	values := []struct {
		value fmt.Formatter
		name  string
	}{
		{name: "custody", value: custody},
		{name: "seed", value: seed},
	}
	formats := []struct {
		name   string
		format string
	}{
		{name: "default verb", format: "%v"},
		{name: "field verb", format: "%+v"},
		{name: "Go syntax verb", format: "%#v"},
		{name: "string verb", format: "%s"},
		{name: "quoted string verb", format: "%q"},
		{name: "lower hexadecimal verb", format: "%x"},
		{name: "upper hexadecimal verb", format: "%X"},
		{name: "decimal verb", format: "%d"},
		{name: "binary verb", format: "%b"},
		{name: "Unicode verb", format: "%U"},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			t.Parallel()

			for _, format := range formats {
				got := fmt.Sprintf(format.format, value.value)
				if got != core.RedactedValueText {
					t.Fatalf(
						"fmt.Sprintf(%q, %s) = %q, want %q",
						format.format,
						value.name,
						got,
						core.RedactedValueText,
					)
				}
			}
		})
	}
}

func TestCustodyObservesCoreOwnedDestruction(t *testing.T) {
	t.Parallel()

	material, gotMaterialErr := core.NewSecretMaterial(bytes.Repeat([]byte{1}, garble.CustodyBytes))
	if gotMaterialErr != nil {
		t.Fatalf("NewSecretMaterial() error = %v, want nil", gotMaterialErr)
	}
	custody, gotCustodyErr := garble.NewCustody(material)
	if gotCustodyErr != nil {
		t.Fatalf("NewCustody() error = %v, want nil", gotCustodyErr)
	}
	if gotDestroyErr := material.Destroy(); gotDestroyErr != nil {
		t.Fatalf("SecretMaterial.Destroy() error = %v, want nil", gotDestroyErr)
	}
	if gotErr := custody.Validate(); !errors.Is(gotErr, core.ErrGarbleContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf("destroyed Custody.Validate() error = %v, want %v and %v", gotErr, core.ErrGarbleContract, core.ErrPrimitiveContract)
	}
}

func TestDeriveMatchesIndependentHKDFOracleAndSeparatesInputs(t *testing.T) {
	t.Parallel()

	materialBytes := bytes.Repeat([]byte{0x42}, garble.CustodyBytes)
	material, gotMaterialErr := core.NewSecretMaterial(materialBytes)
	if gotMaterialErr != nil {
		t.Fatalf("NewSecretMaterial() error = %v, want nil", gotMaterialErr)
	}
	custody, gotCustodyErr := garble.NewCustody(material)
	if gotCustodyErr != nil {
		t.Fatalf("NewCustody() error = %v, want nil", gotCustodyErr)
	}
	firstDigest := core.NewSHA256Digest(sha256.Sum256([]byte("release-one")))
	secondDigest := core.NewSHA256Digest(sha256.Sum256([]byte("release-two")))
	firstIdentity, gotFirstIdentityErr := garble.NewDerivationIdentity(firstDigest)
	if gotFirstIdentityErr != nil {
		t.Fatalf("NewDerivationIdentity(first) error = %v, want nil", gotFirstIdentityErr)
	}
	secondIdentity, gotSecondIdentityErr := garble.NewDerivationIdentity(secondDigest)
	if gotSecondIdentityErr != nil {
		t.Fatalf("NewDerivationIdentity(second) error = %v, want nil", gotSecondIdentityErr)
	}
	request := garble.DeriveRequest{
		Custody:    custody,
		Identity:   firstIdentity,
		Generation: garble.DerivationGenerationOne,
	}
	got, gotErr := garble.Derive(request)
	if gotErr != nil {
		t.Fatalf("Derive() error = %v, want nil", gotErr)
	}
	gotAgain, gotAgainErr := garble.Derive(request)
	if gotAgainErr != nil || gotAgain != got {
		t.Fatalf("Derive(same request) = (%v, %v), want (%v, nil)", gotAgain, gotAgainErr, got)
	}

	digestBytes, gotDigestErr := firstDigest.Bytes()
	if gotDigestErr != nil {
		t.Fatalf("SHA256Digest.Bytes() error = %v, want nil", gotDigestErr)
	}
	wantBytes := referenceHKDFSHA256(
		materialBytes,
		[]byte(garble.DerivationSalt),
		append([]byte{byte(garble.DerivationGenerationOne)}, digestBytes[:]...),
		garble.SeedBytes,
	)
	want := garble.NewSeed([garble.SeedBytes]byte(wantBytes))
	if got != want {
		t.Fatalf("Derive() = %v, want independent HKDF oracle %v", got, want)
	}

	separated, gotSeparatedErr := garble.Derive(garble.DeriveRequest{
		Custody:    custody,
		Identity:   secondIdentity,
		Generation: garble.DerivationGenerationOne,
	})
	if gotSeparatedErr != nil {
		t.Fatalf("Derive(second identity) error = %v, want nil", gotSeparatedErr)
	}
	if separated == got {
		t.Fatalf("Derive(distinct identity) = %v, want value distinct from %v", separated, got)
	}

	secondMaterial, gotSecondMaterialErr := core.NewSecretMaterial(
		bytes.Repeat([]byte{0x43}, garble.CustodyBytes),
	)
	if gotSecondMaterialErr != nil {
		t.Fatalf("NewSecretMaterial(second custody) error = %v, want nil", gotSecondMaterialErr)
	}
	secondCustody, gotSecondCustodyErr := garble.NewCustody(secondMaterial)
	if gotSecondCustodyErr != nil {
		t.Fatalf("NewCustody(second custody) error = %v, want nil", gotSecondCustodyErr)
	}
	custodySeparated, gotCustodySeparatedErr := garble.Derive(garble.DeriveRequest{
		Custody:    secondCustody,
		Identity:   firstIdentity,
		Generation: garble.DerivationGenerationOne,
	})
	if gotCustodySeparatedErr != nil {
		t.Fatalf("Derive(second custody) error = %v, want nil", gotCustodySeparatedErr)
	}
	if custodySeparated == got {
		t.Fatalf("Derive(distinct custody) = %v, want value distinct from %v", custodySeparated, got)
	}
}

func TestDeriveMatchesIndependentOracleAcrossHostileValidMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		identityText string
		custodyByte  byte
	}{
		{name: "lowest nonzero custody byte and empty release bytes", custodyByte: 1},
		{name: "next custody byte and one-byte identity", custodyByte: 2, identityText: "a"},
		{name: "low custody byte and numeric identity", custodyByte: 3, identityText: "1"},
		{name: "mid custody byte and short identity", custodyByte: 0x42, identityText: "release"},
		{name: "high custody byte and hyphenated identity", custodyByte: 0x7f, identityText: "release-one"},
		{name: "high-bit custody byte and spaced identity bytes", custodyByte: 0x80, identityText: "release one"},
		{name: "alternating custody source value and punctuation identity", custodyByte: 0xaa, identityText: "release/one"},
		{name: "inverse custody source value and unicode identity", custodyByte: 0x55, identityText: "release-π"},
		{name: "one below maximum custody byte and long identity", custodyByte: 0xfe, identityText: "primitive-2026-release-identity-boundary"},
		{name: "maximum custody byte and maximum digest preimage variety", custodyByte: 0xff, identityText: "\x00\xff\n\t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			materialBytes := bytes.Repeat([]byte{tc.custodyByte}, garble.CustodyBytes)
			material, gotMaterialErr := core.NewSecretMaterial(materialBytes)
			if gotMaterialErr != nil {
				t.Fatalf("NewSecretMaterial() error = %v, want nil", gotMaterialErr)
			}
			custody, gotCustodyErr := garble.NewCustody(material)
			if gotCustodyErr != nil {
				t.Fatalf("NewCustody() error = %v, want nil", gotCustodyErr)
			}
			digest := sha256.Sum256([]byte(tc.identityText))
			identity, gotIdentityErr := garble.NewDerivationIdentity(core.NewSHA256Digest(digest))
			if gotIdentityErr != nil {
				t.Fatalf("NewDerivationIdentity() error = %v, want nil", gotIdentityErr)
			}
			got, gotErr := garble.Derive(garble.DeriveRequest{
				Custody:    custody,
				Identity:   identity,
				Generation: garble.DerivationGenerationOne,
			})
			if gotErr != nil {
				t.Fatalf("Derive() error = %v, want nil", gotErr)
			}
			wantBytes := referenceHKDFSHA256(
				materialBytes,
				[]byte(garble.DerivationSalt),
				append([]byte{byte(garble.DerivationGenerationOne)}, digest[:]...),
				garble.SeedBytes,
			)
			want := garble.NewSeed([garble.SeedBytes]byte(wantBytes))
			if got != want {
				t.Fatalf("Derive() = %v, want independent HKDF oracle %v", got, want)
			}
		})
	}
}

func TestDeriveRejectsInvalidTypedBoundaries(t *testing.T) {
	t.Parallel()

	material, gotMaterialErr := core.NewSecretMaterial(bytes.Repeat([]byte{0x42}, garble.CustodyBytes))
	if gotMaterialErr != nil {
		t.Fatalf("NewSecretMaterial() error = %v, want nil", gotMaterialErr)
	}
	custody, gotCustodyErr := garble.NewCustody(material)
	if gotCustodyErr != nil {
		t.Fatalf("NewCustody() error = %v, want nil", gotCustodyErr)
	}
	identity, gotIdentityErr := garble.NewDerivationIdentity(
		core.NewSHA256Digest(sha256.Sum256([]byte("valid-release"))),
	)
	if gotIdentityErr != nil {
		t.Fatalf("NewDerivationIdentity() error = %v, want nil", gotIdentityErr)
	}
	destroyedMaterial, gotDestroyedMaterialErr := core.NewSecretMaterial(
		bytes.Repeat([]byte{0x43}, garble.CustodyBytes),
	)
	if gotDestroyedMaterialErr != nil {
		t.Fatalf("NewSecretMaterial(destroyed) error = %v, want nil", gotDestroyedMaterialErr)
	}
	destroyedCustody, gotDestroyedCustodyErr := garble.NewCustody(destroyedMaterial)
	if gotDestroyedCustodyErr != nil {
		t.Fatalf("NewCustody(destroyed) error = %v, want nil", gotDestroyedCustodyErr)
	}
	if gotDestroyErr := destroyedMaterial.Destroy(); gotDestroyErr != nil {
		t.Fatalf("SecretMaterial.Destroy() error = %v, want nil", gotDestroyErr)
	}
	valid := garble.DeriveRequest{
		Custody:    custody,
		Identity:   identity,
		Generation: garble.DerivationGenerationOne,
	}
	cases := []struct {
		mutate func(*garble.DeriveRequest)
		name   string
	}{
		{name: "zero custody rejected", mutate: func(r *garble.DeriveRequest) { r.Custody = garble.Custody{} }},
		{name: "destroyed custody rejected", mutate: func(r *garble.DeriveRequest) { r.Custody = destroyedCustody }},
		{name: "zero identity rejected", mutate: func(r *garble.DeriveRequest) { r.Identity = garble.DerivationIdentity{} }},
		{name: "unknown generation rejected", mutate: func(r *garble.DeriveRequest) { r.Generation = garble.DerivationGenerationUnknown }},
		{name: "first future generation rejected", mutate: func(r *garble.DeriveRequest) { r.Generation = garble.DerivationGenerationOne + 1 }},
		{name: "future generation rejected", mutate: func(r *garble.DeriveRequest) { r.Generation = garble.DerivationGeneration(math.MaxUint8) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			tc.mutate(&request)
			gotValidateErr := request.Validate()
			if !errors.Is(gotValidateErr, core.ErrGarbleContract) ||
				!errors.Is(gotValidateErr, core.ErrPrimitiveContract) {
				t.Fatalf("DeriveRequest.Validate() error = %v, want %v and %v", gotValidateErr, core.ErrGarbleContract, core.ErrPrimitiveContract)
			}
			got, gotErr := garble.Derive(request)
			if got != (garble.Seed{}) ||
				!errors.Is(gotErr, core.ErrGarbleDerivation) ||
				!errors.Is(gotErr, core.ErrGarbleContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"Derive() = (%v, %v), want (zero, %v, %v, and %v)",
					got,
					gotErr,
					core.ErrGarbleDerivation,
					core.ErrGarbleContract,
					core.ErrPrimitiveContract,
				)
			}
		})
	}
}

// TestZeroGarbleValuesRejectEveryOwnedBoundary is the boundary currency and
// keygen already ratchet for their own values and garble did not. Every
// exported garble value has an unusable Go zero value, so every projection off
// one must refuse rather than answer with empty text, empty bytes, or an empty
// argument sequence. Without this, a caller that skips a constructor gets a
// silently empty CLI prefix instead of an error.
func TestZeroGarbleValuesRejectEveryOwnedBoundary(t *testing.T) {
	t.Parallel()

	t.Run("seed", func(t *testing.T) {
		t.Parallel()

		var seed garble.Seed
		if gotErr := seed.Validate(); !errors.Is(gotErr, core.ErrGarbleContract) {
			t.Fatalf("Seed{}.Validate() = %v, want %v", gotErr, core.ErrGarbleContract)
		}
		gotBytes, gotBytesErr := seed.Bytes()
		if gotBytes != ([garble.SeedBytes]byte{}) || !errors.Is(gotBytesErr, core.ErrGarbleContract) {
			t.Fatalf("Seed{}.Bytes() = (%v, %v), want (zero, %v)", gotBytes, gotBytesErr, core.ErrGarbleContract)
		}
		gotEncoded, gotEncodedErr := seed.Encoded()
		if gotEncoded != "" || !errors.Is(gotEncodedErr, core.ErrGarbleContract) {
			t.Fatalf("Seed{}.Encoded() = (%q, %v), want (empty, %v)", gotEncoded, gotEncodedErr, core.ErrGarbleContract)
		}
		gotJSON, gotJSONErr := seed.MarshalJSON()
		if gotJSON != nil ||
			!errors.Is(gotJSONErr, core.ErrJSONContract) ||
			!errors.Is(gotJSONErr, core.ErrGarbleContract) {
			t.Fatalf(
				"Seed{}.MarshalJSON() = (%q, %v), want (nil, %v and %v)",
				gotJSON, gotJSONErr, core.ErrJSONContract, core.ErrGarbleContract,
			)
		}
	})
	t.Run("custody", func(t *testing.T) {
		t.Parallel()

		if gotErr := (garble.Custody{}).Validate(); !errors.Is(gotErr, core.ErrGarbleContract) {
			t.Fatalf("Custody{}.Validate() = %v, want %v", gotErr, core.ErrGarbleContract)
		}
	})
	t.Run("derivation identity", func(t *testing.T) {
		t.Parallel()

		if gotErr := (garble.DerivationIdentity{}).Validate(); !errors.Is(gotErr, core.ErrGarbleContract) {
			t.Fatalf("DerivationIdentity{}.Validate() = %v, want %v", gotErr, core.ErrGarbleContract)
		}
		got, gotErr := garble.NewDerivationIdentity(core.SHA256Digest{})
		if got != (garble.DerivationIdentity{}) ||
			!errors.Is(gotErr, core.ErrGarbleContract) ||
			!errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf(
				"NewDerivationIdentity(zero digest) = (%v, %v), want (zero, %v and %v)",
				got, gotErr, core.ErrGarbleContract, core.ErrPrimitiveContract,
			)
		}
	})
	t.Run("build intent", func(t *testing.T) {
		t.Parallel()

		var intent garble.BuildIntent
		if gotErr := intent.Validate(); !errors.Is(gotErr, core.ErrGarbleBuildIntent) {
			t.Fatalf("BuildIntent{}.Validate() = %v, want %v", gotErr, core.ErrGarbleBuildIntent)
		}
		gotSequence, gotErr := intent.Arguments()
		if gotSequence != nil || !errors.Is(gotErr, core.ErrGarbleBuildIntent) {
			t.Fatalf("BuildIntent{}.Arguments() = (%v, %v), want (nil, %v)", gotSequence, gotErr, core.ErrGarbleBuildIntent)
		}
	})
}

func referenceHKDFSHA256(secret, salt, info []byte, size int) []byte {
	extract := hmac.New(sha256.New, salt)
	_, _ = extract.Write(secret)
	pseudorandomKey := extract.Sum(nil)

	expand := hmac.New(sha256.New, pseudorandomKey)
	_, _ = expand.Write(info)
	_, _ = expand.Write([]byte{1})
	return expand.Sum(nil)[:size]
}

func TestIndependentHKDFOracleMatchesRFC5869CaseOneFirstBlock(t *testing.T) {
	t.Parallel()

	ikm, gotIKMErr := hex.DecodeString("0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b")
	if gotIKMErr != nil {
		t.Fatalf("hex.DecodeString(IKM) error = %v, want nil", gotIKMErr)
	}
	salt, gotSaltErr := hex.DecodeString("000102030405060708090a0b0c")
	if gotSaltErr != nil {
		t.Fatalf("hex.DecodeString(salt) error = %v, want nil", gotSaltErr)
	}
	info, gotInfoErr := hex.DecodeString("f0f1f2f3f4f5f6f7f8f9")
	if gotInfoErr != nil {
		t.Fatalf("hex.DecodeString(info) error = %v, want nil", gotInfoErr)
	}
	want, gotWantErr := hex.DecodeString("3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf")
	if gotWantErr != nil {
		t.Fatalf("hex.DecodeString(OKM) error = %v, want nil", gotWantErr)
	}
	got := referenceHKDFSHA256(ikm, salt, info, sha256.Size)
	if !bytes.Equal(got, want) {
		t.Fatalf("independent HKDF first block = %x, want RFC 5869 %x", got, want)
	}
}

func TestSeedStandardBase64Oracle(t *testing.T) {
	t.Parallel()

	raw := [garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8}
	want := base64.RawStdEncoding.EncodeToString(raw[:])
	got, gotErr := garble.NewSeed(raw).Encoded()
	if gotErr != nil || got != want {
		t.Fatalf("Seed.Encoded() = (%q, %v), want stdlib oracle (%q, nil)", got, gotErr, want)
	}
}
