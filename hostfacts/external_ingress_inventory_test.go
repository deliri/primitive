package hostfacts

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
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
	_ = externalIngressFuzzContract[func([]byte) (process.Liveness, error)]{Door: parseSystemdScopePresence, Fuzz: FuzzSystemdScopePresenceSemanticClosure}
	_ = externalIngressFuzzContract[func([]byte) (resourceCPUCounters, error)]{Door: parseResourceCPU, Fuzz: FuzzResourceCPUSemanticClosure}
	_ = externalIngressFuzzContract[func(context.Context, io.Reader) (uint32, error)]{Door: parseResourcePressure, Fuzz: FuzzResourcePressureSemanticClosure}
	_ = externalIngressFuzzContract[func(context.Context, GoMemoryLimitRequest) (GoMemoryLimitResult, error)]{Door: ApplyGoMemoryLimit, Fuzz: FuzzGoMemoryLimitNativeSemanticClosure}
	_ = externalIngressFuzzContract[func(TimeZoneRequest) (*time.Location, error)]{Door: ObserveTimeZone, Fuzz: FuzzTimeZoneObservationSemanticClosure}
	_ = externalIngressFuzzContract[func(context.Context, core.AbsolutePath) (cgroupMembership, bool, error)]{Door: observeCgroupMembership, Fuzz: FuzzCgroupMembershipFileSemanticClosure}
	_ = externalIngressFuzzContract[func(uint8) (Percent, error)]{Door: NewPercent, Fuzz: FuzzPercentProjectionSemanticClosure}
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
	_ = externalIngressFuzzContract[func(context.Context, core.AbsolutePath, WorkloadMemoryLimitSource) (cgroupLevelLimit, error)]{
		Door: readCgroupLevelLimit,
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
