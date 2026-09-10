package submissionauth

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/objectstore"
)

func BenchmarkAuthentication(b *testing.B) {
	fixture := newAuthCompletionFixture(b, authCompletionFixtureRequest{})
	server := submissionAuthServer(b, fixture.request.trusted)
	verification := CompletionVerification{Document: fixture.credentialed, Request: fixture.verifiedRequest, Grant: fixture.grant, GrantKeys: fixture.request.trusted, Server: server, Nonce: fixture.completionNonce}
	completion, err := VerifyCompletion(verification)
	if err != nil {
		b.Fatal(err)
	}
	payload, err := completion.Payload()
	if err != nil {
		b.Fatal(err)
	}
	version, present := payload.Evidence.Version()
	if !present {
		b.Fatal("provider version present=false, want true")
	}
	reconciliation := benchmarkReconciliation(b, fixture, completion)
	projection := assembleAuthCompletionProjection(b, fixture)
	requestJSON, err := fixture.request.document.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	completionJSON, err := fixture.credentialed.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	if len(requestJSON) == 0 || len(completionJSON) == 0 || version.Validate() != nil {
		b.Fatalf("fixture = %d/%d bytes, version %v, want nonempty valid documents", len(requestJSON), len(completionJSON), version)
	}
	b.ReportAllocs()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"verify_request", func() error {
			got, err := Verify(Verification{Document: fixture.request.document, Server: server})
			if err != nil {
				return err
			}
			doc, err := got.Document()
			if err == nil && doc != fixture.request.document {
				b.Fatalf("request = %v, want %v", doc, fixture.request.document)
			}
			return err
		}},
		{"request_json_roundtrip", func() error {
			var got RequestDocument
			if err := got.UnmarshalJSON(requestJSON); err != nil {
				return err
			}
			encoded, err := got.MarshalJSON()
			if err == nil && (got != fixture.request.document || !bytes.Equal(encoded, requestJSON)) {
				b.Fatalf("request = %v, encoded bytes = %d, want exact %v", got, len(encoded), fixture.request.document)
			}
			return err
		}},
		{"verify_completion", func() error {
			got, err := VerifyCompletion(verification)
			if err != nil {
				return err
			}
			body, err := got.Payload()
			if err == nil && body != payload {
				b.Fatalf("completion = %v, want %v", body, payload)
			}
			return err
		}},
		{"completion_json_roundtrip", func() error {
			var got CompletionDocument
			if err := got.UnmarshalJSON(completionJSON); err != nil {
				return err
			}
			encoded, err := got.MarshalJSON()
			if err == nil && (got != fixture.credentialed || !bytes.Equal(encoded, completionJSON)) {
				b.Fatalf("completion = %v, encoded bytes = %d, want exact %v", got, len(encoded), fixture.credentialed)
			}
			return err
		}},
		{"projection_json", func() error {
			encoded, err := projection.MarshalJSON()
			if err == nil && !bytes.Equal(encoded, completionJSON) {
				b.Fatalf("projection = %d bytes, want exact %d bytes", len(encoded), len(completionJSON))
			}
			return err
		}},
		{"reconcile_receipt", func() error {
			got, err := ReconcileCompletion(reconciliation)
			if err != nil {
				return err
			}
			addition, err := got.Addition()
			if err != nil {
				return err
			}
			body, err := addition.Evidence.Body()
			if err == nil && (body.Submission != reconciliation.Submission || body.Object != reconciliation.Object || body.SHA256 != fixture.request.request.Payload.Declaration.SHA256) {
				b.Fatalf("receipt body = %v, want authority identities %v/%v and declared digest", body, reconciliation.Submission, reconciliation.Object)
			}
			return err
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := tc.run(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkReconciliation(b testing.TB, fixture authCompletionFixture, completion VerifiedCompletion) CompletionReconciliationRequest {
	b.Helper()
	payload, err := completion.Payload()
	if err != nil {
		b.Fatal(err)
	}
	version, present := payload.Evidence.Version()
	if !present {
		b.Fatal("provider version present=false, want true")
	}
	observation := reconciliationObservedUpload(b, objectstore.ProviderUploadObservationRequest{
		Evidence: payload.Evidence, Version: version, Bytes: payload.Evidence.Bytes(), CRC32C: payload.Evidence.CRC32C(),
		ContentType: fixture.request.request.Payload.Declaration.ContentType, OccurredAt: fixture.grant.Payload.IssuedAt,
	})
	return CompletionReconciliationRequest{Key: fixture.request.authority, Completion: completion, Observation: observation, TrustedKeys: fixture.request.trusted,
		Receipt: reconciliationReceiptID(b, 0x41), Submission: reconciliationSubmissionID(b, 0x42), Object: reconciliationObjectID(b, 0x43)}
}
