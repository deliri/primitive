package twilio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"slices"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	twilioclient "github.com/twilio/twilio-go/client"
)

type WebhookRepresentation uint8

const (
	WebhookRepresentationUnknown WebhookRepresentation = iota
	WebhookRepresentationForm
	WebhookRepresentationJSON
	webhookRepresentationLimit
)

func (r WebhookRepresentation) Validate() error {
	if r <= WebhookRepresentationUnknown || r >= webhookRepresentationLimit {
		return core.ErrTwilioContract
	}
	return nil
}
func (r WebhookRepresentation) IsValid() bool { return r.Validate() == nil }
func (r WebhookRepresentation) String() string {
	if !r.IsValid() {
		return ""
	}
	return [...]string{"", "form", "json"}[r]
}
func (WebhookRepresentation) OffWireEnum() {}
func (r WebhookRepresentation) mediaType() (core.HTTPMediaType, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	if r == WebhookRepresentationForm {
		return core.HTTPMediaTypeFormURLEncoded(), nil
	}
	return core.HTTPMediaTypeJSON(), nil
}

type InboundObservation struct {
	Bytes      core.ByteLength
	ObservedAt temporal.Instant
}

func (o InboundObservation) Validate() error {
	if err := o.ObservedAt.Validate(); err != nil {
		return contractError(err)
	}
	_, err := o.Bytes.Int64()
	if err != nil {
		return contractError(err)
	}
	return nil
}

type WebhookReceiverRequest struct {
	Token          AuthToken
	PublicEndpoint core.HTTPEndpoint
	Maximum        core.ByteCount
	Representation WebhookRepresentation
}

func (r WebhookReceiverRequest) Validate() error {
	if err := errors.Join(r.Token.Validate(), r.PublicEndpoint.Validate(), r.Representation.Validate(), validateWebhookMaximum(r.Maximum)); err != nil {
		return contractError(err)
	}
	if r.PublicEndpoint.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.ErrTwilioBinding
	}
	query, err := url.ParseQuery(r.PublicEndpoint.HTTPURL().RawQuery)
	if err != nil || len(query[core.TwilioWebhookBodySHA256QueryName]) != 0 {
		return bindingError(err)
	}
	return nil
}

type WebhookReceiveRequest struct {
	Call        exchange.SocketServerCall
	Destination io.Writer
	ObservedAt  temporal.Instant
}

func (r WebhookReceiveRequest) Validate() error {
	if r.Call.Validate() != nil || r.Destination == nil || r.ObservedAt.Validate() != nil {
		return core.ErrTwilioContract
	}
	return nil
}

type webhookReceiverState struct {
	token          AuthToken
	publicEndpoint core.HTTPEndpoint
	maximum        core.ByteCount
	representation WebhookRepresentation
}
type WebhookReceiver struct{ state *webhookReceiverState }

func NewWebhookReceiver(request WebhookReceiverRequest) (WebhookReceiver, error) {
	if err := request.Validate(); err != nil {
		return WebhookReceiver{}, err
	}
	owned, err := ParseAuthToken(request.Token.value)
	if err != nil {
		return WebhookReceiver{}, err
	}
	return WebhookReceiver{state: &webhookReceiverState{token: owned, publicEndpoint: request.PublicEndpoint, maximum: request.Maximum, representation: request.Representation}}, nil
}
func (r WebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrTwilioContract
	}
	if err := errors.Join(r.state.token.Validate(), r.state.publicEndpoint.Validate(), r.state.representation.Validate(), validateWebhookMaximum(r.state.maximum)); err != nil {
		return contractError(err)
	}
	return nil
}
func (r *WebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrTwilioContract
	}
	err := r.state.token.Close()
	r.state = nil
	return err
}

func (r WebhookReceiver) Receive(request WebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	body, err := r.authenticatedBody(request.Call)
	if err != nil {
		return InboundObservation{}, err
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	written, err := writeBody(ctx, request.Destination, body)
	observation := InboundObservation{Bytes: written, ObservedAt: request.ObservedAt}
	if err != nil {
		return observation, err
	}
	return observation, observation.Validate()
}

func (r WebhookReceiver) authenticatedBody(call exchange.SocketServerCall) ([]byte, error) {
	body, signedURL, err := r.receiveCandidate(call)
	if err != nil {
		return nil, err
	}
	signature, err := webhookSignature(call)
	if err != nil {
		return nil, err
	}
	validator := twilioclient.NewRequestValidator(string(r.state.token.value))
	if !validator.ValidateBody(signedURL, body, signature) {
		return nil, verificationError(core.ErrExchangeContract)
	}
	return body, nil
}

func (r WebhookReceiver) receiveCandidate(call exchange.SocketServerCall) ([]byte, string, error) {
	signedURL, err := signedRequestURL(call, r.state.publicEndpoint, r.state.representation)
	if err != nil {
		return nil, "", err
	}
	media, err := r.state.representation.mediaType()
	if err != nil {
		return nil, "", contractError(err)
	}
	received, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{
		Call:                call,
		ExpectedContentType: media,
		Policy:              exchange.ServerBoundedPolicy{RequestBodyLimit: r.state.maximum},
		Route:               exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
	})
	if err != nil {
		return nil, "", err
	}
	if r.state.representation == WebhookRepresentationForm {
		if err := validateFormSignatureCoverage(received.Body); err != nil {
			return nil, "", err
		}
	}
	return received.Body, signedURL, nil
}

func webhookSignature(call exchange.SocketServerCall) (string, error) {
	headerName, err := core.ParseHTTPHeaderName(core.TwilioWebhookSignatureHeaderName)
	if err != nil {
		return "", contractError(err)
	}
	headerMaximum, err := core.NewByteCount(core.TwilioWebhookSignatureBytes)
	if err != nil {
		return "", contractError(err)
	}
	header, err := call.UniqueHeader(headerName, headerMaximum)
	if err != nil {
		return "", core.ErrTwilioAuthentication
	}
	signature, err := header.Value()
	if err != nil {
		return "", authenticationError(err)
	}
	if len(signature) != core.TwilioWebhookSignatureBytes {
		return "", core.ErrTwilioAuthentication
	}
	return signature, nil
}

func signedRequestURL(call exchange.SocketServerCall, endpoint core.HTTPEndpoint, representation WebhookRepresentation) (string, error) {
	public := endpoint.HTTPURL()
	matches, err := call.MatchesEndpointPath(endpoint)
	if err != nil || !matches {
		return "", bindingError(core.ErrExchangeContract)
	}
	rawQuery, err := call.RawQuery()
	if err != nil {
		return "", bindingError(err)
	}
	if representation == WebhookRepresentationForm {
		if rawQuery != public.RawQuery {
			return "", bindingError(core.ErrExchangeContract)
		}
		return public.String(), nil
	}
	if representation != WebhookRepresentationJSON {
		return "", core.ErrTwilioContract
	}
	return signedJSONRequestURL(rawQuery, public)
}

func signedJSONRequestURL(rawQuery string, public url.URL) (string, error) {
	configured, configuredErr := url.ParseQuery(public.RawQuery)
	observed, observedErr := url.ParseQuery(rawQuery)
	if configuredErr != nil || observedErr != nil {
		return "", bindingError(errors.Join(configuredErr, observedErr))
	}
	digests := observed[core.TwilioWebhookBodySHA256QueryName]
	if len(digests) != 1 || !validBodySHA256(digests[0]) {
		return "", bindingError(core.ErrExchangeContract)
	}
	delete(observed, core.TwilioWebhookBodySHA256QueryName)
	if !sameQueryValues(configured, observed) {
		return "", bindingError(core.ErrExchangeContract)
	}
	public.RawQuery = rawQuery
	return public.String(), nil
}

func sameQueryValues(left, right url.Values) bool {
	if len(left) != len(right) {
		return false
	}
	for name, leftValues := range left {
		rightValues, present := right[name]
		if !present || !slices.Equal(leftValues, rightValues) {
			return false
		}
	}
	return true
}
func validBodySHA256(value string) bool {
	var digest core.SHA256Digest
	return digest.UnmarshalText([]byte(value)) == nil
}
func validateFormSignatureCoverage(body []byte) error {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return verificationError(err)
	}
	for _, entries := range values {
		if len(entries) != 1 {
			return verificationError(errors.New("twilio form contains a repeated field"))
		}
	}
	return nil
}
func writeBody(ctx context.Context, destination io.Writer, body []byte) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	var buffer [exchange.TransferBufferBytes]byte
	written, err := io.CopyBuffer(destination, bytes.NewReader(body), buffer[:])
	count, countErr := core.CheckedUint64FromInt64(written)
	length, lengthErr := core.NewByteLength(count)
	if joined := errors.Join(err, countErr, lengthErr, contextstate.Validate(ctx)); joined != nil {
		return length, joined
	}
	if written != int64(len(body)) {
		return length, io.ErrShortWrite
	}
	return length, nil
}
func validateWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value == 0 || value > core.TwilioWebhookCustodyMaximumBytes {
		return errors.Join(core.ErrTwilioContract, err)
	}
	return nil
}

var (
	_ core.Validatable = WebhookRepresentation(0)
	_ core.OffWireEnum = WebhookRepresentation(0)
	_ core.Validatable = InboundObservation{}
	_ core.Validatable = WebhookReceiverRequest{}
	_ core.Validatable = WebhookReceiveRequest{}
	_ core.Validatable = WebhookReceiver{}
)
