package deploy_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

func FuzzUploadItemTypedAdmission(f *testing.F) {
	f.Add(uint64(1), uint8(release.PublicationRoleWindowsAMD64), false)
	f.Add(uint64(math.MaxUint64), uint8(release.PublicationRoleManifest), false)
	f.Add(uint64(1), uint8(release.PublicationRoleDependencies), true)
	f.Fuzz(func(t *testing.T, extent uint64, role uint8, foreign bool) {
		spec, err := objectstore.Spec(objectstore.ProviderGoogleCloudStorage)
		if err != nil {
			t.Fatalf("Spec error = %v, want nil", err)
		}
		var integrity release.ArtifactIntegrity
		if extent > 0 {
			count, err := core.NewByteCount(extent)
			if err != nil {
				t.Fatalf("NewByteCount error = %v, want nil", err)
			}
			asset, err := release.NewMetadataAsset(release.MetadataAssetRequest{Kind: release.MetadataKindDependencies, Extent: count, SHA256: core.SHA256Of([]byte("typed ingress")), CRC32C: core.NewCRC32C(1)})
			if err != nil {
				t.Fatalf("NewMetadataAsset error = %v, want nil", err)
			}
			integrity = asset.Integrity()
		}
		c := fixtureCapability(t, 0)
		commitment := fixtureCommitment(t, c)
		if foreign {
			commitment = fixtureCommitment(t, fixtureCapability(t, 1))
		}
		source := &unreadSource{}
		request := deploy.UploadItemRequest{Source: source, Capability: c, Commitment: commitment, Integrity: integrity, Role: release.PublicationRole(role)}
		got, err := deploy.NewUploadItem(request)
		admissible := extent > 0 && extent <= spec.UploadMaximum.Uint64() && role >= uint8(release.PublicationRoleWindowsAMD64) && role <= uint8(release.PublicationRoleReleaseNotes) && !foreign
		if admissible {
			if err != nil || got.Validate() != nil {
				t.Fatalf("typed admission = (%v,%v), want valid item", got, err)
			}
		} else if !errors.Is(err, core.ErrDeployContract) || !errors.Is(got.Validate(), core.ErrDeployContract) {
			t.Fatalf("typed refusal = (%v,%v), want no admitted item", got, err)
		}
		if source.reads != 0 {
			t.Fatalf("admission source reads = %d, want zero", source.reads)
		}
	})
}

// Fixture payloads are signed through Release before sources are replaced.
// Only the original byte stream may extend the confirmed publication prefix.
func FuzzReleaseGCSSourceAndPrefix(f *testing.F) {
	f.Add(fixturePayload(0), uint8(0))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("foreign"), uint8(7))
	f.Fuzz(func(t *testing.T, data []byte, slot uint8) {
		index := int(slot) % release.PublicationObjectCount
		request := newDeployFixtureRequest(t)
		wantBytes := fixturePayload(index)
		if index == release.TargetCount {
			var err error
			wantBytes, err = request.Manifest.Document().MarshalJSON()
			if err != nil {
				t.Fatalf("Manifest.MarshalJSON error = %v, want nil", err)
			}
		}
		c := fixtureCapability(t, index)
		item, err := deploy.NewUploadItem(deploy.UploadItemRequest{Source: bytes.NewReader(data), Capability: c, Commitment: fixtureCommitment(t, c), Integrity: fixtureReleaseIntegrity(t, wantBytes), Role: release.PublicationRole(index + 1)})
		if err != nil {
			t.Fatalf("NewUploadItem seed error = %v, want nil", err)
		}
		request.Items[index] = item
		plan, err := deploy.PrepareRelease(request)
		if err != nil {
			t.Fatalf("PrepareRelease error = %v, want nil", err)
		}
		transport := &recordingTransport{failAt: -1}
		got, err := deploy.ReleaseGCS(t.Context(), deployObjectstoreClient(t, transport), plan)
		count := release.PublicationObjectCount
		if !bytes.Equal(data, wantBytes) {
			count = index
			failure, present := errors.AsType[*deploy.UploadError](err)
			wantErr := core.ErrObjectStoreSource
			if len(data) == len(wantBytes) {
				wantErr = core.ErrObjectStoreIntegrity
			}
			if !present || !errors.Is(err, wantErr) || failure.Role != release.PublicationRole(index+1) || failure.Transfer.Commitment() == objectstore.CommitmentConfirmed {
				t.Fatalf("foreign source error = %v, want typed source refusal at role %d", err, index+1)
			}
			if transport.requests != index+1 {
				t.Fatalf("foreign source requests = %d, want %d without retry", transport.requests, index+1)
			}
		} else if err != nil || transport.requests != release.PublicationObjectCount {
			t.Fatalf("signed source = (%d requests,%v), want complete publication", transport.requests, err)
		}
		if got.Count() != count || got.Validate() != nil {
			t.Fatalf("confirmed prefix = (%d,%v), want %d", got.Count(), got.Validate(), count)
		}
		for position := range count {
			receipt, present := got.At(position)
			want := fixtureCommitment(t, fixtureCapability(t, position))
			if !present || receipt.Commitment() != want || receipt.Transfer().Commitment() != objectstore.CommitmentConfirmed {
				t.Fatalf("prefix receipt %d = (%v,%t), want exact confirmed grant", position, receipt, present)
			}
		}
		if receipt, present := got.At(count); present || receipt.Validate() == nil {
			t.Fatalf("beyond prefix = (%v,%t), want absent", receipt, present)
		}
	})
}

type generationTransport struct {
	generation string
	duplicate  bool
	calls      int
}

func (p *generationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	p.calls++
	_, readErr := io.Copy(io.Discard, request.Body)
	if err := errors.Join(readErr, request.Body.Close()); err != nil {
		return nil, err
	}
	h := make(http.Header)
	h.Set("X-Goog-Generation", p.generation)
	if p.duplicate {
		h.Add("X-Goog-Generation", "1")
	}
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: http.NoBody, ContentLength: 0, Request: request}, nil
}

// Numeric, missing, duplicated and malformed provider generation facts cross
// the actual Objectstore producer before Deploy can emit a receipt.
func FuzzReleaseGCSProviderGeneration(f *testing.F) {
	f.Add(uint64(1), uint8(0))
	f.Add(uint64(math.MaxInt64), uint8(0))
	f.Add(uint64(math.MaxInt64)+1, uint8(0))
	f.Add(uint64(1), uint8(1))
	f.Fuzz(func(t *testing.T, value uint64, mutation uint8) {
		mode := mutation % 4
		generation := strconv.FormatUint(value, 10)
		switch mode {
		case 1:
			generation = ""
		case 2:
			generation += "x"
		}
		transport := &generationTransport{generation: generation, duplicate: mode == 3}
		request := newDeployFixtureRequest(t)
		plan, err := deploy.PrepareRelease(request)
		if err != nil {
			t.Fatalf("PrepareRelease error = %v, want nil", err)
		}
		got, err := deploy.ReleaseGCS(t.Context(), deployObjectstoreClient(t, transport), plan)
		admissible := mode == 0 && value > 0 && value <= math.MaxInt64
		if admissible {
			if err != nil || got.Count() != release.PublicationObjectCount || transport.calls != release.PublicationObjectCount {
				t.Fatalf("canonical generation = (%d,%d,%v), want full confirmed publication", got.Count(), transport.calls, err)
			}
			for i := range got.Count() {
				receipt, ok := got.At(i)
				version, present := receipt.Transfer().Version()
				if !ok || !present || version.String() != generation {
					t.Fatalf("provider version %d = (%v,%t), want %q", i, version, present, generation)
				}
			}
		} else {
			failure, present := errors.AsType[*deploy.UploadError](err)
			if !present || !errors.Is(err, core.ErrDeployContract) || got.Count() != 0 || transport.calls != 1 || failure.Transfer.Commitment() != objectstore.CommitmentIndeterminate {
				t.Fatalf("refused generation = (%d,%d,%v), want zero confirmed prefix and indeterminate attempt", got.Count(), transport.calls, err)
			}
			evidence, evidenceErr := failure.Transfer.Evidence()
			if !errors.Is(evidenceErr, core.ErrObjectStoreContract) || evidence.Validate() == nil {
				t.Fatalf("refused generation evidence = (%v,%v), want no confirmation", evidence, evidenceErr)
			}
		}
	})
}
