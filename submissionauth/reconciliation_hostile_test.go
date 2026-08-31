package submissionauth

import (
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/submission"
)

// TestCompletionReconciliationLayerTriad proves the real authority-side
// composition: credential authentication, exact SDK provider observation,
// authority receipt issuance and verification, then one chit-ready addition.
func TestCompletionReconciliationLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	completion, err := VerifyCompletion(CompletionVerification{
		Document: fixture.credentialed, Request: fixture.verifiedRequest,
		Grant: fixture.grant, GrantKeys: fixture.request.trusted,
		Server: submissionAuthServer(t, fixture.request.trusted),
	})
	if err != nil {
		t.Fatalf("VerifyCompletion() error = %v, want nil", err)
	}
	payload, err := completion.Payload()
	if err != nil {
		t.Fatalf("VerifiedCompletion.Payload() error = %v, want nil", err)
	}
	declarationContentType := fixture.request.request.Payload.Declaration.ContentType
	version, present := payload.Evidence.Version()
	if !present {
		t.Fatal("completion evidence provider version present = false, want true")
	}
	observation := reconciliationObservedUpload(t, objectstore.ProviderUploadObservationRequest{
		Evidence: payload.Evidence, Version: version,
		Bytes: payload.Evidence.Bytes(), CRC32C: payload.Evidence.CRC32C(),
		ContentType: declarationContentType, OccurredAt: fixture.grant.Payload.IssuedAt,
	})
	base := CompletionReconciliationRequest{
		Key: fixture.request.authority, Completion: completion, Observation: observation,
		TrustedKeys: fixture.request.trusted,
	}

	t.Run("positive exact authority facts release one verified manifest addition", func(t *testing.T) {
		t.Parallel()

		for marker := byte(1); marker <= 10; marker++ {
			t.Run(strconv.Itoa(int(marker)), func(t *testing.T) {
				t.Parallel()

				request := base
				request.Receipt = reconciliationReceiptID(t, marker)
				request.Submission = reconciliationSubmissionID(t, marker+16)
				request.Object = reconciliationObjectID(t, marker+32)
				got, gotErr := ReconcileCompletion(request)
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("ReconcileCompletion(%d) = (%v, %v), want validated proof and nil", marker, got, gotErr)
				}
				manifest, manifestErr := got.Manifest()
				addition, additionErr := got.Addition()
				if manifestErr != nil || additionErr != nil || addition.Validate() != nil ||
					manifest != fixture.request.request.Payload.Manifest {
					t.Fatalf("reconciled projections = (%v, %v, %v, %v), want exact manifest and authenticated addition",
						manifest, addition, manifestErr, additionErr)
				}
				body, bodyErr := addition.Evidence.Body()
				header, headerErr := addition.Evidence.Header()
				declaration := fixture.request.request.Payload.Declaration
				if bodyErr != nil || headerErr != nil || body.Submission != request.Submission ||
					body.Object != request.Object || body.Extent != declaration.Extent ||
					body.SHA256 != declaration.SHA256 || body.CRC32C != declaration.CRC32C ||
					header.Identity != request.Receipt || header.OccurredAt != fixture.grant.Payload.IssuedAt {
					t.Fatalf("reconciled receipt facts = (%v, %v, %v, %v), want exact authority identities, provider time, and request integrity",
						body, header, bodyErr, headerErr)
				}
			})
		}
	})

	t.Run("negative incomplete foreign and contradictory facts release no custody proof", func(t *testing.T) {
		t.Parallel()

		valid := base
		valid.Receipt = reconciliationReceiptID(t, 0x41)
		valid.Submission = reconciliationSubmissionID(t, 0x42)
		valid.Object = reconciliationObjectID(t, 0x43)
		other := newAuthCompletionFixture(t, authCompletionFixtureRequest{
			authorityByte: 0x71, deviceByte: 0x72, nonceByte: 0x73, generation: 8,
		})
		foreignEvidence := other.completionDocument.Payload.Evidence
		foreignVersion, present := foreignEvidence.Version()
		if !present {
			t.Fatal("foreign completion evidence provider version present = false, want true")
		}
		foreignObservation := reconciliationObservedUpload(t, objectstore.ProviderUploadObservationRequest{
			Evidence: foreignEvidence, Version: foreignVersion,
			Bytes: foreignEvidence.Bytes(), CRC32C: foreignEvidence.CRC32C(),
			ContentType: declarationContentType, OccurredAt: fixture.grant.Payload.IssuedAt,
		})
		wrongTypeObservation := reconciliationObservedUpload(t, objectstore.ProviderUploadObservationRequest{
			Evidence: payload.Evidence, Version: version,
			Bytes: payload.Evidence.Bytes(), CRC32C: payload.Evidence.CRC32C(),
			ContentType: core.HTTPMediaTypeOctetStream(), OccurredAt: fixture.grant.Payload.IssuedAt,
		})
		cases := []struct {
			wantErr error
			mutate  func(*CompletionReconciliationRequest)
			name    string
		}{
			{name: "zero request", mutate: func(v *CompletionReconciliationRequest) { *v = CompletionReconciliationRequest{} }, wantErr: core.ErrControlPlaneContract},
			{name: "completion proof absent", mutate: func(v *CompletionReconciliationRequest) { v.Completion = VerifiedCompletion{} }, wantErr: core.ErrControlPlaneContract},
			{name: "provider observation absent", mutate: func(v *CompletionReconciliationRequest) { v.Observation = objectstore.VerifiedProviderUpload{} }, wantErr: core.ErrControlPlaneContract},
			{name: "authority signing key absent", mutate: func(v *CompletionReconciliationRequest) { v.Key = nil }, wantErr: core.ErrControlPlaneContract},
			{name: "authority verification trust absent", mutate: func(v *CompletionReconciliationRequest) { v.TrustedKeys = attest.TrustedKeys{} }, wantErr: core.ErrControlPlaneContract},
			{name: "receipt identity absent", mutate: func(v *CompletionReconciliationRequest) { v.Receipt = receipt.ReceiptID{} }, wantErr: core.ErrControlPlaneContract},
			{name: "submission identity absent", mutate: func(v *CompletionReconciliationRequest) { v.Submission = receipt.SubmissionIdentity{} }, wantErr: core.ErrControlPlaneContract},
			{name: "object identity absent", mutate: func(v *CompletionReconciliationRequest) { v.Object = receipt.ObjectIdentity{} }, wantErr: core.ErrControlPlaneContract},
			{name: "receipt key is outside authority trust", mutate: func(v *CompletionReconciliationRequest) { v.TrustedKeys = other.request.trusted }, wantErr: core.ErrAttestVerification},
			{name: "provider observation belongs to another generation", mutate: func(v *CompletionReconciliationRequest) { v.Observation = foreignObservation }, wantErr: core.ErrControlPlaneResponseBinding},
			{name: "provider content type differs from authenticated declaration", mutate: func(v *CompletionReconciliationRequest) { v.Observation = wrongTypeObservation }, wantErr: core.ErrControlPlaneResponseBinding},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				request := valid
				tc.mutate(&request)
				got, gotErr := ReconcileCompletion(request)
				if !errors.Is(gotErr, tc.wantErr) || got != (ReconciledCompletion{}) {
					t.Fatalf("ReconcileCompletion(%s) = (%v, %v), want zero and errors.Is(..., %v)", tc.name, got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero result exposes neither manifest nor addition", func(t *testing.T) {
		t.Parallel()

		manifest, manifestErr := (ReconciledCompletion{}).Manifest()
		addition, additionErr := (ReconciledCompletion{}).Addition()
		if !errors.Is(manifestErr, core.ErrControlPlaneContract) ||
			!errors.Is(additionErr, core.ErrControlPlaneContract) ||
			manifest != (submission.ManifestIntent{}) || addition != (chit.ManifestAddition{}) {
			t.Fatalf("zero reconciliation projections = (%v, %v, %v, %v), want zero and typed refusals",
				manifest, addition, manifestErr, additionErr)
		}
	})
}

func reconciliationObservedUpload(
	t testing.TB,
	request objectstore.ProviderUploadObservationRequest,
) objectstore.VerifiedProviderUpload {
	t.Helper()
	got, err := objectstore.VerifyProviderUpload(request)
	if err != nil {
		t.Fatalf("objectstore.VerifyProviderUpload() error = %v, want nil", err)
	}
	return got
}

func reconciliationReceiptID(t testing.TB, marker byte) receipt.ReceiptID {
	t.Helper()
	raw := [receipt.ReceiptIDBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	value, err := receipt.NewReceiptID(raw)
	if err != nil {
		t.Fatalf("receipt.NewReceiptID() error = %v, want nil", err)
	}
	return value
}

func reconciliationSubmissionID(t testing.TB, marker byte) receipt.SubmissionIdentity {
	t.Helper()
	raw := [receipt.LifecycleIdentityBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	value, err := receipt.NewSubmissionIdentity(raw)
	if err != nil {
		t.Fatalf("receipt.NewSubmissionIdentity() error = %v, want nil", err)
	}
	return value
}

func reconciliationObjectID(t testing.TB, marker byte) receipt.ObjectIdentity {
	t.Helper()
	raw := [receipt.LifecycleIdentityBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	value, err := receipt.NewObjectIdentity(raw)
	if err != nil {
		t.Fatalf("receipt.NewObjectIdentity() error = %v, want nil", err)
	}
	return value
}
