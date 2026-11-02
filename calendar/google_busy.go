package calendar

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleFreeBusy returns only checked busy windows. No event title, attendee,
// description or contact information crosses this boundary. The entire result
// fails when any requested calendar is absent or refused by Google.
func GoogleFreeBusy(ctx context.Context, r FreeBusyRequest) (FreeBusyResult, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return FreeBusyResult{}, errors.Join(core.ErrCalendarContract, err)
	}
	operation, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return FreeBusyResult{}, err
	}
	defer cancel()
	// No SDK retry: the parent owns operation idempotency and request budgets.
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationJSON})
	if err != nil {
		return FreeBusyResult{}, err
	}
	transport, err := r.Client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return FreeBusyResult{}, err
	}
	secret, err := r.Credential.CopyBytes()
	if err != nil {
		return FreeBusyResult{}, err
	}
	defer clear(secret)
	token := &oauth2.Token{AccessToken: string(secret), TokenType: "Bearer"}
	defer func() { token.AccessToken = "" }()
	client, err := exchange.NewOfficialSDKHTTPClient(&oauth2.Transport{Source: oauth2.StaticTokenSource(token), Base: transport})
	if err != nil {
		return FreeBusyResult{}, err
	}
	service, err := googlecalendar.NewService(operation, option.WithHTTPClient(client))
	if err != nil {
		return FreeBusyResult{}, errors.Join(core.ErrCalendarContract, err)
	}
	start, err := r.Window.Start.RFC3339Nano()
	if err != nil {
		return FreeBusyResult{}, err
	}
	end, err := r.Window.End.RFC3339Nano()
	if err != nil {
		return FreeBusyResult{}, err
	}
	request := &googlecalendar.FreeBusyRequest{TimeMin: start, TimeMax: end, CalendarExpansionMax: core.CalendarFreeBusyMaximumCalendars}
	for _, id := range r.Calendars {
		request.Items = append(request.Items, &googlecalendar.FreeBusyRequestItem{Id: id.String()})
	}
	response, err := service.Freebusy.Query(request).Context(operation).Do()
	if err != nil {
		return FreeBusyResult{}, errors.Join(core.ErrCalendarAvailabilityUnknown, exchange.SanitizeOAuthTokenError(err))
	}
	if response == nil || len(response.Groups) != 0 || len(response.Calendars) != len(r.Calendars) {
		return FreeBusyResult{}, core.ErrCalendarAvailabilityUnknown
	}
	responseStart, err := temporal.ParseRFC3339(response.TimeMin)
	if err != nil || responseStart != r.Window.Start {
		return FreeBusyResult{}, core.ErrCalendarAvailabilityUnknown
	}
	responseEnd, err := temporal.ParseRFC3339(response.TimeMax)
	if err != nil || responseEnd != r.Window.End {
		return FreeBusyResult{}, core.ErrCalendarAvailabilityUnknown
	}
	result := FreeBusyResult{Window: r.Window, Calendars: make([]BusyCalendar, 0, len(r.Calendars))}
	for _, id := range r.Calendars {
		calendar, ok := response.Calendars[id.String()]
		if !ok || len(calendar.Errors) != 0 || len(calendar.Busy) > core.CalendarMaximumBusyIntervals {
			return FreeBusyResult{}, core.ErrCalendarAvailabilityUnknown
		}
		busy := BusyCalendar{ID: id, Busy: make([]temporal.IntervalBounds, 0, len(calendar.Busy))}
		for _, item := range calendar.Busy {
			if item == nil {
				return FreeBusyResult{}, core.ErrCalendarAvailabilityUnknown
			}
			start, parseErr := temporal.ParseRFC3339(item.Start)
			if parseErr != nil {
				return FreeBusyResult{}, errors.Join(core.ErrCalendarAvailabilityUnknown, parseErr)
			}
			end, parseErr := temporal.ParseRFC3339(item.End)
			if parseErr != nil {
				return FreeBusyResult{}, errors.Join(core.ErrCalendarAvailabilityUnknown, parseErr)
			}
			span := temporal.IntervalBounds{Start: start, End: end}
			elapsed, spanErr := end.Since(start)
			if spanErr != nil || elapsed.IsZero() {
				return FreeBusyResult{}, core.ErrCalendarAvailabilityUnknown
			}
			busy.Busy = append(busy.Busy, span)
		}
		result.Calendars = append(result.Calendars, busy)
	}
	if err := result.Validate(); err != nil {
		return FreeBusyResult{}, errors.Join(core.ErrCalendarAvailabilityUnknown, err)
	}
	return result, nil
}
