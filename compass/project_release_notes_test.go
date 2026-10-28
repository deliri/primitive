package compass_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
)

func TestProjectReleaseNotesPreserveOrderedChanges(t *testing.T) {
	t.Parallel()
	project := projectFixture(t, "Release tool", "example.com/tool", "owner/tool", 2026, 1, 1)
	authored := struct {
		Project struct {
			Name         compass.ProjectName        `json:"name"`
			Module       string                     `json:"module"`
			Repository   string                     `json:"repository"`
			Release      compass.ReleaseCoordinates `json:"release"`
			ReleaseNotes []string                   `json:"release_notes"`
		} `json:"project"`
	}{}
	authored.Project.Name = project.Name
	authored.Project.Module = project.Module.String()
	authored.Project.Repository = project.Repository.String()
	authored.Project.Release = project.Release
	authored.Project.ReleaseNotes = []string{"Preserve native error causes.", "Keep authored paragraphs.\nSecond paragraph.", "  Keep spacing.  "}
	encoded, encodeErr := core.MarshalCanonicalJSONDocument(authored)
	if encodeErr != nil {
		t.Fatalf("encode authored changes=%v, want nil", encodeErr)
	}
	got, decodeErr := compass.Decode[compass.Configuration](bytes.NewReader(encoded))
	if decodeErr != nil {
		t.Fatalf("decode ordered project notes=%v, want one shared project contract", decodeErr)
	}
	roundtrip, roundtripErr := core.MarshalCanonicalJSONDocument(got)
	if roundtripErr != nil || !bytes.Equal(roundtrip, encoded) {
		t.Errorf("roundtrip=%q error=%v, want exact project and ordered notes=%q", roundtrip, roundtripErr, encoded)
	}
}
