package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// signalTransitionCapacity buffers the first signal and the one signal that
	// may escalate it, so os/signal never drops a delivery on a busy observer.
	signalTransitionCapacity = 2
	// escalationCapacity retains the single escalation fact a run can publish,
	// so the owning goroutine never blocks on an absent consumer.
	escalationCapacity = 1
	// controllerChannelCapacity makes the two closed notification channels'
	// fixed ownership explicit to structural analysis.
	controllerChannelCapacity = 1
)

// SignalPolicy owns the two possible escalation triggers.
type SignalPolicy struct {
	GracePeriod  temporal.Duration
	GraceExpiry  GraceExpiryAction
	SecondSignal SecondSignalAction
}

func (p SignalPolicy) Validate() error {
	if err := p.GraceExpiry.Validate(); err != nil {
		return err
	}
	if err := p.SecondSignal.Validate(); err != nil {
		return err
	}
	if err := p.GracePeriod.Validate(); err != nil {
		return contractError(diagnosticGracePeriodInvalid, err)
	}
	if (p.GraceExpiry == GraceExpiryEscalate) != !p.GracePeriod.IsZero() {
		return contractError(diagnosticGracePeriodCoupling)
	}
	return nil
}

// Escalation is one immutable request for composition-root policy.
type Escalation struct {
	reason  EscalationReason
	first   SignalKind
	trigger SignalKind
	valid   bool
}

func (e Escalation) Reason() EscalationReason  { return e.reason }
func (e Escalation) FirstSignal() SignalKind   { return e.first }
func (e Escalation) TriggerSignal() SignalKind { return e.trigger }

func (e Escalation) Validate() error {
	if !e.valid {
		return contractError(diagnosticEscalationUnset)
	}
	if err := e.reason.Validate(); err != nil {
		return err
	}
	if err := e.first.Validate(); err != nil {
		return err
	}
	if e.reason == EscalationSecondSignal {
		return e.trigger.Validate()
	}
	if e.trigger != SignalKindUnknown {
		return contractError(diagnosticGraceEscalationTrigger)
	}
	return nil
}

// SignalCause is the authentic first signal attached to Controller.Context.
type SignalCause struct {
	kind      SignalKind
	authentic bool
}

func (e SignalCause) Kind() SignalKind { return e.kind }

func (e SignalCause) Validate() error {
	if !e.authentic {
		return contractError(diagnosticSignalCauseUnauthentic)
	}
	return e.kind.Validate()
}

func (e SignalCause) Error() string {
	return fmt.Sprintf("shutdown: received %s signal", e.kind)
}

func (e SignalCause) Unwrap() error { return core.ErrShutdownSignalReceived }

// WatchRequest starts one owned OS-signal observation.
type WatchRequest struct {
	Parent context.Context
	Policy SignalPolicy
	Set    SignalSet
}

func (r WatchRequest) Validate() error {
	if err := contextstate.Validate(r.Parent); err != nil {
		return contractError(diagnosticParentContext, err)
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	return r.Set.Validate()
}

type signalSource struct {
	events  <-chan os.Signal
	release func()
}

// Controller owns one signal subscription and its sole goroutine.
type Controller struct {
	ctx         context.Context
	cancel      context.CancelCauseFunc
	source      signalSource
	stop        chan struct{}
	done        chan struct{}
	escalated   chan Escalation
	closeOnce   sync.Once
	releaseOnce sync.Once
}

// Watch registers the requested platform signal set.
func Watch(request WatchRequest) (*Controller, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	registered := operatingSystemSignals(request.Set)
	if len(registered) == 0 {
		return nil, contractError(diagnosticSignalProjectionEmpty)
	}
	events := make(chan os.Signal, signalTransitionCapacity)
	signal.Notify(events, registered...)
	source := signalSource{
		events:  events,
		release: func() { signal.Stop(events) },
	}
	controller, err := watchSource(request, source)
	if err != nil {
		source.release()
		return nil, err
	}
	return controller, nil
}

func watchSource(request WatchRequest, source signalSource) (*Controller, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if source.events == nil || source.release == nil {
		return nil, contractError(diagnosticSignalSourceIncomplete)
	}
	gracePeriod, err := request.Policy.GracePeriod.Stdlib()
	if err != nil {
		return nil, contractError(diagnosticGraceProjection, err)
	}
	ctx, cancel := context.WithCancelCause(request.Parent)
	controller := &Controller{
		ctx:       ctx,
		cancel:    cancel,
		source:    source,
		stop:      make(chan struct{}, controllerChannelCapacity),
		done:      make(chan struct{}, controllerChannelCapacity),
		escalated: make(chan Escalation, escalationCapacity),
	}
	go controller.run(request.Parent, request.Policy, gracePeriod)
	return controller, nil
}

func (c *Controller) Context() context.Context {
	if c == nil {
		return nil
	}
	return c.ctx
}

func (c *Controller) Done() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.done
}

func (c *Controller) Escalated() <-chan Escalation {
	if c == nil {
		return nil
	}
	return c.escalated
}

func (c *Controller) Close() error {
	if c == nil {
		return contractError(diagnosticControllerNil)
	}
	c.closeOnce.Do(func() {
		close(c.stop)
		c.release()
		c.cancel(context.Canceled)
	})
	<-c.done
	return nil
}

func (c *Controller) release() {
	c.releaseOnce.Do(c.source.release)
}

func (c *Controller) run(
	parent context.Context,
	policy SignalPolicy,
	gracePeriod time.Duration,
) {
	defer close(c.done)
	defer close(c.escalated)
	defer c.release()
	first, ok := c.waitFirst()
	if !ok {
		return
	}
	c.cancel(SignalCause{kind: first, authentic: true})
	if policy.SecondSignal == SecondSignalRelease {
		c.release()
	}
	if policy.SecondSignal == SecondSignalRelease &&
		policy.GraceExpiry == GraceExpiryDisabled {
		return
	}
	c.waitEscalation(escalationWait{
		parent: parent, policy: policy, gracePeriod: gracePeriod, first: first,
	})
}

func (c *Controller) waitFirst() (SignalKind, bool) {
	for {
		select {
		case <-c.stop:
			return SignalKindUnknown, false
		case <-c.ctx.Done():
			return SignalKindUnknown, false
		case observed, open := <-c.source.events:
			if !open {
				c.cancel(signalSourceError(diagnosticSignalSourceClosed))
				return SignalKindUnknown, false
			}
			kind := classifyOperatingSystemSignal(observed)
			if kind.IsValid() {
				return kind, true
			}
		}
	}
}

type escalationWait struct {
	parent      context.Context
	gracePeriod time.Duration
	policy      SignalPolicy
	first       SignalKind
}

func (c *Controller) waitEscalation(request escalationWait) {
	var timer *time.Timer
	var timerEvents <-chan time.Time
	if request.policy.GraceExpiry == GraceExpiryEscalate {
		timer = time.NewTimer(request.gracePeriod)
		timerEvents = timer.C
		defer timer.Stop()
	}
	events := c.source.events
	if request.policy.SecondSignal == SecondSignalRelease {
		events = nil
	}
	for {
		select {
		case <-c.stop:
			return
		case <-request.parent.Done():
			return
		case <-timerEvents:
			c.publish(newGraceEscalation(request.first))
			return
		case observed, open := <-events:
			if !open {
				return
			}
			kind := classifyOperatingSystemSignal(observed)
			if !kind.IsValid() {
				continue
			}
			c.publish(newSignalEscalation(request.first, kind))
			return
		}
	}
}

func (c *Controller) publish(escalation Escalation) {
	c.release()
	c.escalated <- escalation
}

func newSignalEscalation(first, trigger SignalKind) Escalation {
	return Escalation{
		reason:  EscalationSecondSignal,
		first:   first,
		trigger: trigger,
		valid:   true,
	}
}

func newGraceEscalation(first SignalKind) Escalation {
	return Escalation{
		reason: EscalationGraceExpired,
		first:  first,
		valid:  true,
	}
}

var (
	_ core.Validatable = SignalPolicy{}
	_ core.Validatable = Escalation{}
	_ core.Validatable = SignalCause{}
	_ core.Validatable = WatchRequest{}
	_ core.OffWireEnum = PhaseUnknown
	_ core.OffWireEnum = StepOutcomeUnknown
	_ core.OffWireEnum = SignalKindUnknown
	_ core.OffWireEnum = SignalSetUnknown
	_ core.OffWireEnum = SecondSignalActionUnknown
	_ core.OffWireEnum = GraceExpiryActionUnknown
	_ core.OffWireEnum = EscalationReasonUnknown
)
