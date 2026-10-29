package runprotocol

import (
	json "encoding/json/v2"
	"testing"

	"github.com/deliri/primitive/v2026/jsonio"
)

// The shared durable record must remain usable after native scratch is gone.
// This observes the actual typed encoder, not a second receipt constructor.
func TestRunReceiptExcludesRawArtifactDependencies(t *testing.T) {
	t.Parallel()
	receipt := runReceiptFixture(t)
	encoded, err := jsonio.MarshalValidated(receipt, runReceiptCodec())
	if err != nil {
		t.Fatal(err)
	}
	var dependencies struct {
		Invocation *ArtifactReference `json:"invocation,omitempty"`
		Execution  *ArtifactReference `json:"execution,omitempty"`
		Results    *ArtifactReference `json:"results,omitempty"`
	}
	if err := json.Unmarshal(encoded, &dependencies); err != nil {
		t.Fatal(err)
	}
	if dependencies.Invocation != nil || dependencies.Execution != nil || dependencies.Results != nil {
		t.Fatalf("encoded receipt raw dependencies = %+v, want none after joined scratch discard", dependencies)
	}
}
