package exchange

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	officialSDKPathAffixMaximumBytes  = 16 * 1024
	officialSDKQueryNameMaximumBytes  = 128
	officialSDKQueryValueMaximumBytes = 1024
)

type officialSDKResponseScope uint8

const (
	officialSDKResponseScopeUnknown officialSDKResponseScope = iota
	officialSDKResponseScopeAllPaths
	officialSDKResponseScopeSelectedPath
	officialSDKResponseScopeLimit
)

func (s officialSDKResponseScope) Validate() error {
	if s <= officialSDKResponseScopeUnknown || s >= officialSDKResponseScopeLimit {
		return core.ErrExchangeContract
	}
	return nil
}

// OfficialSDKResponseRepresentation selects the raw representation Exchange
// must validate before an official SDK may decode a response.
type OfficialSDKResponseRepresentation uint8

const (
	OfficialSDKResponseRepresentationUnknown OfficialSDKResponseRepresentation = iota
	OfficialSDKResponseRepresentationBinary
	OfficialSDKResponseRepresentationJSON
	officialSDKResponseRepresentationLimit
)

func officialSDKResponseRepresentationFacts() [officialSDKResponseRepresentationLimit]string {
	return [...]string{
		OfficialSDKResponseRepresentationUnknown: "",
		OfficialSDKResponseRepresentationBinary:  "binary",
		OfficialSDKResponseRepresentationJSON:    "json",
	}
}

func parseOfficialSDKResponseRepresentation(value string) (OfficialSDKResponseRepresentation, error) {
	for representation := OfficialSDKResponseRepresentationBinary; representation < officialSDKResponseRepresentationLimit; representation++ {
		if representation.String() == value {
			return representation, nil
		}
	}
	return OfficialSDKResponseRepresentationUnknown, errors.Join(
		core.ErrExchangeContract,
		errors.New("official SDK response representation is not admitted"),
	)
}

// String returns the canonical representation token, or empty text when invalid.
func (r OfficialSDKResponseRepresentation) String() string {
	if r >= officialSDKResponseRepresentationLimit {
		return ""
	}
	return officialSDKResponseRepresentationFacts()[r]
}

// Validate rejects an unset or future response representation.
func (r OfficialSDKResponseRepresentation) Validate() error {
	if r <= OfficialSDKResponseRepresentationUnknown ||
		r >= officialSDKResponseRepresentationLimit ||
		officialSDKResponseRepresentationFacts()[r] == "" {
		return errors.Join(
			core.ErrExchangeContract,
			errors.New("official SDK response representation is invalid"),
		)
	}
	return nil
}

// IsValid reports whether r belongs to the supported representation domain.
func (r OfficialSDKResponseRepresentation) IsValid() bool { return r.Validate() == nil }

// MarshalJSON emits the exact canonical representation token.
func (r OfficialSDKResponseRepresentation) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(r.String())
}

// UnmarshalJSON accepts one exact representation supported by Exchange.
func (r *OfficialSDKResponseRepresentation) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(
			core.ErrJSONContract,
			core.ErrExchangeContract,
			errors.New("nil official SDK response representation receiver"),
		)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	parsed, err := parseOfficialSDKResponseRepresentation(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = parsed
	return nil
}

// OfficialSDKResponseBoundary confines one official-SDK response without
// replacing the SDK's provider request or decoding contract.
type OfficialSDKResponseBoundary struct {
	prefix           string
	suffix           string
	streamQueryName  string
	streamQueryValue string
	method           Method
	representation   OfficialSDKResponseRepresentation
	scope            officialSDKResponseScope
	streamSuccess    bool
	set              bool
}

// OfficialSDKResponseBoundaryRequest selects one method and provider path shape.
type OfficialSDKResponseBoundaryRequest struct {
	PathPrefix     string
	PathSuffix     string
	Method         Method
	Representation OfficialSDKResponseRepresentation
}

// Validate rejects an invalid selected-path response boundary request.
func (r OfficialSDKResponseBoundaryRequest) Validate() error {
	return r.boundary().Validate()
}

func (r OfficialSDKResponseBoundaryRequest) boundary() OfficialSDKResponseBoundary {
	return OfficialSDKResponseBoundary{
		method:         r.Method,
		prefix:         r.PathPrefix,
		suffix:         r.PathSuffix,
		representation: r.Representation,
		scope:          officialSDKResponseScopeSelectedPath,
		set:            true,
	}
}

// NewOfficialSDKResponseBoundary selects one method and provider path shape.
func NewOfficialSDKResponseBoundary(request OfficialSDKResponseBoundaryRequest) (OfficialSDKResponseBoundary, error) {
	if err := request.Validate(); err != nil {
		return OfficialSDKResponseBoundary{}, err
	}
	return request.boundary(), nil
}

// OfficialSDKMethodResponseBoundaryRequest selects the representation for one method.
type OfficialSDKMethodResponseBoundaryRequest struct {
	Method         Method
	Representation OfficialSDKResponseRepresentation
}

// Validate rejects an invalid method-wide response contract.
func (r OfficialSDKMethodResponseBoundaryRequest) Validate() error {
	return r.boundary().Validate()
}

func (r OfficialSDKMethodResponseBoundaryRequest) boundary() OfficialSDKResponseBoundary {
	return OfficialSDKResponseBoundary{
		method:         r.Method,
		representation: r.Representation,
		scope:          officialSDKResponseScopeAllPaths,
		set:            true,
	}
}

// NewOfficialSDKMethodResponseBoundary validates responses for one method. It is
// used for SDK-owned authentication endpoints whose provider path is selected
// by the credential implementation rather than by a product.
func NewOfficialSDKMethodResponseBoundary(request OfficialSDKMethodResponseBoundaryRequest) (OfficialSDKResponseBoundary, error) {
	if err := request.Validate(); err != nil {
		return OfficialSDKResponseBoundary{}, err
	}
	return request.boundary(), nil
}

// OfficialSDKStreamingResponseBoundaryRequest leaves successful responses for one
// exact SDK query coordinate streaming. Other responses use the declared
// representation. JSON validation retains one complete value before SDK decoding.
type OfficialSDKStreamingResponseBoundaryRequest struct {
	StreamQueryName         string
	StreamQueryValue        string
	Method                  Method
	AggregateRepresentation OfficialSDKResponseRepresentation
}

// Validate rejects an invalid streaming-success response policy.
func (r OfficialSDKStreamingResponseBoundaryRequest) Validate() error {
	return r.boundary().Validate()
}

func (r OfficialSDKStreamingResponseBoundaryRequest) boundary() OfficialSDKResponseBoundary {
	return OfficialSDKResponseBoundary{
		method:           r.Method,
		streamQueryName:  r.StreamQueryName,
		streamQueryValue: r.StreamQueryValue,
		representation:   r.AggregateRepresentation,
		scope:            officialSDKResponseScopeAllPaths,
		streamSuccess:    true,
		set:              true,
	}
}

// NewOfficialSDKStreamingResponseBoundary confines aggregate SDK responses and
// leaves one exact successful media-style response as an owned stream.
func NewOfficialSDKStreamingResponseBoundary(
	request OfficialSDKStreamingResponseBoundaryRequest,
) (OfficialSDKResponseBoundary, error) {
	if err := request.Validate(); err != nil {
		return OfficialSDKResponseBoundary{}, err
	}
	return request.boundary(), nil
}

// Validate rejects unset, unknown, or malformed response contracts.
func (b OfficialSDKResponseBoundary) Validate() error {
	if err := b.validateIdentity(); err != nil {
		return err
	}
	return b.validateScope()
}

func (b OfficialSDKResponseBoundary) validateIdentity() error {
	if !b.set || b.method.Validate() != nil ||
		b.scope.Validate() != nil || b.representation.Validate() != nil {
		return core.ErrExchangeContract
	}
	return nil
}

func (b OfficialSDKResponseBoundary) validateScope() error {
	if err := b.validatePathScope(); err != nil {
		return err
	}
	return b.validateStreamingScope()
}

func (b OfficialSDKResponseBoundary) validatePathScope() error {
	switch b.scope {
	case officialSDKResponseScopeAllPaths:
		if b.prefix != "" || b.suffix != "" {
			return core.ErrExchangeContract
		}
	case officialSDKResponseScopeSelectedPath:
		if !validOfficialSDKPathPrefix(b.prefix) || !validOfficialSDKPathSuffix(b.suffix) {
			return core.ErrExchangeContract
		}
	default:
		return core.ErrExchangeContract
	}
	return nil
}

func (b OfficialSDKResponseBoundary) validateStreamingScope() error {
	if b.streamSuccess {
		if b.scope != officialSDKResponseScopeAllPaths ||
			!validOfficialSDKQueryName(b.streamQueryName) ||
			!validOfficialSDKQueryValue(b.streamQueryValue) {
			return core.ErrExchangeContract
		}
		return nil
	}
	if b.streamQueryName != "" || b.streamQueryValue != "" {
		return core.ErrExchangeContract
	}
	return nil
}

func validOfficialSDKPathPrefix(value string) bool {
	return len(value) > 0 && len(value) <= officialSDKPathAffixMaximumBytes &&
		value[0] == '/' && !strings.ContainsAny(value, "?#")
}

func validOfficialSDKPathSuffix(value string) bool {
	return len(value) > 0 && len(value) <= officialSDKPathAffixMaximumBytes &&
		!strings.ContainsAny(value, "?#")
}

func validOfficialSDKQueryName(value string) bool {
	if len(value) == 0 || len(value) > officialSDKQueryNameMaximumBytes {
		return false
	}
	for index := range len(value) {
		if !validOfficialSDKQueryNameByte(value[index]) {
			return false
		}
	}
	return true
}

func validOfficialSDKQueryNameByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		strings.IndexByte("_-.", value) >= 0
}

func validOfficialSDKQueryValue(value string) bool {
	if len(value) == 0 || len(value) > officialSDKQueryValueMaximumBytes {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character < 0x21 || character > 0x7e || character == '&' || character == '=' || character == '#' {
			return false
		}
	}
	return true
}

func (b OfficialSDKResponseBoundary) matches(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	method, err := parseMethod(request.Method)
	if err != nil || method != b.method {
		return false
	}
	if b.scope == officialSDKResponseScopeAllPaths {
		return true
	}
	path := request.URL.EscapedPath()
	return officialSDKPathPrefixMatches(path, b.prefix) &&
		officialSDKPathSuffixMatches(path, b.suffix)
}

func officialSDKPathPrefixMatches(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return len(path) == len(prefix) || strings.HasSuffix(prefix, "/") || path[len(prefix)] == '/'
}

func officialSDKPathSuffixMatches(path, suffix string) bool {
	if !strings.HasSuffix(path, suffix) {
		return false
	}
	start := len(path) - len(suffix)
	return start == 0 || strings.HasPrefix(suffix, "/") || strings.HasPrefix(suffix, ":") || path[start-1] == '/'
}

// OfficialSDKResponseTransportRequest binds one standard HTTP transport to
// one validated SDK response boundary.
type OfficialSDKResponseTransportRequest struct {
	Base     http.RoundTripper
	Boundary OfficialSDKResponseBoundary
}

// Validate rejects an absent standard transport or invalid response boundary.
func (r OfficialSDKResponseTransportRequest) Validate() error {
	if r.Base == nil {
		return core.ErrExchangeContract
	}
	return r.Boundary.Validate()
}

type officialSDKResponseTransport struct {
	base     http.RoundTripper
	boundary OfficialSDKResponseBoundary
}

// NewStandardOfficialSDKResponseTransport confines the standard Go transport.
func NewStandardOfficialSDKResponseTransport(
	boundary OfficialSDKResponseBoundary,
) (http.RoundTripper, error) {
	return NewOfficialSDKResponseTransport(OfficialSDKResponseTransportRequest{
		Base: http.DefaultTransport, Boundary: boundary,
	})
}

// NewOfficialSDKResponseTransport confines one already-owned SDK transport.
func NewOfficialSDKResponseTransport(
	request OfficialSDKResponseTransportRequest,
) (http.RoundTripper, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return officialSDKResponseTransport{
		base: request.Base, boundary: request.Boundary,
	}, nil
}

// NewOfficialSDKHTTPClient projects one SDK transport into a redirect-refusing
// standard client. Context owns timeout and cancellation; the SDK consumer must
// disable any SDK retry so one execution policy remains.
func NewOfficialSDKHTTPClient(transport http.RoundTripper) (*http.Client, error) {
	if transport == nil {
		return nil, core.ErrExchangeContract
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: redirectChecker(redirectCheckRequest{
			policy: RedirectPolicy{Mode: RedirectReject},
		}),
	}, nil
}

func (t officialSDKResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := t.validateRequest(request); err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, officialSDKTransportFailure(request, response, err)
	}
	return t.projectResponse(request, response)
}

func (t officialSDKResponseTransport) validateRequest(request *http.Request) error {
	if t.base == nil || t.boundary.Validate() != nil || request == nil {
		return core.ErrExchangeContract
	}
	if err := contextstate.Validate(request.Context()); err != nil {
		return cancelledError(err)
	}
	return nil
}

func officialSDKTransportFailure(request *http.Request, response *http.Response, cause error) error {
	if contextErr := exchangeContextError(request.Context()); contextErr != nil {
		return errors.Join(contextErr, closeHTTPResponse(response))
	}
	return errors.Join(transportError(cause), closeHTTPResponse(response))
}

func (t officialSDKResponseTransport) projectResponse(
	request *http.Request,
	response *http.Response,
) (*http.Response, error) {
	if response == nil {
		return nil, transportError(core.ErrExchangeContract)
	}
	if core.ReaderIsNil(response.Body) {
		return nil, transportError(core.ErrExchangeContract)
	}
	if !t.boundary.matches(request) {
		return response, nil
	}
	if t.boundary.representation == OfficialSDKResponseRepresentationBinary || t.boundary.streamsSuccessfulResponse(request, response) {
		return response, nil
	}
	payload, readErr := readOfficialSDKResponse(officialSDKResponseReadRequest{
		request: request, response: response,
		representation: t.boundary.representation,
	})
	if readErr != nil {
		return nil, readErr
	}
	response.Body = http.NoBody
	if len(payload) != 0 {
		response.Body = io.NopCloser(bytes.NewReader(payload))
	}
	response.ContentLength = int64(len(payload))
	return response, nil
}

func (b OfficialSDKResponseBoundary) streamsSuccessfulResponse(
	request *http.Request,
	response *http.Response,
) bool {
	if !b.streamSuccess || request == nil || request.URL == nil || response == nil ||
		response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return false
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return false
	}
	values, ok := query[b.streamQueryName]
	return ok && len(values) == 1 && values[0] == b.streamQueryValue
}

type officialSDKResponseReadRequest struct {
	request        *http.Request
	response       *http.Response
	representation OfficialSDKResponseRepresentation
}

func (r officialSDKResponseReadRequest) Validate() error {
	if r.request == nil || r.response == nil || core.ReaderIsNil(r.response.Body) ||
		r.representation.Validate() != nil {
		return core.ErrExchangeContract
	}
	return nil
}

func readOfficialSDKResponse(readRequest officialSDKResponseReadRequest) ([]byte, error) {
	if err := readRequest.Validate(); err != nil {
		return nil, err
	}
	if err := contextstate.Validate(readRequest.request.Context()); err != nil {
		return nil, errors.Join(cancelledError(err), closeResponseBody(readRequest.response.Body))
	}
	declared, err := parseDeclaredBodyLength(readRequest.response.ContentLength)
	if err != nil {
		return nil, errors.Join(responseError(err), closeResponseBody(readRequest.response.Body))
	}
	payload, readErr := readWholeBody(wholeBodyRead{
		context: readRequest.request.Context(), source: readRequest.response.Body,
		declared: declared,
	})
	closeErr := closeResponseBody(readRequest.response.Body)
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(officialSDKResponseReadError(readRequest.request, readErr), closeErr)
	}
	if err := validateOfficialSDKResponsePayload(readRequest.representation, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func officialSDKResponseReadError(request *http.Request, readErr error) error {
	if request == nil {
		return responseError(readErr)
	}
	if contextErr := exchangeContextError(request.Context()); contextErr != nil {
		return errors.Join(responseError(readErr), contextErr)
	}
	return responseError(readErr)
}

func validateOfficialSDKResponsePayload(
	representation OfficialSDKResponseRepresentation,
	payload []byte,
) error {
	if representation != OfficialSDKResponseRepresentationJSON || len(payload) == 0 {
		return nil
	}
	// doctrine:local-allowed=external-wire
	if !jsontext.Value(payload).IsValid() {
		return responseError(core.ErrJSONContract)
	}
	return nil
}

var (
	_ core.Validatable            = OfficialSDKResponseBoundary{}
	_ core.Validatable            = OfficialSDKResponseBoundaryRequest{}
	_ core.Validatable            = OfficialSDKMethodResponseBoundaryRequest{}
	_ core.Validatable            = OfficialSDKStreamingResponseBoundaryRequest{}
	_ core.Validatable            = OfficialSDKResponseTransportRequest{}
	_ core.Validatable            = officialSDKResponseReadRequest{}
	_ core.Validatable            = officialSDKResponseScopeUnknown
	_ core.Validatable            = OfficialSDKResponseRepresentationUnknown
	_ core.ValidatedJSONMarshaler = OfficialSDKResponseRepresentation(0)
)
