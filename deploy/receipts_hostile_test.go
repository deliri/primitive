package deploy

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

func TestReceiptCollectionLayerTriad(t *testing.T) {
	t.Parallel()
	capability := receiptBoundaryCapability(t, "collection-proof")
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("Commitment error = %v, want nil", err)
	}
	ec, err := exchange.NewClient(&http.Client{Transport: &receiptBoundaryTransport{}})
	if err != nil {
		t.Fatalf("Exchange.NewClient error = %v, want nil", err)
	}
	client, err := objectstore.NewClient(ec)
	if err != nil {
		t.Fatalf("Objectstore.NewClient error = %v, want nil", err)
	}
	// The empty stream has a complete, independently known integrity contract.
	transfer, err := objectstore.Upload(t.Context(), client, objectstore.UploadCapabilityRequest{Source: bytes.NewReader(nil), Capability: capability, ContentType: core.HTTPMediaTypeOctetStream(), Integrity: objectstore.Integrity{SHA256: core.SHA256Of(nil), CRC32C: core.NewCRC32C(0)}})
	if err != nil {
		t.Fatalf("Upload receipt seed error = %v, want nil", err)
	}
	first := Receipt{transfer: transfer, commitment: commitment, role: release.PublicationRoleWindowsAMD64, valid: true}
	for _, tc := range []struct {
		name      string
		mutate    func(*Receipts)
		wantErr   error
		wantCount int
	}{
		{name: "one confirmed object", wantCount: 1},
		{name: "empty prefix carries no receipt", mutate: func(r *Receipts) { *r = Receipts{} }, wantCount: 0},
		{name: "count past fixed storage", mutate: func(r *Receipts) { r.count = release.PublicationObjectCount + 1 }, wantErr: core.ErrDeployContract},
		{name: "maximum count cannot wrap into prefix", mutate: func(r *Receipts) { r.count = 255 }, wantErr: core.ErrDeployContract},
		{name: "declared second receipt is absent", mutate: func(r *Receipts) { r.count = 2 }, wantErr: core.ErrDeployContract},
		{name: "padding cannot hide confirmed transfer", mutate: func(r *Receipts) { r.count = 0 }, wantErr: core.ErrDeployContract},
		{name: "wrong role cannot occupy first slot", mutate: func(r *Receipts) { r.values[0].role = release.PublicationRoleManifest }, wantErr: core.ErrDeployContract},
		{name: "same granted object cannot certify two roles", mutate: func(r *Receipts) {
			r.count = 2
			r.values[1] = first
			r.values[1].role = release.PublicationRoleDarwinARM64
		}, wantErr: core.ErrDeployContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Receipts{count: 1}
			got.values[0] = first
			if tc.mutate != nil {
				tc.mutate(&got)
			}
			if err := got.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Receipts.Validate = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				receipt, present := got.At(0)
				if present || receipt != (Receipt{}) {
					t.Fatalf("invalid collection.At = (%v,%t), want zero and absent", receipt, present)
				}
			} else {
				if got.Count() != tc.wantCount {
					t.Fatalf("count = %d, want %d", got.Count(), tc.wantCount)
				}
				if tc.wantCount > 0 {
					receipt, present := got.At(0)
					if !present || receipt != first {
						t.Fatalf("first receipt = (%v,%t), want exact producer transfer", receipt, present)
					}
				}
				for _, index := range []int{-1, tc.wantCount, release.PublicationObjectCount} {
					receipt, present := got.At(index)
					if present || receipt != (Receipt{}) {
						t.Fatalf("At(%d) = (%v,%t), want zero and absent", index, receipt, present)
					}
				}
			}
		})
	}
}
