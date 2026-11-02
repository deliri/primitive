package timematch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// Apple iCloud Calendar over CalDAV with an app-specific password.
//
// support.apple.com/en-us/121539: apps that do not support Apple's
// authorization "can sign in using an app-specific password instead";
// support.apple.com/en-us/102654: app-specific passwords require two-factor
// authentication and are revoked by the account owner.
//
// Host: Apple's support pages do not name a CalDAV host. RFC 6764 section 3
// SRV discovery for icloud.com (_caldavs._tcp.icloud.com, observed
// 2026-11-02) returns caldav.icloud.com:443; that is the fixed entry point.
// The account's calendar home is then discovered (RFC 5397
// current-user-principal, RFC 4791 calendar-home-set) and may live on a
// pNN-caldav.icloud.com partition host. No other host receives the
// credential.
const (
	appleCalDAVHost  = "caldav.icloud.com"
	appleCalDAVRoot  = "https://" + appleCalDAVHost + "/"
	appleHostSuffix  = "-caldav.icloud.com"
	davNamespace     = "DAV:"
	caldavNamespace  = "urn:ietf:params:xml:ns:caldav"
	davStatusOK      = " 200 "
	davMultiStatus   = 207
	davDepthZero     = "0"
	davDepthOne      = "1"
	icsResourceExt   = ".ics"
	icsMediaTypeText = "text/calendar; charset=utf-8"
	xmlMediaTypeText = "application/xml; charset=utf-8"
)

const (
	propfindPrincipal = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`
	propfindHome      = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-home-set/></d:prop></d:propfind>`
	propfindCalendars = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:resourcetype/><c:supported-calendar-component-set/><d:current-user-privilege-set/></d:prop></d:propfind>`
)

// appleHost admits only caldav.icloud.com and pNN-caldav.icloud.com on 443.
func appleHost(u url.URL) bool {
	if u.Scheme != core.SchemeHTTPS || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" && u.Port() != "443" {
		return false
	}
	host := u.Hostname()
	if host == appleCalDAVHost {
		return true
	}
	partition, ok := strings.CutSuffix(host, appleHostSuffix)
	if !ok || len(partition) < 2 || partition[0] != 'p' {
		return false
	}
	for i := 1; i < len(partition); i++ {
		if partition[i] < '0' || partition[i] > '9' {
			return false
		}
	}
	return true
}

// appleCollection resolves an href against its base and admits a collection
// URL (trailing slash) on an Apple host.
func appleResolve(base core.HTTPEndpoint, href string, collection bool) (core.HTTPEndpoint, error) {
	baseURL := base.HTTPURL()
	reference, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	resolved := baseURL.ResolveReference(reference)
	if !appleHost(*resolved) || collection && !strings.HasSuffix(resolved.Path, "/") {
		return core.HTTPEndpoint{}, core.ErrTimeMatchResponse
	}
	parsed, err := core.ParseHTTPEndpoint(resolved.String())
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return parsed, nil
}

func appleCalendarURL(id CalendarID) (core.HTTPEndpoint, error) {
	parsed, err := core.ParseHTTPEndpoint(id.value)
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	u := parsed.HTTPURL()
	if !appleHost(u) || !strings.HasSuffix(u.Path, "/") {
		return core.HTTPEndpoint{}, core.ErrTimeMatchContract
	}
	return parsed, nil
}

// basic builds the RFC 7617 header from the borrowed password bytes. The
// returned cleanup zeroes the SDK-owned encoding.
func basic(connection Connection) ([]exchange.SecretHeader, func(), error) {
	name, err := core.ParseHTTPHeaderName(authorizationHeaderText)
	if err != nil {
		return nil, nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	pair := make([]byte, 0, len(connection.AppleAccount.value)+1+len(connection.ApplePassword.value))
	pair = append(append(append(pair, connection.AppleAccount.value...), ':'), connection.ApplePassword.value...)
	value := make([]byte, len("Basic ")+base64.StdEncoding.EncodedLen(len(pair)))
	copy(value, "Basic ")
	base64.StdEncoding.Encode(value[len("Basic "):], pair)
	clear(pair)
	return []exchange.SecretHeader{{Name: name, Value: value, Scheme: exchange.SecretHeaderLiteral}}, func() { clear(value) }, nil
}

func xmlMediaType() core.HTTPMediaType {
	media, _ := core.ParseHTTPMediaType(xmlMediaTypeText)
	return media
}

// dav performs one WebDAV request expecting 207 Multi-Status XML.
func (c Client) dav(ctx context.Context, secrets []exchange.SecretHeader, target core.HTTPEndpoint, method exchange.Method, depth string, body string, policy exchange.OperationPolicy) (davMultistatusWire, error) {
	depthHeader, err := header("Depth", depth)
	if err != nil {
		return davMultistatusWire{}, err
	}
	contentType := core.HTTPHeaderContentType()
	result, err := c.send(ctx, call{target: target, method: method, body: []byte(body), requestType: xmlMediaType(), headers: []exchange.Header{depthHeader}, secrets: secrets, capture: []core.HTTPHeaderName{contentType}, policy: policy, expected: davMultiStatus})
	if err != nil {
		return davMultistatusWire{}, readFailure(result, err)
	}
	defer clear(result.body)
	if !xmlResponse(result.captured) {
		return davMultistatusWire{}, core.ErrTimeMatchResponse
	}
	return decodeMultistatus(result.body)
}

func xmlResponse(captured exchange.CapturedHeaders) bool {
	for _, value := range captured.Values {
		if value.Name != core.HTTPHeaderContentType() || len(value.Values) != 1 {
			continue
		}
		text, err := value.Values[0].Value()
		if err != nil {
			return false
		}
		media, err := core.ParseHTTPMediaType(text)
		if err != nil {
			return false
		}
		for _, admitted := range []string{"application/xml", "text/xml"} {
			want, parseErr := core.ParseHTTPMediaType(admitted)
			if same, sameErr := media.SameBase(want); parseErr == nil && sameErr == nil && same {
				return true
			}
		}
	}
	return false
}

// RFC 4918 section 14.16 multistatus, with the CalDAV properties this
// package reads. Go's encoding/xml expands no external entities.
type davMultistatusWire struct {
	XMLName   xml.Name          `xml:"DAV: multistatus"`
	Responses []davResponseWire `xml:"DAV: response"`
}

type davResponseWire struct {
	Href      string            `xml:"DAV: href"`
	Propstats []davPropstatWire `xml:"DAV: propstat"`
}

type davPropstatWire struct {
	Status string      `xml:"DAV: status"`
	Prop   davPropWire `xml:"DAV: prop"`
}

type davHrefWire struct {
	Href string `xml:"DAV: href"`
}

type davPresenceWire struct{}

type davPrivilegeWire struct {
	Write        *davPresenceWire `xml:"DAV: write"`
	WriteContent *davPresenceWire `xml:"DAV: write-content"`
	All          *davPresenceWire `xml:"DAV: all"`
}

type davComponentWire struct {
	Name string `xml:"name,attr"`
}

type davPropWire struct {
	Principal    *davHrefWire `xml:"DAV: current-user-principal"`
	Home         *davHrefWire `xml:"urn:ietf:params:xml:ns:caldav calendar-home-set"`
	ResourceType *struct {
		Calendar *davPresenceWire `xml:"urn:ietf:params:xml:ns:caldav calendar"`
	} `xml:"DAV: resourcetype"`
	Components *struct {
		Comps []davComponentWire `xml:"urn:ietf:params:xml:ns:caldav comp"`
	} `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
	Privileges *struct {
		Privileges []davPrivilegeWire `xml:"DAV: privilege"`
	} `xml:"DAV: current-user-privilege-set"`
	CalendarData *string `xml:"urn:ietf:params:xml:ns:caldav calendar-data"`
}

func decodeMultistatus(body []byte) (davMultistatusWire, error) {
	var document davMultistatusWire
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = true
	if err := decoder.Decode(&document); err != nil {
		return davMultistatusWire{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return document, nil
}

// okProp returns the property set reported with a 200 propstat.
func (r davResponseWire) okProp() (davPropWire, bool) {
	for _, propstat := range r.Propstats {
		if strings.Contains(propstat.Status, davStatusOK) {
			return propstat.Prop, true
		}
	}
	return davPropWire{}, false
}

// appleHome discovers the account's calendar home: principal, then home set.
func (c Client) appleHome(ctx context.Context, secrets []exchange.SecretHeader, policy exchange.OperationPolicy) (core.HTTPEndpoint, error) {
	root, err := endpoint(appleCalDAVRoot)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	principalStatus, err := c.dav(ctx, secrets, root, exchange.MethodPropfind, davDepthZero, propfindPrincipal, policy)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	principal, err := singleHref(principalStatus, func(p davPropWire) *davHrefWire { return p.Principal })
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	principalURL, err := appleResolve(root, principal, true)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	homeStatus, err := c.dav(ctx, secrets, principalURL, exchange.MethodPropfind, davDepthZero, propfindHome, policy)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	home, err := singleHref(homeStatus, func(p davPropWire) *davHrefWire { return p.Home })
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	return appleResolve(principalURL, home, true)
}

func singleHref(status davMultistatusWire, pick func(davPropWire) *davHrefWire) (string, error) {
	found := ""
	for _, response := range status.Responses {
		prop, ok := response.okProp()
		if !ok {
			continue
		}
		if value := pick(prop); value != nil && strings.TrimSpace(value.Href) != "" {
			if found != "" {
				return "", core.ErrTimeMatchResponse
			}
			found = value.Href
		}
	}
	if found == "" {
		return "", core.ErrTimeMatchResponse
	}
	return found, nil
}

func (c Client) appleCalendarList(ctx context.Context, r CalendarListRequest) (CalendarList, error) {
	secrets, cleanup, err := basic(r.Connection)
	if err != nil {
		return CalendarList{}, err
	}
	defer cleanup()
	home, err := c.appleHome(ctx, secrets, r.Policy)
	if err != nil {
		return CalendarList{}, err
	}
	status, err := c.dav(ctx, secrets, home, exchange.MethodPropfind, davDepthOne, propfindCalendars, r.Policy)
	if err != nil {
		return CalendarList{}, err
	}
	list := CalendarList{Provider: ProviderApple}
	for _, response := range status.Responses {
		prop, ok := response.okProp()
		if !ok || prop.ResourceType == nil || prop.ResourceType.Calendar == nil || !supportsEvents(prop) {
			continue
		}
		collection, err := appleResolve(home, response.Href, true)
		if err != nil {
			return CalendarList{}, err
		}
		id, err := ParseCalendarID(collection.String())
		if err != nil {
			return CalendarList{}, errors.Join(core.ErrTimeMatchResponse, err)
		}
		list.Calendars = append(list.Calendars, Calendar{ID: id, Writable: writable(prop)})
	}
	if err := list.Validate(); err != nil {
		return CalendarList{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return list, nil
}

// supportsEvents: an absent supported-calendar-component-set means every
// component type is accepted (RFC 4791 section 5.2.3).
func supportsEvents(prop davPropWire) bool {
	if prop.Components == nil {
		return true
	}
	for _, component := range prop.Components.Comps {
		if strings.EqualFold(component.Name, "VEVENT") {
			return true
		}
	}
	return false
}

func writable(prop davPropWire) bool {
	if prop.Privileges == nil {
		return false
	}
	for _, privilege := range prop.Privileges.Privileges {
		if privilege.Write != nil || privilege.WriteContent != nil || privilege.All != nil {
			return true
		}
	}
	return false
}

// appleAvailability issues one RFC 4791 section 7.8 calendar-query per
// calendar with CALDAV:expand, so recurring events arrive as UTC instances.
func (c Client) appleAvailability(ctx context.Context, r AvailabilityRequest) (Availability, error) {
	secrets, cleanup, err := basic(r.Connection)
	if err != nil {
		return Availability{}, err
	}
	defer cleanup()
	start, startErr := icsStampTime(r.Window.Start)
	end, endErr := icsCeilingTime(r.Window.End)
	if err := errors.Join(startErr, endErr); err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	query := `<?xml version="1.0" encoding="utf-8"?><c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-data><c:expand start="` + start + `" end="` + end + `"/></c:calendar-data></d:prop><c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"><c:time-range start="` + start + `" end="` + end + `"/></c:comp-filter></c:comp-filter></c:filter></c:calendar-query>`
	availability := Availability{Window: r.Window, Calendars: make([]CalendarAvailability, 0, len(r.Calendars))}
	for _, id := range r.Calendars {
		target, err := appleCalendarURL(id)
		if err != nil {
			return Availability{}, err
		}
		calendar := CalendarAvailability{ID: id, State: AvailabilityUnknown}
		status, err := c.dav(ctx, secrets, target, exchange.MethodReport, davDepthOne, query, r.Policy)
		switch {
		case errors.Is(err, core.ErrTimeMatchConsent) || err != nil && ctx.Err() != nil:
			return Availability{}, err
		case err == nil:
			if spans, known := appleSpans(status); known {
				busy, err := normalizeBusy(r.Window, spans)
				if err != nil {
					return Availability{}, err
				}
				calendar = CalendarAvailability{ID: id, State: AvailabilityKnown, Busy: busy}
			}
		}
		availability.Calendars = append(availability.Calendars, calendar)
	}
	if err := availability.covers(r); err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return availability, nil
}

// appleSpans is Known only when every returned resource carried readable
// calendar data and every blocking event resolved to absolute instants.
func appleSpans(status davMultistatusWire) ([][2]int64, bool) {
	var spans [][2]int64
	for _, response := range status.Responses {
		prop, ok := response.okProp()
		if !ok || prop.CalendarData == nil {
			return nil, false
		}
		events, err := parseICS(*prop.CalendarData)
		if err != nil {
			return nil, false
		}
		for _, event := range events {
			if !event.blocking {
				continue
			}
			if !event.resolvable {
				return nil, false
			}
			if event.start < event.end {
				spans = append(spans, [2]int64{event.start, event.end})
			}
		}
	}
	return spans, true
}

// icsCeilingTime rounds the window end up to a whole second so the query
// never ends before the requested window does.
func icsCeilingTime(instant temporal.Instant) (string, error) {
	value, err := instant.Time()
	if err != nil {
		return "", err
	}
	if value.Nanosecond() != 0 {
		value = value.Truncate(1e9).Add(1e9)
	}
	return value.UTC().Format(icsUTCLayout), nil
}

func appleEventURL(calendar CalendarID, id EventID) (core.HTTPEndpoint, error) {
	base, err := appleCalendarURL(calendar)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	return appleResolve(base, id.value+icsResourceExt, false)
}

func icsMediaType() core.HTTPMediaType {
	media, _ := core.ParseHTTPMediaType(icsMediaTypeText)
	return media
}

// applePut creates {calendar}/{EventID}.ics with If-None-Match: * (RFC 4791
// section 5.3.2: create only if absent). 412 means the resource exists: it is
// read back and accepted only when it is exactly the owned intent.
func (c Client) applePut(ctx context.Context, w EventWrite) (EventReceipt, error) {
	target, err := appleEventURL(w.Intent.Calendar, w.Intent.ID)
	if err != nil {
		return EventReceipt{}, err
	}
	body, err := writeICS(w.Intent, w.ObservedAt)
	if err != nil {
		return EventReceipt{}, err
	}
	secrets, cleanup, err := basic(w.Connection)
	if err != nil {
		return EventReceipt{}, err
	}
	defer cleanup()
	condition, err := header("If-None-Match", "*")
	if err != nil {
		return EventReceipt{}, err
	}
	ref := EventRef{Provider: ProviderApple, Calendar: w.Intent.Calendar, Resource: target.String()}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPut, body: body, requestType: icsMediaType(), headers: []exchange.Header{condition}, secrets: secrets, policy: w.Policy, expected: http.StatusCreated})
	clear(result.body)
	switch {
	case err == nil || result.status == http.StatusNoContent:
		return EventReceipt{Ref: ref}, nil
	case result.status == http.StatusPreconditionFailed:
		return c.appleReadBack(ctx, secrets, target, w.Intent, ref, w.Policy)
	default:
		return EventReceipt{}, writeFailure(result, err)
	}
}

func (c Client) appleReadBack(ctx context.Context, secrets []exchange.SecretHeader, target core.HTTPEndpoint, intent EventIntent, ref EventRef, policy exchange.OperationPolicy) (EventReceipt, error) {
	result, err := c.send(ctx, call{target: target, method: exchange.MethodGet, secrets: secrets, policy: policy, expected: http.StatusOK})
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchConflict, readFailure(result, err))
	}
	defer clear(result.body)
	events, err := parseICS(string(result.body))
	if err != nil || len(events) != 1 {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchConflict, err)
	}
	event := events[0]
	if !event.resolvable || !event.blocking || event.uid != intent.ID.value || event.summary != intent.Summary.value || !sameSpan([2]int64{event.start, event.end}, intent.Interval) {
		return EventReceipt{}, core.ErrTimeMatchConflict
	}
	return EventReceipt{Ref: ref}, nil
}

// appleUpdate replaces the owned resource (If-Match: * requires it to exist).
func (c Client) appleUpdate(ctx context.Context, u EventUpdate) (EventReceipt, error) {
	target, err := appleEventURL(u.Intent.Calendar, u.Intent.ID)
	if err != nil {
		return EventReceipt{}, err
	}
	if target.String() != u.Ref.Resource {
		return EventReceipt{}, core.ErrTimeMatchContract
	}
	body, err := writeICS(u.Intent, u.ObservedAt)
	if err != nil {
		return EventReceipt{}, err
	}
	secrets, cleanup, err := basic(u.Connection)
	if err != nil {
		return EventReceipt{}, err
	}
	defer cleanup()
	condition, err := header("If-Match", "*")
	if err != nil {
		return EventReceipt{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPut, body: body, requestType: icsMediaType(), headers: []exchange.Header{condition}, secrets: secrets, policy: u.Policy, expected: http.StatusNoContent})
	clear(result.body)
	if err != nil && result.status != http.StatusOK && result.status != http.StatusCreated {
		return EventReceipt{}, writeFailure(result, err)
	}
	return EventReceipt{Ref: u.Ref}, nil
}

func (c Client) appleDelete(ctx context.Context, d EventDelete) (DeleteOutcome, error) {
	target, err := core.ParseHTTPEndpoint(d.Ref.Resource)
	if err != nil || !appleHost(target.HTTPURL()) {
		return DeleteInvalid, errors.Join(core.ErrTimeMatchContract, err)
	}
	if _, err := appleCalendarURL(d.Ref.Calendar); err != nil || !strings.HasPrefix(d.Ref.Resource, d.Ref.Calendar.value) {
		return DeleteInvalid, errors.Join(core.ErrTimeMatchContract, err)
	}
	secrets, cleanup, err := basic(d.Connection)
	if err != nil {
		return DeleteInvalid, err
	}
	defer cleanup()
	return c.deleteResource(ctx, call{target: target, method: exchange.MethodDelete, secrets: secrets, policy: d.Policy, expected: http.StatusNoContent})
}
