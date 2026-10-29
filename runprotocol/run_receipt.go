package runprotocol

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const RunReceiptSchemaVersion uint16 = 2

// RunReceipt retains compact observations after joined scratch is discarded.
// Facts binds typed records, never raw output or a provider location.
type RunReceipt struct {
	SchemaVersion uint16           `json:"schema_version"`
	Run           Identifier       `json:"run"`
	Producer      Identifier       `json:"producer"`
	Caller        Identifier       `json:"caller"`
	Kind          RunKind          `json:"kind"`
	Source        RunSource        `json:"source"`
	Scope         RunScope         `json:"scope"`
	Worker        *RunWorker       `json:"worker,omitempty"`
	Budget        *RunBudget       `json:"budget,omitempty"`
	Seed          *RunShuffleSeed  `json:"seed,omitempty"`
	StartedAt     temporal.Instant `json:"started_at"`
	CompletedAt   temporal.Instant `json:"completed_at"`
	Timing        RunTiming        `json:"timing"`
	Summary       RunSummary       `json:"summary"`
	Facts         RunFactSequence  `json:"facts"`
	Retry         *RunRetry        `json:"retry,omitempty"`
	Outcome       Outcome          `json:"outcome"`
	Terminal      TerminalState    `json:"terminal"`
}

func (r RunReceipt) Validate() error {
	if r.SchemaVersion != RunReceiptSchemaVersion {
		return core.ErrRunProtocolContract
	}
	if err := errors.Join(r.Run.Validate(), r.Producer.Validate(), r.Caller.Validate(), r.Kind.Validate(), r.Source.Validate(), r.Scope.Validate(), r.StartedAt.Validate(), r.CompletedAt.Validate(), r.Timing.Validate(), r.Summary.Validate(), r.Facts.Validate(), r.Outcome.Validate(), r.Terminal.Validate()); err != nil {
		return contractError(err)
	}
	if _, err := r.CompletedAt.Since(r.StartedAt); err != nil {
		return conflictError(err)
	}
	if !experimentTerminalMatches(r.Outcome, r.Terminal) {
		return conflictError(nil)
	}
	if r.Worker != nil {
		if err := r.Worker.Validate(); err != nil {
			return err
		}
	}
	if r.Budget != nil {
		if err := r.Budget.Validate(); err != nil {
			return err
		}
	}
	if r.Worker != nil && r.Budget != nil && (uint32(r.Budget.CPU) > r.Worker.LogicalCPUs || r.Budget.MemoryBytes.Uint64() > r.Worker.TotalMemoryBytes.Uint64() || r.Budget.ScratchBytes.Uint64() > r.Worker.ScratchTotalBytes.Uint64()) {
		return conflictError(nil)
	}
	if r.Seed != nil {
		if err := r.Seed.Validate(); err != nil {
			return err
		}
	}
	if r.Retry != nil {
		if err := r.Retry.Validate(); err != nil {
			return err
		}
		if r.Retry.Run == r.Run {
			return conflictError(nil)
		}
	}
	if r.Kind != RunKindBenchmark && r.Summary.Benchmarks != nil || r.Kind != RunKindFuzz && r.Summary.Fuzz != nil {
		return conflictError(nil)
	}
	if r.Summary.ExitCode != nil && (r.Source.Captured == nil || r.Worker == nil || r.Budget == nil) {
		return conflictError(nil)
	}
	switch r.Outcome {
	case OutcomePassed:
		if r.Summary.ExitCode == nil || *r.Summary.ExitCode != core.ProcessExitCodeSuccess || r.Summary.Tests.Failed != 0 || r.Summary.Packages.Failed != 0 || r.Summary.BuildFailed || !r.Summary.StreamDecoded {
			return conflictError(nil)
		}
		switch r.Kind {
		case RunKindTest:
			if r.Summary.Tests.Passed == 0 {
				return conflictError(nil)
			}
		case RunKindBenchmark:
			if r.Summary.Benchmarks == nil || r.Summary.Benchmarks.Measured == 0 {
				return conflictError(nil)
			}
		case RunKindFuzz:
			if r.Summary.Fuzz == nil || r.Summary.Fuzz.Executions == nil || *r.Summary.Fuzz.Executions == 0 {
				return conflictError(nil)
			}
		default:
			return core.ErrRunProtocolContract
		}
	case OutcomeFailed:
		if r.Summary.ExitCode == nil || *r.Summary.ExitCode == core.ProcessExitCodeSuccess {
			return conflictError(nil)
		}
	case OutcomeSkipped, OutcomeUnavailable, OutcomeTimedOut, OutcomeCancelled, OutcomeNotRun, OutcomeSetupFailed, OutcomeInfrastructureFailed, OutcomeNonAccepting:
	default:
		return core.ErrRunProtocolContract
	}
	return nil
}
func (RunReceipt) runProtocolFact() {}
