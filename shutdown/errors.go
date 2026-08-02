package shutdown

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	emptyPanicDiagnostic        = "empty value"
	panicDiagnosticMaximumRunes = 256
)

type diagnostic uint8

const (
	diagnosticUnknown diagnostic = iota
	diagnosticPhaseUnsupported
	diagnosticStepOutcomeUnsupported
	diagnosticSignalKindUnsupported
	diagnosticSignalSetUnsupported
	diagnosticSecondSignalUnsupported
	diagnosticGraceExpiryUnsupported
	diagnosticEscalationReasonUnsupported
	diagnosticStepIdentityZero
	diagnosticStepBudgetInvalid
	diagnosticStepActionNil
	diagnosticTotalBudgetInvalid
	diagnosticPlanNil
	diagnosticPlanRegistrationClosed
	diagnosticPlanFull
	diagnosticStepIdentityDuplicate
	diagnosticPlanStateUnset
	diagnosticCompletedStepFailure
	diagnosticStepResultIdentityMismatch
	diagnosticReportUnset
	diagnosticReportCount
	diagnosticReportResult
	diagnosticParentContext
	diagnosticTotalBudgetConstruction
	diagnosticPlanAlreadyRun
	diagnosticPlanCount
	diagnosticStepSkipped
	diagnosticGracePeriodInvalid
	diagnosticGracePeriodCoupling
	diagnosticEscalationUnset
	diagnosticGraceEscalationTrigger
	diagnosticSignalCauseUnauthentic
	diagnosticSignalProjectionEmpty
	diagnosticSignalSourceIncomplete
	diagnosticGraceProjection
	diagnosticControllerNil
	diagnosticSignalSourceClosed
	diagnosticPanicErrorInvalid
	diagnosticLimit
)

func diagnosticLabels() [diagnosticLimit]string {
	return [...]string{
		diagnosticPhaseUnsupported:            "phase is not a supported cleanup phase",
		diagnosticStepOutcomeUnsupported:      "step outcome is not a supported outcome",
		diagnosticSignalKindUnsupported:       "signal kind is not a supported signal",
		diagnosticSignalSetUnsupported:        "signal set is not a supported platform set",
		diagnosticSecondSignalUnsupported:     "second signal action is not a supported action",
		diagnosticGraceExpiryUnsupported:      "grace expiry action is not a supported action",
		diagnosticEscalationReasonUnsupported: "escalation reason is not a supported reason",
		diagnosticStepIdentityZero:            "step identity is zero",
		diagnosticStepBudgetInvalid:           "step budget is not a positive duration",
		diagnosticStepActionNil:               "step action is nil",
		diagnosticTotalBudgetInvalid:          "total budget is not a positive duration",
		diagnosticPlanNil:                     "plan is nil",
		diagnosticPlanRegistrationClosed:      "plan is no longer open for registration",
		diagnosticPlanFull:                    "plan already holds its maximum step count",
		diagnosticStepIdentityDuplicate:       "step identity duplicates a registered step",
		diagnosticPlanStateUnset:              "plan state is unset",
		diagnosticCompletedStepFailure:        "completed step result carries a failure",
		diagnosticStepResultIdentityMismatch:  "step result failure does not carry its outcome error identity",
		diagnosticReportUnset:                 "report is unset",
		diagnosticReportCount:                 "report count exceeds its fixed capacity",
		diagnosticReportResult:                "report retains an invalid step result",
		diagnosticParentContext:               "parent context is unusable",
		diagnosticTotalBudgetConstruction:     "total budget could not bound the run",
		diagnosticPlanAlreadyRun:              "plan has already started its single run",
		diagnosticPlanCount:                   "plan count exceeds its fixed capacity",
		diagnosticStepSkipped:                 "step was skipped before it started",
		diagnosticGracePeriodInvalid:          "grace period is not a valid duration",
		diagnosticGracePeriodCoupling:         "grace period is set if and only if grace expiry escalates",
		diagnosticEscalationUnset:             "escalation is unset",
		diagnosticGraceEscalationTrigger:      "grace expiry escalation names a trigger signal",
		diagnosticSignalCauseUnauthentic:      "signal cause was not produced by an observation",
		diagnosticSignalProjectionEmpty:       "signal set projects no platform signal",
		diagnosticSignalSourceIncomplete:      "signal source is incomplete",
		diagnosticGraceProjection:             "grace period has no standard projection",
		diagnosticControllerNil:               "controller is nil",
		diagnosticSignalSourceClosed:          "signal source closed before any supported signal",
		diagnosticPanicErrorInvalid:           "step panic error is invalid",
	}
}

func (d diagnostic) Error() string {
	labels := diagnosticLabels()
	if d > diagnosticUnknown && d < diagnosticLimit && labels[d] != "" {
		return labels[d]
	}
	return core.UnknownEnumDiagnostic
}

func contractError(detail diagnostic, causes ...error) error {
	return shutdownError(core.ErrShutdownContract, append([]error{detail}, causes...)...)
}

func stepFailureError(causes ...error) error {
	return shutdownError(core.ErrShutdownStepFailure, causes...)
}

func stepTimeoutError(causes ...error) error {
	return shutdownError(core.ErrShutdownStepTimeout, causes...)
}

func totalTimeoutError(causes ...error) error {
	return shutdownError(core.ErrShutdownTotalTimeout, causes...)
}

func signalSourceError(causes ...error) error {
	return shutdownError(core.ErrShutdownSignalSource, causes...)
}

func shutdownError(identity error, causes ...error) error {
	all := append([]error{identity}, causes...)
	return fmt.Errorf("shutdown: %w", errors.Join(all...))
}

// StepPanicError preserves an error-valued panic through Unwrap while keeping
// Error safe and its non-error diagnostic bounded.
type StepPanicError struct {
	cause      error
	diagnostic string
}

func (e StepPanicError) Error() string {
	return "shutdown: step action panicked: " + e.diagnostic
}

func (e StepPanicError) Unwrap() error { return e.cause }

func (e StepPanicError) Diagnostic() string { return e.diagnostic }

func (e StepPanicError) Validate() error {
	if e.diagnostic == "" ||
		utf8.RuneCountInString(e.diagnostic) > panicDiagnosticMaximumRunes ||
		!errors.Is(e.cause, core.ErrShutdownStepPanic) {
		return contractError(diagnosticPanicErrorInvalid)
	}
	return nil
}

func newStepPanicError(cause error, detail string) StepPanicError {
	return StepPanicError{
		cause:      errors.Join(core.ErrShutdownStepPanic, cause),
		diagnostic: boundedPanicDiagnostic(detail),
	}
}

func newNativeStepPanicError(native error) StepPanicError {
	return newStepPanicError(native, fmt.Sprintf("error value of type %T", native))
}

func newNonErrorStepPanicError(detail string) StepPanicError {
	cause := error(core.ErrShutdownStepPanic)
	return StepPanicError{cause: cause, diagnostic: boundedPanicDiagnostic(detail)}
}

func boundedPanicDiagnostic(value string) string {
	var output strings.Builder
	output.Grow(min(len(value), panicDiagnosticMaximumRunes*utf8.UTFMax))
	count := 0
	for _, character := range value {
		if count == panicDiagnosticMaximumRunes {
			break
		}
		output.WriteRune(character)
		count++
	}
	if output.Len() == 0 {
		return emptyPanicDiagnostic
	}
	return output.String()
}

func boundedPanicBytes(value []byte) string {
	var output strings.Builder
	output.Grow(min(len(value), panicDiagnosticMaximumRunes*utf8.UTFMax))
	for count := 0; count < panicDiagnosticMaximumRunes && len(value) > 0; count++ {
		character, width := utf8.DecodeRune(value)
		output.WriteRune(character)
		value = value[width:]
	}
	if output.Len() == 0 {
		return emptyPanicDiagnostic
	}
	return output.String()
}
