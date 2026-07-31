package shutdown

const (
	escalateLabel = "escalate"
	releaseLabel  = "release"
	unknownLabel  = "unknown"
)

// Phase is a closed cleanup phase executed in declaration order.
type Phase uint8

const (
	PhaseUnknown Phase = iota
	PhaseStopAdmission
	PhaseDrain
	PhasePersist
	PhaseFlush
	PhaseRelease
	phaseLimit
)

var phaseLabels = [...]string{
	unknownLabel,
	"stop-admission",
	"drain",
	"persist",
	"flush",
	releaseLabel,
}

var (
	_ [int(phaseLimit) - len(phaseLabels)]struct{}
	_ [len(phaseLabels) - int(phaseLimit)]struct{}
)

func (p Phase) IsValid() bool { return p > PhaseUnknown && p < phaseLimit }

func (p Phase) Validate() error {
	if !p.IsValid() {
		return contractError(diagnosticPhaseUnsupported)
	}
	return nil
}

func (p Phase) String() string {
	if p >= phaseLimit {
		return unknownLabel
	}
	return phaseLabels[p]
}

func (Phase) OffWireEnum() {}

// StepOutcome classifies one cleanup step.
type StepOutcome uint8

const (
	StepOutcomeUnknown StepOutcome = iota
	StepOutcomeCompleted
	StepOutcomeFailed
	StepOutcomeTimedOut
	StepOutcomePanicked
	StepOutcomeTotalBudgetExceeded
	stepOutcomeLimit
)

var stepOutcomeLabels = [...]string{
	unknownLabel,
	"completed",
	"failed",
	"timed-out",
	"panicked",
	"total-budget-exceeded",
}

var (
	_ [int(stepOutcomeLimit) - len(stepOutcomeLabels)]struct{}
	_ [len(stepOutcomeLabels) - int(stepOutcomeLimit)]struct{}
)

func (o StepOutcome) IsValid() bool {
	return o > StepOutcomeUnknown && o < stepOutcomeLimit
}

func (o StepOutcome) Validate() error {
	if !o.IsValid() {
		return contractError(diagnosticStepOutcomeUnsupported)
	}
	return nil
}

func (o StepOutcome) String() string {
	if o >= stepOutcomeLimit {
		return unknownLabel
	}
	return stepOutcomeLabels[o]
}

func (StepOutcome) OffWireEnum() {}

// SignalKind is a supported operating-system termination signal.
type SignalKind uint8

const (
	SignalKindUnknown SignalKind = iota
	SignalKindInterrupt
	SignalKindTerminate
	SignalKindHangup
	signalKindLimit
)

var signalKindLabels = [...]string{
	unknownLabel,
	"interrupt",
	"terminate",
	"hangup",
}

var (
	_ [int(signalKindLimit) - len(signalKindLabels)]struct{}
	_ [len(signalKindLabels) - int(signalKindLimit)]struct{}
)

func (k SignalKind) IsValid() bool {
	return k > SignalKindUnknown && k < signalKindLimit
}

func (k SignalKind) Validate() error {
	if !k.IsValid() {
		return contractError(diagnosticSignalKindUnsupported)
	}
	return nil
}

func (k SignalKind) String() string {
	if k >= signalKindLimit {
		return unknownLabel
	}
	return signalKindLabels[k]
}

func (SignalKind) OffWireEnum() {}

// SignalSet selects one closed platform projection.
//
// The projection is exact per platform, not uniform across platforms. Windows
// has no SIGTERM or SIGHUP, so every set registers os.Interrupt alone there.
// A caller that needs terminate or hangup semantics gets interrupt on Windows.
type SignalSet uint8

const (
	SignalSetUnknown SignalSet = iota
	SignalSetInteractive
	SignalSetStandard
	SignalSetTerminalLifecycle
	signalSetLimit
)

var signalSetLabels = [...]string{
	unknownLabel,
	"interactive",
	"standard",
	"terminal-lifecycle",
}

var (
	_ [int(signalSetLimit) - len(signalSetLabels)]struct{}
	_ [len(signalSetLabels) - int(signalSetLimit)]struct{}
)

func (s SignalSet) IsValid() bool {
	return s > SignalSetUnknown && s < signalSetLimit
}

func (s SignalSet) Validate() error {
	if !s.IsValid() {
		return contractError(diagnosticSignalSetUnsupported)
	}
	return nil
}

func (s SignalSet) String() string {
	if s >= signalSetLimit {
		return unknownLabel
	}
	return signalSetLabels[s]
}

func (SignalSet) OffWireEnum() {}

// SecondSignalAction controls observation after the first supported signal.
type SecondSignalAction uint8

const (
	SecondSignalActionUnknown SecondSignalAction = iota
	SecondSignalRelease
	SecondSignalEscalate
	secondSignalActionLimit
)

var secondSignalActionLabels = [...]string{
	unknownLabel,
	releaseLabel,
	escalateLabel,
}

var (
	_ [int(secondSignalActionLimit) - len(secondSignalActionLabels)]struct{}
	_ [len(secondSignalActionLabels) - int(secondSignalActionLimit)]struct{}
)

func (a SecondSignalAction) IsValid() bool {
	return a > SecondSignalActionUnknown && a < secondSignalActionLimit
}

func (a SecondSignalAction) Validate() error {
	if !a.IsValid() {
		return contractError(diagnosticSecondSignalUnsupported)
	}
	return nil
}

func (a SecondSignalAction) String() string {
	if a >= secondSignalActionLimit {
		return unknownLabel
	}
	return secondSignalActionLabels[a]
}

func (SecondSignalAction) OffWireEnum() {}

// GraceExpiryAction controls whether grace expiry reports escalation.
type GraceExpiryAction uint8

const (
	GraceExpiryActionUnknown GraceExpiryAction = iota
	GraceExpiryDisabled
	GraceExpiryEscalate
	graceExpiryActionLimit
)

var graceExpiryActionLabels = [...]string{
	unknownLabel,
	"disabled",
	escalateLabel,
}

var (
	_ [int(graceExpiryActionLimit) - len(graceExpiryActionLabels)]struct{}
	_ [len(graceExpiryActionLabels) - int(graceExpiryActionLimit)]struct{}
)

func (a GraceExpiryAction) IsValid() bool {
	return a > GraceExpiryActionUnknown && a < graceExpiryActionLimit
}

func (a GraceExpiryAction) Validate() error {
	if !a.IsValid() {
		return contractError(diagnosticGraceExpiryUnsupported)
	}
	return nil
}

func (a GraceExpiryAction) String() string {
	if a >= graceExpiryActionLimit {
		return unknownLabel
	}
	return graceExpiryActionLabels[a]
}

func (GraceExpiryAction) OffWireEnum() {}

// EscalationReason identifies why observation requested composition-root action.
type EscalationReason uint8

const (
	EscalationReasonUnknown EscalationReason = iota
	EscalationSecondSignal
	EscalationGraceExpired
	escalationReasonLimit
)

var escalationReasonLabels = [...]string{
	unknownLabel,
	"second-signal",
	"grace-expired",
}

var (
	_ [int(escalationReasonLimit) - len(escalationReasonLabels)]struct{}
	_ [len(escalationReasonLabels) - int(escalationReasonLimit)]struct{}
)

func (r EscalationReason) IsValid() bool {
	return r > EscalationReasonUnknown && r < escalationReasonLimit
}

func (r EscalationReason) Validate() error {
	if !r.IsValid() {
		return contractError(diagnosticEscalationReasonUnsupported)
	}
	return nil
}

func (r EscalationReason) String() string {
	if r >= escalationReasonLimit {
		return unknownLabel
	}
	return escalationReasonLabels[r]
}

func (EscalationReason) OffWireEnum() {}
