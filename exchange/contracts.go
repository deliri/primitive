package exchange

import (
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// HeaderMaximumCount bounds caller-supplied request headers.
	HeaderMaximumCount = 64
	// CapturedHeaderMaximumCount bounds retained response metadata.
	CapturedHeaderMaximumCount = 32
	// HeaderValueMaximumBytes bounds one caller-supplied header value.
	HeaderValueMaximumBytes = 8 * 1024
	// HeaderValueMaximumCount bounds repeated values for one field name.
	HeaderValueMaximumCount = 64
	// IdempotencyKeyMaximumBytes bounds one request identity.
	IdempotencyKeyMaximumBytes = 255
	// TransferBufferBytes is the fixed streaming copy buffer.
	TransferBufferBytes = 32 * 1024
)

// Target is a caller-owned validated HTTP destination. HTTPURL returns a value
// copy; Exchange independently enforces generic HTTP confinement before use.
type Target interface {
	core.Validatable
	HTTPURL() url.URL
}

// ReplayMode is the closed request replay contract.
type ReplayMode uint8

const (
	// ReplayUnknown is the invalid zero replay contract.
	ReplayUnknown ReplayMode = iota
	// ReplaySingleAttempt forbids automatic replay.
	ReplaySingleAttempt
	// ReplaySafe admits automatic replay for safe methods.
	ReplaySafe
	// ReplayIdempotent admits automatic replay for idempotent methods.
	ReplayIdempotent
	// ReplayIdempotencyKey admits automatic replay under one explicit key.
	ReplayIdempotencyKey
	replayLimit
)

type replayFact struct {
	diagnostic string
	methods    [methodLimit]bool
}

func replayFacts() [replayLimit]replayFact {
	return [...]replayFact{
		ReplayUnknown: {},
		ReplaySingleAttempt: {
			diagnostic: "single attempt",
			methods: [methodLimit]bool{
				MethodGet: true, MethodHead: true,
				MethodPost: true, MethodPut: true,
				MethodPatch: true, MethodDelete: true,
				MethodOptions: true,
			},
		},
		ReplaySafe: {
			diagnostic: "safe",
			methods: [methodLimit]bool{
				MethodGet: true, MethodHead: true,
				MethodOptions: true,
			},
		},
		ReplayIdempotent: {
			diagnostic: "idempotent",
			methods: [methodLimit]bool{
				MethodPut: true, MethodDelete: true,
			},
		},
		ReplayIdempotencyKey: {
			diagnostic: "idempotency key",
			methods: [methodLimit]bool{
				MethodPost: true, MethodPatch: true,
			},
		},
	}
}

// Validate rejects replay values outside the closed domain.
func (r ReplayMode) Validate() error {
	if r >= replayLimit || replayFacts()[r].diagnostic == "" {
		return core.ErrExchangeContract
	}
	return nil
}

// IsValid reports whether r belongs to the closed replay domain.
func (r ReplayMode) IsValid() bool { return r.Validate() == nil }

// OffWireEnum declares ReplayMode as intentionally off wire.
func (ReplayMode) OffWireEnum() {}

// String returns a diagnostic projection, not a wire value.
func (r ReplayMode) String() string {
	if err := r.Validate(); err != nil {
		return ""
	}
	return replayFacts()[r].diagnostic
}

// RedirectMode is the closed redirect policy.
type RedirectMode uint8

const (
	// RedirectUnknown is the invalid zero redirect contract.
	RedirectUnknown RedirectMode = iota
	// RedirectReject rejects every redirect.
	RedirectReject
	// RedirectSameOrigin permits redirects within one normalized origin only
	// when net/http preserves the typed request method.
	RedirectSameOrigin
	redirectLimit
)

type redirectFact struct {
	diagnostic string
}

func redirectFacts() [redirectLimit]redirectFact {
	return [...]redirectFact{
		RedirectUnknown:    {},
		RedirectReject:     {diagnostic: "reject"},
		RedirectSameOrigin: {diagnostic: "same origin"},
	}
}

// Validate rejects redirect values outside the closed domain.
func (r RedirectMode) Validate() error {
	if r >= redirectLimit ||
		redirectFacts()[r].diagnostic == "" {
		return core.ErrExchangeContract
	}
	return nil
}

// IsValid reports whether r belongs to the closed redirect domain.
func (r RedirectMode) IsValid() bool { return r.Validate() == nil }

// OffWireEnum declares RedirectMode as intentionally off wire.
func (RedirectMode) OffWireEnum() {}

// String returns a diagnostic projection, not a wire value.
func (r RedirectMode) String() string {
	if err := r.Validate(); err != nil {
		return ""
	}
	return redirectFacts()[r].diagnostic
}

// RedirectPolicy owns redirect confinement and the caller-selected hop bound.
// Reject mode requires zero hops. Same-origin mode requires a positive bound
// and rejects redirects that net/http would follow by changing the typed
// request method, including POST to GET rewrites for 301, 302, and 303.
type RedirectPolicy struct {
	MaximumHops uint64
	Mode        RedirectMode
}

// Validate closes redirect mode and hop ownership.
func (p RedirectPolicy) Validate() error {
	if err := p.Mode.Validate(); err != nil {
		return err
	}
	if (p.Mode == RedirectReject) != (p.MaximumHops == 0) {
		return core.ErrExchangeContract
	}
	return nil
}

// IdempotencyKey is a bounded visible-ASCII request identity.
type IdempotencyKey struct {
	value string
}

// ParseIdempotencyKey validates and owns one key.
func ParseIdempotencyKey(value string) (IdempotencyKey, error) {
	if len(value) == 0 || len(value) > IdempotencyKeyMaximumBytes {
		return IdempotencyKey{}, core.ErrExchangeContract
	}
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return IdempotencyKey{}, core.ErrExchangeContract
		}
	}
	return IdempotencyKey{value: value}, nil
}

// Validate rejects the unset key.
func (k IdempotencyKey) Validate() error {
	_, err := ParseIdempotencyKey(k.value)
	return err
}

// IsZero reports whether no key is present.
func (k IdempotencyKey) IsZero() bool {
	return k.value == ""
}

// String returns the wire value.
func (k IdempotencyKey) String() string {
	return k.value
}

// HeaderValue is one bounded HTTP field value. It is a distinct compiler-owned
// wire fact so callers cannot hand Exchange an unclassified string collection.
// The empty value is valid under net/http semantics; collection cardinality is
// owned by Header.
type HeaderValue struct {
	value string
	set   bool
}

// NewHeaderValue admits one exact field value and keeps its bytes behind an
// explicit projection boundary.
func NewHeaderValue(value string) (HeaderValue, error) {
	candidate := HeaderValue{value: value, set: true}
	if err := candidate.Validate(); err != nil {
		return HeaderValue{}, err
	}
	return candidate, nil
}

// Validate rejects oversized values and every byte net/http would interpret as
// a field delimiter or control character.
func (v HeaderValue) Validate() error {
	if !v.set || len(v.value) > HeaderValueMaximumBytes {
		return core.ErrExchangeContract
	}
	if err := core.ValidateHTTPFieldValue(v.value); err != nil {
		return core.ErrExchangeContract
	}
	return nil
}

// Value explicitly projects the validated value for the net/http execution
// leaf or for an owner interpreting captured provider metadata.
func (v HeaderValue) Value() (string, error) {
	if err := v.Validate(); err != nil {
		return "", err
	}
	return v.value, nil
}

// Format prevents ordinary diagnostics from disclosing header values. Owners
// must cross Value explicitly at the exact execution or interpretation leaf.
func (HeaderValue) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Header is one validated field.
type Header struct {
	Name   core.HTTPHeaderName
	Values []HeaderValue
}

// Validate rejects invalid names, absent or excessive values, oversized
// values, and header injection.
func (h Header) Validate() error {
	if err := h.Name.Validate(); err != nil {
		return core.ErrExchangeContract
	}
	if len(h.Values) == 0 || len(h.Values) > HeaderValueMaximumCount {
		return core.ErrExchangeContract
	}
	for _, value := range h.Values {
		if err := value.Validate(); err != nil {
			return core.ErrExchangeContract
		}
	}
	return nil
}

// Headers is an ordered bounded request-header collection.
type Headers struct {
	Values []Header
}

// Validate rejects invalid, duplicate, untransmittable, or Exchange-owned
// fields.
func (h Headers) Validate() error {
	if len(h.Values) > HeaderMaximumCount {
		return core.ErrExchangeContract
	}
	for index, header := range h.Values {
		if err := header.Validate(); err != nil ||
			managedHeader(header.Name) {
			return core.ErrExchangeContract
		}
		for prior := range index {
			if h.Values[prior].Name == header.Name {
				return core.ErrExchangeContract
			}
		}
	}
	return nil
}

// HeaderSelection is a bounded unique response-header projection.
type HeaderSelection struct {
	Names []core.HTTPHeaderName
}

// Validate rejects invalid or duplicate selected names.
func (s HeaderSelection) Validate() error {
	if len(s.Names) > CapturedHeaderMaximumCount {
		return core.ErrExchangeContract
	}
	for index, name := range s.Names {
		if err := name.Validate(); err != nil {
			return core.ErrExchangeContract
		}
		for prior := range index {
			if s.Names[prior] == name {
				return core.ErrExchangeContract
			}
		}
	}
	return nil
}

// CapturedHeaders is the sealed response-header projection requested by the
// caller. It owns its strings and does not alias net/http state.
type CapturedHeaders struct {
	Values []Header
}

// Validate checks the bounded unique projection.
func (h CapturedHeaders) Validate() error {
	if len(h.Values) > CapturedHeaderMaximumCount {
		return core.ErrExchangeContract
	}
	for index, header := range h.Values {
		if err := header.Validate(); err != nil {
			return core.ErrExchangeContract
		}
		for prior := range index {
			if h.Values[prior].Name == header.Name {
				return core.ErrExchangeContract
			}
		}
	}
	return nil
}

// RequestSemantics owns method, replay, and optional idempotency.
type RequestSemantics struct {
	IdempotencyKey IdempotencyKey
	Method         Method
	Replay         ReplayMode
}

// Validate closes the method/replay/idempotency lattice.
func (s RequestSemantics) Validate() error {
	if err := s.Method.Validate(); err != nil {
		return core.ErrExchangeContract
	}
	if err := s.Replay.Validate(); err != nil {
		return err
	}
	hasKey := !s.IdempotencyKey.IsZero()
	if (s.Replay == ReplayIdempotencyKey) != hasKey {
		return core.ErrExchangeContract
	}
	if hasKey {
		if err := s.IdempotencyKey.Validate(); err != nil {
			return err
		}
	}
	return validateReplayMethod(s.Method, s.Replay)
}

func validateReplayMethod(method Method, replay ReplayMode) error {
	if err := method.Validate(); err != nil {
		return core.ErrExchangeContract
	}
	if err := replay.Validate(); err != nil {
		return err
	}
	if replayFacts()[replay].methods[method] {
		return nil
	}
	return core.ErrExchangeContract
}

// AllowsRetry reports whether a complete replayable request may be attempted
// again. Streaming operations separately prohibit retry by construction.
func (s RequestSemantics) AllowsRetry() (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	return s.Replay != ReplaySingleAttempt, nil
}

// RetryPolicy owns finite exponential backoff, jitter, and server-hint bounds.
// A one-attempt policy requires every duration to be zero.
type RetryPolicy struct {
	BaseDelay         temporal.Duration
	MaximumDelay      temporal.Duration
	MaximumJitter     temporal.Duration
	MaximumRetryAfter temporal.Duration
	MaximumWait       temporal.Duration
	MaximumAttempts   uint64
}

// Validate closes retry counts and duration ordering.
func (p RetryPolicy) Validate() error {
	if p.MaximumAttempts == 0 {
		return core.ErrExchangeContract
	}
	if p.MaximumAttempts == 1 {
		return validateZeroRetryDurations(p)
	}
	for _, duration := range [...]temporal.Duration{
		p.BaseDelay,
		p.MaximumDelay,
		p.MaximumJitter,
		p.MaximumRetryAfter,
		p.MaximumWait,
	} {
		if err := duration.Validate(); err != nil || duration.IsZero() {
			return core.ErrExchangeContract
		}
	}
	if greaterDuration(p.BaseDelay, p.MaximumDelay) ||
		greaterDuration(p.MaximumJitter, p.MaximumDelay) ||
		greaterDuration(p.MaximumRetryAfter, p.MaximumWait) {
		return core.ErrExchangeContract
	}
	return nil
}

func validateZeroRetryDurations(policy RetryPolicy) error {
	for _, duration := range [...]temporal.Duration{
		policy.BaseDelay,
		policy.MaximumDelay,
		policy.MaximumJitter,
		policy.MaximumRetryAfter,
		policy.MaximumWait,
	} {
		if err := duration.Validate(); err != nil || !duration.IsZero() {
			return core.ErrExchangeContract
		}
	}
	return nil
}

func greaterDuration(left, right temporal.Duration) bool {
	order, err := left.Compare(right)
	return err != nil || order == core.ComparisonGreater
}

// OperationPolicy owns total and per-attempt budgets, retries, and redirects.
type OperationPolicy struct {
	OperationTimeout temporal.Duration
	AttemptTimeout   temporal.Duration
	Retry            RetryPolicy
	Redirect         RedirectPolicy
}

// Validate closes the complete operation policy.
func (p OperationPolicy) Validate() error {
	if err := validateTimeoutPair(p.OperationTimeout, p.AttemptTimeout); err != nil {
		return err
	}
	if err := p.Retry.Validate(); err != nil {
		return err
	}
	return p.Redirect.Validate()
}

// JSONPolicy owns one bounded strict JSON operation.
type JSONPolicy struct {
	Operation         OperationPolicy
	RequestBodyLimit  core.ByteCount
	ResponseBodyLimit core.ByteCount
}

// Validate enforces Core's strict JSON document maximum.
func (p JSONPolicy) Validate() error {
	if err := p.Operation.Validate(); err != nil {
		return err
	}
	if err := validateJSONLimit(p.RequestBodyLimit); err != nil {
		return err
	}
	return validateJSONLimit(p.ResponseBodyLimit)
}

// NoBodyJSONPolicy owns one body-absent request with a strict JSON response.
type NoBodyJSONPolicy struct {
	Operation         OperationPolicy
	ResponseBodyLimit core.ByteCount
}

// Validate checks the operation and response document limit.
func (p NoBodyJSONPolicy) Validate() error {
	if err := p.Operation.Validate(); err != nil {
		return err
	}
	return validateJSONLimit(p.ResponseBodyLimit)
}

// NoBodyBoundedPolicy owns one body-absent request with an aggregate byte
// response.
type NoBodyBoundedPolicy struct {
	Operation         OperationPolicy
	ResponseBodyLimit core.ByteCount
}

// Validate admits a positive response bound within the aggregate cap.
func (p NoBodyBoundedPolicy) Validate() error {
	if err := p.Operation.Validate(); err != nil {
		return err
	}
	if _, err := p.ResponseBodyLimit.Int64(); err != nil {
		return core.ErrExchangeContract
	}
	return nil
}

// BoundedPolicy owns one aggregate byte request and response.
type BoundedPolicy struct {
	Operation         OperationPolicy
	RequestBodyLimit  core.ByteCount
	ResponseBodyLimit core.ByteCount
}

// Validate admits positive limits representable by net/http and io.
func (p BoundedPolicy) Validate() error {
	if err := p.Operation.Validate(); err != nil {
		return err
	}
	if _, err := p.RequestBodyLimit.Int64(); err != nil {
		return core.ErrExchangeContract
	}
	if _, err := p.ResponseBodyLimit.Int64(); err != nil {
		return core.ErrExchangeContract
	}
	return nil
}

// StreamPolicy owns total and attempt deadlines plus a bounded rejected body.
// Streaming is structurally single-attempt; higher-level owners must reopen or
// rewind their own custody capabilities before another call.
type StreamPolicy struct {
	OperationTimeout temporal.Duration
	AttemptTimeout   temporal.Duration
	ErrorBodyLimit   core.ByteCount
	Redirect         RedirectPolicy
}

// Validate closes the single-attempt streaming policy.
func (p StreamPolicy) Validate() error {
	if err := validateTimeoutPair(p.OperationTimeout, p.AttemptTimeout); err != nil {
		return err
	}
	if _, err := p.ErrorBodyLimit.Int64(); err != nil {
		return core.ErrExchangeContract
	}
	return p.Redirect.Validate()
}

// JSONRequest supplies one structurally present typed JSON body.
type JSONRequest[Body core.ValidatedJSONMarshaler] struct {
	Target         Target
	Body           Body
	Semantics      RequestSemantics
	Headers        Headers
	CaptureHeaders HeaderSelection
	ExpectedStatus core.HTTPStatusCode
}

// NoBodyRequest supplies one request with no body.
type NoBodyRequest struct {
	Target         Target
	Semantics      RequestSemantics
	Headers        Headers
	CaptureHeaders HeaderSelection
	ExpectedStatus core.HTTPStatusCode
}

// NoBodyBoundedRequest supplies one body-absent request with an aggregate byte
// response.
type NoBodyBoundedRequest struct {
	Target                      Target
	Semantics                   RequestSemantics
	ExpectedResponseContentType core.HTTPMediaType
	Headers                     Headers
	CaptureHeaders              HeaderSelection
	ExpectedStatus              core.HTTPStatusCode
}

// BoundedRequest supplies one structurally present aggregate byte body. An
// empty slice is still a body; NoBodyRequest is the body-absent family.
type BoundedRequest struct {
	Target                      Target
	Semantics                   RequestSemantics
	RequestContentType          core.HTTPMediaType
	ExpectedResponseContentType core.HTTPMediaType
	Body                        []byte
	Headers                     Headers
	CaptureHeaders              HeaderSelection
	ExpectedStatus              core.HTTPStatusCode
}

// UploadRequest supplies one caller-owned streaming source.
type UploadRequest struct {
	Target         Target
	Source         io.Reader
	Semantics      RequestSemantics
	ContentType    core.HTTPMediaType
	Headers        Headers
	CaptureHeaders HeaderSelection
	ContentLength  core.ByteLength
	ExpectedStatus core.HTTPStatusCode
}

// DownloadRequest supplies one caller-owned streaming destination.
type DownloadRequest struct {
	Target                      Target
	Destination                 io.Writer
	Semantics                   RequestSemantics
	ExpectedResponseContentType core.HTTPMediaType
	Headers                     Headers
	CaptureHeaders              HeaderSelection
	ResponseBodyLimit           core.ByteCount
	ExpectedStatus              core.HTTPStatusCode
}

func managedHeader(name core.HTTPHeaderName) bool {
	return name == core.HTTPHeaderContentLength() ||
		name == core.HTTPHeaderAcceptEncoding() ||
		name == core.HTTPHeaderContentEncoding() ||
		name == core.HTTPHeaderContentType() ||
		name == core.HTTPHeaderAccept() ||
		name == core.HTTPHeaderIdempotencyKey()
}

func validateTimeoutPair(operation, attempt temporal.Duration) error {
	if err := operation.Validate(); err != nil || operation.IsZero() {
		return core.ErrExchangeContract
	}
	if err := attempt.Validate(); err != nil || attempt.IsZero() {
		return core.ErrExchangeContract
	}
	if greaterDuration(attempt, operation) {
		return core.ErrExchangeContract
	}
	return nil
}

func validateJSONLimit(limit core.ByteCount) error {
	value, err := limit.Uint64()
	defaults := core.DefaultStrictJSONLimits()
	maximum, maximumErr := defaults.DocumentMaximumBytes.Uint64()
	if errors.Join(err, maximumErr) != nil || value > maximum {
		return core.ErrExchangeBodyLimit
	}
	return nil
}

var (
	_ core.ValidatedJSONMarshaler = Method(0)
	_ core.Validatable            = ReplayUnknown
	_ core.OffWireEnum            = ReplayUnknown
	_ core.Validatable            = RedirectUnknown
	_ core.OffWireEnum            = RedirectUnknown
	_ core.Validatable            = RedirectPolicy{}
	_ core.Validatable            = IdempotencyKey{}
	_ core.Validatable            = HeaderValue{}
	_ core.Validatable            = Header{}
	_ core.Validatable            = Headers{}
	_ core.Validatable            = HeaderSelection{}
	_ core.Validatable            = CapturedHeaders{}
	_ core.Validatable            = RequestSemantics{}
	_ core.Validatable            = RetryPolicy{}
	_ core.Validatable            = OperationPolicy{}
	_ core.Validatable            = JSONPolicy{}
	_ core.Validatable            = NoBodyJSONPolicy{}
	_ core.Validatable            = NoBodyBoundedPolicy{}
	_ core.Validatable            = BoundedPolicy{}
	_ core.Validatable            = StreamPolicy{}
)
