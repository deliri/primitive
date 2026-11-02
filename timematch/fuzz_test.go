package timematch

import (
	"bytes"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Typed documented wire forms used to emit valid seeds. Provider pages name
// these members; the production decoders are the doors under test.
type seedToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token,omitzero"`
}

type seedGoogleItem struct {
	ID         string `json:"id"`
	AccessRole string `json:"accessRole"`
	Primary    bool   `json:"primary,omitzero"`
	DataOwner  string `json:"dataOwner,omitzero"`
}

type seedGooglePage struct {
	Items         []seedGoogleItem `json:"items"`
	NextPageToken string           `json:"nextPageToken,omitzero"`
}

type seedBusy struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type seedError struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
}

type seedBusyCalendar struct {
	Errors []seedError `json:"errors,omitzero"`
	Busy   []seedBusy  `json:"busy"`
}

type seedFreeBusy struct {
	TimeMin   string                      `json:"timeMin"`
	TimeMax   string                      `json:"timeMax"`
	Calendars map[string]seedBusyCalendar `json:"calendars"`
}

type seedGraphCalendar struct {
	ID                string `json:"id"`
	CanEdit           bool   `json:"canEdit"`
	IsDefaultCalendar bool   `json:"isDefaultCalendar"`
}

type seedGraphCalendars struct {
	Value    []seedGraphCalendar `json:"value"`
	NextLink string              `json:"@odata.nextLink,omitzero"`
}

type seedGraphEvent struct {
	Start       graphDateTime `json:"start"`
	End         graphDateTime `json:"end"`
	ShowAs      string        `json:"showAs"`
	IsCancelled bool          `json:"isCancelled"`
	IsAllDay    bool          `json:"isAllDay"`
}

type seedGraphView struct {
	Value    []seedGraphEvent `json:"value"`
	NextLink string           `json:"@odata.nextLink,omitzero"`
}

func seedJSON[T any](tb testing.TB, value T) []byte {
	tb.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		tb.Fatalf("json.Marshal(seed) error = %v, want nil", err)
	}
	return encoded
}

func fmtValue[T any](value T) string { return fmt.Sprintf("%#v", value) }

func addHostile(f *testing.F) {
	for _, raw := range []string{"", " ", "null", "{}", "[]", `{"a":`, `{"a":1}{"b":2}`, "\xff", strings.Repeat("[", 80)} {
		f.Add([]byte(raw))
	}
}

func FuzzTokenWireSemanticClosure(f *testing.F) {
	f.Add(seedJSON(f, seedToken{AccessToken: "ya29.a", ExpiresIn: 3600, TokenType: "Bearer", Scope: googleVisitorScope}))
	f.Add(seedJSON(f, seedToken{AccessToken: "eyJ.b", ExpiresIn: 1, TokenType: "bearer", Scope: "Calendars.ReadBasic", RefreshToken: "0.r"}))
	f.Add([]byte(`{"access_token":"a b","expires_in":10,"token_type":"Bearer"}`))
	f.Add([]byte(`{"access_token":"a","access_token":"b","expires_in":10,"token_type":"Bearer"}`))
	f.Add([]byte(`{"access_token":"a","expires_in":0,"token_type":"Bearer"}`))
	f.Add(seedJSON(f, struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}{AccessToken: "ya29.omitted-scope", ExpiresIn: 3600, TokenType: "Bearer"}))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[tokenWire](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.access != nil || got.refresh != nil || got.expiresIn != 0 {
				t.Fatalf("decode(rejected) = (%+v, %v), want zero and ErrTimeMatchResponse", got, err)
			}
			return
		}
		if !visibleASCII(got.access) || got.expiresIn <= 0 || !strings.EqualFold(got.tokenType, bearerTokenType) || len(got.refresh) != 0 && !visibleASCII(got.refresh) {
			t.Fatalf("accepted token %+v violates the admitted credential grammar", got)
		}
		var independentlyDecoded struct {
			Scope *string `json:"scope"`
		}
		if err := jsonv1.Unmarshal(data, &independentlyDecoded); err != nil || got.scopePresent != (independentlyDecoded.Scope != nil) {
			t.Fatalf("scope presence=%t,independent=%t,error=%v,want exact omitted-versus-present fact", got.scopePresent, independentlyDecoded.Scope != nil, err)
		}
		var scope *string
		if got.scopePresent {
			value := got.scope
			scope = &value
		}
		canonical := seedJSON(t, struct {
			AccessToken  string  `json:"access_token"`
			ExpiresIn    int64   `json:"expires_in"`
			TokenType    string  `json:"token_type"`
			Scope        *string `json:"scope,omitzero"`
			RefreshToken string  `json:"refresh_token,omitzero"`
		}{AccessToken: string(got.access), ExpiresIn: got.expiresIn, TokenType: got.tokenType, Scope: scope, RefreshToken: string(got.refresh)})
		again, err := decodeJSON[tokenWire](canonical)
		if err != nil || !bytes.Equal(again.access, got.access) || !bytes.Equal(again.refresh, got.refresh) || again.scope != got.scope || again.scopePresent != got.scopePresent || again.expiresIn != got.expiresIn {
			t.Fatalf("canonical round trip = (%+v, %v), want %+v", again, err, got)
		}
	})
}

func FuzzOAuthFailureNeverPromotesOrEchoes(f *testing.F) {
	for _, code := range []string{"invalid_grant", "consent_required", "interaction_required", "invalid_client", "invalid_request"} {
		f.Add(400, []byte(`{"error":"`+code+`","error_description":"`+privateMarker+`"}`))
	}
	f.Add(401, []byte(`{"error":"invalid_grant"}`))
	f.Add(500, []byte(`{"error":"invalid_grant"}`))
	f.Add(400, []byte(`{"error":"invalid_grant","error":"x"}`))
	f.Add(400, []byte(privateMarker))
	f.Fuzz(func(t *testing.T, status int, body []byte) {
		owned := append([]byte(nil), body...)
		err := oauthFailure(outcome{status: status, body: owned}, core.ErrExchangeResponse)
		consent := errors.Is(err, core.ErrTimeMatchConsent)
		// Independent decoder: the standard v1 generic path, not the SDK's.
		var document struct {
			Error string `json:"error"`
		}
		_ = jsonv1.Unmarshal(body, &document)
		named := document.Error == "invalid_grant" || document.Error == "consent_required" || document.Error == "interaction_required"
		if consent && (status != http.StatusBadRequest && status != http.StatusUnauthorized || !named) || !consent && !errors.Is(err, core.ErrTimeMatchResponse) {
			t.Fatalf("oauthFailure(%d, %q) = %v, want consent only for a documented code on 400/401", status, body, err)
		}
		if !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("oauthFailure() = %v, want a TimeMatch identity", err)
		}
		if strings.Contains(err.Error(), privateMarker) || len(owned) != 0 && bytes.ContainsAny(owned, "abcdefghijklmnopqrstuvwxyz") {
			t.Fatalf("oauthFailure echoed or retained the provider body: %v / %q", err, owned)
		}
	})
}

func FuzzGoogleCalendarPageSemanticClosure(f *testing.F) {
	f.Add(seedJSON(f, seedGooglePage{Items: []seedGoogleItem{{ID: "primary@example.com", AccessRole: "owner", Primary: true}, {ID: "x", AccessRole: "freeBusyReader"}}, NextPageToken: "p2"}))
	f.Add(seedJSON(f, seedGooglePage{Items: []seedGoogleItem{}}))
	f.Add(seedJSON(f, seedGooglePage{Items: []seedGoogleItem{{ID: "x", AccessRole: "admin"}}}))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[googleCalendarPage](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.items != nil || got.next != "" {
				t.Fatalf("decode(rejected) = (%+v, %v), want zero and ErrTimeMatchResponse", got, err)
			}
			return
		}
		page := seedGooglePage{NextPageToken: got.next, Items: []seedGoogleItem{}}
		for _, item := range got.items {
			if item.ID.Validate() != nil {
				t.Fatalf("accepted calendar id %q is invalid", item.ID)
			}
			role := "reader"
			if item.Writable {
				role = "owner"
			}
			page.Items = append(page.Items, seedGoogleItem{ID: item.ID.value, AccessRole: role, Primary: item.Primary, DataOwner: got.dataOwners[item.ID.value]})
		}
		again, err := decodeJSON[googleCalendarPage](seedJSON(t, page))
		if err != nil || again.next != got.next || len(again.items) != len(got.items) {
			t.Fatalf("canonical round trip = (%+v, %v), want %+v", again, err, got)
		}
		for i := range got.items {
			if again.items[i] != got.items[i] || again.dataOwners[got.items[i].ID.value] != got.dataOwners[got.items[i].ID.value] {
				t.Fatalf("item %d round trip = %+v, want %+v", i, again.items[i], got.items[i])
			}
		}
	})
}

func FuzzGoogleFreeBusySemanticClosure(f *testing.F) {
	f.Add(seedJSON(f, seedFreeBusy{TimeMin: "2026-11-02T09:00:00Z", TimeMax: "2026-11-02T17:00:00Z", Calendars: map[string]seedBusyCalendar{"a": {Busy: []seedBusy{{Start: "2026-11-02T10:00:00Z", End: "2026-11-02T11:00:00Z"}}}, "b": {Errors: []seedError{{Domain: "global", Reason: "notFound"}}, Busy: []seedBusy{}}}}))
	f.Add(seedJSON(f, seedFreeBusy{TimeMin: "2026-11-02T09:00:00Z", TimeMax: "2026-11-02T17:00:00Z", Calendars: map[string]seedBusyCalendar{"a": {Busy: []seedBusy{{Start: "2026-11-02T11:00:00Z", End: "2026-11-02T10:00:00Z"}}}}}))
	f.Add(seedJSON(f, seedFreeBusy{TimeMin: "2026-11-02T09:00:00Z", TimeMax: "2026-11-02T17:00:00Z", Calendars: map[string]seedBusyCalendar{"a": {Errors: []seedError{{Domain: "global", Reason: "internalError"}}, Busy: []seedBusy{{Start: "2026-11-02T10:00:00Z", End: "2026-11-02T11:00:00Z"}}}}}))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[googleFreeBusyResponse](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.calendars != nil {
				t.Fatalf("decode(rejected) = (%+v, %v), want zero and ErrTimeMatchResponse", got, err)
			}
			return
		}
		var independent struct {
			Calendars map[string]struct {
				Errors []struct {
					Reason string `json:"reason"`
				} `json:"errors"`
				Busy *[]struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"busy"`
			} `json:"calendars"`
		}
		if err := jsonv1.Unmarshal(data, &independent); err != nil {
			t.Fatalf("accepted freebusy independent decode error=%v,want nil", err)
		}
		for id, calendar := range got.calendars {
			observed := independent.Calendars[id]
			if calendar.known && (observed.Busy == nil || len(observed.Errors) != 0) {
				t.Fatalf("calendar %q known without an explicit complete busy collection", id)
			}
			if !calendar.known && len(calendar.busy) != 0 {
				t.Fatalf("calendar %q unknown but carries %d busy facts", id, len(calendar.busy))
			}
			for _, span := range calendar.busy {
				if span[0] >= span[1] {
					t.Fatalf("calendar %q admitted non-positive span %v", id, span)
				}
			}
		}
	})
}

func FuzzGraphViewPageClassification(f *testing.F) {
	event := func(showAs string) seedGraphEvent {
		return seedGraphEvent{ShowAs: showAs, Start: graphDateTime{DateTime: "2026-11-02T10:00:00.0000000", TimeZone: "UTC"}, End: graphDateTime{DateTime: "2026-11-02T11:00:00.0000000", TimeZone: "UTC"}}
	}
	for _, showAs := range []string{"free", "tentative", "busy", "oof", "workingElsewhere", "unknown", "future"} {
		f.Add(seedJSON(f, seedGraphView{Value: []seedGraphEvent{event(showAs)}}))
	}
	allDay := event("busy")
	allDay.IsAllDay = true
	f.Add(seedJSON(f, seedGraphView{Value: []seedGraphEvent{allDay}, NextLink: "https://graph.microsoft.com/v1.0/me/calendars/x/calendarView?$skip=1"}))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[graphViewPage](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.busy != nil || got.known {
				t.Fatalf("decode(rejected) = (%+v, %v), want zero and ErrTimeMatchResponse", got, err)
			}
			return
		}
		var independent seedGraphView
		if json.Unmarshal(data, &independent) != nil {
			return
		}
		blocking, unclassified := 0, false
		for _, item := range independent.Value {
			switch item.ShowAs {
			case "free", "workingElsewhere":
			case "tentative", "busy", "oof":
				if !item.IsCancelled {
					blocking++
				}
			default:
				unclassified = true
			}
		}
		if unclassified && got.known || len(got.busy) > blocking {
			t.Fatalf("page %+v from %d blocking items (unclassified %t), want unknown or at most that many spans", got, blocking, unclassified)
		}
		for _, span := range got.busy {
			if span[0] >= span[1] {
				t.Fatalf("admitted non-positive span %v", span)
			}
		}
	})
}

func FuzzGraphCalendarPageSemanticClosure(f *testing.F) {
	f.Add(seedJSON(f, seedGraphCalendars{Value: []seedGraphCalendar{{ID: "AAMk==", CanEdit: true, IsDefaultCalendar: true}}, NextLink: "https://graph.microsoft.com/v1.0/me/calendars?$skip=1"}))
	f.Add(seedJSON(f, seedGraphCalendars{Value: []seedGraphCalendar{{ID: ""}}}))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeJSON[graphCalendarPage](data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || got.items != nil {
				t.Fatalf("decode(rejected) = (%+v, %v), want zero and ErrTimeMatchResponse", got, err)
			}
			return
		}
		page := seedGraphCalendars{NextLink: got.next, Value: []seedGraphCalendar{}}
		for _, item := range got.items {
			page.Value = append(page.Value, seedGraphCalendar{ID: item.ID.value, CanEdit: item.Writable, IsDefaultCalendar: item.Primary})
		}
		again, err := decodeJSON[graphCalendarPage](seedJSON(t, page))
		if err != nil || again.next != got.next || len(again.items) != len(got.items) {
			t.Fatalf("canonical round trip = (%+v, %v), want %+v", again, err, got)
		}
	})
}

func FuzzProviderEventObservations(f *testing.F) {
	googleSeed, seedErr := googleEvent(intent(f, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z"))
	if seedErr != nil {
		f.Fatal(seedErr)
	}
	f.Add(googleSeed)
	f.Add(seedJSON(f, graphCreated{ID: "AAMk", Subject: "Visit", TransactionID: eventIDFixture, Start: graphDateTime{DateTime: "2026-11-02T15:00:00.0000000", TimeZone: "UTC"}, End: graphDateTime{DateTime: "2026-11-02T16:00:00.0000000", TimeZone: "UTC"}}))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		owned := intent(t, "primary", "Visit", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
		for _, verify := range []func([]byte) (EventReceipt, error){
			func(body []byte) (EventReceipt, error) {
				return verifyGoogle(body, owned, EventRef{ID: owned.ID, Provider: ProviderGoogle, Calendar: owned.Calendar, Resource: owned.ID.value})
			},
			func(body []byte) (EventReceipt, error) {
				return verifyGraph(body, graphExpectation{intent: owned, calendar: owned.Calendar, create: true})
			},
		} {
			receipt, err := verify(append([]byte(nil), data...))
			if err != nil {
				if receipt != (EventReceipt{}) || !errors.Is(err, core.ErrTimeMatchConflict) && !errors.Is(err, core.ErrTimeMatchIndeterminate) {
					t.Fatalf("verify(rejected) = (%+v, %v), want zero and conflict or indeterminate", receipt, err)
				}
				continue
			}
			var independent struct {
				Summary string `json:"summary"`
				Subject string `json:"subject"`
			}
			if json.Unmarshal(data, &independent) != nil || independent.Summary != "Visit" && independent.Subject != "Visit" || receipt.Validate() != nil {
				t.Fatalf("verify accepted %q without the owned summary or with invalid receipt %+v", data, receipt)
			}
			if receipt.Ref.Provider == ProviderGoogle {
				var binding struct {
					ID                 string                   `json:"id"`
					ExtendedProperties googleExtendedProperties `json:"extendedProperties"`
				}
				if err := json.Unmarshal(data, &binding); err != nil || binding.ID != owned.ID.String() || binding.ExtendedProperties.Private.EventID != owned.ID.String() {
					t.Fatalf("Google receipt accepted missing/foreign private ownership: %+v/%v", binding, err)
				}
			}
		}
	})
}

func FuzzMultistatusDiscovery(f *testing.F) {
	f.Add([]byte(multistatus(davOK("/", `<d:current-user-principal><d:href>/1/principal/</d:href></d:current-user-principal>`))))
	f.Add([]byte(multistatus(davOK("/1/calendars/home/", `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:current-user-privilege-set><d:privilege><d:write/></d:privilege></d:current-user-privilege-set>`))))
	f.Add([]byte(`<!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/passwd">]><d:multistatus xmlns:d="DAV:">&e;</d:multistatus>`))
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		status, err := decodeMultistatus(data)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || len(status.Responses) != 0 {
				t.Fatalf("decodeMultistatus(rejected) = (%d responses, %v), want none and ErrTimeMatchResponse", len(status.Responses), err)
			}
			return
		}
		href, hrefErr := singleHref(status, func(p davPropWire) *davHrefWire { return p.Principal })
		if hrefErr == nil && strings.TrimSpace(href) == "" || hrefErr != nil && !errors.Is(hrefErr, core.ErrTimeMatchResponse) {
			t.Fatalf("singleHref() = (%q, %v), want a non-blank href or a typed refusal", href, hrefErr)
		}
	})
}

func FuzzFreeBusyPeriods(f *testing.F) {
	f.Add(vfreebusy("FREEBUSY:20261102T100000Z/20261102T110000Z", "FREEBUSY;FBTYPE=BUSY-TENTATIVE:20261102T120000Z/PT30M,20261102T130000Z/PT1H"))
	f.Add(vfreebusy("FREEBUSY;FBTYPE=FREE:20261102T100000Z/20261102T110000Z"))
	f.Add(vfreebusy("FREEBUSY;FBTYPE=X-OTHER:20261102T100000Z/20261102T110000Z"))
	f.Add(vfreebusy(`FREEBUSY;X-NOTE="x;FBTYPE=FREE":20261102T100000Z/20261102T110000Z`))
	f.Add(vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z")))
	f.Add("BEGIN:VCALENDAR\r\nBEGIN:VFREEBUSY\r\nFREEBUSY:x/y\r\n")
	f.Fuzz(func(t *testing.T, text string) {
		spans, known := parseFreeBusy(text)
		again, againKnown := parseFreeBusy(text)
		if known != againKnown || len(spans) != len(again) {
			t.Fatalf("parseFreeBusy is not deterministic: (%d, %t) then (%d, %t)", len(spans), known, len(again), againKnown)
		}
		if !known {
			return
		}
		// Independent facts: an admitted answer contains no event component
		// and only positive periods.
		for _, line := range unfoldICS(text) {
			if strings.EqualFold(line, icsBeginEvent) {
				t.Fatalf("parseFreeBusy admitted an answer carrying event data: %q", text)
			}
		}
		for i, span := range spans {
			if span[0] >= span[1] || again[i] != span {
				t.Fatalf("span %d = %v then %v, want positive and identical", i, span, again[i])
			}
		}
	})
}

func FuzzParseICSDeterministicSpans(f *testing.F) {
	f.Add(vcalendar(vevent("UID:a", `DTSTART;X-NOTE="x;VALUE=DATE":20261102T100000Z`, "DTEND:20261102T110000Z")))
	f.Add(vcalendar(vevent("UID:a", "DTSTART:20261102T100000Z", "DURATION:P1DT2H")))
	f.Add(vcalendar(vevent("UID:a", "DTSTART;TZID=X:20261102T100000", "DTEND:20261102T110000Z")))
	f.Add("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n DTSTART:x\r\n")
	f.Fuzz(func(t *testing.T, text string) {
		events, err := parseICS(text)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchResponse) || events != nil {
				t.Fatalf("parseICS(rejected) = (%d, %v), want none and ErrTimeMatchResponse", len(events), err)
			}
			return
		}
		again, againErr := parseICS(text)
		if againErr != nil || len(again) != len(events) {
			t.Fatalf("parseICS is not deterministic: (%d, %v) then (%d, %v)", len(events), err, len(again), againErr)
		}
		for i, event := range events {
			if again[i] != event || event.resolvable && event.end < event.start {
				t.Fatalf("event %d = %+v then %+v, want identical and ordered", i, event, again[i])
			}
		}
	})
}

func FuzzBrowserDocumentsSemanticClosure(f *testing.F) {
	request, _ := BrowserAvailabilityRequest{Window: window(f, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z")}.MarshalJSON()
	known, _ := BrowserAvailability{Window: window(f, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), State: AvailabilityKnown, Busy: []temporal.IntervalBounds{window(f, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z")}}.MarshalJSON()
	unknown, _ := BrowserAvailability{Window: window(f, "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"), State: AvailabilityUnknown}.MarshalJSON()
	connection, _ := BrowserConnectionRequest{Provider: ProviderMicrosoft}.MarshalJSON()
	location, _ := core.ParseHTTPEndpoint("https://accounts.google.com/o/oauth2/v2/auth?client_id=x")
	start, _ := BrowserConnectionStart{Location: location}.MarshalJSON()
	for _, seed := range [][]byte{request, known, unknown, connection, start, []byte(`{"account":"a@icloud.com","password":"abcd-efgh"}`), []byte(`{"start":"2026-11-02T09:00:00Z","end":"2026-11-02T09:00:00Z"}`), []byte(`{"location":"https://attacker.example/o/oauth2/v2/auth"}`)} {
		f.Add(seed)
	}
	addHostile(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		closure[BrowserAvailabilityRequest](t, data)
		closure[BrowserAvailability](t, data)
		closure[BrowserConnectionRequest](t, data)
		closure[BrowserConnectionStart](t, data)
		var apple BrowserAppleConnection
		if err := apple.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) && !errors.Is(err, core.ErrTimeMatchContract) || apple.Password.IsSet() || apple.Account.value != "" {
				t.Fatalf("BrowserAppleConnection(rejected) = %v, want typed refusal and zero receiver", err)
			}
			return
		}
		if apple.Validate() != nil || strings.Contains(fmtValue(apple), string(apple.Password.value)) {
			t.Fatalf("accepted apple connection invalid or printable")
		}
		if err := apple.Close(); err != nil || apple.Password.IsSet() {
			t.Fatalf("Close() = %v, want destroyed", err)
		}
	})
}

type browserDocument interface {
	core.ValidatedJSONMarshaler
}

func closure[T browserDocument, P interface {
	*T
	UnmarshalJSON([]byte) error
}](t *testing.T, data []byte) {
	t.Helper()
	var got T
	if err := P(&got).UnmarshalJSON(data); err != nil {
		var zero T
		if !errors.Is(err, core.ErrJSONContract) && !errors.Is(err, core.ErrTimeMatchContract) || fmtValue(got) != fmtValue(zero) {
			t.Fatalf("%T.UnmarshalJSON(rejected) = %v, want typed refusal and zero receiver", got, err)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("%T accepted invalid value: %v", got, err)
	}
	first, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("%T.MarshalJSON(accepted) error = %v, want nil", got, err)
	}
	var again T
	if err := P(&again).UnmarshalJSON(first); err != nil || fmtValue(again) != fmtValue(got) {
		t.Fatalf("%T canonical round trip = (%v, %v), want %v", got, again, err, got)
	}
	second, err := again.MarshalJSON()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("%T second projection = (%q, %v), want %q", got, second, err, first)
	}
}
