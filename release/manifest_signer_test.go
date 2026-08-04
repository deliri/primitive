package release

import (
	"crypto"
	"crypto/ed25519"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// opaqueManifestSigner is the capability shape a KMS or HSM presents: the
// private key bytes are never exposed, only the signing behavior.
type opaqueManifestSigner struct {
	key ed25519.PrivateKey
}

func (s opaqueManifestSigner) Public() crypto.PublicKey { return s.key.Public() }

func (s opaqueManifestSigner) Sign(
	random io.Reader,
	digest []byte,
	options crypto.SignerOpts,
) ([]byte, error) {
	return s.key.Sign(random, digest, options)
}

type refusingManifestSigner struct {
	public crypto.PublicKey
}

func (s refusingManifestSigner) Public() crypto.PublicKey { return s.public }

func (refusingManifestSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errManifestSignerRefused
}

var errManifestSignerRefused = errors.New("manifest signer refused")

// TestIssueManifestRetainsTheStandardSignerBoundary checks only Release's
// ownership boundary: an opaque crypto.Signer reaches Attest successfully, and
// a provider failure remains identifiable after Release adds manifest context.
// Attest owns the hostile signer matrix and tests it directly.
func TestIssueManifestRetainsTheStandardSignerBoundary(t *testing.T) {
	t.Parallel()

	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 12), 1)
	t.Run("opaque signer issues a verifiable manifest", func(t *testing.T) {
		t.Parallel()

		document, err := IssueManifest(IssueManifestRequest{
			Fact:   fixture.manifest.Fact,
			Signer: opaqueManifestSigner{key: fixture.manifestKey},
		})
		if err != nil {
			t.Fatalf("IssueManifest() error = %v, want nil", err)
		}
		verified, err := VerifyManifest(VerifyManifestRequest{
			Document: document, TrustedKeys: fixture.manifestTrust,
			ExpectedOffering: fixture.manifest.Fact.Offering(),
		})
		if err != nil {
			t.Fatalf("VerifyManifest() error = %v, want nil", err)
		}
		if verified.Identity() != fixture.verified.Identity() {
			t.Fatalf("verified identity = %v, want %v",
				verified.Identity(), fixture.verified.Identity())
		}
	})

	t.Run("provider failure remains reachable through manifest context", func(t *testing.T) {
		t.Parallel()

		document, err := IssueManifest(IssueManifestRequest{
			Fact:   fixture.manifest.Fact,
			Signer: refusingManifestSigner{public: fixture.manifestKey.Public()},
		})
		if !errors.Is(err, core.ErrReleaseManifest) || !errors.Is(err, errManifestSignerRefused) {
			t.Fatalf("IssueManifest() error = %v, want both manifest and provider identities", err)
		}
		if document != (ManifestDocument{}) {
			t.Fatalf("IssueManifest() document = %v, want zero after signer refusal", document)
		}
	})
}
