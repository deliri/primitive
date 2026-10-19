package release

import (
	"crypto/sha256"
	json "encoding/json/v2"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestFramedJSONDigestMatchesIndependentFraming pins the streamed identity
// digests to an independent computation: SHA-256 over the domain, one NUL and
// the exact json.Marshal body. A streamed encoding that differed by one byte
// would change every artifact, manifest and latest identity.
func TestFramedJSONDigestMatchesIndependentFraming(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	independent := func(domain string, body []byte) core.SHA256Digest {
		framed := append(append([]byte(domain), 0), body...)
		return core.NewSHA256Digest(sha256.Sum256(framed))
	}
	for index, artifact := range fixture.artifacts {
		body, err := json.Marshal(artifactIdentityWire{Build: artifact.build, Integrity: artifact.integrity})
		if err != nil {
			t.Fatal(err)
		}
		got, err := artifactDigest(artifact)
		if err != nil || got != independent(artifactIdentityDomain, body) {
			t.Fatalf("artifact %d digest = (%v,%v), want independent framing", index, got, err)
		}
	}
	fact := fixture.manifest.Fact
	body, err := json.Marshal(manifestIdentityWire{
		Revision: fact.revision, Offering: fact.offering, Version: fact.version,
		Commit: fact.commit, CreatedAt: fact.createdAt,
		TotalExtent: fact.totalExtent, Artifacts: fact.artifacts,
		Provenance: fact.provenance, Metadata: fact.metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := manifestFactDigest(fact)
	if err != nil || got != independent(manifestIdentityDomain, body) {
		t.Fatalf("manifest digest = (%v,%v), want independent framing", got, err)
	}
	revision, offering := fact.revision, fact.offering
	body, err = json.Marshal(latestIdentityWire{Revision: revision, Offering: offering})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := latestIdentity(revision, offering)
	if err != nil || identity != newLatestIdentity(independent(latestIdentityDomain, body)) {
		t.Fatalf("latest identity = (%v,%v), want independent framing", identity, err)
	}
}
