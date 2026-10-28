package compass

import (
	json "encoding/json/v2"
	"slices"
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
			if err != nil || !sameCurrentConfiguration(got, want) {
				t.Fatalf("Current got=%v error=%v, want %v", got, err, want)
			}
			if tc.changeCopy {
				got.Project.ReleaseNotes[0] = "Caller-owned mutation."
				got.Project = Project{}
				if got.Validate() == nil {
					t.Fatalf("cleared caller copy=%v, want validation refusal", got)
				}
			}
			again, err := Current()
			if err != nil || !sameCurrentConfiguration(again, want) {
				t.Fatalf("Current after caller access got=%v error=%v, want %v", again, err, want)
			}
		})
	}
}

func sameCurrentConfiguration(got, want Configuration) bool {
	a, b := got.Project, want.Project
	return a.Name == b.Name && a.Module == b.Module && a.Repository == b.Repository && a.Release == b.Release && (a.ReleaseNotes == nil) == (b.ReleaseNotes == nil) && slices.Equal(a.ReleaseNotes, b.ReleaseNotes)
}
