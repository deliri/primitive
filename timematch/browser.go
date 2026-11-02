package timematch

import (
	_ "embed"
	json "encoding/json/v2"
	"errors"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Browser wire: the documents exchanged between the TimeMatch browser module
// and the application's own server routes. The browser never receives a
// provider credential, calendar identifier, event title or attendee: only an
// aggregate busy projection, a provider consent location, and outcomes.

//go:embed browser_module.js
var browserModuleSource string

// BrowserModule emits the TimeMatch browser client. It performs HTTP only
// through exchange.BrowserModule's sendJSON, which the application injects.
func BrowserModule() string {
	return temporal.BrowserModule() + "\n" + strings.NewReplacer(
		"__CONTRACT_IDENTITY__", strconv.Itoa(int(core.ErrTimeMatchContract)),
		"__RESPONSE_IDENTITY__", strconv.Itoa(int(core.ErrTimeMatchResponse)),
	).Replace(browserModuleSource)
}

const (
	browserStateKnown   = "known"
	browserStateUnknown = "unknown"
)

type browserSpanWire struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func spanWire(b temporal.IntervalBounds) (browserSpanWire, error) {
	start, startErr := b.Start.RFC3339Nano()
	end, endErr := b.End.RFC3339Nano()
	if err := errors.Join(startErr, endErr); err != nil {
		return browserSpanWire{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return browserSpanWire{Start: start, End: end}, nil
}

func (w browserSpanWire) bounds() (temporal.IntervalBounds, error) {
	start, startErr := temporal.ParseRFC3339(w.Start)
	end, endErr := temporal.ParseRFC3339(w.End)
	if err := errors.Join(startErr, endErr); err != nil {
		return temporal.IntervalBounds{}, errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	return temporal.IntervalBounds{Start: start, End: end}, nil
}

// BrowserAvailabilityRequest is the browser's window request.
type BrowserAvailabilityRequest struct{ Window temporal.IntervalBounds }

func (r BrowserAvailabilityRequest) Validate() error { return positiveInterval(r.Window) }

func (r BrowserAvailabilityRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	wire, err := spanWire(r.Window)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wire)
}

func (r *BrowserAvailabilityRequest) UnmarshalJSON(raw []byte) error {
	if r == nil {
		return core.ErrTimeMatchContract
	}
	var wire browserSpanWire
	if err := json.Unmarshal(raw, &wire, json.RejectUnknownMembers(true)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	window, err := wire.bounds()
	if err != nil {
		return err
	}
	candidate := BrowserAvailabilityRequest{Window: window}
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

// BrowserAvailability is the aggregate busy projection for one window. It is
// Unknown, with no busy facts, when any contributing calendar was Unknown.
type BrowserAvailability struct {
	Busy   []temporal.IntervalBounds
	Window temporal.IntervalBounds
	State  AvailabilityState
}

// Browser merges every calendar into one provider-neutral projection.
func (a Availability) Browser() (BrowserAvailability, error) {
	if err := a.Validate(); err != nil {
		return BrowserAvailability{}, err
	}
	var spans [][2]int64
	for _, calendar := range a.Calendars {
		if calendar.State == AvailabilityUnknown {
			return BrowserAvailability{Window: a.Window, State: AvailabilityUnknown}, nil
		}
		for _, busy := range calendar.Busy {
			start, end, boundsErr := nanosecondBounds(busy)
			if boundsErr != nil {
				return BrowserAvailability{}, boundsErr
			}
			spans = append(spans, [2]int64{start, end})
		}
	}
	busy, err := normalizeBusy(a.Window, spans)
	if err != nil {
		return BrowserAvailability{}, err
	}
	return BrowserAvailability{Window: a.Window, State: AvailabilityKnown, Busy: busy}, nil
}

func (b BrowserAvailability) Validate() error {
	calendar := Availability{Window: b.Window, Calendars: []CalendarAvailability{{ID: CalendarID{value: "browser"}, State: b.State, Busy: b.Busy}}}
	return calendar.Validate()
}

type browserAvailabilityWire struct {
	Start string            `json:"start"`
	End   string            `json:"end"`
	State string            `json:"state"`
	Busy  []browserSpanWire `json:"busy"`
}

func (b BrowserAvailability) MarshalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	window, err := spanWire(b.Window)
	if err != nil {
		return nil, err
	}
	wire := browserAvailabilityWire{Start: window.Start, End: window.End, State: browserStateKnown, Busy: make([]browserSpanWire, 0, len(b.Busy))}
	if b.State == AvailabilityUnknown {
		wire.State = browserStateUnknown
	}
	for _, busy := range b.Busy {
		span, spanErr := spanWire(busy)
		if spanErr != nil {
			return nil, spanErr
		}
		wire.Busy = append(wire.Busy, span)
	}
	return json.Marshal(wire)
}

func (b *BrowserAvailability) UnmarshalJSON(raw []byte) error {
	if b == nil {
		return core.ErrTimeMatchContract
	}
	var wire browserAvailabilityWire
	if err := json.Unmarshal(raw, &wire, json.RejectUnknownMembers(true)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	window, err := browserSpanWire{Start: wire.Start, End: wire.End}.bounds()
	if err != nil {
		return err
	}
	candidate := BrowserAvailability{Window: window}
	switch wire.State {
	case browserStateKnown:
		candidate.State = AvailabilityKnown
	case browserStateUnknown:
		candidate.State = AvailabilityUnknown
	default:
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract)
	}
	for _, span := range wire.Busy {
		busy, boundsErr := span.bounds()
		if boundsErr != nil {
			return boundsErr
		}
		candidate.Busy = append(candidate.Busy, busy)
	}
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	*b = candidate
	return nil
}

// BrowserConnectionRequest asks the server to begin OAuth consent.
type BrowserConnectionRequest struct{ Provider Provider }

func (r BrowserConnectionRequest) Validate() error {
	if r.Provider != ProviderGoogle && r.Provider != ProviderMicrosoft {
		return core.ErrTimeMatchContract
	}
	return nil
}

type browserConnectionWire struct {
	Provider string `json:"provider"`
}

func (r BrowserConnectionRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return json.Marshal(browserConnectionWire{Provider: r.Provider.String()})
}

func (r *BrowserConnectionRequest) UnmarshalJSON(raw []byte) error {
	if r == nil {
		return core.ErrTimeMatchContract
	}
	var wire browserConnectionWire
	if err := json.Unmarshal(raw, &wire, json.RejectUnknownMembers(true)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	for _, provider := range []Provider{ProviderGoogle, ProviderMicrosoft} {
		if wire.Provider == provider.String() {
			*r = BrowserConnectionRequest{Provider: provider}
			return nil
		}
	}
	return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract)
}

// BrowserConnectionStart carries the provider consent location the browser
// navigates to. Only Google's and Microsoft's documented authorization hosts
// are admitted.
type BrowserConnectionStart struct{ Location core.HTTPEndpoint }

func (s BrowserConnectionStart) Validate() error {
	if err := s.Location.Validate(); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	u := s.Location.HTTPURL()
	if u.Scheme != core.SchemeHTTPS || u.User != nil || u.Port() != "" ||
		!(u.Host == "accounts.google.com" && u.Path == "/o/oauth2/v2/auth" || u.Host == "login.microsoftonline.com" && strings.HasSuffix(u.Path, "/oauth2/v2.0/authorize")) {
		return core.ErrTimeMatchContract
	}
	return nil
}

type browserLocationWire struct {
	Location string `json:"location"`
}

func (s BrowserConnectionStart) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return json.Marshal(browserLocationWire{Location: s.Location.String()})
}

func (s *BrowserConnectionStart) UnmarshalJSON(raw []byte) error {
	if s == nil {
		return core.ErrTimeMatchContract
	}
	var wire browserLocationWire
	if err := json.Unmarshal(raw, &wire, json.RejectUnknownMembers(true)); err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	location, err := core.ParseHTTPEndpoint(wire.Location)
	if err != nil {
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	candidate := BrowserConnectionStart{Location: location}
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = candidate
	return nil
}

// BrowserAppleConnection is the server-side ingress for an Apple Account and
// app-specific password typed by the visitor. It is receive-only: the server
// never emits it, and the password is held in Secret custody.
type BrowserAppleConnection struct {
	Password Secret
	Account  AppleAccount
}

func (c BrowserAppleConnection) Validate() error {
	if !c.Password.IsSet() {
		return core.ErrTimeMatchContract
	}
	return errors.Join(c.Account.Validate(), c.Password.Validate())
}

func (BrowserAppleConnection) MarshalJSON() ([]byte, error) { return nil, core.ErrTimeMatchContract }

func (c *BrowserAppleConnection) UnmarshalJSON(raw []byte) error {
	if c == nil {
		return core.ErrTimeMatchContract
	}
	var wire struct {
		Account  string     `json:"account"`
		Password wireSecret `json:"password"`
	}
	if err := json.Unmarshal(raw, &wire, json.RejectUnknownMembers(true)); err != nil {
		clear(wire.Password.value)
		return errors.Join(core.ErrTimeMatchContract, core.ErrJSONContract, err)
	}
	defer clear(wire.Password.value)
	account, err := ParseAppleAccount(wire.Account)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	password, err := ParseSecret(wire.Password.value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = BrowserAppleConnection{Account: account, Password: password}
	return nil
}

// Connection converts the admitted ingress into an Apple provider capability.
func (c BrowserAppleConnection) Connection() (Connection, error) {
	if err := c.Validate(); err != nil {
		return Connection{}, err
	}
	return Connection{Provider: ProviderApple, AppleAccount: c.Account, ApplePassword: c.Password}, nil
}

func (c *BrowserAppleConnection) Close() error {
	if c == nil {
		return core.ErrTimeMatchContract
	}
	err := c.Password.Close()
	*c = BrowserAppleConnection{}
	return err
}
