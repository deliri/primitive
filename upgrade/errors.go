package upgrade

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

type diagnostic uint8

const (
	diagnosticUnknown diagnostic = iota
	diagnosticSlot
	diagnosticTrialOutcome
	diagnosticFailurePhase
	diagnosticJSON
	diagnosticSelection
	diagnosticRoot
	diagnosticPrimary
	diagnosticTrialTarget
	diagnosticTrialReport
	diagnosticPromotion
	diagnosticObservedBuild
	diagnosticCandidateBytes
	diagnosticCurrentSelection
	diagnosticTrialDocument
	diagnosticActiveTrial
	diagnosticLimit
)

var diagnosticText = [diagnosticLimit]string{
	diagnosticUnknown:          "unknown upgrade rejection",
	diagnosticSlot:             "slot is outside the closed domain",
	diagnosticTrialOutcome:     "trial outcome is outside the closed domain",
	diagnosticFailurePhase:     "failure phase is outside the closed domain",
	diagnosticJSON:             "upgrade document violates its canonical JSON contract",
	diagnosticSelection:        "primary selection is invalid",
	diagnosticRoot:             "root does not name the declared upgrade directory",
	diagnosticPrimary:          "primary is invalid",
	diagnosticTrialTarget:      "trial target is invalid",
	diagnosticTrialReport:      "trial report is invalid",
	diagnosticPromotion:        "promotion is invalid",
	diagnosticObservedBuild:    "trial observed a build other than the exact candidate",
	diagnosticCandidateBytes:   "installed bytes differ from the authenticated artifact",
	diagnosticCurrentSelection: "current primary differs from the promotion authority",
	diagnosticTrialDocument:    "durable trial receipt is invalid",
	diagnosticActiveTrial:      "another exact candidate still owns the trial slot",
}

func (d diagnostic) Error() string {
	if d <= diagnosticUnknown || d >= diagnosticLimit {
		return diagnosticText[diagnosticUnknown]
	}
	return diagnosticText[d]
}

func contractError(causes ...error) error {
	return upgradeError(core.ErrUpgradeContract, causes...)
}

func verificationError(causes ...error) error {
	return upgradeError(core.ErrUpgradeVerification, causes...)
}

func persistenceError(causes ...error) error {
	return upgradeError(core.ErrUpgradePersistence, causes...)
}

func cleanupError(causes ...error) error {
	return upgradeError(core.ErrUpgradeCleanup, causes...)
}

func conflictError(causes ...error) error {
	return upgradeError(core.ErrUpgradeConflict, causes...)
}

func upgradeError(identity error, causes ...error) error {
	all := append([]error{identity}, causes...)
	return fmt.Errorf("upgrade: %w", errors.Join(all...))
}

// AttemptError is the typed failure record returned for one candidate attempt.
// Products own durable recording, customer copy, and ticket submission.
type AttemptError struct {
	cause     error
	candidate core.BuildIdentity
	phase     FailurePhase
}

func newAttemptError(
	phase FailurePhase,
	candidate core.BuildIdentity,
	identity error,
	causes ...error,
) error {
	return AttemptError{
		phase: phase, candidate: candidate,
		cause: upgradeError(identity, causes...),
	}
}

// Error returns a bounded diagnostic without rendering download authority.
func (e AttemptError) Error() string {
	return fmt.Sprintf(
		"upgrade: %s attempt failed for %s %s %s %s",
		e.phase.String(),
		e.candidate.Offering().String(),
		e.candidate.Version().String(),
		e.candidate.Platform().String(),
		e.candidate.Commit().String(),
	)
}

// Unwrap preserves the stable Core identity and native cause.
func (e AttemptError) Unwrap() error { return e.cause }

// Phase returns the operation boundary that failed.
func (e AttemptError) Phase() FailurePhase { return e.phase }

// Candidate returns the exact candidate named by the failed attempt.
func (e AttemptError) Candidate() core.BuildIdentity { return e.candidate }

// Validate proves the record carries a phase, candidate, and Upgrade identity.
func (e AttemptError) Validate() error {
	if err := e.phase.Validate(); err != nil {
		return contractError(err)
	}
	if err := e.candidate.Validate(); err != nil {
		return contractError(err)
	}
	if e.cause == nil || !failurePhaseAccepts(e.phase, e.cause) {
		return contractError(diagnosticFailurePhase)
	}
	return nil
}

func failurePhaseAccepts(phase FailurePhase, cause error) bool {
	identity := failurePhasePrimaryIdentity(phase)
	if identity != core.ErrUnknown && errors.Is(cause, identity) {
		return true
	}
	return (phase == FailurePhasePromotion ||
		phase == FailurePhasePersistence ||
		phase == FailurePhaseCleanup) &&
		errors.Is(cause, core.ErrUpgradeConflict)
}

func failurePhasePrimaryIdentity(phase FailurePhase) core.ErrorIdentity {
	switch phase {
	case FailurePhaseBootstrap:
		return core.ErrUpgradePersistence
	case FailurePhaseCapacity:
		return core.ErrUpgradeCapacity
	case FailurePhaseDownload:
		return core.ErrUpgradeDownload
	case FailurePhaseVerification:
		return core.ErrUpgradeVerification
	case FailurePhaseTrial:
		return core.ErrUpgradeTrial
	case FailurePhasePromotion:
		return core.ErrUpgradePromotion
	case FailurePhasePersistence:
		return core.ErrUpgradePersistence
	case FailurePhaseCleanup:
		return core.ErrUpgradeCleanup
	default:
		return core.ErrUnknown
	}
}
