package core

import (
	"errors"
	"fmt"
	"io"
)

var (
	ErrCalendarContract            = errors.New("calendar contract violation")
	ErrCalendarConsentRevoked      = errors.New("calendar consent revoked")
	ErrCalendarConsentMissing      = errors.New("calendar consent incomplete")
	ErrCalendarAvailabilityUnknown = errors.New("calendar availability unknown")
	ErrCalendarConflict            = errors.New("calendar event conflict")
)

const (
	CalendarCredentialMaximumBytes      = 8192
	CalendarIdentifierMaximumBytes      = 1024
	CalendarEventIdentifierMinimumBytes = 5
	CalendarFreeBusyMaximumCalendars    = 50
	CalendarMaximumBusyIntervals        = 4096
	CalendarGoogleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	CalendarGoogleTokenEndpoint         = "https://oauth2.googleapis.com/token"
	CalendarGoogleBusyScope             = "https://www.googleapis.com/auth/calendar.events.freebusy"
	CalendarGoogleListScope             = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
	CalendarGoogleWriteScope            = "https://www.googleapis.com/auth/calendar.events"
)

// CalendarCredentialKind closes the purpose of secret provider material.
// These credentials authorize provider execution, never an application login.
type CalendarCredentialKind uint8

const (
	CalendarCredentialInvalid CalendarCredentialKind = iota
	CalendarCredentialAccess
	CalendarCredentialRefresh
	CalendarCredentialCalDAVPassword
)

func (k CalendarCredentialKind) Validate() error {
	if k < CalendarCredentialAccess || k > CalendarCredentialCalDAVPassword {
		return ErrCalendarContract
	}
	return nil
}

// CalendarCredential borrows the existing independently copied, destruction-
// shared OAuth secret custody. It cannot be serialized or printed as a token.
type CalendarCredential struct {
	secret OAuthClientSecret
	kind   CalendarCredentialKind
}

func NewCalendarCredential(kind CalendarCredentialKind, value []byte) (CalendarCredential, error) {
	if err := kind.Validate(); err != nil {
		return CalendarCredential{}, err
	}
	if len(value) == 0 || len(value) > CalendarCredentialMaximumBytes {
		return CalendarCredential{}, ErrCalendarContract
	}
	for _, b := range value {
		if b < 33 || b > 126 {
			return CalendarCredential{}, ErrCalendarContract
		}
	}
	secret, err := NewOAuthClientSecret(value)
	if err != nil {
		return CalendarCredential{}, errors.Join(ErrCalendarContract, err)
	}
	return CalendarCredential{secret: secret, kind: kind}, nil
}
func (c CalendarCredential) Validate() error {
	if err := errors.Join(c.kind.Validate(), c.secret.Validate()); err != nil {
		return errors.Join(ErrCalendarContract, err)
	}
	return nil
}
func (c CalendarCredential) Kind() CalendarCredentialKind { return c.kind }
func (c CalendarCredential) IsSet() bool                  { return c.kind != CalendarCredentialInvalid }
func (c CalendarCredential) CopyBytes() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c.secret.CopyBytes()
}
func (c *CalendarCredential) Close() error {
	if c == nil {
		return ErrCalendarContract
	}
	return c.secret.Close()
}
func (CalendarCredential) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, RedactedValueText) }
func (CalendarCredential) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(ErrCalendarContract, ErrJSONContract)
}

// CalendarIdentifier is a provider calendar coordinate, not an account or
// authorization capability. Its exact opaque spelling is retained.
type CalendarIdentifier struct{ value string }

func ParseCalendarIdentifier(value string) (CalendarIdentifier, error) {
	if len(value) == 0 || len(value) > CalendarIdentifierMaximumBytes {
		return CalendarIdentifier{}, ErrCalendarContract
	}
	for _, b := range []byte(value) {
		if b < 33 || b > 126 {
			return CalendarIdentifier{}, ErrCalendarContract
		}
	}
	return CalendarIdentifier{value: value}, nil
}
func (v CalendarIdentifier) Validate() error { _, err := ParseCalendarIdentifier(v.value); return err }
func (v CalendarIdentifier) String() string  { return v.value }

// CalendarEventIdentifier follows Google's documented base32hex event ID
// alphabet. Callers supply a deterministic ID for retry-safe event insertion.
type CalendarEventIdentifier struct{ value string }

func ParseCalendarEventIdentifier(value string) (CalendarEventIdentifier, error) {
	if len(value) < CalendarEventIdentifierMinimumBytes || len(value) > CalendarIdentifierMaximumBytes {
		return CalendarEventIdentifier{}, ErrCalendarContract
	}
	for _, b := range []byte(value) {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'v') {
			return CalendarEventIdentifier{}, ErrCalendarContract
		}
	}
	return CalendarEventIdentifier{value: value}, nil
}
func (v CalendarEventIdentifier) Validate() error {
	_, err := ParseCalendarEventIdentifier(v.value)
	return err
}
func (v CalendarEventIdentifier) String() string { return v.value }
