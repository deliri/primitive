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

func phaseLabels() [phaseLimit]string {
	return [phaseLimit]string{
		PhaseUnknown:       unknownLabel,
		PhaseStopAdmission: "stop-admission",
		PhaseDrain:         "drain",
		PhasePersist:       "persist",
		PhaseFlush:         "flush",
		PhaseRelease:       releaseLabel,
	}
}

func (p Phase) IsValid() bool {
	labels := phaseLabels()
	return p > PhaseUnknown && p < phaseLimit && labels[p] != ""
}

func (p Phase) Validate() error {
	if !p.IsValid() {
		return contractError(diagnosticPhaseUnsupported)
	}
	return nil
}

func (p Phase) String() string {
	labels := phaseLabels()
	if p < phaseLimit && labels[p] != "" {
		return labels[p]
	}
	return unknownLabel
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

func stepOutcomeLabels() [stepOutcomeLimit]string {
	return [stepOutcomeLimit]string{
		StepOutcomeUnknown:             unknownLabel,
		StepOutcomeCompleted:           "completed",
		StepOutcomeFailed:              "failed",
		StepOutcomeTimedOut:            "timed-out",
		StepOutcomePanicked:            "panicked",
		StepOutcomeTotalBudgetExceeded: "total-budget-exceeded",
	}
}

func (o StepOutcome) IsValid() bool {
	labels := stepOutcomeLabels()
	return o > StepOutcomeUnknown && o < stepOutcomeLimit && labels[o] != ""
}

func (o StepOutcome) Validate() error {
	if !o.IsValid() {
		return contractError(diagnosticStepOutcomeUnsupported)
	}
	return nil
}

func (o StepOutcome) String() string {
	labels := stepOutcomeLabels()
	if o < stepOutcomeLimit && labels[o] != "" {
		return labels[o]
	}
	return unknownLabel
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

func signalKindLabels() [signalKindLimit]string {
	return [signalKindLimit]string{
		SignalKindUnknown:   unknownLabel,
		SignalKindInterrupt: "interrupt",
		SignalKindTerminate: "terminate",
		SignalKindHangup:    "hangup",
	}
}

func (k SignalKind) IsValid() bool {
	labels := signalKindLabels()
	return k > SignalKindUnknown && k < signalKindLimit && labels[k] != ""
}

func (k SignalKind) Validate() error {
	if !k.IsValid() {
		return contractError(diagnosticSignalKindUnsupported)
	}
	return nil
}

func (k SignalKind) String() string {
	labels := signalKindLabels()
	if k < signalKindLimit && labels[k] != "" {
		return labels[k]
	}
	return unknownLabel
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

func signalSetLabels() [signalSetLimit]string {
	return [signalSetLimit]string{
		SignalSetUnknown:           unknownLabel,
		SignalSetInteractive:       "interactive",
		SignalSetStandard:          "standard",
		SignalSetTerminalLifecycle: "terminal-lifecycle",
	}
}

func (s SignalSet) IsValid() bool {
	labels := signalSetLabels()
	return s > SignalSetUnknown && s < signalSetLimit && labels[s] != ""
}

func (s SignalSet) Validate() error {
	if !s.IsValid() {
		return contractError(diagnosticSignalSetUnsupported)
	}
	return nil
}

func (s SignalSet) String() string {
	labels := signalSetLabels()
	if s < signalSetLimit && labels[s] != "" {
		return labels[s]
	}
	return unknownLabel
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

func secondSignalActionLabels() [secondSignalActionLimit]string {
	return [secondSignalActionLimit]string{
		SecondSignalActionUnknown: unknownLabel,
		SecondSignalRelease:       releaseLabel,
		SecondSignalEscalate:      escalateLabel,
	}
}

func (a SecondSignalAction) IsValid() bool {
	labels := secondSignalActionLabels()
	return a > SecondSignalActionUnknown &&
		a < secondSignalActionLimit &&
		labels[a] != ""
}

func (a SecondSignalAction) Validate() error {
	if !a.IsValid() {
		return contractError(diagnosticSecondSignalUnsupported)
	}
	return nil
}

func (a SecondSignalAction) String() string {
	labels := secondSignalActionLabels()
	if a < secondSignalActionLimit && labels[a] != "" {
		return labels[a]
	}
	return unknownLabel
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

func graceExpiryActionLabels() [graceExpiryActionLimit]string {
	return [graceExpiryActionLimit]string{
		GraceExpiryActionUnknown: unknownLabel,
		GraceExpiryDisabled:      "disabled",
		GraceExpiryEscalate:      escalateLabel,
	}
}

func (a GraceExpiryAction) IsValid() bool {
	labels := graceExpiryActionLabels()
	return a > GraceExpiryActionUnknown &&
		a < graceExpiryActionLimit &&
		labels[a] != ""
}

func (a GraceExpiryAction) Validate() error {
	if !a.IsValid() {
		return contractError(diagnosticGraceExpiryUnsupported)
	}
	return nil
}

func (a GraceExpiryAction) String() string {
	labels := graceExpiryActionLabels()
	if a < graceExpiryActionLimit && labels[a] != "" {
		return labels[a]
	}
	return unknownLabel
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

func escalationReasonLabels() [escalationReasonLimit]string {
	return [escalationReasonLimit]string{
		EscalationReasonUnknown: unknownLabel,
		EscalationSecondSignal:  "second-signal",
		EscalationGraceExpired:  "grace-expired",
	}
}

func (r EscalationReason) IsValid() bool {
	labels := escalationReasonLabels()
	return r > EscalationReasonUnknown &&
		r < escalationReasonLimit &&
		labels[r] != ""
}

func (r EscalationReason) Validate() error {
	if !r.IsValid() {
		return contractError(diagnosticEscalationReasonUnsupported)
	}
	return nil
}

func (r EscalationReason) String() string {
	labels := escalationReasonLabels()
	if r < escalationReasonLimit && labels[r] != "" {
		return labels[r]
	}
	return unknownLabel
}

func (EscalationReason) OffWireEnum() {}
