//go:build linux

package hostfacts

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestObserveEffectiveWorkloadMemoryLimitNativeCgroup(t *testing.T) {
	t.Parallel()

	got, gotErr := ObserveEffectiveWorkloadMemoryLimit(context.Background())
	if gotErr != nil {
		t.Fatalf("ObserveEffectiveWorkloadMemoryLimit() error = %v, want nil on native Linux", gotErr)
	}
	if gotErr := got.Validate(); gotErr != nil {
		t.Fatalf("ObserveEffectiveWorkloadMemoryLimit().Validate() error = %v, want nil", gotErr)
	}
	path, present := got.InterfacePath()
	limit, limitPresent := got.LimitBytes()
	switch got.State() {
	case WorkloadMemoryLimitLimited, WorkloadMemoryLimitUnlimited:
		if !present {
			t.Fatalf("ObserveEffectiveWorkloadMemoryLimit().InterfacePath() present = false for state %v, want true", got.State())
		}
		if _, gotErr := os.Stat(path.String()); gotErr != nil {
			t.Fatalf("os.Stat(%s) error = %v, want observed interface to remain present", path.String(), gotErr)
		}
	case WorkloadMemoryLimitUnavailable:
		if present || path != (core.AbsolutePath{}) {
			t.Fatalf(
				"ObserveEffectiveWorkloadMemoryLimit().InterfacePath() = (%v, %t), want unset for unavailable state",
				path,
				present,
			)
		}
	default:
		t.Fatalf("ObserveEffectiveWorkloadMemoryLimit().State() = %v, want Linux available, unlimited, or limited", got.State())
	}
	t.Logf(
		"native cgroup observation: state=%v source=%v limit=%d/%t path=%s/%t",
		got.State(),
		got.Source(),
		limit.Uint64(),
		limitPresent,
		path.String(),
		present,
	)

	rootData, rootErr := os.ReadFile("/sys/fs/cgroup/memory.max")
	if rootErr != nil {
		return
	}
	rootToken := strings.TrimSuffix(string(rootData), "\n")
	if rootToken == cgroupV2MaxToken {
		return
	}
	wantLimit, parseErr := strconv.ParseUint(rootToken, 10, 64)
	if parseErr != nil {
		t.Fatalf("strconv.ParseUint(native root memory.max %q) error = %v, want canonical finite value", rootToken, parseErr)
	}
	if got.State() != WorkloadMemoryLimitLimited ||
		!limitPresent ||
		limit.Uint64() != wantLimit {
		t.Fatalf(
			"ObserveEffectiveWorkloadMemoryLimit() = state %v limit %d/%t, want native root limit %d",
			got.State(),
			limit.Uint64(),
			limitPresent,
			wantLimit,
		)
	}
}
