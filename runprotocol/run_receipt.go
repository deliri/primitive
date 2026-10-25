package runprotocol

import "github.com/deliri/primitive/v2026/temporal"

// RunReceipt is a producer's committed-source execution observation. It does
// not require a preceding application request or issue independent acceptance.
// Run, producer, caller and worker are attributed native identities; caller is
// not an authentication claim. Producers retain their exact invocation and
// native execution facts (commands, toolchains, times, exits, usage and raw
// artifact references) in the two referenced evidence objects.
//
// Results names a forward JSONIO object stream of TestResult occurrences.
// Zero observed results have ResultCount zero and no result artifact, never
// an invented passing test or a zero-byte ArtifactReference. Accounting
// describes declared execution units, not
// an inferred population of tests or subtests. Its existing bounded attempt
// sequence preserves failures before retries. This record owns no result list.
//
// Validate checks the local schema and contradictions only. Consumers must
// verify referenced bytes and their source/scope bindings before using them
// as proof. Working-source identities and package Hammer evidence belong to
// subsequent contracts; Source here identifies the exact committed tree.
type RunReceipt struct {
	Run         Identifier          `json:"run"`
	Producer    Identifier          `json:"producer"`
	Caller      Identifier          `json:"caller"`
	Worker      Identifier          `json:"worker"`
	Source      SourceCoordinate    `json:"source"`
	Invocation  ArtifactReference   `json:"invocation"`
	Execution   ArtifactReference   `json:"execution"`
	Results     *ArtifactReference  `json:"results,omitempty"`
	ResultCount uint64              `json:"result_count"`
	Accounting  ExecutionAccounting `json:"accounting"`
	CompletedAt temporal.Instant    `json:"completed_at"`
	Outcome     Outcome             `json:"outcome"`
	Terminal    TerminalState       `json:"terminal"`
}

func (r RunReceipt) Validate() error {
	if err := contractJoin(r.Run.Validate(), r.Producer.Validate(), r.Caller.Validate(), r.Worker.Validate(),
		r.Source.Validate(), r.Invocation.Validate(), r.Execution.Validate(),
		r.Accounting.Validate(), r.CompletedAt.Validate(), r.Outcome.Validate(), r.Terminal.Validate()); err != nil {
		return err
	}
	if (r.Results == nil) != (r.ResultCount == 0) {
		return conflictError(nil)
	}
	if r.Results != nil {
		if err := r.Results.Validate(); err != nil {
			return contractError(err)
		}
	}
	if !experimentTerminalMatches(r.Outcome, r.Terminal) {
		return conflictError(nil)
	}
	if r.Invocation.Path == r.Execution.Path || r.Results != nil && (r.Invocation.Path == r.Results.Path || r.Execution.Path == r.Results.Path) {
		return conflictError(nil)
	}
	latest, _ := r.Accounting.Latest() // Validate proved that at least one attempt exists.
	if !receiptOutcomeAgrees(r.Outcome, latest) {
		return conflictError(nil)
	}
	return nil
}

// receiptOutcomeAgrees requires the receipt's outcome to be explained by the
// latest attempt's own counts. Setup, infrastructure and non-accepting
// outcomes have no per-unit count; they require only that not everything passed.
func receiptOutcomeAgrees(outcome Outcome, latest ExecutionAttempt) bool {
	switch outcome {
	case OutcomePassed:
		return latest.Passed == latest.Planned
	case OutcomeFailed:
		return latest.Failed > 0
	case OutcomeSkipped:
		return latest.Skipped > 0 && latest.Passed == 0 && latest.Failed == 0
	case OutcomeUnavailable:
		return latest.Unavailable > 0
	case OutcomeTimedOut:
		return latest.Expired > 0
	case OutcomeCancelled:
		return latest.Cancelled > 0
	case OutcomeNotRun:
		return latest.NotRun > 0
	case OutcomeSetupFailed, OutcomeInfrastructureFailed, OutcomeNonAccepting:
		return latest.Passed != latest.Planned
	default:
		return false
	}
}

func (RunReceipt) runProtocolFact() {}
