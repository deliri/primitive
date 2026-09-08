package hostfacts

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCgroupMembershipSelectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text, path string
		source           WorkloadMemoryLimitSource
		wantErr          error
	}{
		{name: "empty membership stays unavailable"},
		{name: "unrelated controller cannot manufacture memory", text: "2:cpu:/cpu\n"},
		{name: "v2 alone selects its exact group", text: "0::/unified\n", path: "/unified", source: WorkloadMemoryLimitSourceCgroupV2},
		{name: "v1 memory alone selects its exact group", text: "3:memory:/legacy\n", path: "/legacy", source: WorkloadMemoryLimitSourceCgroupV1},
		{name: "hybrid v1 memory owns the controller after v2", text: "0::/unified\n3:memory:/legacy\n", path: "/legacy", source: WorkloadMemoryLimitSourceCgroupV1},
		{name: "hybrid v1 memory owns the controller before v2", text: "3:memory:/legacy\n0::/unified\n", path: "/legacy", source: WorkloadMemoryLimitSourceCgroupV1},
		{name: "unrelated v1 does not displace v2", text: "2:cpu:/cpu\n0::/unified\n", path: "/unified", source: WorkloadMemoryLimitSourceCgroupV2},
		{name: "duplicate v2 cannot become a unique membership", text: "0::/one\n0::/two\n", wantErr: core.ErrHostFactsObservation},
		{name: "duplicate v1 cannot become a unique controller", text: "3:memory:/one\n4:memory:/two\n", wantErr: core.ErrHostFactsObservation},
		{name: "good controller cannot mask malformed later line", text: "3:memory:/legacy\nbroken\n", wantErr: core.ErrHostFactsObservation},
		{name: "irrelevant controller cannot mask malformed earlier line", text: "broken\n2:cpu:/cpu\n", wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			file := filepath.Join(root, "membership")
			if err := os.WriteFile(file, []byte(tc.text), 0600); err != nil {
				t.Fatalf("write membership = %v, want nil", err)
			}
			got, found, err := observeCgroupMembership(t.Context(), mustAbsolutePathForHostfactsTest(t, file))
			want := cgroupMembership{path: tc.path, source: tc.source}
			if !errors.Is(err, tc.wantErr) || got != want || found != (tc.path != "") {
				t.Fatalf("membership = %+v/%t/%v, want %+v/%t/%v", got, found, err, want, tc.path != "", tc.wantErr)
			}
		})
	}
}

func TestCgroupVersionAndContainmentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, root    string
		source, mountSource WorkloadMemoryLimitSource
		wantErr             error
	}{
		{name: "root membership resolves exact mount", path: "/", root: "/", source: WorkloadMemoryLimitSourceCgroupV2, mountSource: WorkloadMemoryLimitSourceCgroupV2},
		{name: "nested membership resolves bounded child", path: "/tenant/job", root: "/tenant", source: WorkloadMemoryLimitSourceCgroupV1, mountSource: WorkloadMemoryLimitSourceCgroupV1},
		{name: "source mismatch cannot cross controller worlds", path: "/tenant/job", root: "/tenant", source: WorkloadMemoryLimitSourceCgroupV1, mountSource: WorkloadMemoryLimitSourceCgroupV2, wantErr: core.ErrHostFactsObservation},
		{name: "parent traversal cannot escape mounted subtree", path: "/../../escape", root: "/", source: WorkloadMemoryLimitSourceCgroupV2, mountSource: WorkloadMemoryLimitSourceCgroupV2, wantErr: core.ErrHostFactsObservation},
		{name: "relative membership cannot become a mounted path", path: "relative", root: "/", source: WorkloadMemoryLimitSourceCgroupV2, mountSource: WorkloadMemoryLimitSourceCgroupV2, wantErr: core.ErrHostFactsObservation},
		{name: "empty membership cannot become mount root", root: "/", source: WorkloadMemoryLimitSourceCgroupV2, mountSource: WorkloadMemoryLimitSourceCgroupV2, wantErr: core.ErrHostFactsObservation},
		{name: "future membership source cannot select a file", path: "/", root: "/", source: WorkloadMemoryLimitSource(math.MaxUint8), mountSource: WorkloadMemoryLimitSourceCgroupV2, wantErr: core.ErrHostFactsObservation},
		{name: "root prefix sibling cannot count as containment", path: "/tenant-other/job", root: "/tenant", source: WorkloadMemoryLimitSourceCgroupV2, mountSource: WorkloadMemoryLimitSourceCgroupV2, wantErr: core.ErrCgroupContainment},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tc.wantErr == nil {
				if err := os.MkdirAll(filepath.Join(root, "job"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			mount := cgroupMount{root: tc.root, mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: tc.mountSource}
			membership := cgroupMembership{path: tc.path, source: tc.source}
			got, err := foldCgroupLimits(t.Context(), membership, mount)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("fold = %+v/%v, want %v", got, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (WorkloadMemoryLimit{}) {
					t.Fatalf("refused fold = %+v, want zero", got)
				}
				return
			}
			if got.State() != WorkloadMemoryLimitUnavailable || got.Source() != WorkloadMemoryLimitSourceNone || got.Validate() != nil {
				t.Fatalf("empty hierarchy = %+v, want exact unavailable fact", got)
			}
		})
	}
}
