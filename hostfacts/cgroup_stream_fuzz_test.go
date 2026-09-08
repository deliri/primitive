package hostfacts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCgroupMembershipFileSemanticClosure(f *testing.F) {
	for _, membership := range []cgroupMembership{
		{path: "/unified", source: WorkloadMemoryLimitSourceCgroupV2},
		{path: "/legacy", source: WorkloadMemoryLimitSourceCgroupV1},
	} {
		if err := membership.Validate(); err != nil {
			f.Fatalf("membership seed = %v, want nil", err)
		}
		f.Add(canonicalMembershipLine(membership))
	}
	f.Add([]byte("0::/unified\n3:" + cgroupMemoryController + ":/legacy\n"))
	f.Add([]byte("3:" + cgroupMemoryController + ":/legacy\n0::/unified\n"))
	f.Add([]byte("0::/one\n0::/two\n"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		root := t.TempDir()
		file := filepath.Join(root, "membership")
		// One byte above the whole-file ceiling proves oversize refusal without
		// materializing arbitrary fuzz input on disk or in the oracle.
		data = data[:min(len(data), procCgroupMaximumBytes+1)]
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatalf("membership fixture = %v, want nil", err)
		}
		got, found, err := observeCgroupMembership(t.Context(), mustAbsolutePathForHostfactsTest(t, file))
		want, wantFound, wantErr := referenceMembershipFile(data)
		if !errors.Is(err, wantErr) || got != want || found != wantFound {
			t.Fatalf("membership file = %+v/%t/%v, want %+v/%t/%v", got, found, err, want, wantFound, wantErr)
		}
	})
}

func referenceMembershipFile(data []byte) (cgroupMembership, bool, error) {
	if len(data) > procCgroupMaximumBytes {
		return cgroupMembership{}, false, core.ErrHostFactsObservation
	}
	var candidates []cgroupMembership
	text := strings.TrimSuffix(string(data), "\n")
	if len(data) != 0 {
		for line := range strings.SplitSeq(text, "\n") {
			candidate, err := referenceCgroupMembership([]byte(line))
			if err != nil {
				return cgroupMembership{}, false, core.ErrHostFactsObservation
			}
			if candidate.source != WorkloadMemoryLimitSourceUnknown {
				for _, prior := range candidates {
					if prior.source == candidate.source {
						return cgroupMembership{}, false, core.ErrHostFactsObservation
					}
				}
				candidates = append(candidates, candidate)
			}
		}
	}
	// At most one fact from each version remains. The explicit v1 memory
	// attachment owns that controller; a unified hierarchy is the fallback.
	for _, source := range []WorkloadMemoryLimitSource{WorkloadMemoryLimitSourceCgroupV1, WorkloadMemoryLimitSourceCgroupV2} {
		for _, candidate := range candidates {
			if candidate.source == source {
				return candidate, true, nil
			}
		}
	}
	return cgroupMembership{}, false, nil
}
