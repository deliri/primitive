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

// Google Calendar API v3. https://developers.google.com/workspace/calendar/api/v3/reference
const (
	googleCalendarBase = "https://www.googleapis.com/calendar/v3"
	// calendarList.list: "The page size can never be larger than 250 entries."
	googleCalendarListPageSize = "250"
	// freeBusy.query calendarExpansionMax: "Maximum value is 50."
	googleFreeBusyBatch = 50
)

// googleCalendarList reads every page. A repeated page token is a provider
// loop, never a truncated success.
func (c Client) googleCalendarList(ctx context.Context, r CalendarListRequest) (CalendarList, error) {
	secrets, err := bearer(r.Connection.AccessToken)
	if err != nil {
		return CalendarList{}, err
	}
	list := CalendarList{Provider: ProviderGoogle}
	seen := map[string]struct{}{}
	pageToken := ""
	for {
		query := url.Values{}
		query.Set("maxResults", googleCalendarListPageSize)
		query.Set("fields", "items(id,accessRole,primary),nextPageToken")
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		target, err := endpoint(googleCalendarBase + "/users/me/calendarList?" + query.Encode())
		if err != nil {
			return CalendarList{}, err
		}
		result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, secrets: secrets, responseType: core.HTTPMediaTypeJSON(), policy: r.Policy, expected: http.StatusOK})
		if err != nil {
			return CalendarList{}, readFailure(result, err)
		}
		page, err := decodeJSON[googleCalendarPage](result.body)
		if err != nil {
			return CalendarList{}, err
		}
		for _, item := range page.items {
			if _, duplicate := seen[item.ID.value]; duplicate {
				return CalendarList{}, core.ErrTimeMatchResponse
			}
			seen[item.ID.value] = struct{}{}
			list.Calendars = append(list.Calendars, item)
		}
		if page.next == "" {
			break
		}
		if _, repeated := seen["\x00"+page.next]; repeated {
			return CalendarList{}, core.ErrTimeMatchResponse
		}
		seen["\x00"+page.next] = struct{}{}
		pageToken = page.next
	}
	return list, list.Validate()
}

type googleCalendarPage struct {
	items []Calendar
	next  string
}

func (p *googleCalendarPage) UnmarshalJSON(raw []byte) error {
	var document struct {
		Items []struct {
			ID         string `json:"id"`
			AccessRole string `json:"accessRole"`
			Primary    bool   `json:"primary"`
		} `json:"items"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	page := googleCalendarPage{next: document.NextPageToken, items: make([]Calendar, 0, len(document.Items))}
	for _, item := range document.Items {
		id, err := ParseCalendarID(item.ID)
		if err != nil {
			return errors.Join(core.ErrTimeMatchResponse, err)
		}
		// CalendarList accessRole is closed: freeBusyReader, reader, writer,
		// owner. calendar.events.owned writes only calendars the user owns.
		switch item.AccessRole {
		case "freeBusyReader", "reader", "writer", "owner":
		default:
			return core.ErrTimeMatchResponse
		}
		page.items = append(page.items, Calendar{ID: id, Primary: item.Primary, Writable: item.AccessRole == "owner"})
	}
	*p = page
	return nil
}

func (p googleCalendarPage) Validate() error {
	if p.next != "" && !visibleASCII([]byte(p.next)) {
		return core.ErrTimeMatchResponse
	}
	return nil
}

// googleAvailability queries freeBusy in documented batches. A calendar with
// errors[] (notFound, internalError, ...) or absent from the response is
// Unknown. A response whose window differs from the request is refused.
func (c Client) googleAvailability(ctx context.Context, r AvailabilityRequest) (Availability, error) {
	secrets, err := bearer(r.Connection.AccessToken)
	if err != nil {
		return Availability{}, err
	}
	target, err := endpoint(googleCalendarBase + "/freeBusy")
	if err != nil {
		return Availability{}, err
	}
	start, err := r.Window.Start.RFC3339Nano()
	if err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	end, err := r.Window.End.RFC3339Nano()
	if err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	availability := Availability{Window: r.Window, Calendars: make([]CalendarAvailability, 0, len(r.Calendars))}
	for first := 0; first < len(r.Calendars); first += googleFreeBusyBatch {
		batch := r.Calendars[first:min(first+googleFreeBusyBatch, len(r.Calendars))]
		body := googleFreeBusyBody{TimeMin: start, TimeMax: end, Items: make([]googleFreeBusyItem, 0, len(batch))}
		for _, id := range batch {
			body.Items = append(body.Items, googleFreeBusyItem{ID: id.value})
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return Availability{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		result, err := c.send(ctx, call{target: target, method: exchange.MethodPost, body: encoded, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: r.Policy, expected: http.StatusOK})
		if err != nil {
			return Availability{}, readFailure(result, err)
		}
		response, err := decodeJSON[googleFreeBusyResponse](result.body)
		if err != nil {
			return Availability{}, err
		}
		if response.timeMin != r.Window.Start || response.timeMax != r.Window.End {
			return Availability{}, core.ErrTimeMatchResponse
		}
		for _, id := range batch {
			calendar := CalendarAvailability{ID: id, State: AvailabilityUnknown}
			if entry, ok := response.calendars[id.value]; ok && entry.known {
				busy, err := normalizeBusy(r.Window, entry.busy)
				if err != nil {
					return Availability{}, err
				}
				calendar = CalendarAvailability{ID: id, State: AvailabilityKnown, Busy: busy}
			}
			availability.Calendars = append(availability.Calendars, calendar)
		}
	}
	if err := availability.covers(r); err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return availability, nil
}

type googleFreeBusyItem struct {
	ID string `json:"id"`
}

type googleFreeBusyBody struct {
	TimeMin string               `json:"timeMin"`
	TimeMax string               `json:"timeMax"`
	Items   []googleFreeBusyItem `json:"items"`
}

type googleBusyCalendar struct {
	busy  [][2]int64
	known bool
}

type googleFreeBusyResponse struct {
	calendars map[string]googleBusyCalendar
	timeMin   temporal.Instant
	timeMax   temporal.Instant
}

func (r *googleFreeBusyResponse) UnmarshalJSON(raw []byte) error {
	var document struct {
		Calendars map[string]struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Busy []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"busy"`
		} `json:"calendars"`
		TimeMin string `json:"timeMin"`
		TimeMax string `json:"timeMax"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	timeMin, minErr := temporal.ParseRFC3339(document.TimeMin)
	timeMax, maxErr := temporal.ParseRFC3339(document.TimeMax)
	if err := errors.Join(minErr, maxErr); err != nil {
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
	response := googleFreeBusyResponse{timeMin: timeMin, timeMax: timeMax, calendars: make(map[string]googleBusyCalendar, len(document.Calendars))}
	for id, calendar := range document.Calendars {
		entry := googleBusyCalendar{known: len(calendar.Errors) == 0}
		for _, busy := range calendar.Busy {
			span, err := rfc3339Span(busy.Start, busy.End)
			if err != nil {
				// One malformed fact makes that calendar Unknown, not partially free.
				entry = googleBusyCalendar{}
				break
			}
			entry.busy = append(entry.busy, span)
		}
		response.calendars[id] = entry
	}
	*r = response
	return nil
}

func (r googleFreeBusyResponse) Validate() error {
	return errors.Join(r.timeMin.Validate(), r.timeMax.Validate())
}

// rfc3339Span admits one positive half-open span (start inclusive, end exclusive).
func rfc3339Span(startText, endText string) ([2]int64, error) {
	start, startErr := temporal.ParseRFC3339(startText)
	end, endErr := temporal.ParseRFC3339(endText)
	if err := errors.Join(startErr, endErr); err != nil {
		return [2]int64{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	startNanos, startErr := instantNanoseconds(start)
	endNanos, endErr := instantNanoseconds(end)
	if err := errors.Join(startErr, endErr); err != nil || startNanos >= endNanos {
		return [2]int64{}, core.ErrTimeMatchResponse
	}
	return [2]int64{startNanos, endNanos}, nil
}

// googleEventBody is the owned event: caller id, summary and UTC instants,
// opaque (busy). No attendees exist, and sendUpdates=none is set on every write.
// Google warns sendUpdates=none can affect external synchronization: no
// receipt here proves another person's calendar received anything.
type googleEventTime struct {
	DateTime string `json:"dateTime"`
}

type googleEventBody struct {
	ID           string          `json:"id"`
	Summary      string          `json:"summary"`
	Start        googleEventTime `json:"start"`
	End          googleEventTime `json:"end"`
	Transparency string          `json:"transparency"`
}

func googleEvent(intent EventIntent) ([]byte, error) {
	start, startErr := intent.Interval.Start.RFC3339Nano()
	end, endErr := intent.Interval.End.RFC3339Nano()
	if err := errors.Join(startErr, endErr); err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	encoded, err := json.Marshal(googleEventBody{ID: intent.ID.value, Summary: intent.Summary.value, Start: googleEventTime{DateTime: start}, End: googleEventTime{DateTime: end}, Transparency: "opaque"})
	if err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	return encoded, nil
}

func googleEventURL(calendar CalendarID, id string, sendUpdates bool) (core.HTTPEndpoint, error) {
	raw := googleCalendarBase + "/calendars/" + url.PathEscape(calendar.value) + "/events"
	if id != "" {
		raw += "/" + url.PathEscape(id)
	}
	if sendUpdates {
		raw += "?sendUpdates=none"
	}
	return endpoint(raw)
}

// googlePut inserts with the caller's deterministic id. The insert page states
// only that ids are unique per calendar and that collisions may not be
// detected at creation; the duplicate response is not documented. A 409 is
// therefore only a reason to read the event back, and the read-back decides.
func (c Client) googlePut(ctx context.Context, w EventWrite) (EventReceipt, error) {
	secrets, err := bearer(w.Connection.AccessToken)
	if err != nil {
		return EventReceipt{}, err
	}
	body, err := googleEvent(w.Intent)
	if err != nil {
		return EventReceipt{}, err
	}
	target, err := googleEventURL(w.Intent.Calendar, "", true)
	if err != nil {
		return EventReceipt{}, err
	}
	ref := EventRef{Provider: ProviderGoogle, Calendar: w.Intent.Calendar, Resource: w.Intent.ID.value}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPost, body: body, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: w.Policy, expected: http.StatusOK})
	switch {
	case err == nil:
		return verifyGoogle(result.body, w.Intent, ref)
	case result.status == http.StatusConflict:
		clear(result.body)
		return c.googleReadBack(ctx, w, ref)
	default:
		return EventReceipt{}, writeFailure(result, err)
	}
}

func (c Client) googleReadBack(ctx context.Context, w EventWrite, ref EventRef) (EventReceipt, error) {
	secrets, err := bearer(w.Connection.AccessToken)
	if err != nil {
		return EventReceipt{}, err
	}
	target, err := googleEventURL(w.Intent.Calendar, w.Intent.ID.value, false)
	if err != nil {
		return EventReceipt{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: w.Policy, expected: http.StatusOK})
	if err != nil {
		// The id is taken but cannot be read: not ours to claim.
		return EventReceipt{}, errors.Join(core.ErrTimeMatchConflict, readFailure(result, err))
	}
	return verifyGoogle(result.body, w.Intent, ref)
}

func (c Client) googleUpdate(ctx context.Context, u EventUpdate) (EventReceipt, error) {
	secrets, err := bearer(u.Connection.AccessToken)
	if err != nil {
		return EventReceipt{}, err
	}
	if u.Ref.Resource != u.Intent.ID.value {
		return EventReceipt{}, core.ErrTimeMatchContract
	}
	body, err := googleEvent(u.Intent)
	if err != nil {
		return EventReceipt{}, err
	}
	target, err := googleEventURL(u.Intent.Calendar, u.Ref.Resource, true)
	if err != nil {
		return EventReceipt{}, err
	}
	// events.update "always updates the entire event resource".
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPut, body: body, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: u.Policy, expected: http.StatusOK})
	if err != nil {
		return EventReceipt{}, writeFailure(result, err)
	}
	return verifyGoogle(result.body, u.Intent, u.Ref)
}

func (c Client) googleDelete(ctx context.Context, d EventDelete) (DeleteOutcome, error) {
	secrets, err := bearer(d.Connection.AccessToken)
	if err != nil {
		return DeleteInvalid, err
	}
	target, err := googleEventURL(d.Ref.Calendar, d.Ref.Resource, true)
	if err != nil {
		return DeleteInvalid, err
	}
	return c.deleteResource(ctx, call{target: target, method: exchange.MethodDelete, secrets: secrets, policy: d.Policy, expected: http.StatusNoContent})
}

// deleteResource maps 404 and 410 to an already absent resource.
func (c Client) deleteResource(ctx context.Context, request call) (DeleteOutcome, error) {
	result, err := c.send(ctx, request)
	clear(result.body)
	switch {
	case err == nil:
		return Deleted, nil
	case result.status == http.StatusNotFound || result.status == http.StatusGone:
		return DeleteAlreadyAbsent, nil
	default:
		return DeleteInvalid, writeFailure(result, err)
	}
}

type googleEventObservation struct {
	id      string
	status  string
	summary string
	start   string
	end     string
}

func (o *googleEventObservation) UnmarshalJSON(raw []byte) error {
	var document struct {
		ID      string          `json:"id"`
		Status  string          `json:"status"`
		Summary string          `json:"summary"`
		Start   googleEventTime `json:"start"`
		End     googleEventTime `json:"end"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	*o = googleEventObservation{id: document.ID, status: document.Status, summary: document.Summary, start: document.Start.DateTime, end: document.End.DateTime}
	return nil
}

func (o googleEventObservation) Validate() error {
	if o.id == "" {
		return core.ErrTimeMatchResponse
	}
	return nil
}

// verifyGoogle accepts the provider event only when it is the owned intent:
// same id, not cancelled, same summary and the same absolute instants.
func verifyGoogle(body []byte, intent EventIntent, ref EventRef) (EventReceipt, error) {
	defer clear(body)
	observed, err := decodeJSON[googleEventObservation](body)
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	span, err := rfc3339Span(observed.start, observed.end)
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	if observed.id != intent.ID.value || observed.status == "cancelled" || observed.summary != intent.Summary.value || !sameSpan(span, intent.Interval) {
		return EventReceipt{}, core.ErrTimeMatchConflict
	}
	return EventReceipt{Ref: ref}, nil
}

func sameSpan(span [2]int64, interval temporal.IntervalBounds) bool {
	start, end, err := nanosecondBounds(interval)
	return err == nil && span[0] == start && span[1] == end
}
