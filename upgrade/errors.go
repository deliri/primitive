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

func diagnosticTexts() [diagnosticLimit]string {
	return [...]string{
		"unknown upgrade rejection",
		"slot is outside the closed domain",
		"trial outcome is outside the closed domain",
		"failure phase is outside the closed domain",
		"upgrade document violates its canonical JSON contract",
		"primary selection is invalid",
		"root does not name the declared upgrade directory",
		"primary is invalid",
		"trial target is invalid",
		"trial report is invalid",
		"promotion is invalid",
		"trial observed a build other than the exact candidate",
		"installed bytes differ from the authenticated artifact",
		"current primary differs from the promotion authority",
		"durable trial receipt is invalid",
		"another exact candidate still owns the trial slot",
	}
}

func (d diagnostic) Error() string {
	if d <= diagnosticUnknown || d >= diagnosticLimit {
		return diagnosticTexts()[diagnosticUnknown]
	}
	return diagnosticTexts()[d]
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

type attemptErrorRequest struct {
	identity  error
	candidate core.BuildIdentity
	phase     FailurePhase
}

func newAttemptError(request attemptErrorRequest, causes ...error) error {
	return AttemptError{
		phase: request.phase, candidate: request.candidate,
		cause: upgradeError(request.identity, causes...),
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
