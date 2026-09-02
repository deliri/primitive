package primitiveproject_test

import (
	"slices"
	"testing"

	primitivecore "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestProjectStandardFunctionsCloseAuthoredMeaningOverGeneratedSource(t *testing.T) {
	t.Parallel()

	commit, err := primitivecore.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit() error = %v, want nil", err)
	}
	changed := projectstandards.GitOrigin{
		Commit: commit,
		At:     temporal.InstantFromNanoseconds(1_767_225_600_000_000_000),
	}
	knowledge, err := ProjectStandardKnowledge(projectstandards.OptionalGitOrigin{}, changed)
	if err != nil {
		t.Fatalf("ProjectStandardKnowledge() error = %v, want nil", err)
	}
	catalog, err := PackageStandardCode()
	if err != nil {
		t.Fatalf("PackageStandardCode() error = %v, want nil", err)
	}
	if knowledge.Changed != changed {
		t.Fatalf("ProductKnowledge.Changed = %v, want %v", knowledge.Changed, changed)
	}
	paths := make([]projectstandards.SourcePath, len(catalog.Files))
	for index := range catalog.Files {
		paths[index] = catalog.Files[index].Path
	}
	if !slices.Contains(paths, knowledge.SourcePath) {
		t.Fatalf("PackageStandardCode() paths contain authored source = false, want true")
	}
}
