package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func FuzzRunnerCompletionJSONSemanticClosure(f *testing.F) {
	payload := directRunnerCompletionPayloadFixture(f)
	key, trusted := completionSignerFixture(f)
	document, issueErr := runnercontrol.IssueRunnerCompletion(payload, key)
	if issueErr != nil {
		f.Fatalf("IssueRunnerCompletion(seed) error = %v, want nil", issueErr)
	}
	canonical := mustRunnerCompletionJSON(f, document)
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"payload":`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := document
		before := mustRunnerCompletionJSON(t, got)
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("RunnerCompletionDocument.UnmarshalJSON(rejected) error = %v, want errors.Is(..., %v)", gotErr, core.ErrJSONContract)
			}
			after := mustRunnerCompletionJSON(t, got)
			if !bytes.Equal(after, before) {
				t.Fatalf("RunnerCompletionDocument.UnmarshalJSON(rejected) receiver = %q, want preserved %q", after, before)
			}
			return
		}

		encoded := mustRunnerCompletionJSON(t, got)
		if len(encoded) > runnercontrol.RunnerCompletionDocumentMaximumBytes {
			t.Fatalf("RunnerCompletionDocument.MarshalJSON(accepted) bytes = %d, want <= %d", len(encoded), runnercontrol.RunnerCompletionDocumentMaximumBytes)
		}
		var roundTrip runnercontrol.RunnerCompletionDocument
		if roundTripErr := roundTrip.UnmarshalJSON(encoded); roundTripErr != nil {
			t.Fatalf("RunnerCompletionDocument canonical round trip error = %v, want nil", roundTripErr)
		}
		second := mustRunnerCompletionJSON(t, roundTrip)
		if !bytes.Equal(second, encoded) {
			t.Fatalf("RunnerCompletionDocument second canonical projection = %q, want %q", second, encoded)
		}
		verifyErr := runnercontrol.VerifyRunnerCompletion(got, trusted)
		if verifyErr == nil && !bytes.Equal(encoded, canonical) {
			t.Fatalf("VerifyRunnerCompletion(mutated) accepted %q, want only genuinely signed seed %q", encoded, canonical)
		}
		if verifyErr != nil && !errors.Is(verifyErr, core.ErrAttestVerification) {
			t.Fatalf("VerifyRunnerCompletion(structurally accepted mutation) error = %v, want nil or errors.Is(..., %v)", verifyErr, core.ErrAttestVerification)
		}
	})
}
