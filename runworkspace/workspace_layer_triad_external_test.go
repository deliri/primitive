package runworkspace_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runworkspace"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWorkspaceEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive scheduling unit owns isolated member directories and cleanup removes the whole unit", func(t *testing.T) {
		t.Parallel()
		root := mustWorkspaceAbsolutePath(t, t.TempDir())
		manager, openErr := runworkspace.Open(t.Context(), runworkspace.Configuration{RunParent: root})
		if openErr != nil {
			t.Fatalf("runworkspace.Open() error = %v, want nil", openErr)
		}
		defer func() {
			if closeErr := manager.Close(); closeErr != nil {
				t.Errorf("Manager.Close() error = %v, want nil", closeErr)
			}
		}()
		unitID, run := workspaceIdentities(t)
		unit, unitErr := manager.CreateUnit(t.Context(), unitID)
		member, memberErr := manager.CreateMember(t.Context(), unit, run)
		experimentID := workspaceExperimentIdentity(t)
		experiment, experimentErr := manager.CreateExperiment(t.Context(), member, experimentID)
		if err := errors.Join(unitErr, memberErr, experimentErr); err != nil {
			t.Fatalf("workspace create unit/member/experiment error = %v, want nil", err)
		}
		if member.Run != run || experiment.Run != run || experiment.Identity != experimentID || experiment.Root == experiment.Home || experiment.Home == experiment.Output || experiment.Output == experiment.Cache || experiment.Cache == experiment.Temporary {
			t.Fatalf("CreateExperiment() = member %+v/experiment %+v, want exact identities and five distinct experiment-owned coordinates", member, experiment)
		}
		before, observeErr := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
		if observeErr != nil || before.Entries < 9 {
			t.Fatalf("Manager.Observe(populated) = (%+v, %v), want at least 9 workspace entries and nil", before, observeErr)
		}
		if cleanupErr := manager.CleanupUnit(t.Context(), unit); cleanupErr != nil {
			t.Fatalf("Manager.CleanupUnit() error = %v, want nil", cleanupErr)
		}
		clean, cleanErr := manager.ProveClean(t.Context(), temporal.InstantFromNanoseconds(2), runworkspace.Residue{})
		if cleanErr != nil || !clean.Observation.IsClean() {
			t.Fatalf("Manager.ProveClean() = (%+v, %v), want exact zero residue and nil", clean, cleanErr)
		}
	})

	t.Run("negative foreign root identity cannot delete a unit", func(t *testing.T) {
		t.Parallel()
		root := mustWorkspaceAbsolutePath(t, t.TempDir())
		manager, openErr := runworkspace.Open(t.Context(), runworkspace.Configuration{RunParent: root})
		if openErr != nil {
			t.Fatalf("runworkspace.Open() error = %v, want nil", openErr)
		}
		defer func() {
			if closeErr := manager.Close(); closeErr != nil {
				t.Errorf("Manager.Close() error = %v, want nil", closeErr)
			}
		}()
		unitID, _ := workspaceIdentities(t)
		unit, unitErr := manager.CreateUnit(t.Context(), unitID)
		if unitErr != nil {
			t.Fatalf("Manager.CreateUnit() setup error = %v, want nil", unitErr)
		}
		unit.RootIdentity = core.SHA256Of([]byte("foreign-workspace-root"))
		gotErr := manager.CleanupUnit(t.Context(), unit)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("Manager.CleanupUnit(foreign root) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral empty parent proves clean without inventing residue", func(t *testing.T) {
		t.Parallel()
		root := mustWorkspaceAbsolutePath(t, t.TempDir())
		manager, openErr := runworkspace.Open(t.Context(), runworkspace.Configuration{RunParent: root})
		if openErr != nil {
			t.Fatalf("runworkspace.Open() error = %v, want nil", openErr)
		}
		defer func() {
			if closeErr := manager.Close(); closeErr != nil {
				t.Errorf("Manager.Close() error = %v, want nil", closeErr)
			}
		}()
		got, gotErr := manager.ProveClean(t.Context(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
		if gotErr != nil || got.Observation.Entries != 0 || !got.Observation.IsClean() {
			t.Fatalf("Manager.ProveClean(empty) = (%+v, %v), want zero clean observation and nil", got, gotErr)
		}
	})
}

func workspaceExperimentIdentity(t testing.TB) projectstandards.ExperimentID {
	t.Helper()
	uuid, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000002")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7(experiment) setup error = %v, want nil", err)
	}
	identity, err := projectstandards.NewExperimentID(uuid)
	if err != nil {
		t.Fatalf("projectstandards.NewExperimentID() setup error = %v, want nil", err)
	}
	return identity
}

func workspaceIdentities(t testing.TB) (runnercontrol.SchedulingUnitIdentity, projectstandards.RunID) {
	t.Helper()
	uuid, uuidErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	run, runErr := projectstandards.NewRunID(uuid)
	if err := errors.Join(uuidErr, runErr); err != nil {
		t.Fatalf("workspace identity fixture error = %v, want nil", err)
	}
	return runnercontrol.SchedulingUnitIdentity{Kind: runnercontrol.SchedulingUnitRunPlan, Identity: uuid}, run
}

func mustWorkspaceAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	got, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) workspace fixture error = %v, want nil", value, err)
	}
	return got
}
