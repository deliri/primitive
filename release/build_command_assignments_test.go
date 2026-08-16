package release_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/release"
)

func TestBuildCommandLinkerAssignmentsLayerTriadPositiveMatchesPreparedPlan(t *testing.T) {
	t.Parallel()

	request := buildPlanRequestForHostileTest(t)
	plan, err := release.PrepareBuildPlan(request)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	command, ok := plan.At(0)
	if !ok {
		t.Fatal("release.BuildPlan.At(0) ok = false, want true")
	}
	got := command.LinkerAssignments()
	if err := got.Validate(); err != nil {
		t.Fatalf("BuildCommand.LinkerAssignments().Validate() error = %v, want nil", err)
	}
	if got != request.LinkerAssignments {
		t.Fatalf("BuildCommand.LinkerAssignments() = %v, want the prepared plan set", got)
	}
}

func TestBuildCommandLinkerAssignmentsLayerTriadNegativeZeroCommandHasZeroAssignments(t *testing.T) {
	t.Parallel()

	got := release.BuildCommand{}.LinkerAssignments()
	if got != (release.LinkerAssignments{}) {
		t.Fatalf("BuildCommand{}.LinkerAssignments() = %v, want zero", got)
	}
}

func TestBuildCommandLinkerAssignmentsLayerTriadNeutralEmptyPreparedSetSurvives(t *testing.T) {
	t.Parallel()

	request := buildPlanRequestForHostileTest(t)
	empty, err := release.NewLinkerAssignments(nil)
	if err != nil {
		t.Fatalf("NewLinkerAssignments(nil) setup error = %v, want nil", err)
	}
	request.LinkerAssignments = empty
	plan, err := release.PrepareBuildPlan(request)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	command, ok := plan.At(0)
	if !ok {
		t.Fatal("release.BuildPlan.At(0) ok = false, want true")
	}
	got := command.LinkerAssignments()
	if got != empty {
		t.Fatalf("BuildCommand.LinkerAssignments() = %v, want empty prepared set", got)
	}
}
