package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"runtime"
	"runtime/debug"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// AssessDisk observes capacity through one held directory capability and
// classifies it against caller policy.
func AssessDisk(ctx context.Context, request DiskAssessmentRequest) (DiskAssessment, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return DiskAssessment{}, err
	}
	if err := request.Validate(); err != nil {
		return DiskAssessment{}, err
	}
	root, err := openRoot(request.Directory.String())
	if err != nil {
		return DiskAssessment{}, failRootOpen(OperationOpenRoot, diskOpenIdentity(), err)
	}
	capacity, observeErr := root.diskCapacity()
	closeErr := root.close()
	if err := errors.Join(observeErr, closeErr); err != nil {
		identity := core.ErrHostFactsObservation
		if errors.Is(observeErr, core.ErrDiskCapacityUnsupported) {
			identity = core.ErrDiskCapacityUnsupported
		}
		return DiskAssessment{}, fail(OperationDiskCapacity, identity, err)
	}
	return assessDiskCapacity(capacity, request.Policy)
}

// ObserveDiskRotation reports whether the block device backing one held
// directory is rotational.
//
// The backing device's identity comes from the same held capability every
// other disk observation opens, and on Linux the answer comes from the
// kernel's own block-device index, so no mount-table text or device-name
// heuristic ever decides which disk a directory lives on. A directory no
// single block device backs answers unavailable; an operating system with
// no portable rotation interface answers unsupported after validating the
// directory the same way. Both are observations a caller records, never
// errors to swallow.
func ObserveDiskRotation(ctx context.Context, request DiskRotationRequest) (DiskRotation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return DiskRotationUnknown, err
	}
	if err := request.Validate(); err != nil {
		return DiskRotationUnknown, err
	}
	rotation, err := observeDiskRotation(ctx, request.Directory)
	if err != nil {
		return DiskRotationUnknown, err
	}
	return rotation, rotation.Validate()
}

// ObserveTerminalGeometry reports whether one open descriptor is attached to
// a terminal, and the terminal's column count when it is.
//
// Detachment is an observation rather than a failure: a renderer deciding how
// wide to draw needs "you are piped" as an answer, not an error to swallow.
// The request is refused when the descriptor cannot be interrogated at all,
// because the caller must not record a detachment nobody observed.
func ObserveTerminalGeometry(request TerminalGeometryRequest) (TerminalGeometry, error) {
	if err := request.Validate(); err != nil {
		return TerminalGeometry{}, err
	}
	return observedTerminalGeometry(request.File)
}

// AssessGoMemory observes the exact Go soft-limit accounting metric and
// classifies it against caller policy. Its runtime.ReadMemStats call briefly
// stops all application goroutines to obtain an up-to-date snapshot.
func AssessGoMemory(request GoMemoryAssessmentRequest) (GoMemoryAssessment, error) {
	if err := request.Validate(); err != nil {
		return GoMemoryAssessment{}, err
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	snapshot, err := newGoMemorySnapshot(stats.Sys, stats.HeapReleased, debug.SetMemoryLimit(-1))
	if err != nil {
		return GoMemoryAssessment{}, fail(OperationGoMemory, core.ErrHostFactsObservation, err)
	}
	return assessGoMemorySnapshot(snapshot, request.Policy)
}

// ObserveEffectiveWorkloadMemoryLimit observes the current Linux cgroup
// membership and folds finite memory ceilings from that cgroup through its
// mounted ancestors. Other operating systems return an Unsupported result.
func ObserveEffectiveWorkloadMemoryLimit(ctx context.Context) (WorkloadMemoryLimit, error) {
	return observeEffectiveWorkloadMemoryLimit(ctx)
}

// MeasureTree measures logical regular-file extent and count beneath one held
// root without following links, reparse points, or crossing volumes.
func MeasureTree(ctx context.Context, request TreeUsageRequest) (TreeUsage, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return TreeUsage{}, err
	}
	if err := request.Validate(); err != nil {
		return TreeUsage{}, err
	}
	root, err := openRoot(request.Root.String())
	if errors.Is(err, fs.ErrNotExist) && request.MissingPolicy == MissingPathIsEmpty {
		usage, closeErr := (treeAccumulator{}).close()
		if closeErr != nil {
			return TreeUsage{}, closeErr
		}
		return usage, usage.Validate()
	}
	if err != nil {
		return TreeUsage{}, failRootOpen(OperationOpenRoot, treeOpenIdentity(), err)
	}
	usage, walkErr := walkTree(ctx, root)
	if err := errors.Join(walkErr, root.close()); err != nil {
		identity := core.ErrHostFactsObservation
		if errors.Is(walkErr, core.ErrTreeMeasurementUnsupported) {
			identity = core.ErrTreeMeasurementUnsupported
		}
		return TreeUsage{}, fail(OperationTreeWalk, identity, err)
	}
	return usage, usage.Validate()
}

// ClassifyGoOOMBanner consumes exactly the declared bounded extent and reports
// only canonical Go runtime OOM banner presence.
func ClassifyGoOOMBanner(ctx context.Context, request GoOOMBannerRequest) (GoOOMBannerEvidence, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return GoOOMBannerEvidence{}, err
	}
	if err := request.Validate(); err != nil {
		return GoOOMBannerEvidence{}, err
	}
	scanner := oomScanner{
		source: request.Source, matcher: newBannerMatcher(), remaining: request.Length.Uint64(),
	}
	for scanner.remaining > 0 {
		if err := scanner.read(ctx); err != nil {
			return GoOOMBannerEvidence{}, fail(OperationGoOOMBanner, core.ErrHostFactsObservation, err)
		}
	}
	state := GoOOMBannerAbsent
	if scanner.matcher.found {
		state = GoOOMBannerPresent
	}
	evidence := GoOOMBannerEvidence{examined: request.Length, state: state}
	return evidence, evidence.Validate()
}
