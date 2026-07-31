package upgrade

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

// TrialTarget is the exact unselected executable a product may exercise.
type TrialTarget struct {
	directory core.AbsolutePath
	command   core.AbsolutePath
	path      core.RelativePath
	prior     selectionDocument
	candidate release.Artifact
	slot      Slot
	valid     bool
}

func newTrialTarget(
	directory core.AbsolutePath,
	prior selectionDocument,
	candidate release.Artifact,
) (TrialTarget, error) {
	if err := directory.Validate(); err != nil {
		return TrialTarget{}, contractError(diagnosticTrialTarget, err)
	}
	if err := prior.Validate(); err != nil {
		return TrialTarget{}, contractError(diagnosticTrialTarget, err)
	}
	if err := candidate.Validate(); err != nil {
		return TrialTarget{}, contractError(diagnosticTrialTarget, err)
	}
	slot, err := prior.Slot.other()
	if err != nil {
		return TrialTarget{}, err
	}
	path, err := binaryPath(slot, candidate.Build())
	if err != nil {
		return TrialTarget{}, err
	}
	command, err := absoluteBinaryPath(directory, slot, candidate.Build())
	if err != nil {
		return TrialTarget{}, err
	}
	value := TrialTarget{
		directory: directory, prior: prior, candidate: candidate,
		slot: slot, path: path, command: command, valid: true,
	}
	if err := value.Validate(); err != nil {
		return TrialTarget{}, err
	}
	return value, nil
}

// Validate proves every cached projection from the authenticated artifacts.
func (t TrialTarget) Validate() error {
	if !t.valid {
		return contractError(diagnosticTrialTarget)
	}
	if err := t.validateValues(); err != nil {
		return err
	}
	return t.validateProjection()
}

func (t TrialTarget) validateValues() error {
	if err := t.directory.Validate(); err != nil {
		return contractError(diagnosticTrialTarget, err)
	}
	if err := t.prior.Validate(); err != nil {
		return contractError(diagnosticTrialTarget, err)
	}
	if err := t.candidate.Validate(); err != nil {
		return contractError(diagnosticTrialTarget, err)
	}
	return validateUpgradePair(t.prior.Artifact, t.candidate)
}

func (t TrialTarget) validateProjection() error {
	slot, err := t.prior.Slot.other()
	if err != nil || slot != t.slot {
		return contractError(diagnosticTrialTarget, err)
	}
	path, err := binaryPath(t.slot, t.candidate.Build())
	if err != nil || path != t.path {
		return contractError(diagnosticTrialTarget, err)
	}
	command, err := absoluteBinaryPath(t.directory, t.slot, t.candidate.Build())
	if err != nil || command != t.command {
		return contractError(diagnosticTrialTarget, err)
	}
	return nil
}

// Candidate returns the exact candidate artifact.
func (t TrialTarget) Candidate() release.Artifact { return t.candidate }

// Path returns the exact root-relative executable path products must run.
func (t TrialTarget) Path() core.RelativePath { return t.path }

// Command returns the exact absolute path products pass to Process.
func (t TrialTarget) Command() core.AbsolutePath { return t.command }

// Directory returns the absolute directory named by the required os.Root.
func (t TrialTarget) Directory() core.AbsolutePath { return t.directory }

// TrialReport is a product-owned observation of the exact TrialTarget.
type TrialReport struct {
	Target      TrialTarget
	Observation temporal.Instant
	Observed    core.BuildIdentity
	Outcome     TrialOutcome
}

func (r TrialReport) Validate() error {
	if err := r.Target.Validate(); err != nil {
		return contractError(diagnosticTrialReport, err)
	}
	if err := r.Observed.Validate(); err != nil {
		return contractError(diagnosticTrialReport, err)
	}
	if err := r.Observation.Validate(); err != nil {
		return contractError(diagnosticTrialReport, err)
	}
	if err := r.Outcome.Validate(); err != nil {
		return contractError(diagnosticTrialReport, err)
	}
	if r.Observed != r.Target.candidate.Build() {
		return contractError(diagnosticObservedBuild)
	}
	return nil
}

// Promotion is authority to select one candidate that passed its product-owned
// trial. It does not claim Primitive executed the product test.
type Promotion struct {
	target      TrialTarget
	observation temporal.Instant
	valid       bool
}

func (p Promotion) Validate() error {
	if !p.valid {
		return contractError(diagnosticPromotion)
	}
	if err := p.target.Validate(); err != nil {
		return contractError(diagnosticPromotion, err)
	}
	if err := p.observation.Validate(); err != nil {
		return contractError(diagnosticPromotion, err)
	}
	return nil
}

// CompleteTrial admits only a passing report bound to the exact candidate.
func CompleteTrial(report TrialReport) (Promotion, error) {
	if err := report.Validate(); err != nil {
		return Promotion{}, err
	}
	if report.Outcome != TrialPassed {
		return Promotion{}, newAttemptError(
			FailurePhaseTrial, report.Observed, core.ErrUpgradeTrial,
		)
	}
	return Promotion{
		target: report.Target, observation: report.Observation, valid: true,
	}, nil
}

func validateUpgradePair(installed, candidate release.Artifact) error {
	if err := installed.Validate(); err != nil {
		return contractError(err)
	}
	if err := candidate.Validate(); err != nil {
		return contractError(err)
	}
	from, to := installed.Build(), candidate.Build()
	if from.Offering() != to.Offering() || from.Platform() != to.Platform() {
		return contractError(errors.New("candidate offering or platform differs from primary"))
	}
	order, err := from.Version().Compare(to.Version())
	if err != nil || order != core.ComparisonLess {
		return contractError(errors.New("candidate version is not newer"), err)
	}
	return nil
}
