package hostfacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCgroupDuplicateRootsUseFirstNativeMount(t *testing.T) {
	t.Parallel()
	for _, duplicates := range []int{1, 2, 255, 256, 257, 511} {
		t.Run(fmt.Sprintf("same-root-mounts-%d", duplicates), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			first := mustAbsolutePathForHostfactsTest(t, filepath.Join(dir, "first"))
			other := mustAbsolutePathForHostfactsTest(t, filepath.Join(dir, "other"))
			firstRow := fmt.Sprintf("30 23 0:27 /tenant %s rw - cgroup2 cgroup rw\n", first.String())
			otherRow := fmt.Sprintf("31 23 0:27 /tenant %s rw - cgroup2 cgroup rw\n", other.String())
			mounts := mustAbsolutePathForHostfactsTest(t, filepath.Join(dir, "mountinfo"))
			if err := os.WriteFile(mounts.String(), []byte(firstRow+strings.Repeat(otherRow, duplicates-1)), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := selectCgroupMount(t.Context(), mounts, cgroupMembership{path: "/tenant/job", source: WorkloadMemoryLimitSourceCgroupV2})
			if err != nil || got.mountPoint != first {
				t.Fatalf("same-root mounts = %+v/%v, want first mount %v without ambiguity", got, err, first)
			}
		})
	}
}
