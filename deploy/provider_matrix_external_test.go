package deploy_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

type uploadObservation struct {
	path, contentType, precondition string
	bytes                           int64
	sha                             core.SHA256Digest
	crc                             core.CRC32C
}

type exactProvider struct {
	mu             sync.Mutex
	observations   []uploadObservation
	selected       int
	status         int
	omitGeneration bool
}

func (p *exactProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sha := sha256.New()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	size, err := io.Copy(io.MultiWriter(sha, crc), r.Body)
	if err != nil {
		http.Error(w, "source did not complete", http.StatusBadRequest)
		return
	}
	var digest [sha256.Size]byte
	copy(digest[:], sha.Sum(nil))
	fact := uploadObservation{path: r.URL.Path, contentType: r.Header.Get("Content-Type"), precondition: r.Header.Get("X-Goog-If-Generation-Match"), bytes: size, sha: core.NewSHA256Digest(digest), crc: core.NewCRC32C(crc.Sum32())}
	p.mu.Lock()
	index := len(p.observations)
	p.observations = append(p.observations, fact)
	p.mu.Unlock()
	status := http.StatusOK
	if index == p.selected {
		status = p.status
	}
	if index != p.selected || !p.omitGeneration {
		w.Header().Set("X-Goog-Generation", strconv.Itoa(index+1))
	}
	w.WriteHeader(status)
}

func (p *exactProvider) snapshot() []uploadObservation {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]uploadObservation(nil), p.observations...)
}

// Exhaust the fixed publication positions across the three provider outcomes:
// confirmed, explicitly rejected, and acceptance without sufficient proof.
// Every row binds the failed role, exact prefix and independently observed bytes.
func TestProviderToReceiptPrefixLayerTriad(t *testing.T) {
	t.Parallel()
	for index := range release.PublicationObjectCount {
		for _, tc := range []struct {
			name           string
			status         int
			omit           bool
			wantErr        error
			wantCommitment objectstore.Commitment
		}{
			{name: "confirmed", status: http.StatusOK, wantCommitment: objectstore.CommitmentConfirmed},
			{name: "create conflict", status: http.StatusPreconditionFailed, wantErr: core.ErrObjectStoreConflict, wantCommitment: objectstore.CommitmentRejected},
			{name: "missing generation cannot certify acceptance", status: http.StatusOK, omit: true, wantErr: core.ErrObjectStoreContract, wantCommitment: objectstore.CommitmentIndeterminate},
		} {
			if tc.wantErr == nil && index != 0 {
				continue
			} // One complete publication proves every confirmed position.
			t.Run(fmt.Sprintf("%s at role %s", tc.name, release.PublicationRole(index+1)), func(t *testing.T) {
				t.Parallel()
				fixture := newDeployFixture(t)
				provider := &exactProvider{selected: index, status: tc.status, omitGeneration: tc.omit}
				got, err := deploy.ReleaseGCS(t.Context(), deployLoopbackClient(t, provider), fixture.plan)
				wantCount, wantRequests := release.PublicationObjectCount, release.PublicationObjectCount
				if tc.wantErr != nil {
					wantCount, wantRequests = index, index+1
				}
				observed := provider.snapshot()
				if !errors.Is(err, tc.wantErr) || got.Count() != wantCount || len(observed) != wantRequests || got.Validate() != nil {
					t.Fatalf("ReleaseGCS = (%d receipts,%d requests,%v), want (%d,%d,%v)", got.Count(), len(observed), err, wantCount, wantRequests, tc.wantErr)
				}
				for slot, fact := range observed {
					want := fixtureIntegrity(t, fixture.payloads[slot])
					if fact.bytes != int64(len(fixture.payloads[slot])) || fact.sha != want.SHA256 || fact.crc != want.CRC32C || fact.path != "/bucket/object-"+strconv.Itoa(slot) || fact.precondition != "0" {
						t.Fatalf("provider object %d = %+v, want exact source integrity, role destination and create-only precondition", slot, fact)
					}
					if slot < got.Count() {
						receipt, present := got.At(slot)
						if !present {
							t.Fatalf("Receipts.At(%d) = absent, want confirmed", slot)
						}
						transfer := receipt.Transfer()
						version, hasVersion := transfer.Version()
						commitment := fixtureCommitment(t, fixtureCapability(t, slot))
						if transfer.Bytes().Uint64() != uint64(fact.bytes) || transfer.SHA256() != fact.sha || transfer.CRC32C() != fact.crc || transfer.Commitment() != objectstore.CommitmentConfirmed || receipt.Commitment() != commitment || receipt.Role() != release.PublicationRole(slot+1) || !hasVersion || version.String() != strconv.Itoa(slot+1) {
							t.Fatalf("receipt %d = %v, want exact provider observation %+v and grant", slot, receipt, fact)
						}
					}
				}
				if tc.wantErr != nil {
					failure, present := errors.AsType[*deploy.UploadError](err)
					if !present || failure.Role != release.PublicationRole(index+1) || failure.Transfer.Commitment() != tc.wantCommitment {
						t.Fatalf("failed upload = %v, want exact role %d and %v", err, index+1, tc.wantCommitment)
					}
					evidence, evidenceErr := failure.Transfer.Evidence()
					if !errors.Is(evidenceErr, core.ErrObjectStoreContract) || evidence.Validate() == nil {
						t.Fatalf("failed evidence = (%v,%v), want no confirmed evidence", evidence, evidenceErr)
					}
				}
				if next, present := got.At(got.Count()); present || next.Validate() == nil {
					t.Fatalf("receipt after prefix = (%v,%t), want absent", next, present)
				}
			})
		}
	}
	t.Run("zero timeout policy inherits caller and completes exact publication", func(t *testing.T) {
		t.Parallel()
		request := newDeployFixtureRequest(t)
		request.Policy = objectstore.Policy{}
		plan, err := deploy.PrepareRelease(request)
		if err != nil {
			t.Fatalf("PrepareRelease zero timeout policy error = %v, want nil", err)
		}
		transport := &recordingTransport{failAt: -1}
		got, err := deploy.ReleaseGCS(t.Context(), deployObjectstoreClient(t, transport), plan)
		if err != nil || got.Count() != release.PublicationObjectCount || transport.requests != release.PublicationObjectCount {
			t.Fatalf("inherited policy = (%d,%d,%v), want complete publication", got.Count(), transport.requests, err)
		}
	})
}
