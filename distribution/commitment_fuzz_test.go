package distribution

import (
	"crypto/sha256"
	"errors"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func commitmentSeed(t testing.TB) UpdateRequestPayload {
	t.Helper()
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("ParseBuildCommit()=%v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: core.Offering{Token: "commitment-frame-fixture"}, Version: core.NewReleaseVersion(2026, 1, 34), Commit: commit,
		Platform: core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64},
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity()=%v, want nil", err)
	}
	nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{1})
	if err != nil {
		t.Fatalf("NewRequestNonce()=%v, want nil", err)
	}
	return UpdateRequestPayload{Build: build, Nonce: nonce, Revision: controlwire.Revision2026V1}
}
func FuzzRequestCommitmentCanonicalFrame(f *testing.F) {
	seed := commitmentSeed(f)
	wire, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed)=%v, want nil", err)
	}
	f.Add(wire)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var request UpdateRequestPayload
		decodeErr := request.UnmarshalJSON(data)
		got, err := CommitRequest(request)
		if decodeErr != nil {
			if request != (UpdateRequestPayload{}) || got != (RequestCommitment{}) || !errors.Is(err, core.ErrDistributionContract) {
				t.Fatalf("refused commitment=(%v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		canonical, encodeErr := request.MarshalJSON()
		if encodeErr != nil {
			t.Fatalf("MarshalJSON()=%v, want nil", encodeErr)
		}
		frame := append([]byte(SigningDomainUpdateRequestV1.String()), documentCommitmentFrameSeparator)
		frame = append(frame, canonical...)
		want := core.NewSHA256Digest(sha256.Sum256(frame))
		if err != nil || got.domain != SigningDomainUpdateRequestV1 || got.digest != want || got.Validate() != nil {
			t.Fatalf("CommitRequest()=(%v,%v), want independently framed SHA256 %v", got, err, want)
		}
	})
}

func TestRequestCommitmentEmptyDigestCannotBecomeProof(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		digest  core.SHA256Digest
		wantErr error
	}{
		{name: "unset digest", wantErr: core.ErrDistributionContract},
		{name: "explicit all zero digest", digest: core.NewSHA256Digest([sha256.Size]byte{}), wantErr: core.ErrDistributionContract},
		{name: "nonzero digest", digest: core.SHA256Of([]byte("committed fact"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newRequestCommitment(SigningDomainUpdateRequestV1, tc.digest)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("newRequestCommitment()=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got != (RequestCommitment{}) {
				t.Fatalf("refused commitment=%v, want zero", got)
			}
		})
	}
}
