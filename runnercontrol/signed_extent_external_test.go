package runnercontrol_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestSignedCompletionNominalExtentPreservesAuthentication(t *testing.T) {
	t.Parallel()
	t.Run("runner completion preserves signed payload across transport whitespace", func(t *testing.T) {
		t.Parallel()
		key, trusted := completionSignerFixture(t)
		document, err := runnercontrol.IssueRunnerCompletion(directRunnerCompletionPayloadFixture(t), key)
		if err != nil {
			t.Fatal(err)
		}
		canonical := mustRunnerCompletionJSON(t, document)
		data := append(bytes.Repeat([]byte(" "), (1<<20)+1), canonical...)
		var got runnercontrol.RunnerCompletionDocument
		if err := got.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON(padded) = %v, want nil", err)
		}
		if err := runnercontrol.VerifyRunnerCompletion(got, trusted); err != nil {
			t.Fatalf("VerifyRunnerCompletion(padded) = %v, want nil", err)
		}
		if encoded := mustRunnerCompletionJSON(t, got); !bytes.Equal(encoded, canonical) {
			t.Fatalf("canonical bytes = %d, want exact %d signed bytes", len(encoded), len(canonical))
		}
	})
	t.Run("experiment completion preserves signed payload across transport whitespace", func(t *testing.T) {
		t.Parallel()
		key, trusted := completionSignerFixture(t)
		document, err := runnercontrol.IssueExperimentCompletion(experimentCompletionPayloadFixture(t, true), key)
		if err != nil {
			t.Fatal(err)
		}
		canonical := mustCompletionJSON(t, document)
		data := append(bytes.Repeat([]byte(" "), (1<<20)+1), canonical...)
		var got runnercontrol.ExperimentCompletionDocument
		if err := got.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON(padded) = %v, want nil", err)
		}
		if err := runnercontrol.VerifyExperimentCompletion(got, trusted); err != nil {
			t.Fatalf("VerifyExperimentCompletion(padded) = %v, want nil", err)
		}
		if encoded := mustCompletionJSON(t, got); !bytes.Equal(encoded, canonical) {
			t.Fatalf("canonical bytes = %d, want exact %d signed bytes", len(encoded), len(canonical))
		}
	})
}
