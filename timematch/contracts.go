package timematch

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
	"github.com/deliri/primitive/v2026/temporal"
)

// Provider closes the calendar services this package connects to.
type Provider uint8

const (
	ProviderInvalid Provider = iota
	ProviderGoogle
	ProviderMicrosoft
	ProviderApple
)

func (p Provider) Validate() error {
	if p < ProviderGoogle || p > ProviderApple {
		return core.ErrTimeMatchContract
	}
	return nil
}

// Access closes what a connection may do. Availability reads free/busy and
// the calendar list only; ManageEvents additionally writes owned events.
type Access uint8

const (
	AccessInvalid Access = iota
	AccessAvailability
	AccessManageEvents
)

func (a Access) Validate() error {
	if a < AccessAvailability || a > AccessManageEvents {
		return core.ErrTimeMatchContract
	}
	return nil
}

// Lifetime separates a one-shot visitor grant from a retained connection.
// A visitor grant is online-only: it exists to read availability and, with
// AccessManageEvents, to write the one confirmed appointment; it never
// requests offline access, never carries a refresh credential and is never an
// application account. The caller revokes it where the provider supports
// revocation and closes it after confirmation or abandonment. Business is the
// explicit opt-in for continued (repeating) access.
type Lifetime uint8

const (
	LifetimeInvalid Lifetime = iota
	LifetimeVisitor
	LifetimeBusiness
)

func (l Lifetime) Validate() error {
	if l < LifetimeVisitor || l > LifetimeBusiness {
		return core.ErrTimeMatchContract
	}
	return nil
}

// MicrosoftTenant is the documented v2 endpoint tenant segment: common
// (personal and work/school), consumers (personal) or organizations
// (work/school). https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow
type MicrosoftTenant uint8

const (
	MicrosoftTenantInvalid MicrosoftTenant = iota
	MicrosoftTenantCommon
	MicrosoftTenantConsumers
	MicrosoftTenantOrganizations
)

func (t MicrosoftTenant) Validate() error {
	if t < MicrosoftTenantCommon || t > MicrosoftTenantOrganizations {
		return core.ErrTimeMatchContract
	}
	return nil
}

// AvailabilityState is Known only when the provider returned a complete,
// admitted busy projection for that calendar. Unknown is never free time.
type AvailabilityState uint8

const (
	AvailabilityInvalid AvailabilityState = iota
	AvailabilityKnown
	AvailabilityUnknown
)

func (s AvailabilityState) Validate() error {
	if s < AvailabilityKnown || s > AvailabilityUnknown {
		return core.ErrTimeMatchContract
	}
	return nil
}

// RevokeOutcome reports what the provider did. ProviderHasNoRevocation means
// the provider documents no delegated-token revocation door; the caller must
// still destroy its custody.
type RevokeOutcome uint8

const (
	RevokeInvalid RevokeOutcome = iota
	RevokedAtProvider
	RevokeProviderHasNoRevocation
)

func (o RevokeOutcome) Validate() error {
	if o < RevokedAtProvider || o > RevokeProviderHasNoRevocation {
		return core.ErrTimeMatchContract
	}
	return nil
}

// DeleteOutcome distinguishes an actual deletion from an already absent event.
type DeleteOutcome uint8

const (
	DeleteInvalid DeleteOutcome = iota
	Deleted
	DeleteAlreadyAbsent
)

func (o DeleteOutcome) Validate() error {
	if o < Deleted || o > DeleteAlreadyAbsent {
		return core.ErrTimeMatchContract
	}
	return nil
}

// Closed wire names, ordered by enum value starting at 1. The Microsoft
// tenant names are the documented v2 endpoint path segments.
func providerNames() []string { return []string{"google", "microsoft", "apple"} }
func accessNames() []string   { return []string{"availability", "manage_events"} }
func lifetimeNames() []string { return []string{"visitor", "business"} }
func tenantNames() []string   { return []string{"common", "consumers", "organizations"} }
func stateNames() []string    { return []string{"known", "unknown"} }
func revokeNames() []string   { return []string{"revoked_at_provider", "provider_has_no_revocation"} }
func deleteNames() []string   { return []string{"deleted", "already_absent"} }

// enumName renders an admitted value; an invalid value renders as empty text.
func enumName[E ~uint8](value E, names []string) string {
	if value == 0 || int(value) > len(names) {
		return ""
	}
	return names[value-1]
}

func marshalEnum[E ~uint8](value E, names []string) ([]byte, error) {
	name := enumName(value, names)
	if name == "" {
		return nil, errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract)
	}
	return core.MarshalCanonicalJSONString(name)
}

func unmarshalEnum[E ~uint8](target *E, data []byte, names []string) error {
	if target == nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	for index, name := range names {
		if name == text {
			*target = E(index + 1)
			return nil
		}
	}
	return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract)
}

func (p Provider) String() string                   { return enumName(p, providerNames()) }
func (p Provider) IsValid() bool                    { return p.Validate() == nil }
func (p Provider) MarshalJSON() ([]byte, error)     { return marshalEnum(p, providerNames()) }
func (p *Provider) UnmarshalJSON(data []byte) error { return unmarshalEnum(p, data, providerNames()) }

func (a Access) String() string                   { return enumName(a, accessNames()) }
func (a Access) IsValid() bool                    { return a.Validate() == nil }
func (a Access) MarshalJSON() ([]byte, error)     { return marshalEnum(a, accessNames()) }
func (a *Access) UnmarshalJSON(data []byte) error { return unmarshalEnum(a, data, accessNames()) }

func (l Lifetime) String() string                   { return enumName(l, lifetimeNames()) }
func (l Lifetime) IsValid() bool                    { return l.Validate() == nil }
func (l Lifetime) MarshalJSON() ([]byte, error)     { return marshalEnum(l, lifetimeNames()) }
func (l *Lifetime) UnmarshalJSON(data []byte) error { return unmarshalEnum(l, data, lifetimeNames()) }

func (t MicrosoftTenant) String() string               { return enumName(t, tenantNames()) }
func (t MicrosoftTenant) IsValid() bool                { return t.Validate() == nil }
func (t MicrosoftTenant) MarshalJSON() ([]byte, error) { return marshalEnum(t, tenantNames()) }
func (t *MicrosoftTenant) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(t, data, tenantNames())
}

func (s AvailabilityState) String() string               { return enumName(s, stateNames()) }
func (s AvailabilityState) IsValid() bool                { return s.Validate() == nil }
func (s AvailabilityState) MarshalJSON() ([]byte, error) { return marshalEnum(s, stateNames()) }
func (s *AvailabilityState) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(s, data, stateNames())
}

func (o RevokeOutcome) String() string               { return enumName(o, revokeNames()) }
func (o RevokeOutcome) IsValid() bool                { return o.Validate() == nil }
func (o RevokeOutcome) MarshalJSON() ([]byte, error) { return marshalEnum(o, revokeNames()) }
func (o *RevokeOutcome) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(o, data, revokeNames())
}

func (o DeleteOutcome) String() string               { return enumName(o, deleteNames()) }
func (o DeleteOutcome) IsValid() bool                { return o.Validate() == nil }
func (o DeleteOutcome) MarshalJSON() ([]byte, error) { return marshalEnum(o, deleteNames()) }
func (o *DeleteOutcome) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(o, data, deleteNames())
}

// Secret holds one provider credential: an access or refresh token, an OAuth
// client secret or an Apple app-specific password. Copies share destruction;
// it cannot be formatted or serialized.
type Secret struct{ value []byte }

func ParseSecret(value []byte) (Secret, error) {
	candidate := Secret{value: append([]byte(nil), value...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.value)
		return Secret{}, err
	}
	return candidate, nil
}

func SecretFromSecretStore(value secretstore.Value) (Secret, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return Secret{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	defer clear(material)
	return ParseSecret(material)
}

// Validate admits visible ASCII only: every documented token and app-specific
// password form is visible ASCII, and nothing else is header-safe.
func (s Secret) Validate() error {
	if !visibleASCII(s.value) {
		return core.ErrTimeMatchContract
	}
	return nil
}

func (s Secret) IsSet() bool { return len(s.value) != 0 }

// CopyBytes explicitly transfers an independent credential copy to the caller
// for protected storage. The caller owns clearing the returned bytes. Normal
// formatting and JSON remain redacted/refused; closing the source does not
// destroy a copy already handed to a storage boundary.
func (s Secret) CopyBytes() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return slices.Clone(s.value), nil
}

func (s *Secret) Close() error {
	if s == nil {
		return core.ErrTimeMatchContract
	}
	clear(s.value)
	*s = Secret{}
	return nil
}

func (Secret) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, core.RedactedValueText) }
func (Secret) MarshalJSON() ([]byte, error)   { return nil, core.ErrTimeMatchContract }

// AppleAccount is the Apple Account name used with an app-specific password.
// RFC 7617 forbids ':' in a Basic user-id.
type AppleAccount struct{ value string }

func ParseAppleAccount(value string) (AppleAccount, error) {
	candidate := AppleAccount{value: value}
	if err := candidate.Validate(); err != nil {
		return AppleAccount{}, err
	}
	return candidate, nil
}

func (a AppleAccount) Validate() error {
	for i := range len(a.value) {
		if a.value[i] == ':' {
			return core.ErrTimeMatchContract
		}
	}
	if !visibleASCII([]byte(a.value)) {
		return core.ErrTimeMatchContract
	}
	return nil
}
func (AppleAccount) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Application is one confidential OAuth client registration (server only).
type Application struct {
	ClientID     core.OAuthClientID
	ClientSecret Secret
	RedirectURI  core.OAuthRedirectURI
	Provider     Provider
	Tenant       MicrosoftTenant
}

func (a Application) Validate() error {
	if err := errors.Join(a.ClientID.Validate(), a.ClientSecret.Validate(), a.RedirectURI.Validate()); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	switch a.Provider {
	case ProviderGoogle:
		if a.Tenant != MicrosoftTenantInvalid {
			return core.ErrTimeMatchContract
		}
		return nil
	case ProviderMicrosoft:
		return a.Tenant.Validate()
	default:
		// Apple publishes no OAuth client onboarding for calendar access.
		return core.ErrTimeMatchContract
	}
}

func validateGrantShape(access Access, lifetime Lifetime) error {
	return errors.Join(access.Validate(), lifetime.Validate())
}

// AuthorizationRequest binds application-stored state and PKCE challenge to
// one calendar-only consent request.
type AuthorizationRequest struct {
	State       core.OAuthState
	Challenge   core.OAuthPKCEChallenge
	Application Application
	Access      Access
	Lifetime    Lifetime
}

func (r AuthorizationRequest) Validate() error {
	if err := errors.Join(r.Application.Validate(), r.State.Validate(), r.Challenge.Validate(), validateGrantShape(r.Access, r.Lifetime)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// CodeExchange redeems one authorization code. The application atomically
// consumes its state before calling. ObservedAt is captured before the call;
// expiry is ObservedAt plus the provider's expires_in, never time.Now.
type CodeExchange struct {
	Code        core.OAuthAuthorizationCode
	Verifier    core.OAuthPKCEVerifier
	ObservedAt  temporal.Instant
	Application Application
	Policy      exchange.OperationPolicy
	Access      Access
	Lifetime    Lifetime
}

func (r CodeExchange) Validate() error {
	if err := errors.Join(r.Application.Validate(), r.Code.Validate(), r.Verifier.Validate(), r.ObservedAt.Validate(), validateGrantShape(r.Access, r.Lifetime), validatePolicy(r.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// RefreshRequest renews a retained business grant.
type RefreshRequest struct {
	RefreshToken Secret
	ObservedAt   temporal.Instant
	Application  Application
	Policy       exchange.OperationPolicy
	Access       Access
}

func (r RefreshRequest) Validate() error {
	if err := errors.Join(r.Application.Validate(), r.RefreshToken.Validate(), r.ObservedAt.Validate(), r.Access.Validate(), validatePolicy(r.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// RevokeRequest asks the provider to invalidate one token.
type RevokeRequest struct {
	Token    Secret
	Policy   exchange.OperationPolicy
	Provider Provider
}

func (r RevokeRequest) Validate() error {
	if err := errors.Join(r.Provider.Validate(), r.Token.Validate(), validatePolicy(r.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// Grant owns provider credentials. RefreshToken is absent for visitors and
// for refresh results whose provider did not rotate it (Google). The caller
// seals what it retains and closes the grant.
type Grant struct {
	AccessToken  Secret
	RefreshToken Secret
	ExpiresAt    temporal.Instant
	Provider     Provider
	Lifetime     Lifetime
	Access       Access
}

func (g Grant) Validate() error {
	if err := errors.Join(g.Provider.Validate(), g.AccessToken.Validate(), g.ExpiresAt.Validate(), validateGrantShape(g.Access, g.Lifetime)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if g.Provider == ProviderApple || !g.AccessToken.IsSet() {
		return core.ErrTimeMatchContract
	}
	if g.RefreshToken.IsSet() {
		if g.Lifetime == LifetimeVisitor {
			return core.ErrTimeMatchContract
		}
		return g.RefreshToken.Validate()
	}
	return nil
}

func (g *Grant) Close() error {
	if g == nil {
		return core.ErrTimeMatchContract
	}
	err := errors.Join(g.AccessToken.Close(), g.RefreshToken.Close())
	*g = Grant{}
	return err
}

// Connection is the per-call provider capability. OAuth providers use an
// access token; Apple uses an Apple Account and app-specific password.
type Connection struct {
	AccessToken   Secret
	ApplePassword Secret
	AppleAccount  AppleAccount
	Provider      Provider
}

func (c Connection) Validate() error {
	if err := c.Provider.Validate(); err != nil {
		return err
	}
	if c.Provider == ProviderApple {
		if c.AccessToken.IsSet() || c.AppleAccount.value == "" || !c.ApplePassword.IsSet() {
			return core.ErrTimeMatchContract
		}
		return errors.Join(c.AppleAccount.Validate(), c.ApplePassword.Validate())
	}
	if !c.AccessToken.IsSet() || c.AppleAccount.value != "" || c.ApplePassword.IsSet() {
		return core.ErrTimeMatchContract
	}
	return c.AccessToken.Validate()
}

// Close zeroes the connection's credentials (shared with the Grant or
// BrowserAppleConnection they came from) and clears the connection.
func (c *Connection) Close() error {
	if c == nil {
		return core.ErrTimeMatchContract
	}
	err := errors.Join(c.AccessToken.Close(), c.ApplePassword.Close())
	*c = Connection{}
	return err
}

// CalendarID is an opaque provider calendar coordinate (a Google calendar ID,
// a Graph calendar ID, or an absolute iCloud CalDAV collection URL).
type CalendarID struct{ value string }

func ParseCalendarID(value string) (CalendarID, error) {
	candidate := CalendarID{value: value}
	if err := candidate.Validate(); err != nil {
		return CalendarID{}, err
	}
	return candidate, nil
}

func (c CalendarID) Validate() error {
	if !visibleASCII([]byte(c.value)) {
		return core.ErrTimeMatchContract
	}
	return nil
}
func (c CalendarID) String() string { return c.value }

// Calendar is one discovered calendar without its display name, color or
// owner details.
type Calendar struct {
	ID       CalendarID
	Writable bool
	Primary  bool
}

type CalendarListRequest struct {
	Connection Connection
	Policy     exchange.OperationPolicy
}

func (r CalendarListRequest) Validate() error {
	if err := errors.Join(r.Connection.Validate(), validatePolicy(r.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// CalendarList is the complete, unique set returned by every provider page.
type CalendarList struct {
	Calendars []Calendar
	Provider  Provider
}

func (l CalendarList) Validate() error {
	if err := l.Provider.Validate(); err != nil {
		return err
	}
	for i, calendar := range l.Calendars {
		if err := calendar.ID.Validate(); err != nil {
			return err
		}
		for _, prior := range l.Calendars[:i] {
			if prior.ID == calendar.ID {
				return core.ErrTimeMatchResponse
			}
		}
	}
	return nil
}

// AvailabilityRequest asks for busy time in one half-open window.
type AvailabilityRequest struct {
	Connection Connection
	Calendars  []CalendarID
	Window     temporal.IntervalBounds
	Policy     exchange.OperationPolicy
}

func (r AvailabilityRequest) Validate() error {
	if err := errors.Join(r.Connection.Validate(), validatePolicy(r.Policy), positiveInterval(r.Window)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if len(r.Calendars) == 0 {
		return core.ErrTimeMatchContract
	}
	for i, id := range r.Calendars {
		if err := id.Validate(); err != nil {
			return err
		}
		for _, prior := range r.Calendars[:i] {
			if prior == id {
				return core.ErrTimeMatchContract
			}
		}
	}
	return nil
}

// CalendarAvailability holds sorted, merged, positive busy intervals inside
// the window for a Known calendar, and nothing for an Unknown one.
type CalendarAvailability struct {
	ID    CalendarID
	Busy  []temporal.IntervalBounds
	State AvailabilityState
}

// Availability covers exactly the requested window and calendars in order.
type Availability struct {
	Calendars []CalendarAvailability
	Window    temporal.IntervalBounds
}

func (a Availability) Validate() error {
	if err := positiveInterval(a.Window); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if len(a.Calendars) == 0 {
		return core.ErrTimeMatchContract
	}
	windowStart, windowEnd, _ := nanosecondBounds(a.Window)
	for i, calendar := range a.Calendars {
		if err := errors.Join(calendar.ID.Validate(), calendar.State.Validate()); err != nil {
			return err
		}
		for _, prior := range a.Calendars[:i] {
			if prior.ID == calendar.ID {
				return core.ErrTimeMatchContract
			}
		}
		if calendar.State == AvailabilityUnknown && len(calendar.Busy) != 0 {
			return core.ErrTimeMatchContract
		}
		previousEnd := windowStart
		for j, busy := range calendar.Busy {
			start, end, err := nanosecondBounds(busy)
			if err != nil || start >= end || start < windowStart || end > windowEnd {
				return core.ErrTimeMatchContract
			}
			// Sorted and merged: a touching or overlapping successor is a
			// projection defect, never two facts.
			if j > 0 && start <= previousEnd {
				return core.ErrTimeMatchContract
			}
			previousEnd = end
		}
	}
	return nil
}

// covers binds a result to its request: same window, same calendars, same order.
func (a Availability) covers(request AvailabilityRequest) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.Window != request.Window || len(a.Calendars) != len(request.Calendars) {
		return core.ErrTimeMatchContract
	}
	for i, calendar := range a.Calendars {
		if calendar.ID != request.Calendars[i] {
			return core.ErrTimeMatchContract
		}
	}
	return nil
}

// EventID is the caller's deterministic event identity: Google's documented
// base32hex alphabet (a-v, 0-9), 5 to 1024 characters. It is the Google event
// id, the Graph transactionId and the iCalendar UID/resource name, so a retry
// of the same intent always names the same provider event.
// https://developers.google.com/workspace/calendar/api/v3/reference/events/insert
type EventID struct{ value string }

const (
	eventIDMinimumBytes = 5
	eventIDMaximumBytes = 1024
)

func ParseEventID(value string) (EventID, error) {
	candidate := EventID{value: value}
	if err := candidate.Validate(); err != nil {
		return EventID{}, err
	}
	return candidate, nil
}

func (e EventID) Validate() error {
	if len(e.value) < eventIDMinimumBytes || len(e.value) > eventIDMaximumBytes {
		return core.ErrTimeMatchContract
	}
	for i := range len(e.value) {
		b := e.value[i]
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'v') {
			return core.ErrTimeMatchContract
		}
	}
	return nil
}
func (e EventID) String() string { return e.value }

// EventSummary is the owned event title written to the provider: non-empty
// UTF-8 without control characters.
type EventSummary struct{ value string }

func ParseEventSummary(value string) (EventSummary, error) {
	candidate := EventSummary{value: value}
	if err := candidate.Validate(); err != nil {
		return EventSummary{}, err
	}
	return candidate, nil
}

func (s EventSummary) Validate() error {
	if s.value == "" || !utf8.ValidString(s.value) {
		return core.ErrTimeMatchContract
	}
	for _, r := range s.value {
		if unicode.IsControl(r) {
			return core.ErrTimeMatchContract
		}
	}
	return nil
}
func (s EventSummary) String() string { return s.value }

// EventIntent is the complete owned event: no attendees, so no provider
// invitation is ever sent by this package.
type EventIntent struct {
	Calendar CalendarID
	ID       EventID
	Summary  EventSummary
	Interval temporal.IntervalBounds
}

func (i EventIntent) Validate() error {
	if err := errors.Join(i.Calendar.Validate(), i.ID.Validate(), i.Summary.Validate(), positiveInterval(i.Interval)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// EventRef locates a provider event. Resource is the Google event id, the
// Graph event id, or the absolute CalDAV resource URL.
type EventRef struct {
	ID       EventID
	Calendar CalendarID
	Resource string
	Provider Provider
}

func (r EventRef) Validate() error {
	if err := errors.Join(r.ID.Validate(), r.Provider.Validate(), r.Calendar.Validate()); err != nil {
		return err
	}
	if !visibleASCII([]byte(r.Resource)) {
		return core.ErrTimeMatchContract
	}
	if r.Provider == ProviderGoogle && r.Resource != r.ID.value {
		return core.ErrTimeMatchContract
	}
	return nil
}

// EventReceipt proves the provider holds an event matching the owned intent.
// A provider calendar is independently editable: this is an observation at
// receipt time, not an atomic reservation.
type EventReceipt struct{ Ref EventRef }

func (r EventReceipt) Validate() error { return r.Ref.Validate() }

// EventWrite creates the intent or verifies that the same intent already
// exists. ObservedAt is the iCalendar DTSTAMP for Apple.
type EventWrite struct {
	Attempt    EventCreationAttempt
	Connection Connection
	Intent     EventIntent
	ObservedAt temporal.Instant
	Policy     exchange.OperationPolicy
}

func (w EventWrite) Validate() error {
	if err := errors.Join(w.Attempt.Validate(), w.Connection.Validate(), w.Intent.Validate(), w.ObservedAt.Validate(), validatePolicy(w.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// EventUpdate replaces an owned event's interval and summary.
type EventUpdate struct {
	Connection Connection
	Ref        EventRef
	Intent     EventIntent
	ObservedAt temporal.Instant
	Policy     exchange.OperationPolicy
}

func (u EventUpdate) Validate() error {
	if err := errors.Join(u.Connection.Validate(), u.Ref.Validate(), u.Intent.Validate(), u.ObservedAt.Validate(), validatePolicy(u.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if u.Ref.Provider != u.Connection.Provider || u.Ref.Calendar != u.Intent.Calendar || u.Ref.ID != u.Intent.ID {
		return core.ErrTimeMatchContract
	}
	return nil
}

// EventDelete removes one owned event.
type EventDelete struct {
	Connection Connection
	Ref        EventRef
	Policy     exchange.OperationPolicy
}

func (d EventDelete) Validate() error {
	if err := errors.Join(d.Connection.Validate(), d.Ref.Validate(), validatePolicy(d.Policy)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	if d.Ref.Provider != d.Connection.Provider {
		return core.ErrTimeMatchContract
	}
	return nil
}

// validatePolicy keeps every provider effect single-attempt with redirects
// refused: retries of writes belong to the application's durable intent, and
// a redirect could carry a bearer credential to another origin.
func validatePolicy(p exchange.OperationPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Retry.MaximumAttempts != 1 || p.Redirect.Mode != exchange.RedirectReject {
		return core.ErrTimeMatchContract
	}
	return nil
}

func positiveInterval(b temporal.IntervalBounds) error {
	start, end, err := nanosecondBounds(b)
	if err != nil {
		return err
	}
	if start >= end {
		return core.ErrTimeMatchContract
	}
	return nil
}

func nanosecondBounds(b temporal.IntervalBounds) (int64, int64, error) {
	if err := b.Validate(); err != nil {
		return 0, 0, err
	}
	start, startErr := b.Start.Nanoseconds()
	end, endErr := b.End.Nanoseconds()
	if err := errors.Join(startErr, endErr); err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func visibleASCII(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	for _, b := range value {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}

var (
	_ core.Validatable = Application{}
	_ core.Validatable = AuthorizationRequest{}
	_ core.Validatable = CodeExchange{}
	_ core.Validatable = RefreshRequest{}
	_ core.Validatable = RevokeRequest{}
	_ core.Validatable = Grant{}
	_ core.Validatable = Connection{}
	_ core.Validatable = CalendarListRequest{}
	_ core.Validatable = CalendarList{}
	_ core.Validatable = AvailabilityRequest{}
	_ core.Validatable = Availability{}
	_ core.Validatable = EventWrite{}
	_ core.Validatable = EventUpdate{}
	_ core.Validatable = EventDelete{}
	_ core.Validatable = EventReceipt{}
)
