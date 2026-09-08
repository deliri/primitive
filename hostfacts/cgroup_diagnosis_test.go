package hostfacts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCgroupMembershipRecheckDiagnosisTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		missing    bool
		wantErr    error
	}{
		{name: "unchanged membership survives recheck", text: "0::/team/job\n"},
		{name: "path move cannot masquerade as read failure", text: "0::/other/job\n", wantErr: core.ErrCgroupMembershipChanged},
		{name: "controller move cannot retain old limit", text: "3:memory:/team/job\n", wantErr: core.ErrCgroupMembershipChanged},
		{name: "empty membership has disappearance identity", wantErr: core.ErrCgroupMembershipDisappeared},
		{name: "unrelated controller cannot preserve memory membership", text: "2:cpu:/team/job\n", wantErr: core.ErrCgroupMembershipDisappeared},
		{name: "duplicate v2 has duplicate identity", text: "0::/team/job\n0::/team/job\n", wantErr: core.ErrCgroupMembershipDuplicate},
		{name: "duplicate v1 cannot be hidden by v2", text: "3:memory:/one\n4:memory:/two\n0::/team/job\n", wantErr: core.ErrCgroupMembershipDuplicate},
		{name: "missing proc file retains native cause", missing: true, wantErr: fs.ErrNotExist},
		{name: "malformed proc file remains observation refusal", text: "malformed\n", wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "membership")
			if !tc.missing {
				if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}
			err := recheckCgroupMembership(t.Context(), mustAbsolutePathForHostfactsTest(t, path), before)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("recheck = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				failure := fail(OperationCgroupMembership, core.ErrHostFactsObservation, err)
				var detail Failure
				if !errors.As(failure, &detail) || detail.Cause == nil || detail.Validate() != nil || !errors.Is(detail.Cause, tc.wantErr) {
					t.Fatalf("failure = %+v, want retained typed diagnosis", detail)
				}
			}
		})
	}
}

func TestCgroupMountSelectionDiagnosisTable(t *testing.T) {
	t.Parallel()
	const rootMount = "30 23 0:27 / /mount rw - " + cgroupV2Filesystem + " cgroup rw\n"
	const nestedMount = "31 23 0:27 /team /nested rw - " + cgroupV2Filesystem + " cgroup rw\n"
	for _, tc := range []struct {
		name, text, wantPath string
		wantErr              error
	}{
		{name: "single root mount remains exact", text: rootMount, wantPath: "/mount"},
		{name: "more specific mount owns subtree", text: rootMount + nestedMount, wantPath: "/nested"},
		{name: "absent mount has missing identity", wantErr: core.ErrCgroupMountMissing},
		{name: "equal specific mounts have ambiguous identity", text: rootMount + rootMount, wantErr: core.ErrCgroupMountAmbiguous},
		{name: "nested ambiguity cannot fall back to root", text: nestedMount + rootMount + nestedMount, wantErr: core.ErrCgroupMountAmbiguous},
		{name: "malformed trailing line cannot be ignored", text: rootMount + "malformed\n", wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(t.TempDir(), "mounts")
			if err := os.WriteFile(file, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := selectCgroupMount(t.Context(), mustAbsolutePathForHostfactsTest(t, file), cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("mount = %+v/%v, want %v", got, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (cgroupMount{}) {
					t.Fatalf("refusal leaked mount %+v", got)
				}
				return
			}
			if got.Validate() != nil || got.mountPoint.String() != tc.wantPath || got.source != WorkloadMemoryLimitSourceCgroupV2 {
				t.Fatalf("mount = %+v, want exact path %s and v2 source", got, tc.wantPath)
			}
		})
	}
}
