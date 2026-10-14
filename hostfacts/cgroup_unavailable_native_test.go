package hostfacts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Owned native files exercise the exact production acquisition, parsing, mount
// selection and ancestor fold. They are proc/cgroup fixtures, not claims that
// these directories are actual kernel cgroup filesystems.
func TestCgroupUnavailableNativeObservationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, membership, mounts string
		wantState                WorkloadMemoryLimitState
		wantErr                  error
	}{
		{"visible finite v2", "0::/job\n", "v2", WorkloadMemoryLimitLimited, nil},
		{"visible finite v1", "5:memory:/job\n", "v1", WorkloadMemoryLimitLimited, nil},
		{"v2 filesystem hidden", "0::/job\n", "none", WorkloadMemoryLimitUnavailable, nil},
		{"v1 filesystem hidden", "5:memory:/job\n", "none", WorkloadMemoryLimitUnavailable, nil},
		{"hybrid memory owner hidden", "0::/job\n5:memory:/job\n", "v2", WorkloadMemoryLimitUnavailable, nil},
		{"v2 path outside namespace", "0::/../..\n", "v2", WorkloadMemoryLimitUnavailable, nil},
		{"v1 path outside namespace", "5:memory:/../..\n", "v1", WorkloadMemoryLimitUnavailable, nil},
		{"hybrid memory owner outside namespace", "0::/job\n5:memory:/../..\n", "v2", WorkloadMemoryLimitUnavailable, nil},
		{"no memory membership", "2:cpu:/job\n", "v2", WorkloadMemoryLimitUnavailable, nil},
		{"missing mountinfo source", "0::/job\n", "missing", WorkloadMemoryLimitUnknown, fs.ErrNotExist},
		{"malformed membership", "0::/a/../b\n", "v2", WorkloadMemoryLimitUnknown, core.ErrHostFactsObservation},
		{"malformed mountinfo", "0::/job\n", "broken", WorkloadMemoryLimitUnknown, core.ErrHostFactsObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			group := filepath.Join(dir, "cgroup")
			if err := os.MkdirAll(filepath.Join(group, "job"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"memory.max", "memory.limit_in_bytes"} {
				if err := os.WriteFile(filepath.Join(group, "job", name), []byte("8192\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			membership := mustAbsolutePathForHostfactsTest(t, filepath.Join(dir, "cgroup-membership"))
			mounts := mustAbsolutePathForHostfactsTest(t, filepath.Join(dir, "mountinfo"))
			if err := os.WriteFile(membership.String(), []byte(tc.membership), 0600); err != nil {
				t.Fatal(err)
			}
			var row string
			switch tc.mounts {
			case "v2":
				row = fmt.Sprintf("30 23 0:27 / %s rw - cgroup2 cgroup rw\n", group)
			case "v1":
				row = fmt.Sprintf("30 23 0:27 / %s rw - cgroup cgroup rw,memory\n", group)
			case "none":
				row = "30 23 0:27 / /unrelated rw - ext4 /dev/disk rw\n"
			case "broken":
				row = "truncated\n"
			case "missing":
			default:
				t.Fatalf("unowned mount fixture %q", tc.mounts)
			}
			if tc.mounts != "missing" {
				if err := os.WriteFile(mounts.String(), []byte(row), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := observeWorkloadMemoryAt(t.Context(), membership, mounts)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (WorkloadMemoryLimit{}) {
					t.Fatalf("native observation = %+v/%v, want zero/%v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.state != tc.wantState || got.Validate() != nil {
				t.Fatalf("native observation = %+v/%v, want state %v", got, err, tc.wantState)
			}
			if tc.wantState == WorkloadMemoryLimitLimited && got.limit.Uint64() != 8192 {
				t.Fatalf("finite limit = %d, want 8192", got.limit.Uint64())
			}
		})
	}
}
