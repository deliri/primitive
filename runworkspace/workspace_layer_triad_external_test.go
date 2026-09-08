package runworkspace_test

import (
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/runworkspace"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWorkspaceEffectLayerTriad(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		restrictUnit  bool
		restrictCache bool
	}{
		{name: "readable unit removes all owned experiment directories"},
		{name: "unreadable unit cannot borrow mode-change capability", restrictUnit: true},
		{name: "unreadable nested cache cannot borrow mode-change capability", restrictCache: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			resolved, resolveErr := manager.ResolveExperiment(member, experimentID)
			absoluteRoot, rootErr := manager.Absolute(experiment.Root)
			absoluteHome, homeErr := manager.Absolute(experiment.Home)
			absoluteOutput, outputErr := manager.Absolute(experiment.Output)
			absoluteCache, cacheErr := manager.Absolute(experiment.Cache)
			absoluteTemporary, temporaryErr := manager.Absolute(experiment.Temporary)
			environment, environmentErr := process.ParseExactEnvironment([]string{
				core.EnvironmentHomeName + "=" + absoluteHome.String(),
				core.EnvironmentTemporaryName + "=" + absoluteTemporary.String(),
				core.EnvironmentCacheName + "=" + absoluteCache.String(),
			})
			binding := runnercontrol.WritableWorkspace{
				Root: absoluteRoot, Home: absoluteHome, Output: absoluteOutput,
				Cache: absoluteCache, Temporary: absoluteTemporary,
			}
			bindingErr := manager.ValidateWritableWorkspace(experiment, binding, environment)
			if err := errors.Join(resolveErr, rootErr, homeErr, outputErr, cacheErr, temporaryErr, environmentErr, bindingErr); err != nil {
				t.Fatalf("workspace resolve/absolute/writable binding error = %v, want nil", err)
			}
			if resolved != experiment {
				t.Fatalf("Manager.ResolveExperiment() = %+v, want %+v", resolved, experiment)
			}
			if member.Run != run || experiment.Run != run || experiment.Identity != experimentID || experiment.Root == experiment.Home || experiment.Home == experiment.Output || experiment.Output == experiment.Cache || experiment.Cache == experiment.Temporary {
				t.Fatalf("CreateExperiment() = member %+v/experiment %+v, want exact identities and five distinct experiment-owned coordinates", member, experiment)
			}
			before, observeErr := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
			if observeErr != nil || before.Entries < 9 {
				t.Fatalf("Manager.Observe(populated) = (%+v, %v), want at least 9 workspace entries and nil", before, observeErr)
			}
			absoluteUnit, unitRootErr := manager.Absolute(unit.Root)
			if unitRootErr != nil {
				t.Fatalf("Manager.Absolute(unit root) error = %v, want nil", unitRootErr)
			}
			restricted := absoluteUnit
			if tc.restrictCache {
				restricted = absoluteCache
			}
			var wantErr error
			if tc.restrictUnit || tc.restrictCache {
				// Restore fixture permissions before TempDir cleanup, including on failure.
				t.Cleanup(func() {
					if err := os.Chmod(restricted.String(), 0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
						t.Errorf("restore fixture permissions = %v, want nil or removed entry", err)
					}
				})
				if err := os.Chmod(restricted.String(), 0o000); err != nil {
					t.Fatalf("restrict fixture permissions = %v, want nil", err)
				}
				// Go and the real OS decide whether this account can acquire the
				// directory. Windows and privileged accounts need not refuse 000.
				native, err := os.Open(restricted.String())
				if err != nil {
					if !errors.Is(err, os.ErrPermission) {
						t.Fatalf("native acquisition = %v, want permission refusal", err)
					}
					wantErr = os.ErrPermission
				} else if err := native.Close(); err != nil {
					t.Fatalf("native fixture close = %v, want nil", err)
				}
			}
			cleanupErr := manager.CleanupUnit(t.Context(), unit)
			if !errors.Is(cleanupErr, wantErr) {
				t.Fatalf("Manager.CleanupUnit() = %v, want native outcome %v", cleanupErr, wantErr)
			}
			if wantErr != nil {
				if !errors.Is(cleanupErr, core.ErrFilestoreActivation) {
					t.Fatalf("cleanup refusal = %v, want owning Filestore error identity", cleanupErr)
				}
				info, err := os.Stat(restricted.String())
				if err != nil || !info.IsDir() || info.Mode().Perm() != 0 {
					t.Fatalf("refused directory = %v/%v, want original directory with unchanged 000 permissions", info, err)
				}
				return
			}
			clean, cleanErr := manager.ProveClean(t.Context(), temporal.InstantFromNanoseconds(2), runworkspace.Residue{})
			if cleanErr != nil || !clean.Observation.IsClean() {
				t.Fatalf("Manager.ProveClean() = (%+v, %v), want exact zero residue and nil", clean, cleanErr)
			}
		})

	}

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
		unitID, run := workspaceIdentities(t)
		unit, unitErr := manager.CreateUnit(t.Context(), unitID)
		if unitErr != nil {
			t.Fatalf("Manager.CreateUnit() setup error = %v, want nil", unitErr)
		}
		member, memberErr := manager.CreateMember(t.Context(), unit, run)
		experiment, experimentErr := manager.CreateExperiment(t.Context(), member, workspaceExperimentIdentity(t))
		absoluteHome, homeErr := manager.Absolute(experiment.Home)
		absoluteOutput, outputErr := manager.Absolute(experiment.Output)
		absoluteCache, cacheErr := manager.Absolute(experiment.Cache)
		absoluteTemporary, temporaryErr := manager.Absolute(experiment.Temporary)
		environment, environmentErr := process.ParseExactEnvironment([]string{
			core.EnvironmentHomeName + "=" + absoluteHome.String(),
			core.EnvironmentTemporaryName + "=" + absoluteTemporary.String(),
			core.EnvironmentCacheName + "=" + absoluteCache.String(),
		})
		if err := errors.Join(memberErr, experimentErr, homeErr, outputErr, cacheErr, temporaryErr, environmentErr); err != nil {
			t.Fatalf("foreign workspace binding setup error = %v, want nil", err)
		}
		foreignBinding := runnercontrol.WritableWorkspace{
			Home: absoluteHome, Output: absoluteOutput, Cache: absoluteCache, Temporary: absoluteTemporary,
		}
		if gotErr := manager.ValidateWritableWorkspace(experiment, foreignBinding, environment); !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("Manager.ValidateWritableWorkspace(zero root) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
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
		if bindingErr := manager.ValidateWritableWorkspace(runworkspace.Experiment{}, runnercontrol.WritableWorkspace{}, process.Environment{}); !errors.Is(bindingErr, core.ErrPrimitiveContract) {
			t.Fatalf("Manager.ValidateWritableWorkspace(zero) error = %v, want errors.Is(..., %v)", bindingErr, core.ErrPrimitiveContract)
		}
		got, gotErr := manager.ProveClean(t.Context(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
		if gotErr != nil || got.Observation.Entries != 0 || !got.Observation.IsClean() {
			t.Fatalf("Manager.ProveClean(empty) = (%+v, %v), want zero clean observation and nil", got, gotErr)
		}
	})
}

func workspaceExperimentIdentity(t testing.TB) runprotocol.ExperimentID {
	t.Helper()
	uuid, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000002")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7(experiment) setup error = %v, want nil", err)
	}
	identity, err := runprotocol.NewExperimentID(uuid)
	if err != nil {
		t.Fatalf("runprotocol.NewExperimentID() setup error = %v, want nil", err)
	}
	return identity
}

func workspaceIdentities(t testing.TB) (runnercontrol.SchedulingUnitIdentity, runprotocol.RunID) {
	t.Helper()
	uuid, uuidErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	run, runErr := runprotocol.NewRunID(uuid)
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
