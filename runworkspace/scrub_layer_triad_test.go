package runworkspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runworkspace"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestManagerScrubLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive populated run parent is removed and proven clean", func(t *testing.T) {
		t.Parallel()
		manager := openScrubManager(t)
		defer closeScrubManager(t, manager)
		unit := createScrubUnit(t, manager)

		before, err := manager.Observe(context.Background(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
		if err != nil {
			t.Fatalf("Manager.Observe(before scrub) error = %v, want nil", err)
		}
		if before.Entries == 0 {
			t.Fatalf("Manager.Observe(before scrub).Entries = %d, want non-zero after CreateUnit(%v)", before.Entries, unit.Identity)
		}
		if err := manager.Scrub(context.Background()); err != nil {
			t.Fatalf("Manager.Scrub(populated run parent) error = %v, want nil", err)
		}
		clean, err := manager.ProveClean(context.Background(), temporal.InstantFromNanoseconds(2), runworkspace.Residue{})
		if err != nil {
			t.Fatalf("Manager.ProveClean(after scrub) error = %v, want nil", err)
		}
		if !clean.Observation.IsClean() {
			t.Fatalf("Manager.ProveClean(after scrub).Observation = %+v, want clean machine state", clean.Observation)
		}
	})

	t.Run("negative cancelled scrub preserves cancellation identity", func(t *testing.T) {
		t.Parallel()
		manager := openScrubManager(t)
		defer closeScrubManager(t, manager)
		createScrubUnit(t, manager)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		gotErr := manager.Scrub(ctx)
		if !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("Manager.Scrub(cancelled context) error = %v, want errors.Is %v", gotErr, context.Canceled)
		}
	})

	t.Run("neutral empty run parent remains clean without fabricated residue", func(t *testing.T) {
		t.Parallel()
		manager := openScrubManager(t)
		defer closeScrubManager(t, manager)

		if err := manager.Scrub(context.Background()); err != nil {
			t.Fatalf("Manager.Scrub(empty run parent) error = %v, want nil", err)
		}
		clean, err := manager.ProveClean(context.Background(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
		if err != nil {
			t.Fatalf("Manager.ProveClean(empty run parent) error = %v, want nil", err)
		}
		if got := clean.Observation.Entries; got != 0 {
			t.Fatalf("clean machine entry count = %d, want 0 after neutral scrub", got)
		}
	})
}

func TestMachineResidueProofRejectsEveryOwnedHostResource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		residue runworkspace.Residue
	}{
		{name: "subject process still exists", residue: runworkspace.Residue{Processes: 1}},
		{name: "transient cgroup still exists", residue: runworkspace.Residue{ControlGroups: 1}},
		{name: "subject namespace still exists", residue: runworkspace.Residue{Namespaces: 1}},
		{name: "isolated mount still exists", residue: runworkspace.Residue{Mounts: 1}},
		{name: "owned descriptor remains open", residue: runworkspace.Residue{Descriptors: 1}},
		{name: "subject socket still exists", residue: runworkspace.Residue{Sockets: 1}},
		{name: "credential custody remains active", residue: runworkspace.Residue{CredentialCustody: 1}},
		{name: "secret custody remains active", residue: runworkspace.Residue{SecretCustody: 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			manager := openScrubManager(t)
			defer closeScrubManager(t, manager)
			got, gotErr := manager.ProveClean(t.Context(), temporal.InstantFromNanoseconds(1), tc.residue)
			if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Observation.IsClean() {
				t.Fatalf("Manager.ProveClean(%s) = (%+v, %v), want non-clean zero proof and errors.Is(..., %v)", tc.name, got, gotErr, core.ErrPrimitiveContract)
			}
		})
	}
}

func openScrubManager(t *testing.T) runworkspace.Manager {
	t.Helper()
	root, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(t.TempDir()) setup error = %v, want nil", err)
	}
	manager, err := runworkspace.Open(context.Background(), runworkspace.Configuration{RunParent: root})
	if err != nil {
		t.Fatalf("runworkspace.Open() setup error = %v, want nil", err)
	}
	return manager
}

func createScrubUnit(t *testing.T, manager runworkspace.Manager) runworkspace.Unit {
	t.Helper()
	uuid, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7() setup error = %v, want nil", err)
	}
	unit, err := manager.CreateUnit(context.Background(), runnercontrol.SchedulingUnitIdentity{Kind: runnercontrol.SchedulingUnitRunPlan, Identity: uuid})
	if err != nil {
		t.Fatalf("Manager.CreateUnit() setup error = %v, want nil", err)
	}
	return unit
}

func closeScrubManager(t *testing.T, manager runworkspace.Manager) {
	t.Helper()
	if err := manager.Close(); err != nil {
		t.Fatalf("Manager.Close() cleanup error = %v, want nil", err)
	}
}
