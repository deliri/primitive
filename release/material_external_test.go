package release_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
	"github.com/deliri/primitive/v2026/release"
)

func TestReleaseMaterialExternalBoundaryHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture, _ := materialResponseFixture(t)
	route, routeErr := fixture.Request.ControlRoute()
	if routeErr != nil || route.Offering() != fixture.Request.Offering ||
		route.Family() != controlwire.RouteFamilyReleaseMaterials ||
		fixture.Request.ControlNonce() != fixture.Request.Nonce {
		t.Fatalf("material control projection = (%v, %v, %v), want exact route and request nonce",
			route, fixture.Request.ControlNonce(), routeErr)
	}
	canonical, err := fixture.MarshalJSON()
	if err != nil {
		t.Fatalf("MaterialResponse.MarshalJSON() error = %v, want nil", err)
	}
	validSigning := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, keygen.SeedSize))
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "truncated", data: canonical[:len(canonical)-1]},
		{name: "trailing document", data: append(append([]byte{}, canonical...), canonical...)},
		{name: "unknown top-level field", data: append(canonical[:len(canonical)-1], []byte(`,"future":1}`)...)},
		{name: "missing signing seed", data: fmt.Appendf(nil, `{"request":%s,"server_public_key":%s}`, mustMaterialRequestJSON(t, fixture.Request), mustPublicKeyJSON(t, fixture.ServerPublicKey))},
		{name: "null signing seed", data: bytes.Replace(canonical, []byte(`"release_signing_seed":"`), []byte(`"release_signing_seed":null,"ignored":"`), 1)},
		{name: "noncanonical signing base64", data: bytes.Replace(canonical, []byte(validSigning), []byte(strings.TrimRight(validSigning, "=")), 1)},
		{name: "signing seed one byte short", data: bytes.Replace(canonical, []byte(validSigning), []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, keygen.SeedSize-1))), 1)},
		{name: "signing seed one byte long", data: bytes.Replace(canonical, []byte(validSigning), []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, keygen.SeedSize+1))), 1)},
		{name: "zero signing seed", data: bytes.Replace(canonical, []byte(validSigning), []byte(base64.StdEncoding.EncodeToString(make([]byte, keygen.SeedSize))), 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidate := fixture
			gotErr := candidate.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || candidate != fixture {
				t.Fatalf("MaterialResponse.UnmarshalJSON(hostile) = (error %v, preserved %t), want typed refusal and exact receiver preservation", gotErr, candidate == fixture)
			}
		})
	}
}

func TestReleaseMaterialOpensExactCapabilitiesAndRedactsEverySecret(t *testing.T) {
	t.Parallel()

	fixture, _ := materialResponseFixture(t)
	wantSigning, err := fixture.ReleaseSigningSeed.SigningKey()
	if err != nil {
		t.Fatalf("ReleaseSigningSeed.SigningKey() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = wantSigning.Destroy() })
	wantServer := fixture.ServerPublicKey
	opened, err := fixture.Open()
	if err != nil {
		t.Fatalf("MaterialResponse.Open() error = %v, want nil", err)
	}
	fixtureErr := fixture.Validate()
	if fixture != (release.MaterialResponse{}) || !errors.Is(fixtureErr, core.ErrReleaseContract) {
		t.Fatalf("MaterialResponse.Open() source = (%v, %v), want zero and errors.Is %v", fixture, fixtureErr, core.ErrReleaseContract)
	}
	gotPublic, err := opened.SigningKey.PublicKey()
	if err != nil {
		t.Fatalf("opened SigningKey.PublicKey() error = %v, want nil", err)
	}
	wantPublic, err := wantSigning.PublicKey()
	if err != nil {
		t.Fatalf("fixture SigningKey.PublicKey() error = %v, want nil", err)
	}
	if gotPublic != wantPublic || opened.ServerPublicKey != wantServer {
		t.Fatalf("opened material bindings = (signer %t, server %t), want both exact", gotPublic == wantPublic, opened.ServerPublicKey == wantServer)
	}
	if err := opened.Destroy(); err != nil {
		t.Fatalf("Material.Destroy() error = %v, want nil", err)
	}
	gotMaterialErr, gotSigningErr := opened.Validate(), opened.SigningKey.Validate()
	if !errors.Is(gotMaterialErr, core.ErrReleaseContract) ||
		!errors.Is(gotSigningErr, core.ErrKeygenContract) {
		t.Fatalf(
			"destroyed capability errors = (material %v, signing %v), want errors.Is (%v, %v)",
			gotMaterialErr,
			gotSigningErr,
			core.ErrReleaseContract,
			core.ErrKeygenContract,
		)
	}
}

func TestReleaseMaterialRedactsEveryFormattingPath(t *testing.T) {
	t.Parallel()

	fixture, signingBytes := materialResponseFixture(t)
	openedResponse, _ := materialResponseFixture(t)
	opened, err := openedResponse.Open()
	if err != nil {
		t.Fatalf("MaterialResponse.Open() setup error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = opened.Destroy() })
	formats := []struct {
		name      string
		pattern   string
		wantExact bool
	}{
		{name: "default value", pattern: "%v", wantExact: true},
		{name: "field value", pattern: "%+v", wantExact: true},
		{name: "Go syntax", pattern: "%#v", wantExact: true},
		{name: "string", pattern: "%s", wantExact: true},
		{name: "quoted string", pattern: "%q", wantExact: true},
		{name: "binary", pattern: "%b", wantExact: true},
		{name: "character", pattern: "%c", wantExact: true},
		{name: "decimal", pattern: "%d", wantExact: true},
		{name: "octal", pattern: "%o", wantExact: true},
		{name: "prefixed octal", pattern: "%O", wantExact: true},
		{name: "lower hexadecimal", pattern: "%x", wantExact: true},
		{name: "upper hexadecimal", pattern: "%X", wantExact: true},
		{name: "Unicode", pattern: "%U", wantExact: true},
		{name: "boolean", pattern: "%t", wantExact: true},
		{name: "lower exponent", pattern: "%e", wantExact: true},
		{name: "upper exponent", pattern: "%E", wantExact: true},
		{name: "lower decimal point", pattern: "%f", wantExact: true},
		{name: "upper decimal point", pattern: "%F", wantExact: true},
		{name: "compact lower exponent", pattern: "%g", wantExact: true},
		{name: "compact upper exponent", pattern: "%G", wantExact: true},
		{name: "left width", pattern: "%-20v", wantExact: true},
		{name: "zero width", pattern: "%020v", wantExact: true},
		{name: "precision", pattern: "%.3v", wantExact: true},
		{name: "space flag", pattern: "% v", wantExact: true},
		{name: "dynamic type", pattern: "%T"},
		{name: "pointer identity", pattern: "%p"},
	}
	values := []struct {
		name      string
		value     any
		forbidden []string
	}{
		{name: "release signing seed", value: fixture.ReleaseSigningSeed, forbidden: releaseSigningSeedProjections(t, signingBytes)},
		{name: "unopened material response", value: fixture, forbidden: releaseSigningSeedProjections(t, signingBytes)},
		{name: "opened material", value: opened, forbidden: releaseSigningSeedProjections(t, signingBytes)},
	}
	for _, valueCase := range values {
		t.Run(valueCase.name, func(t *testing.T) {
			t.Parallel()
			for _, formatCase := range formats {
				got := fmt.Sprintf(formatCase.pattern, valueCase.value)
				if formatCase.wantExact && got != core.RedactedValueText {
					t.Fatalf("fmt.Sprintf(%q) = %q, want %q", formatCase.pattern, got, core.RedactedValueText)
				}
				for _, forbidden := range valueCase.forbidden {
					if forbidden != "" && strings.Contains(got, forbidden) {
						t.Fatalf("fmt.Sprintf(%q) disclosed %s material", formatCase.pattern, valueCase.name)
					}
				}
			}
		})
	}
}

func TestReleaseMaterialDestructionInvalidatesEverySharedHandle(t *testing.T) {
	t.Parallel()

	response, _ := materialResponseFixture(t)
	responseCopy := response
	signingCopy := response.ReleaseSigningSeed
	if err := response.Destroy(); err != nil {
		t.Fatalf("MaterialResponse.Destroy() error = %v, want nil", err)
	}
	for _, tc := range []struct {
		validate func() error
		name     string
	}{
		{name: "destroyed response", validate: response.Validate},
		{name: "copied response handle", validate: responseCopy.Validate},
		{name: "copied release signing seed", validate: signingCopy.Validate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.validate()
			if !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("Validate() error = %v, want errors.Is %v", gotErr, core.ErrReleaseContract)
			}
		})
	}
	if err := response.Destroy(); err != nil {
		t.Fatalf("MaterialResponse.Destroy(repeated) error = %v, want nil", err)
	}

	invalid, _ := materialResponseFixture(t)
	invalid.ServerPublicKey = core.Ed25519PublicKey{}
	opened, openErr := invalid.Open()
	if !errors.Is(openErr, core.ErrReleaseContract) || opened != (release.Material{}) || invalid != (release.MaterialResponse{}) {
		t.Fatalf("MaterialResponse.Open(invalid) = (%v, %v, source zero %t), want zero, errors.Is %v, and consumed source",
			opened, openErr, invalid == (release.MaterialResponse{}), core.ErrReleaseContract)
	}
}

func TestReleaseMaterialJSONReplacementDestroysPreviousSecretCustody(t *testing.T) {
	t.Parallel()

	response, signingBytes := materialResponseFixture(t)
	replacement, _ := materialResponseFixture(t)
	canonicalResponse, err := replacement.MarshalJSON()
	if err != nil {
		t.Fatalf("MaterialResponse.MarshalJSON(replacement) error = %v, want nil", err)
	}
	previousResponse := response
	if err := response.UnmarshalJSON(canonicalResponse); err != nil {
		t.Fatalf("MaterialResponse.UnmarshalJSON(replacement) error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = response.Destroy() })
	if response.Validate() != nil || !errors.Is(previousResponse.Validate(), core.ErrReleaseContract) {
		t.Fatalf("MaterialResponse replacement = (new %v, previous %v), want valid and errors.Is %v",
			response.Validate(), previousResponse.Validate(), core.ErrReleaseContract)
	}

	signing, err := release.NewReleaseSigningSeed(signingBytes)
	if err != nil {
		t.Fatalf("NewReleaseSigningSeed() setup error = %v, want nil", err)
	}
	canonicalSigning, err := signing.MarshalJSON()
	if err != nil {
		t.Fatalf("ReleaseSigningSeed.MarshalJSON() error = %v, want nil", err)
	}
	previousSigning := signing
	if err := signing.UnmarshalJSON(canonicalSigning); err != nil {
		t.Fatalf("ReleaseSigningSeed.UnmarshalJSON(replacement) error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = signing.Destroy() })
	if signing.Validate() != nil || !errors.Is(previousSigning.Validate(), core.ErrReleaseContract) {
		t.Fatalf("ReleaseSigningSeed replacement = (new %v, previous %v), want valid and errors.Is %v",
			signing.Validate(), previousSigning.Validate(), core.ErrReleaseContract)
	}

}

func FuzzReleaseMaterialResponseExternalSemanticOracle(f *testing.F) {
	fixture, _ := materialResponseFixture(f)
	canonical, err := fixture.MarshalJSON()
	if err != nil {
		f.Fatalf("MaterialResponse.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(canonical[:len(canonical)-1])
	f.Add(append(append([]byte{}, canonical...), byte(' ')))
	f.Fuzz(func(t *testing.T, data []byte) {
		var candidate release.MaterialResponse
		gotErr := candidate.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || candidate != (release.MaterialResponse{}) {
				t.Fatalf("MaterialResponse.UnmarshalJSON(fuzz) refusal = (%v, zero %t), want typed refusal and zero receiver", gotErr, candidate == (release.MaterialResponse{}))
			}
			return
		}
		defer func() { _ = candidate.Destroy() }()
		if candidate.Validate() != nil {
			t.Fatalf("MaterialResponse.UnmarshalJSON(fuzz) accepted value whose Validate() refuses")
		}
		first, firstErr := candidate.MarshalJSON()
		if firstErr != nil {
			t.Fatalf("MaterialResponse.MarshalJSON(first accepted projection) error = %v, want nil", firstErr)
		}
		var second release.MaterialResponse
		if secondErr := second.UnmarshalJSON(first); secondErr != nil {
			t.Fatalf("accepted material round trip error = %v, want nil", secondErr)
		}
		defer func() { _ = second.Destroy() }()
		secondBytes, secondErr := second.MarshalJSON()
		if secondErr != nil || !bytes.Equal(first, secondBytes) {
			t.Fatalf("accepted material second projection = (error %v, identical %t), want nil and byte-identical", secondErr, bytes.Equal(first, secondBytes))
		}
		opened, openErr := candidate.Open()
		if openErr != nil || opened.Validate() != nil {
			t.Fatalf("accepted material capability projection = (error %v, validation %v), want usable", openErr, opened.Validate())
		}
		if destroyErr := opened.Destroy(); destroyErr != nil {
			t.Fatalf("accepted material Destroy() error = %v, want nil", destroyErr)
		}
		if validateErr := opened.Validate(); !errors.Is(validateErr, core.ErrReleaseContract) {
			t.Fatalf("destroyed accepted material Validate() error = %v, want errors.Is %v", validateErr, core.ErrReleaseContract)
		}
		candidateErr := candidate.Validate()
		if candidate != (release.MaterialResponse{}) || !errors.Is(candidateErr, core.ErrReleaseContract) {
			t.Fatalf("accepted material source after Open = (%v, %v), want zero and errors.Is %v", candidate, candidateErr, core.ErrReleaseContract)
		}
	})
}

func releaseSigningSeedProjections(tb testing.TB, seed [keygen.SeedSize]byte) []string {
	tb.Helper()
	return []string{
		base64.StdEncoding.EncodeToString(seed[:]),
		fmt.Sprint(seed[:]),
		fmt.Sprintf("%x", seed[:]),
	}
}

func materialResponseFixture(tb testing.TB) (
	release.MaterialResponse,
	[keygen.SeedSize]byte,
) {
	tb.Helper()
	var version core.ReleaseVersion
	if err := version.UnmarshalText([]byte("2026.1.2")); err != nil {
		tb.Fatalf("ReleaseVersion.UnmarshalText() error = %v, want nil", err)
	}
	commitText, err := core.SHA256Of([]byte("release-material-fixture")).Hex()
	if err != nil {
		tb.Fatalf("SHA256Digest.Hex() error = %v, want nil", err)
	}
	commit, err := core.ParseBuildCommit(commitText)
	if err != nil {
		tb.Fatalf("ParseBuildCommit() error = %v, want nil", err)
	}
	var nonceBytes [core.SHA256DigestBytes]byte
	for index := range nonceBytes {
		nonceBytes[index] = byte(index + 1)
	}
	nonce, err := controlwire.NewRequestNonce(nonceBytes)
	if err != nil {
		tb.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	request, err := release.NewMaterialRequest(release.MaterialRequestInput{
		Version: version, Commit: commit, Offering: core.OfferingPeachfuzz, Nonce: nonce,
	})
	if err != nil {
		tb.Fatalf("NewMaterialRequest() error = %v, want nil", err)
	}
	var signingBytes [keygen.SeedSize]byte
	for index := range signingBytes {
		signingBytes[index] = 0x31
	}
	signing, err := release.NewReleaseSigningSeed(signingBytes)
	if err != nil {
		tb.Fatalf("NewReleaseSigningSeed() error = %v, want nil", err)
	}
	server, err := keygen.GenerateSigningKey()
	if err != nil {
		tb.Fatalf("GenerateSigningKey() error = %v, want nil", err)
	}
	tb.Cleanup(func() { _ = server.Destroy() })
	public, err := server.PublicKey()
	if err != nil {
		tb.Fatalf("SigningKey.PublicKey() error = %v, want nil", err)
	}
	response := release.MaterialResponse{
		Request: request, ReleaseSigningSeed: signing,
		ServerPublicKey: public,
	}
	if err := response.Validate(); err != nil {
		tb.Fatalf("MaterialResponse.Validate() error = %v, want nil", err)
	}
	tb.Cleanup(func() { _ = response.Destroy() })
	return response, signingBytes
}

func mustMaterialRequestJSON(tb testing.TB, request release.MaterialRequest) []byte {
	tb.Helper()
	encoded, err := request.MarshalJSON()
	if err != nil {
		tb.Fatalf("MaterialRequest.MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func mustPublicKeyJSON(tb testing.TB, key core.Ed25519PublicKey) []byte {
	tb.Helper()
	encoded, err := key.MarshalJSON()
	if err != nil {
		tb.Fatalf("Ed25519PublicKey.MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}
