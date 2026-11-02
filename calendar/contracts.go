package calendar

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// Access closes the scopes requested for this connection. A visiting customer
// needs only busy periods and calendar names; event writes are business-only.
type Access uint8

const (
	AccessInvalid Access = iota
	AccessAvailability
	AccessManageEvents
)

func (a Access) Validate() error {
	if a < AccessAvailability || a > AccessManageEvents {
		return core.ErrCalendarContract
	}
	return nil
}

// Lifetime distinguishes one-shot visitor consent from a persistent business
// connection. A visitor grant never requests offline access or a refresh token.
type Lifetime uint8

const (
	LifetimeInvalid Lifetime = iota
	LifetimeVisitor
	LifetimeBusiness
)

func (l Lifetime) Validate() error {
	if l < LifetimeVisitor || l > LifetimeBusiness {
		return core.ErrCalendarContract
	}
	return nil
}

// AuthorizationRequest binds the application's previously stored state and
// PKCE challenge to one provider consent request. No OpenID/profile scope.
type AuthorizationRequest struct {
	ClientID    core.OAuthClientID
	RedirectURI core.OAuthRedirectURI
	State       core.OAuthState
	Challenge   core.OAuthPKCEChallenge
	Access      Access
	Lifetime    Lifetime
}

func (r AuthorizationRequest) Validate() error {
	if err := errors.Join(r.ClientID.Validate(), r.RedirectURI.Validate(), r.State.Validate(), r.Challenge.Validate(), r.Access.Validate(), r.Lifetime.Validate()); err != nil {
		return errors.Join(core.ErrCalendarContract, err)
	}
	if r.Lifetime == LifetimeVisitor && r.Access != AccessAvailability {
		return core.ErrCalendarContract
	}
	return nil
}

// AuthorizationExchange is a single-use code exchange. The application must
// atomically redeem state before calling it and preserve its original PKCE.
type AuthorizationExchange struct {
	Client       exchange.Client
	Policy       exchange.OperationPolicy
	ClientID     core.OAuthClientID
	ClientSecret core.OAuthClientSecret
	RedirectURI  core.OAuthRedirectURI
	Code         core.OAuthAuthorizationCode
	Verifier     core.OAuthPKCEVerifier
	ObservedAt   temporal.Instant
	Access       Access
	Lifetime     Lifetime
}

func (r AuthorizationExchange) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.Policy.Validate(), r.ClientID.Validate(), r.ClientSecret.Validate(), r.RedirectURI.Validate(), r.Code.Validate(), r.Verifier.Validate(), r.ObservedAt.Validate(), r.Access.Validate(), r.Lifetime.Validate()); err != nil {
		return errors.Join(core.ErrCalendarContract, err)
	}
	if r.Policy.Retry.MaximumAttempts != 1 || r.Policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrCalendarContract
	}
	if r.Lifetime == LifetimeVisitor && r.Access != AccessAvailability {
		return core.ErrCalendarContract
	}
	return nil
}

// Grant owns provider credentials. Optional Refresh is absent for visitors.
// The caller seals retained credentials or destroys the entire grant on return.
type Grant struct {
	AccessToken  core.CalendarCredential
	RefreshToken core.CalendarCredential
	ExpiresAt    temporal.Instant
}

func (g Grant) Validate() error {
	if err := errors.Join(g.AccessToken.Validate(), g.ExpiresAt.Validate()); err != nil {
		return errors.Join(core.ErrCalendarContract, err)
	}
	if g.AccessToken.Kind() != core.CalendarCredentialAccess {
		return core.ErrCalendarContract
	}
	if g.RefreshToken.IsSet() {
		if err := g.RefreshToken.Validate(); err != nil {
			return err
		}
		if g.RefreshToken.Kind() != core.CalendarCredentialRefresh {
			return core.ErrCalendarContract
		}
	}
	return nil
}
func (g *Grant) Close() error {
	if g == nil {
		return core.ErrCalendarContract
	}
	var refreshErr error
	if g.RefreshToken.IsSet() {
		refreshErr = g.RefreshToken.Close()
	}
	return errors.Join(g.AccessToken.Close(), refreshErr)
}

// FreeBusyRequest has an explicit checked coverage window and bounded calendar
// set. Per-calendar failure is unknown availability, never an empty busy list.
type FreeBusyRequest struct {
	Client     exchange.Client
	Policy     exchange.OperationPolicy
	Credential core.CalendarCredential
	Window     temporal.IntervalBounds
	Calendars  []core.CalendarIdentifier
}

func (r FreeBusyRequest) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.Policy.Validate(), r.Credential.Validate(), r.Window.Validate()); err != nil {
		return errors.Join(core.ErrCalendarContract, err)
	}
	if r.Credential.Kind() != core.CalendarCredentialAccess || len(r.Calendars) == 0 || len(r.Calendars) > core.CalendarFreeBusyMaximumCalendars {
		return core.ErrCalendarContract
	}
	elapsed, err := r.Window.End.Since(r.Window.Start)
	if err != nil || elapsed.IsZero() {
		return core.ErrCalendarContract
	}
	for i, id := range r.Calendars {
		if err := id.Validate(); err != nil {
			return err
		}
		for _, previous := range r.Calendars[:i] {
			if previous == id {
				return core.ErrCalendarContract
			}
		}
	}
	return nil
}

type BusyCalendar struct {
	ID   core.CalendarIdentifier
	Busy []temporal.IntervalBounds
}
type FreeBusyResult struct {
	Window    temporal.IntervalBounds
	Calendars []BusyCalendar
}

func (r FreeBusyResult) Validate() error {
	if err := r.Window.Validate(); err != nil {
		return errors.Join(core.ErrCalendarContract, err)
	}
	if len(r.Calendars) == 0 || len(r.Calendars) > core.CalendarFreeBusyMaximumCalendars {
		return core.ErrCalendarContract
	}
	for i, calendar := range r.Calendars {
		if err := calendar.ID.Validate(); err != nil {
			return err
		}
		if len(calendar.Busy) > core.CalendarMaximumBusyIntervals {
			return core.ErrCalendarContract
		}
		for _, prior := range r.Calendars[:i] {
			if prior.ID == calendar.ID {
				return core.ErrCalendarContract
			}
		}
		for _, busy := range calendar.Busy {
			if err := busy.Validate(); err != nil {
				return errors.Join(core.ErrCalendarContract, err)
			}
		}
	}
	return nil
}
