package timematch

import (
	"bytes"
	"context"
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

// basic builds the RFC 7617 Authorization header through Exchange, which
// owns the encoding and clears its scratch copies.
func basic(connection Connection) (exchange.SecretHeader, error) {
	identity, err := exchange.ParseBasicAuthorizationIdentity(connection.AppleAccount.value)
	if err != nil {
		return exchange.SecretHeader{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	authorization, err := exchange.NewBasicAuthorizationSecretHeader(exchange.BasicAuthorizationRequest{Identity: identity, Secret: connection.ApplePassword.value})
	if err != nil {
		return exchange.SecretHeader{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return authorization, nil
}

func xmlMediaType() core.HTTPMediaType {
	media, _ := core.ParseHTTPMediaType(xmlMediaTypeText)
	return media
}

// davRequest is one WebDAV request expecting 207 Multi-Status XML.
type davRequest struct {
	auth   exchange.SecretHeader
	target core.HTTPEndpoint
	depth  string
	body   string
	policy exchange.OperationPolicy
	method exchange.Method
}

func (c Client) dav(ctx context.Context, r davRequest) (davMultistatusWire, error) {
	depthHeader, err := header("Depth", r.depth)
	if err != nil {
		return davMultistatusWire{}, err
	}
	contentType := core.HTTPHeaderContentType()
	result, err := c.send(ctx, call{target: r.target, method: r.method, body: []byte(r.body), requestType: xmlMediaType(), headers: []exchange.Header{depthHeader}, secrets: []exchange.SecretHeader{r.auth}, capture: []core.HTTPHeaderName{contentType}, policy: r.policy, expected: davMultiStatus})
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
	return mediaIs(captured, "application/xml") || mediaIs(captured, "text/xml")
}

// mediaIs reports whether the single captured Content-Type has base type want.
func mediaIs(captured exchange.CapturedHeaders, want string) bool {
	for _, value := range captured.Values {
		if value.Name != core.HTTPHeaderContentType() || len(value.Values) != 1 {
			continue
		}
		text, err := value.Values[0].Value()
		if err != nil {
			return false
		}
		media, mediaErr := core.ParseHTTPMediaType(text)
		admitted, admittedErr := core.ParseHTTPMediaType(want)
		if mediaErr != nil || admittedErr != nil {
			return false
		}
		same, sameErr := media.SameBase(admitted)
		return sameErr == nil && same
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
func (c Client) appleHome(ctx context.Context, auth exchange.SecretHeader, policy exchange.OperationPolicy) (core.HTTPEndpoint, error) {
	root, err := endpoint(appleCalDAVRoot)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	principalStatus, err := c.dav(ctx, davRequest{auth: auth, target: root, method: exchange.MethodPropfind, depth: davDepthZero, body: propfindPrincipal, policy: policy})
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
	homeStatus, err := c.dav(ctx, davRequest{auth: auth, target: principalURL, method: exchange.MethodPropfind, depth: davDepthZero, body: propfindHome, policy: policy})
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
	auth, err := basic(r.Connection)
	if err != nil {
		return CalendarList{}, err
	}
	defer clear(auth.Value)
	home, err := c.appleHome(ctx, auth, r.Policy)
	if err != nil {
		return CalendarList{}, err
	}
	status, err := c.dav(ctx, davRequest{auth: auth, target: home, method: exchange.MethodPropfind, depth: davDepthOne, body: propfindCalendars, policy: r.Policy})
	if err != nil {
		return CalendarList{}, err
	}
	list := CalendarList{Provider: ProviderApple}
	for _, response := range status.Responses {
		prop, ok := response.okProp()
		if !ok || prop.ResourceType == nil || prop.ResourceType.Calendar == nil || !supportsEvents(prop) {
			continue
		}
		collection, resolveErr := appleResolve(home, response.Href, true)
		if resolveErr != nil {
			return CalendarList{}, resolveErr
		}
		id, parseErr := ParseCalendarID(collection.String())
		if parseErr != nil {
			return CalendarList{}, errors.Join(core.ErrTimeMatchResponse, parseErr)
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

// appleAvailability issues one RFC 4791 section 7.10 free-busy-query REPORT
// per calendar. The server answers with a VFREEBUSY of busy periods only: no
// event resource, UID, title or detail is requested or admitted. A server
// that refuses the report, or answers with anything but periods, leaves that
// calendar Unknown; TimeMatch never falls back to downloading event data.
func (c Client) appleAvailability(ctx context.Context, r AvailabilityRequest) (Availability, error) {
	auth, authErr := basic(r.Connection)
	if authErr != nil {
		return Availability{}, authErr
	}
	defer clear(auth.Value)
	start, startErr := icsStampTime(r.Window.Start)
	end, endErr := icsCeilingTime(r.Window.End)
	depth, depthErr := header("Depth", davDepthOne)
	if setupErr := errors.Join(startErr, endErr, depthErr); setupErr != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchContract, setupErr)
	}
	query := []byte(`<?xml version="1.0" encoding="utf-8"?><c:free-busy-query xmlns:c="urn:ietf:params:xml:ns:caldav"><c:time-range start="` + start + `" end="` + end + `"/></c:free-busy-query>`)
	availability := Availability{Window: r.Window, Calendars: make([]CalendarAvailability, 0, len(r.Calendars))}
	for _, id := range r.Calendars {
		target, targetErr := appleCalendarURL(id)
		if targetErr != nil {
			return Availability{}, targetErr
		}
		calendar := CalendarAvailability{ID: id, State: AvailabilityUnknown}
		result, err := c.send(ctx, call{target: target, method: exchange.MethodReport, body: query, requestType: xmlMediaType(), headers: []exchange.Header{depth}, secrets: []exchange.SecretHeader{auth}, capture: []core.HTTPHeaderName{core.HTTPHeaderContentType()}, policy: r.Policy, expected: http.StatusOK})
		if err != nil {
			if failure := readFailure(result, err); !providerUnknown(ctx, failure) {
				return Availability{}, failure
			}
		} else if spans, known := parseFreeBusy(string(result.body)); known && mediaIs(result.captured, "text/calendar") {
			busy, normalizeErr := normalizeBusy(r.Window, spans)
			if normalizeErr != nil {
				return Availability{}, normalizeErr
			}
			calendar = CalendarAvailability{ID: id, State: AvailabilityKnown, Busy: busy}
		}
		clear(result.body)
		availability.Calendars = append(availability.Calendars, calendar)
	}
	if err := availability.covers(r); err != nil {
		return Availability{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return availability, nil
}

// icsCeilingTime rounds the window end up to a whole second so the query
// never ends before the requested window does.
func icsCeilingTime(instant temporal.Instant) (string, error) {
	ceiling, err := instant.Ceiling(temporal.PrecisionSecond)
	if err != nil {
		return "", err
	}
	return icsStampTime(ceiling)
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
	auth, err := basic(w.Connection)
	if err != nil {
		return EventReceipt{}, err
	}
	defer clear(auth.Value)
	condition, err := header("If-None-Match", "*")
	if err != nil {
		return EventReceipt{}, err
	}
	ref := EventRef{ID: w.Intent.ID, Provider: ProviderApple, Calendar: w.Intent.Calendar, Resource: target.String()}
	// Recovery repeats only the exact child URI with the same create-only
	// precondition. It can fill an unapplied intent, never replace a resource.
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPut, body: body, requestType: icsMediaType(), headers: []exchange.Header{condition}, secrets: []exchange.SecretHeader{auth}, policy: w.Policy, expected: http.StatusCreated})
	clear(result.body)
	switch {
	case err == nil || result.status == http.StatusNoContent:
		return EventReceipt{Ref: ref}, nil
	case result.status == http.StatusPreconditionFailed:
		return c.appleReadBack(ctx, appleReadBackRequest{auth: auth, target: target, intent: w.Intent, ref: ref, policy: w.Policy})
	default:
		return EventReceipt{}, writeFailure(result, err)
	}
}

// appleReadBackRequest reads an existing owned resource after 412.
type appleReadBackRequest struct {
	auth   exchange.SecretHeader
	target core.HTTPEndpoint
	ref    EventRef
	intent EventIntent
	policy exchange.OperationPolicy
}

func (c Client) appleReadBack(ctx context.Context, r appleReadBackRequest) (EventReceipt, error) {
	intent, ref := r.intent, r.ref
	result, err := c.send(ctx, call{target: r.target, method: exchange.MethodGet, secrets: []exchange.SecretHeader{r.auth}, capture: []core.HTTPHeaderName{core.HTTPHeaderContentType()}, policy: r.policy, expected: http.StatusOK})
	if err != nil {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, readFailure(result, err))
	}
	defer clear(result.body)
	if !mediaIs(result.captured, "text/calendar") {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
	}
	events, err := parseICS(string(result.body))
	if err != nil || len(events) != 1 {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse, err)
	}
	event := events[0]
	if event.uid == "" || !event.resolvable {
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, core.ErrTimeMatchResponse)
	}
	if !event.blocking || event.uid != intent.ID.value || event.summary != intent.Summary.value || !sameSpan([2]int64{event.start, event.end}, intent.Interval) {
		return EventReceipt{}, core.ErrTimeMatchConflict
	}
	return EventReceipt{Ref: ref}, nil
}

// appleRefTarget binds a persisted reference to exactly one deterministic event
// child. A collection prefix never authorizes a calendar-wide operation.
func appleRefTarget(ref EventRef) (core.HTTPEndpoint, error) {
	if err := ref.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	target, err := appleEventURL(ref.Calendar, ref.ID)
	if err != nil || ref.Provider != ProviderApple || target.String() != ref.Resource {
		return core.HTTPEndpoint{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return target, nil
}

// appleMutationMatch reads only the specifically owned resource. RFC4791
// section5.3.4 requires its GET response to carry a strong ETag. No provider
// event population or calendar details are read as an availability fallback.
func (c Client) appleMutationMatch(ctx context.Context, r appleReadBackRequest) (outcome, error) {
	etag, err := core.ParseHTTPHeaderName("ETag")
	if err != nil {
		return outcome{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	result, err := c.send(ctx, call{target: r.target, method: exchange.MethodGet, secrets: []exchange.SecretHeader{r.auth}, capture: []core.HTTPHeaderName{core.HTTPHeaderContentType(), etag}, policy: r.policy, expected: http.StatusOK})
	if err != nil {
		return outcome{}, errors.Join(core.ErrTimeMatchIndeterminate, readFailure(result, err))
	}
	if !mediaIs(result.captured, "text/calendar") {
		clear(result.body)
		return outcome{}, core.ErrTimeMatchResponse
	}
	events, err := parseICS(string(result.body))
	if err != nil || len(events) != 1 || events[0].uid == "" || !events[0].resolvable || !events[0].blocking {
		clear(result.body)
		return outcome{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	if events[0].uid != r.ref.ID.value {
		clear(result.body)
		return outcome{}, core.ErrTimeMatchConflict
	}
	if _, err = appleIfMatch(result.captured); err != nil {
		clear(result.body)
		return outcome{}, err
	}
	return result, nil
}

// appleUpdatedResource changes only TimeMatch's top-level event fields. The
// remaining logical properties and nested components are retained from the
// single owned GET representation (RFC4791 sections5.3.2/5.3.3). Unsupported
// recurrence is refused rather than turning a series into a single event.
func appleUpdatedResource(original, replacement []byte) ([]byte, error) {
	lines := unfoldICS(string(original))
	if len(lines) < 2 || !strings.EqualFold(lines[0], "BEGIN:VCALENDAR") || !strings.EqualFold(lines[len(lines)-1], "END:VCALENDAR") {
		return nil, core.ErrTimeMatchResponse
	}
	var controlled []string
	for _, line := range unfoldICS(string(replacement)) {
		name, _, ok := cutICSValue(line)
		if ok && (name == "DTSTAMP" || name == "DTSTART" || name == "DTEND" || name == "SUMMARY") {
			controlled = append(controlled, line)
		}
	}
	var out strings.Builder
	var components []string
	eventDepth := 0
	for _, line := range lines {
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, icsBeginGeneric) {
			component := strings.TrimPrefix(upper, icsBeginGeneric)
			if component == "VEVENT" {
				if eventDepth != 0 || len(components) != 1 {
					return nil, core.ErrTimeMatchResponse
				}
				eventDepth = len(components) + 1
			}
			components = append(components, component)
		} else if strings.HasPrefix(upper, icsEndGeneric) {
			component := strings.TrimPrefix(upper, icsEndGeneric)
			if len(components) == 0 || components[len(components)-1] != component {
				return nil, core.ErrTimeMatchResponse
			}
			if component == "VEVENT" {
				for _, next := range controlled {
					foldICS(&out, next)
				}
				eventDepth = 0
			}
			components = components[:len(components)-1]
		} else if eventDepth != 0 && len(components) == eventDepth {
			name, _, ok := cutICSValue(line)
			if !ok {
				return nil, core.ErrTimeMatchResponse
			}
			name, _, _ = strings.Cut(strings.ToUpper(name), ";")
			switch name {
			case "RRULE", "RDATE", "EXDATE", "RECURRENCE-ID":
				return nil, core.ErrTimeMatchResponse
			case "DTSTAMP", "DTSTART", "DTEND", "DURATION", "SUMMARY":
				continue
			}
		}
		foldICS(&out, line)
	}
	if len(components) != 0 {
		return nil, core.ErrTimeMatchResponse
	}
	return []byte(out.String()), nil
}

func appleIfMatch(captured exchange.CapturedHeaders) (exchange.Header, error) {
	name, err := core.ParseHTTPHeaderName("ETag")
	if err != nil {
		return exchange.Header{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	observed := ""
	found := false
	for _, field := range captured.Values {
		if field.Name != name {
			continue
		}
		if found || len(field.Values) != 1 {
			return exchange.Header{}, core.ErrTimeMatchResponse
		}
		found = true
		observed, err = field.Values[0].Value()
		if err != nil {
			return exchange.Header{}, errors.Join(core.ErrTimeMatchResponse, err)
		}
	}
	if !found || !appleStrongETag(observed) {
		return exchange.Header{}, core.ErrTimeMatchResponse
	}
	return header("If-Match", observed)
}

// RFC9110 section8.8.3: one strong entity-tag is a quoted opaque sequence;
// weak tags, wildcard/list conditions and embedded quotes cannot establish CAS.
func appleStrongETag(value string) bool {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return false
	}
	for index := 1; index < len(value)-1; index++ {
		octet := value[index]
		if octet < 0x21 || octet == 0x22 || octet == 0x7f {
			return false
		}
	}
	return true
}

// appleUpdate replaces only the observed owned resource and never recreates
// one deleted before the conditional PUT. RFC4791 section5.3.2 uses its ETag.
func (c Client) appleUpdate(ctx context.Context, u EventUpdate) (EventReceipt, error) {
	target, err := appleRefTarget(u.Ref)
	if err != nil {
		return EventReceipt{}, err
	}
	body, err := writeICS(u.Intent, u.ObservedAt)
	if err != nil {
		return EventReceipt{}, err
	}
	defer clear(body)
	auth, err := basic(u.Connection)
	if err != nil {
		return EventReceipt{}, err
	}
	defer clear(auth.Value)
	observed, err := c.appleMutationMatch(ctx, appleReadBackRequest{auth: auth, target: target, ref: u.Ref, policy: u.Policy})
	if err != nil {
		return EventReceipt{}, err
	}
	defer clear(observed.body)
	condition, err := appleIfMatch(observed.captured)
	if err != nil {
		return EventReceipt{}, err
	}
	preserved, err := appleUpdatedResource(observed.body, body)
	if err != nil {
		return EventReceipt{}, err
	}
	defer clear(preserved)
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPut, body: preserved, requestType: icsMediaType(), headers: []exchange.Header{condition}, secrets: []exchange.SecretHeader{auth}, policy: u.Policy, expected: http.StatusNoContent})
	clear(result.body)
	switch {
	case err == nil || result.status == http.StatusOK:
		return EventReceipt{Ref: u.Ref}, nil
	case result.status == http.StatusPreconditionFailed || result.status == http.StatusNotFound || result.status == http.StatusGone:
		return EventReceipt{}, errors.Join(core.ErrTimeMatchConflict, err)
	case result.status == http.StatusCreated || result.status == http.StatusAccepted:
		return EventReceipt{}, errors.Join(core.ErrTimeMatchIndeterminate, err)
	default:
		return EventReceipt{}, writeFailure(result, err)
	}
}

func (c Client) appleDelete(ctx context.Context, d EventDelete) (DeleteOutcome, error) {
	target, err := appleRefTarget(d.Ref)
	if err != nil {
		return DeleteInvalid, err
	}
	auth, err := basic(d.Connection)
	if err != nil {
		return DeleteInvalid, err
	}
	defer clear(auth.Value)
	observed, err := c.appleMutationMatch(ctx, appleReadBackRequest{auth: auth, target: target, ref: d.Ref, policy: d.Policy})
	if err != nil {
		return DeleteInvalid, err
	}
	defer clear(observed.body)
	condition, err := appleIfMatch(observed.captured)
	if err != nil {
		return DeleteInvalid, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodDelete, headers: []exchange.Header{condition}, secrets: []exchange.SecretHeader{auth}, policy: d.Policy, expected: http.StatusNoContent})
	clear(result.body)
	switch {
	case err == nil || result.status == http.StatusOK:
		return Deleted, nil
	case result.status == http.StatusNotFound || result.status == http.StatusGone:
		return DeleteAlreadyAbsent, nil
	case result.status == http.StatusPreconditionFailed:
		return DeleteInvalid, errors.Join(core.ErrTimeMatchConflict, err)
	case result.status == http.StatusAccepted:
		return DeleteInvalid, errors.Join(core.ErrTimeMatchIndeterminate, err)
	default:
		return DeleteInvalid, writeFailure(result, err)
	}
}
