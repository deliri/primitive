package deploy_test

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"io"
	"strconv"
	"testing"
)

func TestTransferCapabilityEvidenceLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                    string
		failAt                  int
		absent                  bool
		wantCount, wantRequests int
		wantErr                 error
	}{
		{name: "complete publication preserves every capability", failAt: -1, wantCount: release.PublicationObjectCount, wantRequests: release.PublicationObjectCount},
		{name: "absent plan emits no capability proof", failAt: -1, absent: true, wantErr: core.ErrDeployContract},
	}
	for i := range release.PublicationObjectCount {
		cases = append(cases, struct {
			name                    string
			failAt                  int
			absent                  bool
			wantCount, wantRequests int
			wantErr                 error
		}{
			name: "failure at publication slot " + strconv.Itoa(i), failAt: i, wantCount: i, wantRequests: i + 1, wantErr: core.ErrDeployContract})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newDeployFixture(t)
			plan := fixture.plan
			if tc.absent {
				plan = deploy.ReleasePlan{}
			}
			transport := &recordingTransport{failAt: tc.failAt}
			got, err := deploy.ReleaseGCS(t.Context(), deployObjectstoreClient(t, transport), plan)
			if !errors.Is(err, tc.wantErr) || got.Count() != tc.wantCount || transport.requests != tc.wantRequests {
				t.Fatalf("ReleaseGCS()=(%d,%d,%v), want (%d,%d,%v)", got.Count(), transport.requests, err, tc.wantCount, tc.wantRequests, tc.wantErr)
			}

			if !tc.absent && tc.failAt >= 0 {
				failure, present := errors.AsType[*deploy.UploadError](err)
				if !present {
					t.Fatalf("ReleaseGCS() error=%v, want *deploy.UploadError", err)
				}
				want := fixtureCommitment(t, fixtureCapability(t, tc.failAt))
				gotCapability, hasCapability := failure.Transfer.UploadCapability()
				if failure.Role != release.PublicationRole(tc.failAt+1) || !hasCapability || gotCapability != want || failure.Transfer.Commitment() != objectstore.CommitmentIndeterminate {
					t.Fatalf("failed role/capability/outcome=(%v,%v,%t,%v), want (%v,%v,true,%v)", failure.Role, gotCapability, hasCapability, failure.Transfer.Commitment(), release.PublicationRole(tc.failAt+1), want, objectstore.CommitmentIndeterminate)
				}
				evidence, evidenceErr := failure.Transfer.Evidence()
				wire, wireErr := evidence.MarshalJSON()
				if !errors.Is(evidenceErr, core.ErrObjectStoreContract) || wire != nil || !errors.Is(wireErr, core.ErrJSONContract) {
					t.Fatalf("failed transfer evidence=(%v,%q,%v), want typed refusal and no wire", evidenceErr, wire, wireErr)
				}
			}
			for i := range got.Count() {
				receipt, ok := got.At(i)
				if !ok {
					t.Fatalf("Receipts.At(%d)=%t, want true", i, ok)
				}
				commitment, present := receipt.Transfer().UploadCapability()
				want := fixtureCommitment(t, fixtureCapability(t, i))
				if !present || commitment != want || receipt.Commitment() != want || receipt.Transfer().Commitment() != objectstore.CommitmentConfirmed {
					t.Fatalf("receipt %d capability=(%v,%t), want exact %v on confirmed transfer", i, commitment, present, want)
				}
			}
			if receipt, present := got.At(got.Count()); present || receipt.Validate() == nil {
				t.Fatalf("prefix successor=(%v,%t), want absent invalid receipt", receipt, present)
			}
		})
	}
}

func TestUploadItemSourceAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		source  func() io.Reader
		wantErr error
	}{
		{"owned empty stream", func() io.Reader { return bytes.NewReader(nil) }, nil},
		{"absent reader", func() io.Reader { return nil }, core.ErrDeployContract},
		{"typed nil reader", func() io.Reader { return (*bytes.Reader)(nil) }, core.ErrDeployContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := fixtureCapability(t, 0)
			request := deploy.UploadItemRequest{Source: tc.source(), Capability: c, Commitment: fixtureCommitment(t, c), Integrity: fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleWindowsAMD64}
			got, err := deploy.NewUploadItem(request)
			if !errors.Is(request.Validate(), tc.wantErr) || !errors.Is(err, tc.wantErr) {
				t.Fatalf("source admission=(%v,%v), want %v", request.Validate(), err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(got.Validate(), core.ErrDeployContract) {
					t.Fatalf("refused item=%v, want typed invalid item", got)
				}
			} else if got.Validate() != nil {
				t.Fatalf("owned stream item=%v, want valid item", got.Validate())
			}
		})
	}
}
