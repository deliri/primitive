package controlplane

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
)

// TestSigningDomainSatisfiesTheAttestSigningDomainContract names the obligation
// the package-level witness declares.
//
// The Attest constraint embeds comparable, so the contract cannot be asserted by
// assigning a value to an interface. Instantiating the generic witness is the
// assertion, and it is written here as a call so the obligation is reachable
// from a test rather than sitting in a declaration nothing exercises.
func TestSigningDomainSatisfiesTheAttestSigningDomainContract(t *testing.T) {
	t.Parallel()

	signingDomainWitness[SigningDomain]()
}

// TestSigningDomainParsesEveryTextItRenders is the self-referential half of the
// Attest contract, checked over the whole closed set from inside the package so
// a domain added without a token cannot slip past the exported surface.
func TestSigningDomainParsesEveryTextItRenders(t *testing.T) {
	t.Parallel()

	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		text, err := domain.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText() for domain %d error = %v, want nil", domain, err)
		}
		got, err := SigningDomainUnknown.ParseCanonicalText(text)
		if err != nil {
			t.Fatalf("ParseCanonicalText(%q) error = %v, want nil", text, err)
		}
		if got != domain {
			t.Fatalf("ParseCanonicalText(%q) = %v, want %v", text, got, domain)
		}
	}
}

// TestSigningDomainTokensAreDistinct proves no two domains share a text. Two
// documents in one namespace would let a signature over the first be presented
// as a signature over the second.
func TestSigningDomainTokensAreDistinct(t *testing.T) {
	t.Parallel()

	seen := make(map[string]SigningDomain, signingDomainLimit)
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		token := signingDomainTokens()[domain]
		if token == "" {
			t.Fatalf("domain %d has no canonical text, want one", domain)
		}
		if previous, repeated := seen[token]; repeated {
			t.Fatalf("domain %v shares the text %q with domain %v, want distinct namespaces",
				domain, token, previous)
		}
		seen[token] = domain
	}
	if got, want := len(seen), int(signingDomainLimit)-1; got != want {
		t.Fatalf("distinct domain texts = %d, want %d", got, want)
	}
}

// TestAttestSigningDomainBoundAdmitsEveryPublishedText keeps the closed set
// inside the frame Attest will carry. A domain longer than the bound would sign
// locally and be refused on the wire.
func TestAttestSigningDomainBoundAdmitsEveryPublishedText(t *testing.T) {
	t.Parallel()

	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		if got := len(signingDomainTokens()[domain]); got > attest.SigningDomainMaximumBytes {
			t.Fatalf("domain %v text length = %d, want at most %d",
				domain, got, attest.SigningDomainMaximumBytes)
		}
	}
}
