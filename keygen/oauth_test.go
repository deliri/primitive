package keygen

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/cryptotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestOAuthProofUsesThreeIndependentGoDraws(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardGlobalRegistry, Scope: core.TestIsolationScopePackageProcess})
	cryptotest.SetGlobalRandom(t, 0xa173)
	var state, verifier, nonce, next [32]byte
	for _, value := range [][]byte{state[:], verifier[:], nonce[:], next[:]} {
		if _, err := rand.Read(value); err != nil {
			t.Fatalf("native reference entropy = %v, want nil", err)
		}
	}
	cryptotest.SetGlobalRandom(t, 0xa173)
	proof, err := GenerateOAuthBrowserProof()
	if err != nil || proof.Validate() != nil {
		t.Fatalf("OAuth proof generation = (%+v,%v), want valid native proof", proof, err)
	}
	if proof.State.String() != base64.RawURLEncoding.EncodeToString(state[:]) || proof.Verifier.String() != base64.RawURLEncoding.EncodeToString(verifier[:]) || proof.Nonce.String() != base64.RawURLEncoding.EncodeToString(nonce[:]) {
		t.Fatalf("OAuth proof = %+v, want three independent native32byte draws", proof)
	}
	var gotNext [32]byte
	if _, err := rand.Read(gotNext[:]); err != nil || gotNext != next {
		t.Fatalf("next entropy = (%x,%v), want exact native fourth draw %x", gotNext, err, next)
	}
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		text := fmt.Sprintf(verb, proof)
		if strings.Contains(text, proof.State.String()) || strings.Contains(text, proof.Nonce.String()) || strings.Contains(text, proof.Verifier.String()) {
			t.Fatalf("OAuth proof formatting %s retained private fields", verb)
		}
	}
	for _, row := range []struct {
		name  string
		clear func(*OAuthBrowserProof)
	}{{name: "neutral/unissued proof", clear: func(p *OAuthBrowserProof) { *p = OAuthBrowserProof{} }}, {name: "negative/missing state", clear: func(p *OAuthBrowserProof) { p.State = core.OAuthState{} }}, {name: "negative/missing verifier", clear: func(p *OAuthBrowserProof) { p.Verifier = core.OAuthPKCEVerifier{} }}, {name: "negative/missing nonce", clear: func(p *OAuthBrowserProof) { p.Nonce = core.OAuthNonce{} }}} {
		t.Run(row.name, func(t *testing.T) {
			candidate := proof
			row.clear(&candidate)
			if err := candidate.Validate(); !errors.Is(err, core.ErrKeygenContract) {
				t.Fatalf("proof Validate = %v, want owning refusal", err)
			}
		})
	}
}
