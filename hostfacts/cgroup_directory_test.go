package hostfacts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The parent must exist before an absent interface can mean no declaration.
// Each refusal also proves that a real ancestor cannot rescue a vanished group.
func TestCgroupDirectoryDisappearanceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, provider := range []struct {
		name, component, unlimited string
		source                     WorkloadMemoryLimitSource
	}{
		{name: "v2", component: cgroupV2LimitName, unlimited: cgroupV2MaxToken, source: WorkloadMemoryLimitSourceCgroupV2},
		{name: "v1", component: cgroupV1LimitName, unlimited: strconv.FormatUint(cgroupV1UnlimitedMin, 10), source: WorkloadMemoryLimitSourceCgroupV1},
	} {
		for _, tc := range []struct {
			name, remove, rootValue string
			wantState               WorkloadMemoryLimitState
			wantErr                 error
		}{
			{name: "existing directories without declarations stay unavailable", wantState: WorkloadMemoryLimitUnavailable},
			{name: "existing leaf without interface inherits finite ancestor", rootValue: "400", wantState: WorkloadMemoryLimitLimited},
			{name: "existing leaf without interface inherits unlimited ancestor", rootValue: provider.unlimited, wantState: WorkloadMemoryLimitUnlimited},
			{name: "missing leaf cannot inherit finite ancestor", remove: "team/job", rootValue: "400", wantErr: fs.ErrNotExist},
			{name: "missing leaf cannot inherit unlimited ancestor", remove: "team/job", rootValue: provider.unlimited, wantErr: fs.ErrNotExist},
			{name: "missing intermediate cannot inherit finite root", remove: "team", rootValue: "400", wantErr: fs.ErrNotExist},
			{name: "missing intermediate cannot inherit unlimited root", remove: "team", rootValue: provider.unlimited, wantErr: fs.ErrNotExist},
			{name: "missing mount cannot become unavailable", remove: ".", wantErr: fs.ErrNotExist},
		} {
			t.Run(provider.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				root := filepath.Join(t.TempDir(), "mount")
				writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{root: root, limitName: provider.component, rootValue: tc.rootValue})
				if tc.remove != "" {
					if err := os.RemoveAll(filepath.Join(root, tc.remove)); err != nil {
						t.Fatal(err)
					}
				}
				got, err := foldCgroupLimits(t.Context(), cgroupMembership{path: "/team/job", source: provider.source}, cgroupMount{root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: provider.source})
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("fold = %+v/%v, want %v", got, err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (WorkloadMemoryLimit{}) {
						t.Fatalf("refused fold = %+v, want zero", got)
					}
					return
				}
				limit, present := got.LimitBytes()
				path, pathPresent := got.InterfacePath()
				if got.Validate() != nil || got.State() != tc.wantState || present != (tc.wantState == WorkloadMemoryLimitLimited) || (present && limit.Uint64() != 400) || pathPresent != (tc.rootValue != "") || (pathPresent && path.String() != filepath.Join(root, provider.component)) {
					t.Fatalf("fold = %+v, limit=%v/%t path=%v/%t, want exact ancestor fact in state %v", got, limit, present, path, pathPresent, tc.wantState)
				}
			})
		}
	}
}

func BenchmarkCgroupExistingHierarchy(b *testing.B) {
	root := b.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "team", "job"), 0700); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "team", cgroupV2LimitName), []byte("400\n"), 0600); err != nil {
		b.Fatal(err)
	}
	membership := cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}
	rootPath, err := core.ParseAbsolutePath(root)
	if err != nil {
		b.Fatal(err)
	}
	mount := cgroupMount{root: "/", mountPoint: rootPath, source: WorkloadMemoryLimitSourceCgroupV2}
	b.ReportAllocs()
	for b.Loop() {
		got, err := foldCgroupLimits(b.Context(), membership, mount)
		limit, present := got.LimitBytes()
		if err != nil || !present || limit.Uint64() != 400 || got.State() != WorkloadMemoryLimitLimited {
			b.Fatalf("fold = %+v/%v, want finite 400", got, err)
		}
	}
}
