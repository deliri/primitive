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
	"github.com/deliri/primitive/v2026/garble"
	"github.com/deliri/primitive/v2026/keygen"
	"github.com/deliri/primitive/v2026/release"
)

func TestReleaseMaterialExternalBoundaryHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := materialResponseFixture(t)
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
	validCustody := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x63}, garble.CustodyBytes))
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "truncated", data: canonical[:len(canonical)-1]},
		{name: "trailing document", data: append(append([]byte{}, canonical...), canonical...)},
		{name: "unknown top-level field", data: append(canonical[:len(canonical)-1], []byte(`,"future":1}`)...)},
		{name: "missing signing seed", data: []byte(fmt.Sprintf(`{"request":%s,"garble_custody_seed":%q,"server_public_key":%s}`, mustMaterialRequestJSON(t, fixture.Request), validCustody, mustPublicKeyJSON(t, fixture.ServerPublicKey)))},
		{name: "null signing seed", data: bytes.Replace(canonical, []byte(`"release_signing_seed":"`), []byte(`"release_signing_seed":null,"ignored":"`), 1)},
		{name: "noncanonical signing base64", data: bytes.Replace(canonical, []byte(validSigning), []byte(strings.TrimRight(validSigning, "=")), 1)},
		{name: "signing seed one byte short", data: bytes.Replace(canonical, []byte(validSigning), []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, keygen.SeedSize-1))), 1)},
		{name: "signing seed one byte long", data: bytes.Replace(canonical, []byte(validSigning), []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, keygen.SeedSize+1))), 1)},
		{name: "zero signing seed", data: bytes.Replace(canonical, []byte(validSigning), []byte(base64.StdEncoding.EncodeToString(make([]byte, keygen.SeedSize))), 1)},
		{name: "custody one byte short", data: bytes.Replace(canonical, []byte(validCustody), []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x63}, garble.CustodyBytes-1))), 1)},
		{name: "custody one byte long", data: bytes.Replace(canonical, []byte(validCustody), []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x63}, garble.CustodyBytes+1))), 1)},
		{name: "zero custody", data: bytes.Replace(canonical, []byte(validCustody), []byte(base64.StdEncoding.EncodeToString(make([]byte, garble.CustodyBytes))), 1)},
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

	fixture := materialResponseFixture(t)
	opened, err := fixture.Open()
	if err != nil {
		t.Fatalf("MaterialResponse.Open() error = %v, want nil", err)
	}
	wantSigning, err := fixture.ReleaseSigningSeed.SigningKey()
	if err != nil {
		t.Fatalf("ReleaseSigningSeed.SigningKey() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = wantSigning.Destroy() })
	gotPublic, err := opened.SigningKey.PublicKey()
	if err != nil {
		t.Fatalf("opened SigningKey.PublicKey() error = %v, want nil", err)
	}
	wantPublic, err := wantSigning.PublicKey()
	if err != nil {
		t.Fatalf("fixture SigningKey.PublicKey() error = %v, want nil", err)
	}
	if gotPublic != wantPublic || opened.ServerPublicKey != fixture.ServerPublicKey || opened.Custody.Validate() != nil {
		t.Fatalf("opened material bindings = (signer %t, server %t, custody %v), want all exact", gotPublic == wantPublic, opened.ServerPublicKey == fixture.ServerPublicKey, opened.Custody.Validate())
	}
	for _, secret := range []any{fixture.ReleaseSigningSeed, fixture.GarbleCustodySeed, opened} {
		if got := fmt.Sprintf("%v", secret); got != core.RedactedValueText {
			t.Fatalf("formatted release secret = %q, want %q", got, core.RedactedValueText)
		}
	}
	if err := opened.Destroy(); err != nil {
		t.Fatalf("Material.Destroy() error = %v, want nil", err)
	}
	gotMaterialErr, gotSigningErr, gotCustodyErr := opened.Validate(), opened.SigningKey.Validate(), opened.Custody.Validate()
	if gotMaterialErr == nil || gotSigningErr == nil || gotCustodyErr == nil {
		t.Fatalf("destroyed capability errors = (material %v, signing %v, custody %v), want all non-nil", gotMaterialErr, gotSigningErr, gotCustodyErr)
	}
}

func FuzzReleaseMaterialResponseExternalSemanticOracle(f *testing.F) {
	fixture := materialResponseFixture(f)
	canonical, err := fixture.MarshalJSON()
	if err != nil {
		f.Fatalf("MaterialResponse.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(canonical[:len(canonical)-1])
	f.Add(append(append([]byte{}, canonical...), byte(' ')))
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := fixture
		gotErr := candidate.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || candidate != fixture {
				t.Fatalf("MaterialResponse.UnmarshalJSON(fuzz) refusal = (%v, preserved %t), want typed refusal and exact preservation", gotErr, candidate == fixture)
			}
			return
		}
		if candidate.Validate() != nil {
			t.Fatalf("MaterialResponse.UnmarshalJSON(fuzz) accepted value whose Validate() refuses")
		}
		first, firstErr := candidate.MarshalJSON()
		if firstErr != nil {
			t.Fatalf("MaterialResponse.MarshalJSON(first accepted projection) error = %v, want nil", firstErr)
		}
		var second release.MaterialResponse
		if secondErr := second.UnmarshalJSON(first); secondErr != nil || second != candidate {
			t.Fatalf("accepted material round trip = (error %v, exact %t), want nil and exact", secondErr, second == candidate)
		}
		secondBytes, secondErr := second.MarshalJSON()
		if secondErr != nil || !bytes.Equal(first, secondBytes) {
			t.Fatalf("accepted material second projection = (error %v, identical %t), want nil and byte-identical", secondErr, bytes.Equal(first, secondBytes))
		}
		opened, openErr := candidate.Open()
		if openErr != nil || opened.Validate() != nil {
			t.Fatalf("accepted material capability projection = (error %v, validation %v), want usable", openErr, opened.Validate())
		}
		if destroyErr := opened.Destroy(); destroyErr != nil || opened.Validate() == nil {
			t.Fatalf("accepted material destruction = (error %v, still valid %t), want nil and invalidated", destroyErr, opened.Validate() == nil)
		}
	})
}

func materialResponseFixture(tb testing.TB) release.MaterialResponse {
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
	var nonceBytes [controlwire.NonceBytes]byte
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
	var custodyBytes [garble.CustodyBytes]byte
	for index := range custodyBytes {
		custodyBytes[index] = 0x63
	}
	custody, err := release.NewGarbleCustodySeed(custodyBytes)
	if err != nil {
		tb.Fatalf("NewGarbleCustodySeed() error = %v, want nil", err)
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
		GarbleCustodySeed: custody, ServerPublicKey: public,
	}
	if err := response.Validate(); err != nil {
		tb.Fatalf("MaterialResponse.Validate() error = %v, want nil", err)
	}
	return response
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
