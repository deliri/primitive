package shutdown

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// MaximumSteps is the fixed cleanup-plan capacity.
const MaximumSteps = 64

// StepID is a caller-owned nonzero cleanup identity.
type StepID struct {
	value uint16
}

// NewStepID constructs one cleanup identity.
func NewStepID(value uint16) (StepID, error) {
	id := StepID{value: value}
	if err := id.Validate(); err != nil {
		return StepID{}, err
	}
	return id, nil
}

func (id StepID) Validate() error {
	if id.value == 0 {
		return contractError(diagnosticStepIdentityZero)
	}
	return nil
}

func (id StepID) String() string {
	if id.Validate() != nil {
		return core.UnknownEnumDiagnostic
	}
	return strconv.FormatUint(uint64(id.value), 10)
}

// StepAction performs one cooperative cleanup operation.
type StepAction func(context.Context) error

// Step is one phased, independently bounded cleanup operation.
type Step struct {
	Action StepAction
	Budget temporal.Duration
	ID     StepID
	Phase  Phase
}

func (s Step) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}
	if err := s.Phase.Validate(); err != nil {
		return err
	}
	if err := s.Budget.Validate(); err != nil || s.Budget.IsZero() {
		return contractError(diagnosticStepBudgetInvalid, err)
	}
	if s.Action == nil {
		return contractError(diagnosticStepActionNil)
	}
	return nil
}

// PlanPolicy bounds the complete cleanup run.
type PlanPolicy struct {
	TotalBudget temporal.Duration
}

func (p PlanPolicy) Validate() error {
	if err := p.TotalBudget.Validate(); err != nil || p.TotalBudget.IsZero() {
		return contractError(diagnosticTotalBudgetInvalid, err)
	}
	return nil
}

// Plan owns one fixed-capacity cleanup registration and run.
type Plan struct {
	steps   [MaximumSteps]Step
	policy  PlanPolicy
	mu      sync.Mutex
	started atomic.Bool
	count   uint8
}

// NewPlan constructs an empty cleanup plan.
func NewPlan(policy PlanPolicy) (*Plan, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Plan{policy: policy}, nil
}

// Register adds one step while the plan is open.
func (p *Plan) Register(step Step) error {
	if p == nil {
		return contractError(diagnosticPlanNil)
	}
	if err := step.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started.Load() {
		return contractError(diagnosticPlanRegistrationClosed)
	}
	if int(p.count) > len(p.steps) {
		return contractError(diagnosticPlanCount)
	}
	if int(p.count) == len(p.steps) {
		return contractError(diagnosticPlanFull)
	}
	if p.hasID(step.ID) {
		return contractError(diagnosticStepIdentityDuplicate)
	}
	p.steps[p.count] = step
	p.count++
	return nil
}

func (p *Plan) hasID(id StepID) bool {
	for index := uint8(0); index < p.count; index++ {
		if p.steps[index].ID == id {
			return true
		}
	}
	return false
}

// Validate checks the plan's owned structure.
func (p *Plan) Validate() error {
	if p == nil {
		return contractError(diagnosticPlanNil)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.validateLocked()
}

func (p *Plan) validateLocked() error {
	if int(p.count) > len(p.steps) {
		return contractError(diagnosticPlanCount)
	}
	if err := p.policy.Validate(); err != nil {
		return err
	}
	for index := uint8(0); index < p.count; index++ {
		if err := p.steps[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// StepResult is one immutable cleanup observation.
type StepResult struct {
	failure error
	id      StepID
	outcome StepOutcome
	phase   Phase
}

func (r StepResult) ID() StepID           { return r.id }
func (r StepResult) Phase() Phase         { return r.phase }
func (r StepResult) Outcome() StepOutcome { return r.outcome }
func (r StepResult) Failure() error       { return r.failure }

func (r StepResult) Validate() error {
	if err := r.id.Validate(); err != nil {
		return err
	}
	if err := r.phase.Validate(); err != nil {
		return err
	}
	if err := r.outcome.Validate(); err != nil {
		return err
	}
	identity := stepOutcomeIdentity(r.outcome)
	if identity == nil {
		if r.outcome == StepOutcomeCompleted && r.failure == nil {
			return nil
		}
		return contractError(diagnosticCompletedStepFailure)
	}
	if r.failure == nil || !errors.Is(r.failure, identity) {
		return contractError(diagnosticStepResultIdentityMismatch)
	}
	return nil
}

func stepOutcomeIdentities() [stepOutcomeLimit]error {
	return [stepOutcomeLimit]error{
		StepOutcomeFailed:              core.ErrShutdownStepFailure,
		StepOutcomeTimedOut:            core.ErrShutdownStepTimeout,
		StepOutcomePanicked:            core.ErrShutdownStepPanic,
		StepOutcomeTotalBudgetExceeded: core.ErrShutdownTotalTimeout,
	}
}

func stepOutcomeIdentity(outcome StepOutcome) error {
	identities := stepOutcomeIdentities()
	if outcome < stepOutcomeLimit {
		return identities[outcome]
	}
	return nil
}

// Report is one sealed fixed-capacity cleanup result.
type Report struct {
	results [MaximumSteps]StepResult
	count   uint8
	valid   bool
}

// Validate proves the sealed report rather than trusting its own seal: every
// retained observation must independently satisfy its own contract.
func (r Report) Validate() error {
	if !r.valid {
		return contractError(diagnosticReportUnset)
	}
	if int(r.count) > len(r.results) {
		return contractError(diagnosticReportCount)
	}
	for index := uint8(0); index < r.count; index++ {
		if err := r.results[index].Validate(); err != nil {
			return contractError(diagnosticReportResult, err)
		}
	}
	return nil
}

func (r Report) Count() uint8 { return r.count }

// Result reads one sealed observation. It checks the seal and the index only,
// so walking a report stays linear in its step count rather than quadratic.
func (r Report) Result(index uint8) (StepResult, bool) {
	if !r.valid || int(r.count) > len(r.results) || index >= r.count {
		return StepResult{}, false
	}
	return r.results[index], true
}

// Run executes phases in order and each phase in reverse registration order.
func (p *Plan) Run(parent context.Context) (Report, error) {
	if _, err := contextstate.Observe(parent); err != nil {
		return Report{}, contractError(diagnosticParentContext, err)
	}
	steps, count, policy, err := p.beginRun()
	if err != nil {
		return Report{}, err
	}
	root, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{
		Parent: context.WithoutCancel(parent), Duration: policy.TotalBudget,
	})
	if err != nil {
		return Report{}, contractError(diagnosticTotalBudgetConstruction, err)
	}
	defer cancel()
	return runPhases(root, steps, count)
}

func (p *Plan) beginRun() ([MaximumSteps]Step, uint8, PlanPolicy, error) {
	if p == nil {
		return [MaximumSteps]Step{}, 0, PlanPolicy{}, contractError(diagnosticPlanNil)
	}
	if !p.started.CompareAndSwap(false, true) {
		return [MaximumSteps]Step{}, 0, PlanPolicy{},
			contractError(diagnosticPlanAlreadyRun)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.validateLocked(); err != nil {
		return [MaximumSteps]Step{}, 0, PlanPolicy{}, err
	}
	return p.steps, p.count, p.policy, nil
}

func runPhases(
	root context.Context,
	steps [MaximumSteps]Step,
	count uint8,
) (Report, error) {
	var report Report
	var failure error
	for phase := PhaseStopAdmission; phase < phaseLimit; phase++ {
		for index := int(count) - 1; index >= 0; index-- {
			if steps[index].Phase != phase {
				continue
			}
			result := runStep(root, steps[index])
			report.add(result)
			failure = errors.Join(failure, result.failure)
		}
	}
	report.valid = true
	return report, failure
}

func (r *Report) add(result StepResult) {
	r.results[r.count] = result
	r.count++
}

type actionResult struct {
	failure  error
	panicked bool
}

func runStep(root context.Context, step Step) StepResult {
	if contextTerminal(root) {
		return newStepResult(step, StepOutcomeTotalBudgetExceeded,
			totalTimeoutError(diagnosticStepSkipped))
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{
		Parent: root, Duration: step.Budget,
	})
	if err != nil {
		return newStepResult(step, StepOutcomeFailed, stepFailureError(err))
	}
	defer cancel()
	result := invoke(ctx, step.Action)
	return classifyStep(stepClassification{
		root: root, stepContext: ctx, step: step, result: result,
	})
}

// invoke contains one cooperative callback. An error-valued panic stays
// reachable. Other values become a bounded diagnostic without calling
// user-defined formatting methods or retaining an arbitrary object graph.
func invoke(ctx context.Context, action StepAction) (result actionResult) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		var panicError StepPanicError
		switch value := recovered.(type) {
		case error:
			panicError = newNativeStepPanicError(value)
		case string:
			panicError = newNonErrorStepPanicError(value)
		case []byte:
			panicError = newNonErrorStepPanicError(boundedPanicBytes(value))
		case bool, int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64, uintptr,
			float32, float64, complex64, complex128:
			panicError = newNonErrorStepPanicError(fmt.Sprint(value))
		default:
			panicError = newNonErrorStepPanicError(
				fmt.Sprintf("non-error value of type %T", recovered),
			)
		}
		result = actionResult{failure: panicError, panicked: true}
	}()
	result.failure = action(ctx)
	return result
}

type stepClassification struct {
	root        context.Context
	stepContext context.Context
	result      actionResult
	step        Step
}

// classifyStep names one started step. A contained panic is an unambiguous fact
// about the callback, so it outranks the clock observations. Every branch joins
// whatever the callback actually produced, because a step that reached this
// point ran: its native error must stay reachable even when a budget expired.
func classifyStep(request stepClassification) StepResult {
	observed := request.result.failure
	if request.result.panicked {
		return newStepResult(request.step, StepOutcomePanicked, observed)
	}
	if contextTerminal(request.root) {
		return newStepResult(request.step, StepOutcomeTotalBudgetExceeded,
			totalTimeoutError(context.DeadlineExceeded, observed))
	}
	if contextTerminal(request.stepContext) {
		return newStepResult(request.step, StepOutcomeTimedOut,
			stepTimeoutError(context.DeadlineExceeded, observed))
	}
	if observed != nil {
		return newStepResult(request.step, StepOutcomeFailed, stepFailureError(observed))
	}
	return newStepResult(request.step, StepOutcomeCompleted, nil)
}

func contextTerminal(ctx context.Context) bool {
	state, err := contextstate.Observe(ctx)
	return err != nil || state != contextstate.StateNone
}

func newStepResult(step Step, outcome StepOutcome, failure error) StepResult {
	return StepResult{
		id: step.ID, phase: step.Phase, outcome: outcome, failure: failure,
	}
}

var (
	_ core.Validatable = Step{}
	_ core.Validatable = PlanPolicy{}
	_ core.Validatable = StepResult{}
	_ core.Validatable = Report{}
	_ core.Validatable = StepPanicError{}
)
