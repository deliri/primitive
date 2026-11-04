package timematch

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

// Google Calendar API v3. https://developers.google.com/workspace/calendar/api/v3/reference
const (
	googleCalendarBase = "https://www.googleapis.com/calendar/v3"
	// calendarList.list: "The page size can never be larger than 250 entries."
	googleCalendarListPageSize = "250"
	// freeBusy.query calendarExpansionMax: "Maximum value is 50."
	googleFreeBusyBatch       = 50
	googleOwnershipProperty   = "timematchEventId"
	googleOwnedMetadataFields = "items(id,etag),nextPageToken"
	googleOwnedDetailsFields  = "id,etag,status,summary,location,start/dateTime,end/dateTime,extendedProperties/private"
)

// googleCalendarList reads every page. A repeated page token is a provider
// loop, never a truncated success.
func (c Client) googleCalendarList(ctx context.Context, r CalendarListRequest) (CalendarList, error) {
	secrets, bearerErr := bearer(r.Connection.AccessToken)
	if bearerErr != nil {
		return CalendarList{}, bearerErr
	}
	list := CalendarList{Provider: ProviderGoogle}
	seen := map[string]struct{}{}
	dataOwners := map[string]string{}
	primary := ""
	pageToken := ""
	for {
		query := url.Values{}
		query.Set("maxResults", googleCalendarListPageSize)
		query.Set("fields", "items(id,accessRole,primary,dataOwner),nextPageToken")
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
			if item.Primary {
				if primary != "" {
					return CalendarList{}, core.ErrTimeMatchResponse
				}
				primary = item.ID.value
			}
			dataOwners[item.ID.value] = page.dataOwners[item.ID.value]
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
	// The manager accessRole "owner" is not data ownership. The narrow
	// calendar.events.owned grant permits only the authenticated user's data.
	// Primary identifies that user; secondary dataOwner must match exactly.
	// https://developers.google.com/workspace/calendar/api/v3/reference/calendarList
	for i := range list.Calendars {
		item := &list.Calendars[i]
		item.Writable = item.Writable && (item.Primary || primary != "" && dataOwners[item.ID.value] == primary)
	}
	return list, list.Validate()
}

type googleCalendarPage struct {
	items      []Calendar
	dataOwners map[string]string
	next       string
}

func (p *googleCalendarPage) UnmarshalJSON(raw []byte) error {
	var document struct {
		Items []struct {
			ID         string `json:"id"`
			AccessRole string `json:"accessRole"`
			Primary    bool   `json:"primary"`
			DataOwner  string `json:"dataOwner"`
		} `json:"items"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	page := googleCalendarPage{next: document.NextPageToken, items: make([]Calendar, 0, len(document.Items)), dataOwners: make(map[string]string, len(document.Items))}
	for _, item := range document.Items {
		id, err := ParseCalendarID(item.ID)
		if err != nil {
			return errors.Join(core.ErrTimeMatchResponse, err)
		}
		// CalendarList accessRole is closed: freeBusyReader, reader, writer,
		// owner and writerWithoutPrivateAccess. calendar.events.owned writes
		// only calendars the user owns.
		switch item.AccessRole {
		case "freeBusyReader", "reader", "writer", "writerWithoutPrivateAccess", "owner":
		default:
			return core.ErrTimeMatchResponse
		}
		page.items = append(page.items, Calendar{ID: id, Primary: item.Primary, Writable: item.AccessRole == "owner"})
		page.dataOwners[id.value] = item.DataOwner
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
	secrets, bearerErr := bearer(r.Connection.AccessToken)
	target, targetErr := endpoint(googleCalendarBase + "/freeBusy")
	start, startErr := r.Window.Start.RFC3339Nano()
	end, endErr := r.Window.End.RFC3339Nano()
	if setupErr := errors.Join(bearerErr, targetErr, startErr, endErr); setupErr != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchContract, setupErr)
	}
	availability := Availability{Window: r.Window, Calendars: make([]CalendarAvailability, 0, len(r.Calendars))}
	for first := 0; first < len(r.Calendars); first += googleFreeBusyBatch {
		batch := r.Calendars[first:min(first+googleFreeBusyBatch, len(r.Calendars))]
		body := googleFreeBusyBody{CalendarExpansionMax: googleFreeBusyBatch, TimeMin: start, TimeMax: end, Items: make([]googleFreeBusyItem, 0, len(batch))}
		for _, id := range batch {
			body.Items = append(body.Items, googleFreeBusyItem{ID: id.value})
		}
		encoded, err := jsonio.MarshalProjection(body, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted})
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
				busy, normalizeErr := normalizeBusy(r.Window, entry.busy)
				if normalizeErr != nil {
					return Availability{}, normalizeErr
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
	CalendarExpansionMax int                  `json:"calendarExpansionMax"`
	TimeMin              string               `json:"timeMin"`
	TimeMax              string               `json:"timeMax"`
	Items                []googleFreeBusyItem `json:"items"`
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
		entry := googleBusyCalendar{known: len(calendar.Errors) == 0 && calendar.Busy != nil}
		for _, busy := range calendar.Busy {
			span, err := rfc3339Span(busy.Start, busy.End)
			if err != nil {
				// One malformed fact makes that calendar Unknown, not partially free.
				entry = googleBusyCalendar{}
				break
			}
			entry.busy = append(entry.busy, span)
		}
		if !entry.known {
			entry.busy = nil
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

// Google private extensions are scoped to this calendarId/eventId copy.
type googlePrivateProperties struct {
	EventID string `json:"timematchEventId"`
}

type googleExtendedProperties struct {
	Private googlePrivateProperties `json:"private"`
}

func (googlePrivateProperties) timematchInternalFlow()  {}
func (googleExtendedProperties) timematchInternalFlow() {}

type googleEventBody struct {
	ConferenceData     *googleConferenceData     `json:"conferenceData,omitzero"`
	Location           string                    `json:"location,omitzero"`
	ID                 string                    `json:"id,omitzero"`
	Summary            string                    `json:"summary"`
	Start              googleEventTime           `json:"start"`
	End                googleEventTime           `json:"end"`
	Transparency       string                    `json:"transparency"`
	ExtendedProperties *googleExtendedProperties `json:"extendedProperties,omitzero"`
}

func googleEventProjection(intent EventIntent) (googleEventBody, error) {
	// The SDK's Google mutation contract is whole UTC seconds. Reject finer
	// instants before dispatch instead of allowing a provider to quantize an
	// accepted write and subsequently misclassifying its readback as conflict.
	startNanos, endNanos, precisionErr := nanosecondBounds(intent.Interval)
	if precisionErr != nil || startNanos%int64(temporal.NanosecondsPerSecond) != 0 || endNanos%int64(temporal.NanosecondsPerSecond) != 0 {
		return googleEventBody{}, errors.Join(core.ErrTimeMatchContract, precisionErr)
	}
	start, startErr := intent.Interval.Start.RFC3339Nano()
	end, endErr := intent.Interval.End.RFC3339Nano()
	if err := errors.Join(startErr, endErr); err != nil {
		return googleEventBody{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return googleEventBody{ConferenceData: googleMeetCreateData(intent), Location: intent.Venue.Text(), Summary: intent.Summary.value, Start: googleEventTime{DateTime: start}, End: googleEventTime{DateTime: end}, Transparency: "opaque"}, nil
}

func googleEvent(intent EventIntent) ([]byte, error) {
	body, err := googleEventProjection(intent)
	if err != nil {
		return nil, err
	}
	body.ID = intent.ID.value
	body.ExtendedProperties = &googleExtendedProperties{Private: googlePrivateProperties{EventID: intent.ID.value}}
	encoded, err := jsonio.MarshalProjection(body, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted})
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
	query := url.Values{}
	if sendUpdates {
		query.Set("sendUpdates", "none")
	}
	if len(query) > 0 {
		raw += "?" + query.Encode()
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
	if err == nil {
		target, err = googleOwnedWriteTarget(target, w.Intent)
	}
	if err != nil {
		return EventReceipt{}, err
	}
	ref := EventRef{ID: w.Intent.ID, Provider: ProviderGoogle, Calendar: w.Intent.Calendar, Resource: w.Intent.ID.value}
	if w.Attempt == EventCreateRecoverOnly {
		return c.googleReadBack(ctx, w, ref, false)
	}
	// The caller retains authority for this exact deterministic intent.
	// Google documents client-supplied IDs as its duplicate-create safeguard;
	// a collision still needs the existing ownership/revision readback.
	// https://developers.google.com/workspace/calendar/api/guides/create-events
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPost, body: body, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: w.Policy, expected: http.StatusOK})
	switch {
	case err == nil:
		return verifyGoogle(result.body, w.Intent, ref)
	case result.status == http.StatusConflict:
		clear(result.body)
		return c.googleReadBack(ctx, w, ref, true)
	default:
		return EventReceipt{}, writeFailure(result, err)
	}
}

// googleOwnedRevision uses Google's documented private-property filter before
// requesting any event details. The field mask reveals only matching IDs and
// revisions; it never downloads an unfiltered appointment or private map.
func (c Client) googleOwnedRevision(ctx context.Context, connection Connection, ref EventRef, policy exchange.OperationPolicy) (exchange.Header, error) {
	if err := ref.Validate(); err != nil || ref.Provider != ProviderGoogle {
		return exchange.Header{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	secrets, err := bearer(connection.AccessToken)
	if err != nil {
		return exchange.Header{}, err
	}
	base, err := googleEventURL(ref.Calendar, "", false)
	if err != nil {
		return exchange.Header{}, err
	}
	query := url.Values{}
	query.Set("privateExtendedProperty", googleOwnershipProperty+"="+ref.ID.value)
	query.Set("fields", googleOwnedMetadataFields)
	// Two entries suffice to disprove one deterministic event's ownership.
	// This is a provider page size, not a cap that hides later pages.
	query.Set("maxResults", "2")
	seen := map[string]bool{}
	var condition exchange.Header
	found := false
	for {
		target, targetErr := endpoint(base.String() + "?" + query.Encode())
		if targetErr != nil {
			return exchange.Header{}, targetErr
		}
		result, readErr := c.send(ctx, call{target: target, method: exchange.MethodGet, responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: policy, expected: http.StatusOK})
		if readErr != nil {
			return exchange.Header{}, errors.Join(core.ErrTimeMatchIndeterminate, readFailure(result, readErr))
		}
		page, decodeErr := decodeJSON[googleOwnedEventPage](result.body)
		clear(result.body)
		if decodeErr != nil {
			return exchange.Header{}, errors.Join(core.ErrTimeMatchIndeterminate, decodeErr)
		}
		for _, observed := range page.Items {
			if observed.ID != ref.ID.value {
				return exchange.Header{}, core.ErrTimeMatchConflict
			}
			if found || !appleStrongETag(observed.ETag) {
				return exchange.Header{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
			}
			condition, err = header("If-Match", observed.ETag)
			if err != nil {
				return exchange.Header{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse, err)
			}
			found = true
		}
		if page.NextPageToken == "" {
			break
		}
		if seen[page.NextPageToken] {
			return exchange.Header{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
		}
		seen[page.NextPageToken] = true
		query.Set("pageToken", page.NextPageToken)
	}
	if !found {
		// A zero condition records an empty filtered ownership observation.
		// The operation owns whether that fact is conflict or still unknown.
		return exchange.Header{}, nil
	}
	return condition, nil
}

type googleOwnedEventMetadata struct {
	ID   string `json:"id"`
	ETag string `json:"etag"`
}

func (googleOwnedEventMetadata) timematchInternalFlow() {}

type googleOwnedEventPage struct {
	Items         []googleOwnedEventMetadata `json:"items"`
	NextPageToken string                     `json:"nextPageToken"`
}

func (googleOwnedEventPage) timematchInternalFlow() {}

func (p *googleOwnedEventPage) UnmarshalJSON(raw []byte) error {
	var document struct {
		Items         []googleOwnedEventMetadata `json:"items"`
		NextPageToken string                     `json:"nextPageToken"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	*p = googleOwnedEventPage{Items: document.Items, NextPageToken: document.NextPageToken}
	return nil
}

func (p googleOwnedEventPage) Validate() error {
	if p.Items == nil || p.NextPageToken != "" && !visibleASCII([]byte(p.NextPageToken)) {
		return core.ErrTimeMatchResponse
	}
	for _, item := range p.Items {
		if item.ID == "" {
			return core.ErrTimeMatchResponse
		}
	}
	return nil
}

func (c Client) googleReadBack(ctx context.Context, w EventWrite, ref EventRef, duplicate bool) (EventReceipt, error) {
	condition, err := c.googleOwnedRevision(ctx, w.Connection, ref, w.Policy)
	if err != nil {
		return EventReceipt{}, err
	}
	if len(condition.Values) == 0 {
		return EventReceipt{}, googleMissingOwnedIdentity(duplicate)
	}
	observed, err := c.googleOwnedObservation(ctx, w.Connection, ref, w.Policy, condition, w.Intent.Venue.Kind == core.TimeMatchVenueAutomaticVideo)
	if err != nil {
		return EventReceipt{}, err
	}
	return verifyGoogleObservation(observed, w.Intent, ref)
}

// The metadata filter admits ownership before any event details are read.
// The conditional GET binds those details to the same observed revision.
func (c Client) googleOwnedObservation(ctx context.Context, connection Connection, ref EventRef, policy exchange.OperationPolicy, condition exchange.Header, conference bool) (googleEventObservation, error) {
	secrets, err := bearer(connection.AccessToken)
	if err != nil {
		return googleEventObservation{}, err
	}
	base, err := googleEventURL(ref.Calendar, ref.Resource, false)
	if err != nil {
		return googleEventObservation{}, err
	}
	fields := googleOwnedDetailsFields
	if conference {
		fields += googleOwnedConferenceFields
	}
	query := url.Values{"fields": {fields}}
	target, err := endpoint(base.String() + "?" + query.Encode())
	if err != nil {
		return googleEventObservation{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, headers: []exchange.Header{condition}, responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: policy, expected: http.StatusOK})
	if err != nil {
		return googleEventObservation{}, errors.Join(core.ErrTimeMatchIndeterminate, readFailure(result, err))
	}
	defer clear(result.body)
	observed, err := decodeJSON[googleEventObservation](result.body)
	if err != nil {
		return googleEventObservation{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	revision, err := condition.Values[0].Value()
	if err != nil || observed.etag != revision {
		return googleEventObservation{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse, err)
	}
	return observed, nil
}

func (c Client) googleUpdate(ctx context.Context, u EventUpdate) (EventReceipt, error) {
	secrets, err := bearer(u.Connection.AccessToken)
	if err != nil {
		return EventReceipt{}, err
	}
	if u.Ref.Resource != u.Intent.ID.value {
		return EventReceipt{}, core.ErrTimeMatchContract
	}
	projection, err := googleEventProjection(u.Intent)
	if err != nil {
		return EventReceipt{}, err
	}
	target, err := googleEventURL(u.Intent.Calendar, u.Ref.Resource, true)
	if err == nil {
		target, err = googleOwnedWriteTarget(target, u.Intent)
	}
	if err != nil {
		return EventReceipt{}, err
	}
	condition, err := c.googleOwnedRevision(ctx, u.Connection, u.Ref, u.Policy)
	if err != nil {
		return EventReceipt{}, err
	}
	body, previous, err := c.googleUpdateBody(ctx, u, condition, projection)
	if err != nil {
		return EventReceipt{}, err
	}
	// Patch only the explicitly owned fields. Preserve notes, locations and
	// other provider fields the user may have added to this owned appointment.
	// Location changes only when the organizer explicitly supplied a venue.
	// https://developers.google.com/workspace/calendar/api/v3/reference/events/patch
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPatch, body: body, headers: []exchange.Header{condition}, requestType: core.HTTPMediaTypeJSON(), responseType: core.HTTPMediaTypeJSON(), secrets: secrets, policy: u.Policy, expected: http.StatusOK})
	if err != nil {
		if result.status == http.StatusPreconditionFailed {
			return EventReceipt{}, errors.Join(core.ErrTimeMatchConflict, err)
		}
		return EventReceipt{}, writeFailure(result, err)
	}
	return verifyGoogleUpdatedConference(result.body, u, previous)
}

// An automatic video update first reads the exact owned event at the guarded
// revision; its old interval is not compared with the requested one.
func (c Client) googleUpdateBody(ctx context.Context, u EventUpdate, condition exchange.Header, projection googleEventBody) ([]byte, EventReceipt, error) {
	if len(condition.Values) == 0 {
		return nil, EventReceipt{}, core.ErrTimeMatchIndeterminate
	}
	previous := EventReceipt{}
	if u.Intent.Venue.Kind == core.TimeMatchVenueAutomaticVideo {
		observed, err := c.googleOwnedObservation(ctx, u.Connection, u.Ref, u.Policy, condition, true)
		if err != nil {
			return nil, EventReceipt{}, err
		}
		if err := verifyGoogleOwnedIdentity(observed, u.Intent); err != nil {
			return nil, EventReceipt{}, err
		}
		if observed.conference != nil {
			if previous, err = googleEventReceipt(observed, u.Intent, u.Ref); err != nil {
				return nil, EventReceipt{}, err
			}
			// PATCH preserves omitted fields. Keep the conference already admitted
			// for this owned event; pending or failed generation is never restarted.
			projection.ConferenceData = nil
		}
	}
	body, err := jsonio.MarshalProjection(projection, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted})
	if err != nil {
		return nil, EventReceipt{}, err
	}
	return body, previous, nil
}

func verifyGoogleUpdatedConference(body []byte, u EventUpdate, previous EventReceipt) (EventReceipt, error) {
	receipt, err := verifyGoogle(body, u.Intent, u.Ref)
	if err != nil {
		return EventReceipt{}, err
	}
	if previous.VenueStatus == EventVenueReady && (receipt.VenueStatus != EventVenueReady || receipt.Venue != previous.Venue) {
		return EventReceipt{}, core.ErrTimeMatchConflict
	}
	return receipt, nil
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
	condition, err := c.googleOwnedRevision(ctx, d.Connection, d.Ref, d.Policy)
	if err != nil {
		return DeleteInvalid, err
	}
	if len(condition.Values) == 0 {
		return c.googleDeleteAbsent(ctx, d, secrets)
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodDelete, headers: []exchange.Header{condition}, secrets: secrets, policy: d.Policy, expected: http.StatusNoContent})
	clear(result.body)
	switch {
	case err == nil:
		return Deleted, nil
	case result.status == http.StatusPreconditionFailed:
		return DeleteInvalid, errors.Join(core.ErrTimeMatchConflict, err)
	case result.status == http.StatusNotFound || result.status == http.StatusGone:
		return DeleteAlreadyAbsent, nil
	default:
		return DeleteInvalid, writeFailure(result, err)
	}
}

type googleEventObservation struct {
	conference *googleConferenceData
	location   string
	etag       string
	marker     string
	id         string
	status     string
	summary    string
	start      string
	end        string
}

func (o *googleEventObservation) UnmarshalJSON(raw []byte) error {
	var document struct {
		ConferenceData     *googleConferenceData    `json:"conferenceData"`
		ID                 string                   `json:"id"`
		ETag               string                   `json:"etag"`
		ExtendedProperties googleExtendedProperties `json:"extendedProperties"`
		Status             string                   `json:"status"`
		Summary            string                   `json:"summary"`
		Location           string                   `json:"location"`
		Start              googleEventTime          `json:"start"`
		End                googleEventTime          `json:"end"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		return err
	}
	*o = googleEventObservation{conference: document.ConferenceData, location: document.Location, etag: document.ETag, marker: document.ExtendedProperties.Private.EventID, id: document.ID, status: document.Status, summary: document.Summary, start: document.Start.DateTime, end: document.End.DateTime}
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
	return verifyGoogleObservation(observed, intent, ref)
}

func verifyGoogleObservation(observed googleEventObservation, intent EventIntent, ref EventRef) (EventReceipt, error) {
	if err := verifyGoogleOwnedIdentity(observed, intent); err != nil {
		return EventReceipt{}, err
	}
	span, err := rfc3339Span(observed.start, observed.end)
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	if observed.id != intent.ID.value || observed.status == "cancelled" || observed.summary != intent.Summary.value || intent.Venue.Text() != "" && observed.location != intent.Venue.Text() || !sameSpan(span, intent.Interval) {
		return EventReceipt{}, core.ErrTimeMatchConflict
	}
	return googleEventReceipt(observed, intent, ref)
}

func sameSpan(span [2]int64, interval temporal.IntervalBounds) bool {
	start, end, err := nanosecondBounds(interval)
	return err == nil && span[0] == start && span[1] == end
}

func verifyGoogleOwnedIdentity(observed googleEventObservation, intent EventIntent) error {
	// A cancelled placeholder may omit its old interval entirely.
	if observed.id != intent.ID.value || observed.marker != intent.ID.value || observed.status == "cancelled" {
		return core.ErrTimeMatchConflict
	}
	return nil
}

func googleMissingOwnedIdentity(duplicate bool) error {
	if duplicate {
		return core.ErrTimeMatchConflict
	}
	return core.ErrTimeMatchIndeterminate
}

// Only a matching, explicitly cancelled identity proves the old event absent.
// Active, incomplete and inaccessible observations retain cleanup uncertainty;
// this path never downloads private fields or sends a DELETE.
func (c Client) googleDeleteAbsent(ctx context.Context, d EventDelete, secrets []exchange.SecretHeader) (DeleteOutcome, error) {
	base, err := googleEventURL(d.Ref.Calendar, d.Ref.Resource, false)
	if err != nil {
		return DeleteInvalid, err
	}
	query := url.Values{"fields": {"id,status"}}
	target, err := endpoint(base.String() + "?" + query.Encode())
	if err != nil {
		return DeleteInvalid, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, secrets: secrets, responseType: core.HTTPMediaTypeJSON(), policy: d.Policy, expected: http.StatusOK})
	if err != nil {
		return DeleteInvalid, errors.Join(core.ErrTimeMatchIndeterminate, readFailure(result, err))
	}
	defer clear(result.body)
	observed, err := decodeJSON[googleEventObservation](result.body)
	if err != nil {
		return DeleteInvalid, errors.Join(core.ErrTimeMatchIndeterminate, err)
	}
	if observed.id != d.Ref.ID.value {
		return DeleteInvalid, core.ErrTimeMatchConflict
	}
	if observed.status != "cancelled" {
		return DeleteInvalid, core.ErrTimeMatchIndeterminate
	}
	return DeleteAlreadyAbsent, nil
}
