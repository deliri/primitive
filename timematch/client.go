package timematch

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"slices"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const authorizationHeaderText = "Authorization"

// Client borrows the caller's Exchange transport. It holds no credential,
// cache, schedule or retry state; every operation receives its Connection.
type Client struct{ network exchange.Client }

func NewClient(network exchange.Client) (Client, error) {
	client := Client{network: network}
	if err := client.Validate(); err != nil {
		return Client{}, err
	}
	return client, nil
}

func (c Client) Validate() error {
	if err := c.network.Validate(); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

func (c Client) begin(ctx context.Context, request core.Validatable) error {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// call is one admitted Exchange aggregate request. Credentials travel only in
// SecretHeaders or the request body; never in the URL.
type call struct {
	target       core.HTTPEndpoint
	body         []byte
	requestType  core.HTTPMediaType
	responseType core.HTTPMediaType
	headers      []exchange.Header
	secrets      []exchange.SecretHeader
	capture      []core.HTTPHeaderName
	method       exchange.Method
	policy       exchange.OperationPolicy
	expected     int
}

// outcome is the transport fact behind one call: the HTTP status when one
// arrived and the admitted body and captured headers.
type outcome struct {
	body     []byte
	captured exchange.CapturedHeaders
	status   int
}

func (c Client) send(ctx context.Context, request call) (outcome, error) {
	expected := core.HTTPStatusOK()
	if err := expected.AdmitInt(request.expected); err != nil {
		return outcome{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	semantics := exchange.RequestSemantics{Method: request.method, Replay: exchange.ReplaySingleAttempt}
	selection := exchange.HeaderSelection{Names: request.capture}
	headers := exchange.Headers{Values: request.headers}
	var response exchange.BoundedResponse
	var err error
	if request.body == nil {
		response, err = exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
			Context: ctx, Client: c.network, Policy: exchange.NoBodyBoundedPolicy{Operation: request.policy},
			Request: exchange.NoBodyBoundedRequest{
				Target: request.target, Semantics: semantics, SecretHeaders: request.secrets, Headers: headers,
				ExpectedResponseContentType: request.responseType, CaptureHeaders: selection, ExpectedStatus: expected,
			},
		})
	} else {
		response, err = exchange.SendBounded(exchange.BoundedCall{
			Context: ctx, Client: c.network, Policy: exchange.BoundedPolicy{Operation: request.policy},
			Request: exchange.BoundedRequest{
				Target: request.target, Semantics: semantics, SecretHeaders: request.secrets, Headers: headers, Body: request.body,
				RequestContentType: request.requestType, ExpectedResponseContentType: request.responseType,
				CaptureHeaders: selection, ExpectedStatus: expected,
			},
		})
	}
	result := outcome{body: response.Body, captured: response.Metadata.Headers}
	var statusErr exchange.StatusError
	switch {
	case err == nil:
		result.status = request.expected
		return result, nil
	case errors.As(err, &statusErr):
		status, statusIntErr := statusErr.Status().Int()
		if statusIntErr == nil {
			result.status = status
		}
	}
	return result, err
}

// readFailure maps a failed read. The provider failure body is discarded: it
// may echo private calendar data and is never part of an error.
func readFailure(result outcome, err error) error {
	clear(result.body)
	switch {
	case result.status == 0 && errors.Is(err, core.ErrExchangeRequest):
		return errors.Join(core.ErrTimeMatchContract, err)
	case result.status == http.StatusUnauthorized || result.status == http.StatusForbidden:
		return errors.Join(core.ErrTimeMatchConsent, err)
	default:
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
}

// providerUnknown reports a failure that only makes one calendar Unknown: a
// provider-side refusal or malformed answer while the caller is still live.
// Consent loss, cancellation and this package's own contract refusals are
// never hidden behind Unknown.
func providerUnknown(ctx context.Context, err error) bool {
	return ctx.Err() == nil && errors.Is(err, core.ErrTimeMatchResponse) && !errors.Is(err, core.ErrTimeMatchConsent) && !errors.Is(err, core.ErrExchangeRequest)
}

// writeFailure maps a failed effect. Only Exchange's request refusal proves
// nothing left the process. Any other failure without a definitive provider
// status (transport loss, cancellation, unreadable response) may have been
// applied: the caller retries the same deterministic intent, which is
// verified rather than duplicated.
func writeFailure(result outcome, err error) error {
	clear(result.body)
	switch {
	case result.status == 0 && errors.Is(err, core.ErrExchangeRequest):
		return errors.Join(core.ErrTimeMatchContract, err)
	case result.status == http.StatusUnauthorized || result.status == http.StatusForbidden:
		return errors.Join(core.ErrTimeMatchConsent, err)
	case result.status == 0 || result.status >= http.StatusInternalServerError:
		return errors.Join(core.ErrTimeMatchIndeterminate, err)
	default:
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
}

func bearer(token Secret) ([]exchange.SecretHeader, error) {
	name, err := core.ParseHTTPHeaderName(authorizationHeaderText)
	if err != nil {
		return nil, errors.Join(core.ErrTimeMatchContract, err)
	}
	return []exchange.SecretHeader{{Name: name, Value: token.value, Scheme: exchange.SecretHeaderBearer}}, nil
}

func header(name, value string) (exchange.Header, error) {
	parsed, err := core.ParseHTTPHeaderName(name)
	if err != nil {
		return exchange.Header{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	headerValue, err := exchange.NewHeaderValue(value)
	if err != nil {
		return exchange.Header{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return exchange.Header{Name: parsed, Values: []exchange.HeaderValue{headerValue}}, nil
}

func endpoint(raw string) (core.HTTPEndpoint, error) {
	parsed, err := core.ParseHTTPEndpoint(raw)
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return parsed, nil
}

// decodeJSON applies Core's strict JSON grammar (UTF-8, duplicate names,
// nesting) through T's own UnmarshalJSON, which admits documented members
// and ignores provider additions. Every decoded value is then validated.
func decodeJSON[T core.Validatable](body []byte) (T, error) {
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		var zero T
		return zero, core.ErrTimeMatchResponse
	}
	value, err := core.DecodeStrictJSONStructure[T](body, core.ExtensibleJSONLimits())
	if err != nil {
		var zero T
		return zero, errors.Join(core.ErrTimeMatchResponse, err)
	}
	if err := value.Validate(); err != nil {
		var zero T
		return zero, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return value, nil
}

// unmarshalExtensible decodes one provider object with Go's default member
// policy: unknown members are ignored, duplicate names are refused.
func unmarshalExtensible[T any](raw []byte, target *T) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
	return nil
}

// normalizeBusy clips intervals to the window, drops empty ones, sorts and
// merges overlapping or touching intervals. Busy facts are never invented.
func normalizeBusy(window temporal.IntervalBounds, spans [][2]int64) ([]temporal.IntervalBounds, error) {
	windowStart, windowEnd, err := nanosecondBounds(window)
	if err != nil {
		return nil, err
	}
	clipped := make([][2]int64, 0, len(spans))
	for _, span := range spans {
		start, end := max(span[0], windowStart), min(span[1], windowEnd)
		if start < end {
			clipped = append(clipped, [2]int64{start, end})
		}
	}
	slices.SortFunc(clipped, func(a, b [2]int64) int {
		if a[0] != b[0] {
			if a[0] < b[0] {
				return -1
			}
			return 1
		}
		return 0
	})
	merged := make([]temporal.IntervalBounds, 0, len(clipped))
	var current [2]int64
	for i, span := range clipped {
		if i > 0 && span[0] <= current[1] {
			current[1] = max(current[1], span[1])
			continue
		}
		if i > 0 {
			merged = append(merged, bounds(current))
		}
		current = span
	}
	if len(clipped) > 0 {
		merged = append(merged, bounds(current))
	}
	return merged, nil
}

func bounds(span [2]int64) temporal.IntervalBounds {
	return temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(span[0]), End: temporal.InstantFromNanoseconds(span[1])}
}

func instantNanoseconds(instant temporal.Instant) (int64, error) {
	value, err := instant.Nanoseconds()
	if err != nil {
		return 0, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return value, nil
}

// expiry derives the access-token expiry from the caller's pre-request
// observation and the provider's admitted expires_in seconds.
func expiry(observed temporal.Instant, seconds int64) (temporal.Instant, error) {
	if seconds <= 0 {
		return temporal.Instant{}, core.ErrTimeMatchResponse
	}
	lifetime, err := temporal.DurationFromSeconds(uint64(seconds))
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	expires, err := observed.Add(lifetime)
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return expires, nil
}
