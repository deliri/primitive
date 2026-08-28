package hostfacts

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// externalIngressFuzzContract binds one external-input door to the fuzz target
// whose mutated input reaches that exact production boundary. A signature
// change or a deleted fuzz target therefore breaks compilation instead of
// silently reducing the package's hostile-input inventory.
type externalIngressFuzzContract[Door any] struct {
	Door Door
	Fuzz func(*testing.F)
}

var (
	_ = externalIngressFuzzContract[func(string) (Hostname, error)]{
		Door: admitHostname,
		Fuzz: FuzzHostnameIngressSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(string) (core.AbsolutePath, error)]{
		Door: admitObservedPath,
		Fuzz: FuzzObservedPathIngressSemanticClosure,
	}
	_ = externalIngressFuzzContract[func([]byte) (DiskRotation, error)]{
		Door: classifyRotationalFlag,
		Fuzz: FuzzRotationalFlagSemanticClosure,
	}
	_ = externalIngressFuzzContract[func([]byte) (cgroupMembership, error)]{
		Door: parseCgroupMembershipLine,
		Fuzz: FuzzCgroupMembershipLineSemanticClosure,
	}
	_ = externalIngressFuzzContract[func([]byte, WorkloadMemoryLimitSource) (cgroupMount, bool, error)]{
		Door: parseMountInfoLine,
		Fuzz: FuzzMountInfoLineSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(context.Context, core.AbsolutePath, WorkloadMemoryLimitSource) (uint64, bool, error)]{
		Door: readCgroupLimit,
		Fuzz: FuzzCgroupLimitFileSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(context.Context, GoOOMBannerRequest) (GoOOMBannerEvidence, error)]{
		Door: ClassifyGoOOMBanner,
		Fuzz: FuzzGoOOMBannerClassifier,
	}
	_ = externalIngressFuzzContract[func(*GoOOMBannerState, []byte) error]{
		Door: (*GoOOMBannerState).UnmarshalJSON,
		Fuzz: FuzzGoOOMBannerStateJSONSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(*GoOOMBannerEvidence, []byte) error]{
		Door: (*GoOOMBannerEvidence).UnmarshalJSON,
		Fuzz: FuzzGoOOMBannerEvidenceJSONSemanticClosure,
	}
)
