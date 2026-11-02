package timematch

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

// ListCalendars returns every calendar the connection can see, reading all
// provider pages. Display names, colors and owners are not projected.
func (c Client) ListCalendars(ctx context.Context, r CalendarListRequest) (CalendarList, error) {
	if err := c.begin(ctx, r); err != nil {
		return CalendarList{}, err
	}
	switch r.Connection.Provider {
	case ProviderGoogle:
		return c.googleCalendarList(ctx, r)
	case ProviderMicrosoft:
		return c.microsoftCalendarList(ctx, r)
	case ProviderApple:
		return c.appleCalendarList(ctx, r)
	default:
		return CalendarList{}, core.ErrTimeMatchContract
	}
}

// Availability returns busy intervals per requested calendar. A calendar the
// provider could not fully answer is AvailabilityUnknown, never free.
func (c Client) Availability(ctx context.Context, r AvailabilityRequest) (Availability, error) {
	if err := c.begin(ctx, r); err != nil {
		return Availability{}, err
	}
	switch r.Connection.Provider {
	case ProviderGoogle:
		return c.googleAvailability(ctx, r)
	case ProviderMicrosoft:
		return c.microsoftAvailability(ctx, r)
	case ProviderApple:
		return c.appleAvailability(ctx, r)
	default:
		return Availability{}, core.ErrTimeMatchContract
	}
}

// PutEvent creates the owned event or verifies that the same intent already
// exists. ErrTimeMatchConflict: a different event holds that identity.
// ErrTimeMatchIndeterminate: the request was sent and its outcome is unknown;
// retry PutEvent with the identical intent.
func (c Client) PutEvent(ctx context.Context, w EventWrite) (EventReceipt, error) {
	if err := c.begin(ctx, w); err != nil {
		return EventReceipt{}, err
	}
	switch w.Connection.Provider {
	case ProviderGoogle:
		return c.googlePut(ctx, w)
	case ProviderMicrosoft:
		return c.microsoftPut(ctx, w)
	case ProviderApple:
		return c.applePut(ctx, w)
	default:
		return EventReceipt{}, core.ErrTimeMatchContract
	}
}

// UpdateEvent replaces an owned event's interval and summary.
func (c Client) UpdateEvent(ctx context.Context, u EventUpdate) (EventReceipt, error) {
	if err := c.begin(ctx, u); err != nil {
		return EventReceipt{}, err
	}
	switch u.Connection.Provider {
	case ProviderGoogle:
		return c.googleUpdate(ctx, u)
	case ProviderMicrosoft:
		return c.microsoftUpdate(ctx, u)
	case ProviderApple:
		return c.appleUpdate(ctx, u)
	default:
		return EventReceipt{}, core.ErrTimeMatchContract
	}
}

// DeleteEvent removes an owned event; an absent event is DeleteAlreadyAbsent.
func (c Client) DeleteEvent(ctx context.Context, d EventDelete) (DeleteOutcome, error) {
	if err := c.begin(ctx, d); err != nil {
		return DeleteInvalid, err
	}
	switch d.Connection.Provider {
	case ProviderGoogle:
		return c.googleDelete(ctx, d)
	case ProviderMicrosoft:
		return c.microsoftDelete(ctx, d)
	case ProviderApple:
		return c.appleDelete(ctx, d)
	default:
		return DeleteInvalid, core.ErrTimeMatchContract
	}
}
