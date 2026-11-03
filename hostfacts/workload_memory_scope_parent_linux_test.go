//go:build linux

package hostfacts

import (
	"strings"
	"testing"
)

// This queries the actual user manager but creates no scope or reservation.
// The managed test's own transient scope is not the future job's parent.
func TestWorkloadMemoryScopeParentNativeUserManager(t *testing.T) {
	t.Parallel()
	got, err := ObserveWorkloadMemoryScopeParent(t.Context())
	if err != nil || got.Validate() != nil {
		t.Fatalf("native scope parent=%+v/%v, want validated observation", got, err)
	}
	if got.BindingLimit().State() != WorkloadMemoryLimitLimited {
		t.Fatalf("native binding state=%s, want finite managed parent", got.BindingLimit().State())
	}
	path, present := got.BindingLimit().InterfacePath()
	if !present || strings.Contains(path.String(), "/run-p") {
		t.Fatalf("native parent binding=%s/%t, want parent ancestry outside this managed job", path, present)
	}
	self, err := ObserveEffectiveWorkloadMemoryLimit(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	selfLimit, selfFinite := self.LimitBytes()
	parentLimit, parentFinite := got.BindingLimit().LimitBytes()
	if !selfFinite || !parentFinite || parentLimit.Uint64() < selfLimit.Uint64() {
		t.Fatalf("parent/self limits=%d/%d finite=%t/%t, want parent capacity not below descendant effective limit", parentLimit.Uint64(), selfLimit.Uint64(), parentFinite, selfFinite)
	}
}
