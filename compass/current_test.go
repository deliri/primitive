package compass

import (
	json "encoding/json/v2"
	"testing"
)

func TestCurrentPreservesAuthoredConfiguration(t *testing.T) {
	t.Parallel()
	var want Configuration
	if err := json.Unmarshal([]byte(currentConfigurationJSON), &want, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if err := want.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		changeCopy bool
	}{
		{name: "exact_authored_fields"},
		{name: "caller_mutation_cannot_change_embedded_source", changeCopy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Current()
			if err != nil || got != want {
				t.Fatalf("Current got=%v error=%v, want %v", got, err, want)
			}
			if tc.changeCopy {
				got.Project = Project{}
				if got.Validate() == nil {
					t.Fatalf("cleared caller copy=%v, want validation refusal", got)
				}
			}
			again, err := Current()
			if err != nil || again != want {
				t.Fatalf("Current after caller access got=%v error=%v, want %v", again, err, want)
			}
		})
	}
}
