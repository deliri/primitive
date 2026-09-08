package release_test

import (
	"crypto/ed25519"
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

func TestReleaseMaterialOpenLayerTriadConsumesExactCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*testing.T, *release.MaterialResponse)
		wantErr error
	}{
		{name: "valid response opens the exact Go signing identity"},
		{name: "invalid request cannot expose otherwise live signing custody", mutate: func(_ *testing.T, r *release.MaterialResponse) { r.Request = release.MaterialRequest{} }, wantErr: core.ErrReleaseContract},
		{name: "invalid server cannot expose otherwise live signing custody", mutate: func(_ *testing.T, r *release.MaterialResponse) { r.ServerPublicKey = core.Ed25519PublicKey{} }, wantErr: core.ErrReleaseContract},
		{name: "destroyed signing custody cannot be revived", mutate: func(t *testing.T, r *release.MaterialResponse) {
			if err := r.ReleaseSigningSeed.Destroy(); err != nil {
				t.Fatalf("seed Destroy error = %v, want nil", err)
			}
		}, wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response, seed := materialResponseFixture(t)
			copiedResponse, copiedSeed := response, response.ReleaseSigningSeed
			server := response.ServerPublicKey
			if tc.mutate != nil {
				tc.mutate(t, &response)
			}
			opened, err := response.Open()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Open error = %v, want %v", err, tc.wantErr)
			}
			if response != (release.MaterialResponse{}) || !errors.Is(copiedResponse.Validate(), core.ErrReleaseContract) || !errors.Is(copiedSeed.Validate(), core.ErrReleaseContract) {
				t.Fatalf("consumed response/copies = (%v, %v, %v), want zero response and invalidated shared custody", response, copiedResponse.Validate(), copiedSeed.Validate())
			}
			if tc.wantErr == nil {
				private := ed25519.NewKeyFromSeed(seed[:])
				defer clear(private)
				public, publicErr := opened.SigningKey.PublicKey()
				want, err := core.NewEd25519PublicKey(ed25519.PublicKey(private[ed25519.SeedSize:]))
				if err != nil {
					t.Fatalf("NewEd25519PublicKey(Go derivation) error = %v, want nil", err)
				}
				if publicErr != nil || public != want || opened.ServerPublicKey != server || opened.Validate() != nil {
					t.Fatalf("opened keys = (%v, %v, %v), want Go public key %v and server %v", public, opened.ServerPublicKey, publicErr, want, server)
				}
				copy := opened
				if err := opened.Destroy(); err != nil {
					t.Fatalf("opened Destroy error = %v, want nil", err)
				}
				if !errors.Is(copy.Validate(), core.ErrReleaseContract) || !errors.Is(copy.SigningKey.Validate(), core.ErrKeygenContract) {
					t.Fatalf("destroyed material copy = (%v, %v), want invalid material and signing custody", copy.Validate(), copy.SigningKey.Validate())
				}
			} else if opened != (release.Material{}) {
				t.Fatalf("refused Open material = %v, want zero", opened)
			}
			replayed, err := response.Open()
			if !errors.Is(err, core.ErrReleaseContract) || replayed != (release.Material{}) {
				t.Fatalf("second Open = (%v, %v), want zero and typed refusal", replayed, err)
			}
			if err := response.Destroy(); err != nil {
				t.Fatalf("consumed response Destroy error = %v, want nil", err)
			}
		})
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
	t.Cleanup(func() {
		if err := opened.Destroy(); err != nil {
			t.Errorf("opened cleanup Destroy error = %v, want nil", err)
		}
	})
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
		Version: version, Commit: commit, Offering: releaseExternalOffering(tb, 3), Nonce: nonce,
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
	var serverSeed [keygen.SeedSize]byte
	serverSeed[0] = 0x72
	server, err := keygen.AdoptSigningKey(serverSeed)
	if err != nil {
		tb.Fatalf("AdoptSigningKey() error = %v, want nil", err)
	}
	tb.Cleanup(func() {
		if err := server.Destroy(); err != nil {
			tb.Errorf("server cleanup Destroy error = %v, want nil", err)
		}
	})
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
	tb.Cleanup(func() {
		if err := response.Destroy(); err != nil {
			tb.Errorf("response cleanup Destroy error = %v, want nil", err)
		}
	})
	return response, signingBytes
}
