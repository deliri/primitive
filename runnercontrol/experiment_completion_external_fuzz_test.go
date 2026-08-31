package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func FuzzExperimentCompletionJSONSemanticClosure(f *testing.F) {
	payload := experimentCompletionPayloadFixture(f, true)
	key, trusted := completionSignerFixture(f)
	document, issueErr := runnercontrol.IssueExperimentCompletion(payload, key)
	if issueErr != nil {
		f.Fatalf("IssueExperimentCompletion(seed) error = %v, want nil", issueErr)
	}
	canonical := mustCompletionJSON(f, document)
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"payload":`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := document
		before := mustCompletionJSON(t, got)
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("ExperimentCompletionDocument.UnmarshalJSON(rejected) error = %v, want errors.Is(..., %v)", gotErr, core.ErrJSONContract)
			}
			after := mustCompletionJSON(t, got)
			if !bytes.Equal(after, before) {
				t.Fatalf("ExperimentCompletionDocument.UnmarshalJSON(rejected) receiver = %q, want preserved %q", after, before)
			}
			return
		}

		encoded := mustCompletionJSON(t, got)
		if len(encoded) > runnercontrol.ExperimentCompletionDocumentMaximumBytes {
			t.Fatalf("ExperimentCompletionDocument.MarshalJSON(accepted) bytes = %d, want <= %d", len(encoded), runnercontrol.ExperimentCompletionDocumentMaximumBytes)
		}
		var roundTrip runnercontrol.ExperimentCompletionDocument
		if roundTripErr := roundTrip.UnmarshalJSON(encoded); roundTripErr != nil {
			t.Fatalf("ExperimentCompletionDocument canonical round trip error = %v, want nil", roundTripErr)
		}
		second := mustCompletionJSON(t, roundTrip)
		if !bytes.Equal(second, encoded) {
			t.Fatalf("ExperimentCompletionDocument second canonical projection = %q, want %q", second, encoded)
		}
		verifyErr := runnercontrol.VerifyExperimentCompletion(got, trusted)
		if verifyErr == nil && !bytes.Equal(encoded, canonical) {
			t.Fatalf("VerifyExperimentCompletion(mutated) accepted %q, want only genuinely signed seed %q", encoded, canonical)
		}
		if verifyErr != nil && !errors.Is(verifyErr, core.ErrAttestVerification) {
			t.Fatalf("VerifyExperimentCompletion(structurally accepted mutation) error = %v, want nil or errors.Is(..., %v)", verifyErr, core.ErrAttestVerification)
		}
	})
}
