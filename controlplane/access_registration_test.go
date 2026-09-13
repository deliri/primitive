package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func accessRegistrationFixture(t testing.TB) (controlplane.AccessRegistrationRequest, controlplane.Authority) {
	t.Helper()
	pub, _ := testSigningKey(t, 31)
	device, err := lease.DeviceIDForPublicKey(pub)
	if err != nil {
		t.Fatalf("DeviceIDForPublicKey() error = %v, want nil", err)
	}
	token, err := controlwire.NewAccessToken([controlwire.AccessTokenBytes]byte{41})
	if err != nil {
		t.Fatalf("NewAccessToken() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := token.Destroy(); err != nil {
			t.Errorf("Destroy() error = %v, want nil", err)
		}
	})
	nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{42})
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	request := controlplane.AccessRegistrationRequest{Token: token, Build: testBuildForOffering(t, controlplaneOffering(t, 1)), DeviceKey: pub, Installation: device, RequestNonce: nonce, Revision: controlwire.Revision2026V1}
	if err := request.Validate(); err != nil {
		t.Fatalf("request.Validate() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{pub}})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	server, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("NewAuthority() error = %v, want nil", err)
	}
	return request, server
}

func TestAccessRegistrationAuthorityLayerTriad(t *testing.T) {
	t.Parallel()
	request, server := accessRegistrationFixture(t)
	verifier, err := request.Token.Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	proof, err := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{Request: request, ExpectedVerifier: verifier})
	if err != nil {
		t.Fatalf("VerifyAccessRegistration() error = %v, want nil", err)
	}
	wantIdentity, err := request.Identity()
	if err != nil {
		t.Fatalf("Identity() error = %v, want nil", err)
	}
	identity, err := proof.Identity()
	if err != nil || identity != wantIdentity {
		t.Fatalf("proof.Identity() = (%+v, %v), want (%+v, nil)", identity, err, wantIdentity)
	}
	replay, disposition, err := proof.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionFresh {
		t.Fatalf("Replay() = (%v, %v), want fresh and nil", disposition, err)
	}
	exact, err := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{Request: request, ExpectedVerifier: verifier, PriorReplay: &replay})
	if err != nil {
		t.Fatalf("VerifyAccessRegistration(retry) error = %v, want nil", err)
	}
	gotReplay, disposition, err := exact.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionExact || !gotReplay.Equal(replay) {
		t.Fatalf("retry Replay() = (%v, %v), want exact and unchanged commitment", disposition, err)
	}
	changed := request
	changed.RequestNonce = otherRequestNonce(t)
	if changed.RequestNonce == request.RequestNonce {
		t.Fatal("mutation = same nonce, want different")
	}
	refused, err := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{Request: changed, ExpectedVerifier: verifier, PriorReplay: &replay})
	if !errors.Is(err, core.ErrControlWireReplayConflict) || refused != (controlplane.VerifiedRegistrationAuthority{}) {
		t.Fatalf("changed retry = (%+v, %v), want zero and replay conflict", refused, err)
	}
	// Reusing the same key for a different device is fresh in a different
	// installation slot, rather than consuming global key authority.
	changed.DeviceKey, _ = testSigningKey(t, 32)
	changed.Installation, err = lease.DeviceIDForPublicKey(changed.DeviceKey)
	if err != nil {
		t.Fatalf("DeviceIDForPublicKey(second) error = %v, want nil", err)
	}
	second, err := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{Request: changed, ExpectedVerifier: verifier})
	if err != nil {
		t.Fatalf("VerifyAccessRegistration(second device) error = %v, want nil", err)
	}
	secondIdentity, err := second.Identity()
	if err != nil || secondIdentity.Installation != changed.Installation || secondIdentity.Installation == identity.Installation {
		t.Fatalf("second identity = (%+v, %v), want distinct bound installation", secondIdentity, err)
	}
	if err := request.Token.Validate(); err != nil {
		t.Fatalf("caller-owned reusable token Validate() error = %v, want nil", err)
	}
	zero, err := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{})
	if !errors.Is(err, core.ErrControlPlaneRegistration) || zero != (controlplane.VerifiedRegistrationAuthority{}) {
		t.Fatalf("absent authority = (%+v, %v), want zero and typed refusal", zero, err)
	}
}

func TestAccessRegistrationRejectsBrokenFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*testing.T, *controlplane.AccessRegistrationVerification)
		wantErr error
	}{
		{"missing verifier", func(_ *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.ExpectedVerifier = controlwire.AccessTokenVerifier{}
		}, core.ErrControlWireToken},
		{"foreign verifier", func(t *testing.T, v *controlplane.AccessRegistrationVerification) {
			token, err := controlwire.NewAccessToken([controlwire.AccessTokenBytes]byte{43})
			if err != nil {
				t.Fatalf("NewAccessToken() error = %v, want nil", err)
			}
			defer token.Destroy()
			v.ExpectedVerifier, err = token.Verifier()
			if err != nil {
				t.Fatalf("Verifier() error = %v, want nil", err)
			}
		}, core.ErrControlWireToken},
		{"missing token", func(_ *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.Request.Token = controlwire.AccessToken{}
		}, core.ErrControlWireToken},
		{"missing nonce", func(_ *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.Request.RequestNonce = controlwire.RequestNonce{}
		}, core.ErrControlPlaneRegistration},
		{"unknown revision", func(_ *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.Request.Revision = controlwire.Revision(255)
		}, core.ErrControlPlaneRegistration},
		{"missing build", func(_ *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.Request.Build = core.BuildIdentity{}
		}, core.ErrControlPlaneRegistration},
		{"device key mismatches installation", func(t *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.Request.DeviceKey, _ = testSigningKey(t, 33)
		}, core.ErrControlPlaneInstallationBinding},
		{"empty replay is not absence", func(_ *testing.T, v *controlplane.AccessRegistrationVerification) {
			v.PriorReplay = &controlwire.ReplayIdentity{}
		}, core.ErrControlPlaneRegistration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request, server := accessRegistrationFixture(t)
			verifier, err := request.Token.Verifier()
			if err != nil {
				t.Fatalf("Verifier() error = %v, want nil", err)
			}
			input := controlplane.AccessRegistrationVerification{Request: request, ExpectedVerifier: verifier}
			before := input
			tc.mutate(t, &input)
			if input == before {
				t.Fatal("mutation = unchanged, want changed load-bearing fact")
			}
			got, err := server.VerifyAccessRegistration(input)
			if !errors.Is(err, tc.wantErr) || got != (controlplane.VerifiedRegistrationAuthority{}) {
				t.Fatalf("VerifyAccessRegistration() = (%+v, %v), want zero and %v", got, err, tc.wantErr)
			}
		})
	}
}

func FuzzAccessRegistrationSemanticClosure(f *testing.F) {
	seed, server := accessRegistrationFixture(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Cleanup(func() { clear(canonical) })
	verifier, err := seed.Token.Verifier()
	if err != nil {
		f.Fatalf("Verifier(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(canonical[:len(canonical)-1])
	f.Add(bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes+1))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneRegistration) || got != seed {
				t.Fatalf("decode rejection = (%v, preserved=%t), want typed JSON/registration refusal and preserved receiver", err, got == seed)
			}
			return
		}
		defer got.Token.Destroy()
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		defer clear(encoded)
		if err != nil || len(encoded) > controlplane.AccessRegistrationRequestJSONMaximumBytes {
			t.Fatalf("MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var round controlplane.AccessRegistrationRequest
		if err := round.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("canonical decode error = %v, want nil", err)
		}
		defer round.Token.Destroy()
		gotIdentity, err := got.Identity()
		if err != nil {
			t.Fatalf("Identity() error = %v, want nil", err)
		}
		roundIdentity, err := round.Identity()
		if err != nil || gotIdentity != roundIdentity {
			t.Fatalf("round identity = (%+v, %v), want %+v and nil", roundIdentity, err, gotIdentity)
		}
		second, err := round.MarshalJSON()
		defer clear(second)
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("canonical second equality = %t, error = %v, want true and nil", bytes.Equal(encoded, second), err)
		}
		// Independently compare revealed token bytes to the pinned seed; the
		// verifier must authenticate exactly that key, not any valid token.
		presented, err := got.Token.Reveal()
		if err != nil {
			t.Fatalf("Reveal() error = %v, want nil", err)
		}
		defer clear(presented)
		known, err := seed.Token.Reveal()
		if err != nil {
			t.Fatalf("Reveal(seed) error = %v, want nil", err)
		}
		defer clear(known)
		proof, verifyErr := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{Request: got, ExpectedVerifier: verifier})
		if !bytes.Equal(presented, known) {
			if !errors.Is(verifyErr, core.ErrControlWireToken) || proof != (controlplane.VerifiedRegistrationAuthority{}) {
				t.Fatalf("foreign token verification = (%+v, %v), want zero and token refusal", proof, verifyErr)
			}
			return
		}
		if verifyErr != nil {
			t.Fatalf("known token verification error = %v, want nil", verifyErr)
		}
		identity, err := proof.Identity()
		if err != nil || identity != gotIdentity {
			t.Fatalf("verified identity = (%+v, %v), want (%+v, nil)", identity, err, gotIdentity)
		}
	})
}
