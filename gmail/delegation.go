package gmail

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

// DelegatedRequest pins one Workspace subject and signer for a long-lived
// read capability. The only permitted OAuth scope is Gmail read-only.
type DelegatedRequest struct {
	Signer  googleidentity.ServiceAccountEmail
	Subject googleidentity.WorkspaceSubject
	Policy  exchange.OperationPolicy
}

func (r DelegatedRequest) Validate() error {
	return errors.Join(r.Signer.Validate(), r.Subject.Validate(), validatePolicy(r.Policy))
}

// Delegate builds an account-bound access source. The first Gmail read mints
// a bearer and verifies its profile before using it. Reuse this handle across
// reads and close it when its owning application stops.
func (c Client) Delegate(ctx context.Context, r DelegatedRequest) (Access, error) {
	if err := c.admit(ctx, r); err != nil {
		return Access{}, err
	}
	runtime, err := googleidentity.NewClient(c.network)
	if err != nil {
		return Access{}, errors.Join(core.ErrGmailContract, err)
	}
	source := &delegatedMailbox{client: c, runtime: runtime, request: r, observe: wallInstant}
	return Access{mailbox: r.Subject.Email(), delegated: source}, nil
}

type delegatedMailbox struct {
	client  Client
	runtime googleidentity.Client
	request DelegatedRequest
	observe func() (temporal.Instant, error)

	mu         sync.Mutex
	current    googleidentity.DelegatedAccessToken
	generation uint64
	pending    *mailRenewal
	closed     bool
}

type mailRenewal struct {
	done      chan struct{}
	err       error
	abandoned bool
}

type bearerLease struct {
	secret     core.OAuthClientSecret
	generation uint64
}

func wallInstant() (temporal.Instant, error) {
	now, err := temporal.Observe()
	if err != nil {
		return temporal.Instant{}, err
	}
	return now.Instant()
}

func (s *delegatedMailbox) validate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrGmailConsent
	}
	return nil
}

func (s *delegatedMailbox) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	err := s.clearBearer()
	return errors.Join(err, s.runtime.CloseIdleConnections())
}

func (s *delegatedMailbox) clearBearer() error {
	if s.current == (googleidentity.DelegatedAccessToken{}) {
		return nil
	}
	err := s.current.Close()
	s.current = googleidentity.DelegatedAccessToken{}
	return err
}

func (s *delegatedMailbox) invalidate(generation uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generation == generation {
		return s.clearBearer()
	}
	return nil
}

func (s *delegatedMailbox) borrow(ctx context.Context) (bearerLease, error) {
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return bearerLease{}, errors.Join(err, context.Cause(ctx))
		}
		now, err := s.observe()
		if err != nil {
			return bearerLease{}, errors.Join(core.ErrGmailConsent, err)
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return bearerLease{}, core.ErrGmailConsent
		}
		if s.current.FreshAt(now) {
			got, borrowErr := s.copyBearer()
			s.mu.Unlock()
			return got, borrowErr
		}
		if shared := s.pending; shared != nil {
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return bearerLease{}, errors.Join(ctx.Err(), context.Cause(ctx))
			case <-shared.done:
				if shared.err != nil && !shared.abandoned {
					return bearerLease{}, shared.err
				}
			}
			continue
		}
		flight := &mailRenewal{done: make(chan struct{})}
		s.pending = flight
		s.mu.Unlock()
		return s.completeRenewal(ctx, flight)
	}
}

func (s *delegatedMailbox) copyBearer() (bearerLease, error) {
	secret, err := s.current.CopyBearer()
	if err != nil {
		return bearerLease{}, errors.Join(core.ErrGmailConsent, err)
	}
	return bearerLease{secret: secret, generation: s.generation}, nil
}

func (s *delegatedMailbox) completeRenewal(ctx context.Context, flight *mailRenewal) (bearerLease, error) {
	fresh, err := s.renew(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	defer close(flight.done)
	s.pending = nil
	if s.closed && err == nil {
		err = errors.Join(core.ErrGmailConsent, fresh.Close())
	}
	if err != nil {
		flight.err, flight.abandoned = err, ctx.Err() != nil && !s.closed
		return bearerLease{}, err
	}
	if err := s.clearBearer(); err != nil {
		flight.err = errors.Join(core.ErrGmailConsent, err, fresh.Close())
		return bearerLease{}, flight.err
	}
	s.current = fresh
	s.generation++
	return s.copyBearer()
}

func (s *delegatedMailbox) renew(ctx context.Context) (googleidentity.DelegatedAccessToken, error) {
	access, err := s.mint(ctx)
	if err == nil || ctx.Err() != nil || !retryDelegation(err) {
		return access, err
	}
	wait, waitErr := temporal.DurationFromMilliseconds(250)
	if waitErr != nil {
		return googleidentity.DelegatedAccessToken{}, errors.Join(core.ErrGmailConsent, waitErr)
	}
	if waitErr := temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: wait}); waitErr != nil {
		return googleidentity.DelegatedAccessToken{}, errors.Join(err, waitErr)
	}
	return s.mint(ctx)
}

func retryDelegation(err error) bool {
	// Quota and configuration refusals should not add traffic. A stale
	// profile bearer or a transient transport/provider failure gets one try.
	return hasProviderStatus(err, http.StatusUnauthorized) && errors.Is(err, core.ErrGmailConsent) ||
		errors.Is(err, core.ErrExchangeTransport) || errors.Is(err, core.ErrExchangeUnavailable)
}

func (s *delegatedMailbox) mint(ctx context.Context) (googleidentity.DelegatedAccessToken, error) {
	at, err := s.observe()
	if err != nil {
		return googleidentity.DelegatedAccessToken{}, errors.Join(core.ErrGmailConsent, err)
	}
	bearer, err := googleidentity.AcquireDelegatedAccess(ctx, s.runtime, googleidentity.DelegatedAccessRequest{
		Signer: s.request.Signer, Subject: s.request.Subject, Scope: googleidentity.DelegatedScopeGmailReadOnly, ObservedAt: at,
		Policy: googleidentity.Policy{OperationTimeout: s.request.Policy.OperationTimeout, AttemptTimeout: s.request.Policy.AttemptTimeout},
	})
	if err != nil {
		return googleidentity.DelegatedAccessToken{}, err
	}
	secret, err := bearer.CopyBearer()
	if err != nil {
		return googleidentity.DelegatedAccessToken{}, errors.Join(err, bearer.Close())
	}
	err = s.client.verifyMailbox(ctx, secret, mailboxVerification{policy: s.request.Policy, mailbox: s.request.Subject.Email()})
	if err := errors.Join(err, secret.Close()); err != nil {
		return googleidentity.DelegatedAccessToken{}, errors.Join(err, bearer.Close())
	}
	now, err := s.observe()
	if err != nil || !bearer.FreshAt(now) {
		return googleidentity.DelegatedAccessToken{}, errors.Join(core.ErrGmailConsent, err, bearer.Close())
	}
	return bearer, nil
}

func readWithAccess[T any](ctx context.Context, access Access, request mailboxRead) (T, error) {
	if access.delegated == nil {
		return readJSON[T](ctx, access.token, request)
	}
	var zero T
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: request.policy.OperationTimeout})
	if err != nil {
		return zero, errors.Join(core.ErrGmailContract, err)
	}
	defer cancel()
	got, generation, err := readDelegated[T](ctx, access.delegated, request)
	if generation == 0 || !hasProviderStatus(err, http.StatusUnauthorized) {
		return got, err
	}
	if err := access.delegated.invalidate(generation); err != nil {
		return zero, errors.Join(core.ErrGmailConsent, err)
	}
	got, generation, err = readDelegated[T](ctx, access.delegated, request)
	if generation != 0 && hasProviderStatus(err, http.StatusUnauthorized) {
		return zero, errors.Join(err, access.delegated.invalidate(generation))
	}
	return got, err
}

func readDelegated[T any](ctx context.Context, source *delegatedMailbox, request mailboxRead) (T, uint64, error) {
	lease, err := source.borrow(ctx)
	if err != nil {
		var zero T
		return zero, 0, err
	}
	got, err := readJSON[T](ctx, lease.secret, request)
	if err := errors.Join(err, lease.secret.Close()); err != nil {
		var zero T
		return zero, lease.generation, err
	}
	return got, lease.generation, nil
}

func hasProviderStatus(err error, code int) bool {
	status, ok := errors.AsType[exchange.StatusError](err)
	if !ok {
		return false
	}
	actual, err := status.Status().Int()
	return err == nil && actual == code
}

func (DelegatedRequest) gmailProtocolFact()       {}
func (*delegatedMailbox) gmailCapabilityWrapper() {}
func (mailRenewal) gmailInternalFlow()            {}
func (bearerLease) gmailCapabilityWrapper()       {}
