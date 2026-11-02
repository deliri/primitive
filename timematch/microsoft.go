package timematch

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// Microsoft Graph v1.0.
// List calendars: https://learn.microsoft.com/en-us/graph/api/user-list-calendars
// calendarView:   https://learn.microsoft.com/en-us/graph/api/calendar-list-calendarview
// Event resource: https://learn.microsoft.com/en-us/graph/api/resources/event
// Create/update/delete: calendar-post-events, event-update, event-delete.
const (
	graphHost          = "graph.microsoft.com"
	graphBase          = "https://" + graphHost + "/v1.0"
	graphCalendarsPath = "/v1.0/me/calendars"
	// calendarView $top: "minimum value of 1 and maximum of 1000".
	graphCalendarViewPageSize = "1000"
	// "Use this to specify the time zone for start and end times in the response."
	graphPreferUTC = `outlook.timezone="UTC"`
	graphUTCZone   = "UTC"
	// Graph DateTimeTimeZone dateTime has no offset; its zone is a sibling field.
	graphDateTimeLayout = "2006-01-02T15:04:05.0000000"
)

// PrimaryCalendar names the account's default calendar for every provider
// that has one (Google's documented "primary" calendar id; Graph's
// /me/calendar). A visitor matches against it without listing calendars.
const PrimaryCalendar = "primary"

// graphCalendarPath is /me/calendar for the default calendar, otherwise the
// named calendar. Graph calendar ids are opaque base64 and never "primary".
func graphCalendarPath(id CalendarID) string {
	if id.value == PrimaryCalendar {
		return "/v1.0/me/calendar"
	}
	return graphCalendarsPath + "/" + url.PathEscape(id.value)
}

// graphNext admits an @odata.nextLink only on the same Graph host and path.
// A link anywhere else would carry the bearer credential to another origin.
func graphNext(link string, path string) (core.HTTPEndpoint, error) {
	next, err := endpoint(link)
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	u := next.HTTPURL()
	if u.Scheme != core.SchemeHTTPS || u.Host != graphHost || u.Path != path || u.User != nil {
		return core.HTTPEndpoint{}, core.ErrTimeMatchResponse
	}
	return next, nil
}

func (c Client) microsoftCalendarList(ctx context.Context, r CalendarListRequest) (CalendarList, error) {
	secrets, bearerErr := bearer(r.Connection.AccessToken)
	target, targetErr := endpoint(graphBase + "/me/calendars?" + url.Values{"$select": {"id,canEdit,isDefaultCalendar"}}.Encode())
	if setupErr := errors.Join(bearerErr, targetErr); setupErr != nil {
		return CalendarList{}, setupErr
	}
	list := CalendarList{Provider: ProviderMicrosoft}
	visited := map[string]struct{}{}
	for {
		result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, secrets: secrets, responseType: core.HTTPMediaTypeJSON(), policy: r.Policy, expected: http.StatusOK})
		if err != nil {
			return CalendarList{}, readFailure(result, err)
		}
		page, err := decodeJSON[graphCalendarPage](result.body)
		if err != nil {
			return CalendarList{}, err
		}
		list.Calendars = append(list.Calendars, page.items...)
		if page.next == "" {
			break
		}
		if _, repeated := visited[page.next]; repeated {
			return CalendarList{}, core.ErrTimeMatchResponse
		}
		visited[page.next] = struct{}{}
		if target, err = graphNext(page.next, graphCalendarsPath); err != nil {
			return CalendarList{}, err
		}
	}
	if err := list.Validate(); err != nil {
		return CalendarList{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return list, nil
}

type graphCalendarPage struct {
	items []Calendar
	next  string
}

func (p *graphCalendarPage) UnmarshalJSON(raw []byte) error {
	var document struct {
		Value []struct {
			ID                string `json:"id"`
			CanEdit           bool   `json:"canEdit"`
			IsDefaultCalendar bool   `json:"isDefaultCalendar"`
		} `json:"value"`
		NextLink string `json:"@odata.nextLink"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	page := graphCalendarPage{next: document.NextLink, items: make([]Calendar, 0, len(document.Value))}
	for _, item := range document.Value {
		id, err := ParseCalendarID(item.ID)
		if err != nil {
			return errors.Join(core.ErrTimeMatchResponse, err)
		}
		page.items = append(page.items, Calendar{ID: id, Writable: item.CanEdit, Primary: item.IsDefaultCalendar})
	}
	*p = page
	return nil
}

func (graphCalendarPage) Validate() error { return nil }

// microsoftAvailability reads each calendar's calendarView. A calendar whose
// view fails (other than consent), or which contains an event this package
// cannot place in absolute time, is Unknown.
func (c Client) microsoftAvailability(ctx context.Context, r AvailabilityRequest) (Availability, error) {
	secrets, bearerErr := bearer(r.Connection.AccessToken)
	prefer, preferErr := header("Prefer", graphPreferUTC)
	start, startErr := r.Window.Start.RFC3339Nano()
	end, endErr := r.Window.End.RFC3339Nano()
	if setupErr := errors.Join(bearerErr, preferErr, startErr, endErr); setupErr != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchContract, setupErr)
	}
	availability := Availability{Window: r.Window, Calendars: make([]CalendarAvailability, 0, len(r.Calendars))}
	for _, id := range r.Calendars {
		path := graphCalendarPath(id) + "/calendarView"
		query := url.Values{"startDateTime": {start}, "endDateTime": {end}, "$select": {"start,end,showAs,isCancelled,isAllDay"}, "$top": {graphCalendarViewPageSize}}
		target, err := endpoint("https://" + graphHost + path + "?" + query.Encode())
		if err != nil {
			return Availability{}, err
		}
		spans, known, err := c.graphCalendarView(ctx, call{target: target, method: exchange.MethodGet, secrets: secrets, headers: []exchange.Header{prefer}, responseType: core.HTTPMediaTypeJSON(), policy: r.Policy, expected: http.StatusOK}, path)
		if err != nil {
			return Availability{}, err
		}
		calendar := CalendarAvailability{ID: id, State: AvailabilityUnknown}
		if known {
			busy, normalizeErr := normalizeBusy(r.Window, spans)
			if normalizeErr != nil {
				return Availability{}, normalizeErr
			}
			calendar = CalendarAvailability{ID: id, State: AvailabilityKnown, Busy: busy}
		}
		availability.Calendars = append(availability.Calendars, calendar)
	}
	if err := availability.covers(r); err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return availability, nil
}

func (c Client) graphCalendarView(ctx context.Context, request call, path string) ([][2]int64, bool, error) {
	var spans [][2]int64
	visited := map[string]struct{}{}
	for {
		result, err := c.send(ctx, request)
		if err != nil {
			failure := readFailure(result, err)
			if !providerUnknown(ctx, failure) {
				return nil, false, failure
			}
			return nil, false, nil
		}
		page, err := decodeJSON[graphViewPage](result.body)
		if err != nil || !page.known {
			return nil, false, nil
		}
		spans = append(spans, page.busy...)
		if page.next == "" {
			return spans, true, nil
		}
		if _, repeated := visited[page.next]; repeated {
			return nil, false, nil
		}
		visited[page.next] = struct{}{}
		if request.target, err = graphNext(page.next, path); err != nil {
			return nil, false, nil
		}
	}
}

type graphDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

func (t graphDateTime) instant() (int64, error) {
	if t.TimeZone != graphUTCZone {
		return 0, core.ErrTimeMatchResponse
	}
	// The zone is a sibling field; with it fixed to UTC the dateTime is an
	// RFC 3339 local time, parsed by Temporal.
	instant, err := temporal.ParseRFC3339(t.DateTime + "Z")
	if err != nil {
		return 0, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return instantNanoseconds(instant)
}

type graphViewPage struct {
	busy  [][2]int64
	next  string
	known bool
}

// showAs is closed: free, tentative, busy, oof, workingElsewhere, unknown.
// free and workingElsewhere do not block; tentative, busy and oof do;
// unknown cannot be classified and makes the calendar Unknown. All-day
// blocking events have no absolute instants in this projection: Unknown.
func (p *graphViewPage) UnmarshalJSON(raw []byte) error {
	var document struct {
		Value []struct {
			Start       graphDateTime `json:"start"`
			End         graphDateTime `json:"end"`
			ShowAs      string        `json:"showAs"`
			IsCancelled bool          `json:"isCancelled"`
			IsAllDay    bool          `json:"isAllDay"`
		} `json:"value"`
		NextLink string `json:"@odata.nextLink"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	page := graphViewPage{next: document.NextLink, known: true}
	for _, event := range document.Value {
		blocking := false
		switch event.ShowAs {
		case "free", "workingElsewhere":
		case "tentative", "busy", "oof":
			blocking = true
		default:
			page.known = false
		}
		if event.IsCancelled || !blocking {
			continue
		}
		if event.IsAllDay {
			page.known = false
			continue
		}
		start, startErr := event.Start.instant()
		end, endErr := event.End.instant()
		if errors.Join(startErr, endErr) != nil || start >= end {
			page.known = false
			continue
		}
		page.busy = append(page.busy, [2]int64{start, end})
	}
	*p = page
	return nil
}

func (graphViewPage) Validate() error { return nil }

type graphEventBody struct {
	Subject       string        `json:"subject"`
	Start         graphDateTime `json:"start"`
	End           graphDateTime `json:"end"`
	ShowAs        string        `json:"showAs"`
	TransactionID string        `json:"transactionId,omitzero"`
	IsReminderOn  bool          `json:"isReminderOn"`
}

func graphEvent(intent EventIntent, create bool) ([]byte, error) {
	start, startErr := intent.Interval.Start.Time()
	end, endErr := intent.Interval.End.Time()
	if err := errors.Join(startErr, endErr); err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	body := graphEventBody{
		Subject: intent.Summary.value, ShowAs: "busy",
		Start: graphDateTime{DateTime: start.UTC().Format(graphDateTimeLayout), TimeZone: graphUTCZone},
		End:   graphDateTime{DateTime: end.UTC().Format(graphDateTimeLayout), TimeZone: graphUTCZone},
	}
	if create {
		// transactionId: "A custom identifier specified by a client app for the
		// server to avoid redundant POST operations in case of client retries
		// to create the same event." It cannot change in a later update.
		body.TransactionID = intent.ID.value
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	return encoded, nil
}

// microsoftPut creates the event with transactionId = EventID. Graph does not
// document its duplicate-transactionId response, so the returned event is
// always verified against the intent; a retry after Indeterminate reuses the
// same transactionId as documented.
func (c Client) microsoftPut(ctx context.Context, w EventWrite) (EventReceipt, error) {
	secrets, err := bearer(w.Connection.AccessToken)
	if err != nil {
		return EventReceipt{}, err
	}
	prefer, err := header("Prefer", graphPreferUTC)
	if err != nil {
		return EventReceipt{}, err
	}
	body, err := graphEvent(w.Intent, true)
	if err != nil {
		return EventReceipt{}, err
	}
	target, err := endpoint("https://" + graphHost + graphCalendarPath(w.Intent.Calendar) + "/events")
	if err != nil {
		return EventReceipt{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPost, body: body, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), headers: []exchange.Header{prefer}, secrets: secrets, policy: w.Policy, expected: http.StatusCreated})
	if err != nil {
		return EventReceipt{}, writeFailure(result, err)
	}
	return verifyGraph(result.body, graphExpectation{intent: w.Intent, calendar: w.Intent.Calendar, create: true})
}

func (c Client) microsoftUpdate(ctx context.Context, u EventUpdate) (EventReceipt, error) {
	secrets, err := bearer(u.Connection.AccessToken)
	if err != nil {
		return EventReceipt{}, err
	}
	prefer, err := header("Prefer", graphPreferUTC)
	if err != nil {
		return EventReceipt{}, err
	}
	body, err := graphEvent(u.Intent, false)
	if err != nil {
		return EventReceipt{}, err
	}
	target, err := endpoint(graphBase + "/me/events/" + url.PathEscape(u.Ref.Resource))
	if err != nil {
		return EventReceipt{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPatch, body: body, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), headers: []exchange.Header{prefer}, secrets: secrets, policy: u.Policy, expected: http.StatusOK})
	if err != nil {
		return EventReceipt{}, writeFailure(result, err)
	}
	return verifyGraph(result.body, graphExpectation{intent: u.Intent, calendar: u.Ref.Calendar, resource: u.Ref.Resource})
}

func (c Client) microsoftDelete(ctx context.Context, d EventDelete) (DeleteOutcome, error) {
	secrets, err := bearer(d.Connection.AccessToken)
	if err != nil {
		return DeleteInvalid, err
	}
	target, err := endpoint(graphBase + "/me/events/" + url.PathEscape(d.Ref.Resource))
	if err != nil {
		return DeleteInvalid, err
	}
	return c.deleteResource(ctx, call{target: target, method: exchange.MethodDelete, secrets: secrets, policy: d.Policy, expected: http.StatusNoContent})
}

type graphEventObservation struct {
	id            string
	subject       string
	transactionID string
	start         graphDateTime
	end           graphDateTime
	cancelled     bool
}

func (o *graphEventObservation) UnmarshalJSON(raw []byte) error {
	var document struct {
		ID            string        `json:"id"`
		Subject       string        `json:"subject"`
		TransactionID string        `json:"transactionId"`
		Start         graphDateTime `json:"start"`
		End           graphDateTime `json:"end"`
		IsCancelled   bool          `json:"isCancelled"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	*o = graphEventObservation{id: document.ID, subject: document.Subject, transactionID: document.TransactionID, start: document.Start, end: document.End, cancelled: document.IsCancelled}
	return nil
}

func (o graphEventObservation) Validate() error {
	if !visibleASCII([]byte(o.id)) {
		return core.ErrTimeMatchResponse
	}
	return nil
}

// graphExpectation is the owned intent a returned Graph event must match:
// a created event by transactionId, an updated event by its Graph id.
type graphExpectation struct {
	calendar CalendarID
	resource string
	intent   EventIntent
	create   bool
}

func verifyGraph(body []byte, expected graphExpectation) (EventReceipt, error) {
	intent, calendar, resource, create := expected.intent, expected.calendar, expected.resource, expected.create
	defer clear(body)
	observed, err := decodeJSON[graphEventObservation](body)
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	start, startErr := observed.start.instant()
	end, endErr := observed.end.instant()
	if err := errors.Join(startErr, endErr); err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	if observed.cancelled || observed.subject != intent.Summary.value || !sameSpan([2]int64{start, end}, intent.Interval) ||
		create && observed.transactionID != intent.ID.value || !create && observed.id != resource {
		return EventReceipt{}, core.ErrTimeMatchConflict
	}
	return EventReceipt{Ref: EventRef{Provider: ProviderMicrosoft, Calendar: calendar, Resource: observed.id}}, nil
}
