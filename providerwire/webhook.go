package providerwire

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	stripesdk "github.com/stripe/stripe-go/v86"
	twilioclient "github.com/twilio/twilio-go/client"
)

const (
	// StripeWebhookMaximumBytes is Primitive's Stripe-specific aggregate
	// verification ceiling because Stripe's SDK verifier requires raw []byte.
	StripeWebhookMaximumBytes = 1 << 20
	// StripeWebhookSecretMinimumBytes requires the whsec_ prefix and nonempty
	// secret material.
	StripeWebhookSecretMinimumBytes = 7
	// StripeWebhookSecretMaximumBytes is Primitive's Stripe-specific secret
	// custody ceiling where Stripe publishes no smaller wire maximum.
	StripeWebhookSecretMaximumBytes = 4 * 1024
	// StripeWebhookSignatureMaximumBytes bounds Stripe's versioned signature
	// list before the official SDK parses it.
	StripeWebhookSignatureMaximumBytes = 8 * 1024
	// TwilioWebhookMaximumBytes is Primitive's Twilio-specific aggregate
	// verification ceiling because Twilio's SDK verifier requires raw []byte.
	TwilioWebhookMaximumBytes = 1 << 20
	// TwilioWebhookSignatureBytes is one base64-encoded HMAC-SHA1 digest.
	TwilioWebhookSignatureBytes = 28
	// PlunkWebhookMaximumBytes is Primitive's Plunk-specific streaming ceiling
	// for customizable webhook payloads.
	PlunkWebhookMaximumBytes         = 1 << 20
	StripeWebhookSignatureHeaderName = "Stripe-Signature"
	TwilioWebhookSignatureHeaderName = "X-Twilio-Signature"
	TwilioBodySHA256QueryName        = "bodySHA256"
)

type providerFact struct{ diagnostic string }

func providerFacts() [providerLimit]providerFact {
	return [...]providerFact{
		ProviderUnknown: {},
		ProviderStripe:  {diagnostic: "Stripe"},
		ProviderTwilio:  {diagnostic: "Twilio"},
		ProviderPlunk:   {diagnostic: "Plunk"},
		ProviderPayPal:  {diagnostic: "PayPal"},
	}
}

// Provider identifies one closed third-party protocol authority.
type Provider uint8

const (
	ProviderUnknown Provider = iota
	ProviderStripe
	ProviderTwilio
	ProviderPlunk
	ProviderPayPal
	providerLimit
)

func (p Provider) Validate() error {
	if p <= ProviderUnknown || p >= providerLimit || providerFacts()[p].diagnostic == "" {
		return core.ErrProviderWireContract
	}
	return nil
}

func (p Provider) IsValid() bool { return p.Validate() == nil }

func (p Provider) String() string {
	if err := p.Validate(); err != nil {
		return ""
	}
	return providerFacts()[p].diagnostic
}

func (Provider) OffWireEnum() {}

// InboundObservation proves that one provider-authenticated body was copied
// completely into caller-owned custody.
type InboundObservation struct {
	Provider Provider
	Bytes    core.ByteLength
}

func (o InboundObservation) Validate() error {
	if err := o.Provider.Validate(); err != nil {
		return err
	}
	_, err := o.Bytes.Int64()
	return err
}

// StripeWebhookSecret is one copied endpoint-specific whsec_ secret.
type StripeWebhookSecret struct{ value []byte }

func ParseStripeWebhookSecret(value []byte) (StripeWebhookSecret, error) {
	if !bytesHavePrefix(value, "whsec_") || !validCredentialBytes(value, StripeWebhookSecretMinimumBytes, StripeWebhookSecretMaximumBytes) {
		return StripeWebhookSecret{}, core.ErrProviderWireContract
	}
	return StripeWebhookSecret{value: append([]byte(nil), value...)}, nil
}

func (s StripeWebhookSecret) Validate() error {
	if !bytesHavePrefix(s.value, "whsec_") || !validCredentialBytes(s.value, StripeWebhookSecretMinimumBytes, StripeWebhookSecretMaximumBytes) {
		return core.ErrProviderWireContract
	}
	return nil
}

func (s *StripeWebhookSecret) Close() error {
	if s == nil {
		return core.ErrProviderWireContract
	}
	clear(s.value)
	*s = StripeWebhookSecret{}
	return nil
}

func (StripeWebhookSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type stripeWebhookReceiver struct {
	secret  StripeWebhookSecret
	maximum core.ByteCount
}

// StripeWebhookReceiver verifies Stripe signatures with the official Go SDK.
type StripeWebhookReceiver struct{ state *stripeWebhookReceiver }

func NewStripeWebhookReceiver(secret StripeWebhookSecret, maximum core.ByteCount) (StripeWebhookReceiver, error) {
	if err := errors.Join(secret.Validate(), validateStripeWebhookMaximum(maximum)); err != nil {
		return StripeWebhookReceiver{}, contractError(err)
	}
	owned, err := ParseStripeWebhookSecret(secret.value)
	if err != nil {
		return StripeWebhookReceiver{}, err
	}
	return StripeWebhookReceiver{state: &stripeWebhookReceiver{secret: owned, maximum: maximum}}, nil
}

func (r StripeWebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrProviderWireContract
	}
	return errors.Join(r.state.secret.Validate(), validateStripeWebhookMaximum(r.state.maximum))
}

func (r *StripeWebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrProviderWireContract
	}
	err := r.state.secret.Close()
	r.state = nil
	return err
}

// StripeWebhookReceiveRequest supplies explicit wall time and caller custody.
type StripeWebhookReceiveRequest struct {
	Request     *http.Request
	Destination io.Writer
	ObservedAt  temporal.Instant
	Tolerance   temporal.Duration
}

func (r StripeWebhookReceiveRequest) Validate() error {
	if r.Request == nil || r.Destination == nil || r.ObservedAt.Validate() != nil ||
		r.Tolerance.Validate() != nil || r.Tolerance.IsZero() {
		return core.ErrProviderWireContract
	}
	return nil
}

func (r StripeWebhookReceiver) Receive(request StripeWebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	media, err := jsonMediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	body, err := receiveWebhookBody(request.Request, r.state.maximum, media)
	if err != nil {
		return InboundObservation{}, err
	}
	signature, err := uniqueHeader(request.Request, StripeWebhookSignatureHeaderName, StripeWebhookSignatureMaximumBytes)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	if err := stripesdk.ValidatePayload(body, signature, string(r.state.secret.value), stripesdk.WithIgnoreTolerance()); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	if err := validateStripeSignatureTime(signature, request.ObservedAt, request.Tolerance); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	return writeAuthenticatedBody(request.Request.Context(), ProviderStripe, request.Destination, body)
}

// TwilioWebhookRepresentation closes the two webhook bodies supported by the
// official Twilio request validator.
type TwilioWebhookRepresentation uint8

const (
	TwilioWebhookRepresentationUnknown TwilioWebhookRepresentation = iota
	TwilioWebhookRepresentationForm
	TwilioWebhookRepresentationJSON
	twilioWebhookRepresentationLimit
)

type twilioWebhookRepresentationFact struct {
	diagnostic string
	mediaType  string
}

func twilioWebhookRepresentationFacts() [twilioWebhookRepresentationLimit]twilioWebhookRepresentationFact {
	return [...]twilioWebhookRepresentationFact{
		TwilioWebhookRepresentationUnknown: {},
		TwilioWebhookRepresentationForm:    {diagnostic: "form", mediaType: "application/x-www-form-urlencoded"},
		TwilioWebhookRepresentationJSON:    {diagnostic: "JSON", mediaType: "application/json"},
	}
}

func (r TwilioWebhookRepresentation) Validate() error {
	if r <= TwilioWebhookRepresentationUnknown || r >= twilioWebhookRepresentationLimit || twilioWebhookRepresentationFacts()[r].mediaType == "" {
		return core.ErrProviderWireContract
	}
	return nil
}

func (r TwilioWebhookRepresentation) IsValid() bool { return r.Validate() == nil }

func (r TwilioWebhookRepresentation) String() string {
	if err := r.Validate(); err != nil {
		return ""
	}
	return twilioWebhookRepresentationFacts()[r].diagnostic
}

func (TwilioWebhookRepresentation) OffWireEnum() {}

func (r TwilioWebhookRepresentation) mediaType() (core.HTTPMediaType, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	return core.ParseHTTPMediaType(twilioWebhookRepresentationFacts()[r].mediaType)
}

// TwilioAuthToken is one copied webhook-signing Auth Token.
type TwilioAuthToken struct{ value []byte }

func ParseTwilioAuthToken(value []byte) (TwilioAuthToken, error) {
	if len(value) != TwilioAuthTokenBytes || !asciiAlphanumeric(value) {
		return TwilioAuthToken{}, core.ErrProviderWireContract
	}
	return TwilioAuthToken{value: append([]byte(nil), value...)}, nil
}

func (t TwilioAuthToken) Validate() error {
	if len(t.value) != TwilioAuthTokenBytes || !asciiAlphanumeric(t.value) {
		return core.ErrProviderWireContract
	}
	return nil
}

func (t *TwilioAuthToken) Close() error {
	if t == nil {
		return core.ErrProviderWireContract
	}
	clear(t.value)
	*t = TwilioAuthToken{}
	return nil
}

func (TwilioAuthToken) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type twilioWebhookReceiver struct {
	token          TwilioAuthToken
	publicEndpoint core.HTTPEndpoint
	maximum        core.ByteCount
	representation TwilioWebhookRepresentation
}

// TwilioWebhookReceiverRequest binds one token to the exact public URL and
// representation Twilio signs.
type TwilioWebhookReceiverRequest struct {
	Token          TwilioAuthToken
	PublicEndpoint core.HTTPEndpoint
	Maximum        core.ByteCount
	Representation TwilioWebhookRepresentation
}

func (r TwilioWebhookReceiverRequest) Validate() error {
	if err := errors.Join(r.Token.Validate(), r.PublicEndpoint.Validate(), r.Representation.Validate(), validateTwilioWebhookMaximum(r.Maximum)); err != nil {
		return contractError(err)
	}
	if r.PublicEndpoint.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.ErrProviderWireBinding
	}
	query, err := url.ParseQuery(r.PublicEndpoint.HTTPURL().RawQuery)
	if err != nil || len(query[TwilioBodySHA256QueryName]) != 0 {
		return bindingError(err)
	}
	return nil
}

// TwilioWebhookReceiver verifies X-Twilio-Signature with Twilio's official SDK.
type TwilioWebhookReceiver struct{ state *twilioWebhookReceiver }

func NewTwilioWebhookReceiver(request TwilioWebhookReceiverRequest) (TwilioWebhookReceiver, error) {
	if err := request.Validate(); err != nil {
		return TwilioWebhookReceiver{}, err
	}
	owned, ownedErr := ParseTwilioAuthToken(request.Token.value)
	if ownedErr != nil {
		return TwilioWebhookReceiver{}, ownedErr
	}
	return TwilioWebhookReceiver{state: &twilioWebhookReceiver{
		token: owned, publicEndpoint: request.PublicEndpoint, maximum: request.Maximum, representation: request.Representation,
	}}, nil
}

func (r TwilioWebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrProviderWireContract
	}
	return errors.Join(r.state.token.Validate(), r.state.publicEndpoint.Validate(),
		r.state.representation.Validate(), validateTwilioWebhookMaximum(r.state.maximum))
}

func (r *TwilioWebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrProviderWireContract
	}
	err := r.state.token.Close()
	r.state = nil
	return err
}

// Receive authenticates one exact raw Twilio callback before copying it.
func (r TwilioWebhookReceiver) Receive(request *http.Request, destination io.Writer) (InboundObservation, error) {
	if err := r.Validate(); err != nil || request == nil || destination == nil {
		return InboundObservation{}, contractError(err)
	}
	signedURL, err := twilioSignedRequestURL(request, r.state.publicEndpoint, r.state.representation)
	if err != nil {
		return InboundObservation{}, err
	}
	media, err := r.state.representation.mediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	body, err := receiveWebhookBody(request, r.state.maximum, media)
	if err != nil {
		return InboundObservation{}, err
	}
	signature, err := uniqueHeader(request, TwilioWebhookSignatureHeaderName, TwilioWebhookSignatureBytes)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	validator := twilioclient.NewRequestValidator(string(r.state.token.value))
	if !validator.ValidateBody(signedURL, body, signature) {
		return InboundObservation{}, verificationError(core.ErrExchangeContract)
	}
	return writeAuthenticatedBody(request.Context(), ProviderTwilio, destination, body)
}

type plunkWebhookReceiver struct {
	secret  PlunkWebhookSecret
	maximum core.ByteCount
}

// PlunkWebhookReceiver authenticates the configured Plunk workflow bearer
// header before streaming the customizable body without interpreting it.
type PlunkWebhookReceiver struct{ state *plunkWebhookReceiver }

func NewPlunkWebhookReceiver(secret PlunkWebhookSecret, maximum core.ByteCount) (PlunkWebhookReceiver, error) {
	if err := errors.Join(secret.Validate(), validatePlunkWebhookMaximum(maximum)); err != nil {
		return PlunkWebhookReceiver{}, contractError(err)
	}
	owned, err := ParsePlunkWebhookSecret(secret.token)
	if err != nil {
		return PlunkWebhookReceiver{}, err
	}
	return PlunkWebhookReceiver{state: &plunkWebhookReceiver{secret: owned, maximum: maximum}}, nil
}

func (r PlunkWebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrProviderWireContract
	}
	return errors.Join(r.state.secret.Validate(), validatePlunkWebhookMaximum(r.state.maximum))
}

func (r *PlunkWebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrProviderWireContract
	}
	err := r.state.secret.Close()
	r.state = nil
	return err
}

func (r PlunkWebhookReceiver) Receive(request *http.Request, destination io.Writer) (InboundObservation, error) {
	if err := r.Validate(); err != nil || request == nil || destination == nil {
		return InboundObservation{}, contractError(err)
	}
	received, err := exchange.ReceiveBearerAuthorization(request)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	defer clear(received.Token)
	want := exchange.BearerAuthorization{Token: r.state.secret.token}
	matches, err := exchange.BearerAuthorizationMatches(received, want)
	if err != nil || !matches {
		return InboundObservation{}, authenticationError(err)
	}
	media, err := jsonMediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	stream, err := exchange.ReceiveStream(exchange.StreamReceiveCall{
		Destination: destination, Request: request,
		ExpectedContentType: media,
		Policy:              exchange.ServerStreamPolicy{RequestBodyLimit: r.state.maximum},
		Route:               webhookRoute(),
	})
	if err != nil {
		return InboundObservation{}, err
	}
	observation := InboundObservation{Provider: ProviderPlunk, Bytes: stream.Bytes}
	return observation, observation.Validate()
}

func receiveWebhookBody(request *http.Request, maximum core.ByteCount, media core.HTTPMediaType) ([]byte, error) {
	received, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{
		Request: request, ExpectedContentType: media,
		Policy: exchange.ServerBoundedPolicy{RequestBodyLimit: maximum},
		Route:  webhookRoute(),
	})
	if err != nil {
		return nil, err
	}
	return received.Body, nil
}

func webhookRoute() exchange.RouteSemantics {
	return exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}
}

func jsonMediaType() (core.HTTPMediaType, error) {
	return exchange.StandardMediaTypeJSON.HTTPMediaType()
}

func uniqueHeader(request *http.Request, name string, maximum int) (string, error) {
	values := request.Header.Values(name)
	if len(values) != 1 || len(values[0]) == 0 || len(values[0]) > maximum {
		return "", core.ErrProviderWireAuthentication
	}
	return values[0], nil
}

func validateStripeSignatureTime(header string, observed temporal.Instant, tolerance temporal.Duration) error {
	signed, err := stripeSignatureInstant(header)
	if err != nil {
		return err
	}
	comparison, err := observed.Compare(signed)
	if err != nil || comparison == core.ComparisonLess {
		return err
	}
	age, err := observed.Since(signed)
	if err != nil {
		return err
	}
	ageComparison, err := age.Compare(tolerance)
	if err != nil || ageComparison == core.ComparisonGreater {
		return errors.Join(core.ErrProviderWireVerification, err)
	}
	return nil
}

func stripeSignatureInstant(header string) (temporal.Instant, error) {
	nanosecondsPerSecond := int64(temporal.NanosecondsPerSecond)
	var seconds int64
	found := false
	for member := range strings.SplitSeq(header, ",") {
		if !strings.HasPrefix(member, "t=") {
			continue
		}
		if found {
			return temporal.Instant{}, core.ErrProviderWireVerification
		}
		parsed, err := strconv.ParseInt(strings.TrimPrefix(member, "t="), 10, 64)
		if err != nil || parsed > math.MaxInt64/nanosecondsPerSecond || parsed < math.MinInt64/nanosecondsPerSecond {
			return temporal.Instant{}, errors.Join(core.ErrProviderWireVerification, err)
		}
		seconds = parsed
		found = true
	}
	if !found {
		return temporal.Instant{}, core.ErrProviderWireVerification
	}
	return temporal.InstantFromNanoseconds(seconds * nanosecondsPerSecond), nil
}

func writeAuthenticatedBody(ctx context.Context, provider Provider, destination io.Writer, body []byte) (InboundObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return InboundObservation{}, err
	}
	var buffer [exchange.TransferBufferBytes]byte
	written, err := io.CopyBuffer(destination, bytes.NewReader(body), buffer[:])
	writtenBytes, writtenErr := core.CheckedUint64FromInt64(written)
	length, lengthErr := core.NewByteLength(writtenBytes)
	observation := InboundObservation{Provider: provider, Bytes: length}
	if err := errors.Join(err, writtenErr, lengthErr, contextstate.Validate(ctx)); err != nil {
		return observation, err
	}
	if written != int64(len(body)) {
		return observation, io.ErrShortWrite
	}
	return observation, observation.Validate()
}

func validateStripeWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value > StripeWebhookMaximumBytes {
		return errors.Join(core.ErrProviderWireContract, err)
	}
	return nil
}

func validateTwilioWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value > TwilioWebhookMaximumBytes {
		return errors.Join(core.ErrProviderWireContract, err)
	}
	return nil
}

func validatePlunkWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value > PlunkWebhookMaximumBytes {
		return errors.Join(core.ErrProviderWireContract, err)
	}
	return nil
}

func twilioSignedRequestURL(request *http.Request, endpoint core.HTTPEndpoint, representation TwilioWebhookRepresentation) (string, error) {
	if request == nil || request.URL == nil {
		return "", bindingError(core.ErrExchangeContract)
	}
	public := endpoint.HTTPURL()
	if request.URL.Path != public.Path || request.URL.RawPath != public.RawPath {
		return "", bindingError(core.ErrExchangeContract)
	}
	if representation == TwilioWebhookRepresentationForm {
		return twilioSignedFormRequestURL(request, public)
	}
	if representation != TwilioWebhookRepresentationJSON {
		return "", contractError(core.ErrProviderWireContract)
	}
	return twilioSignedJSONRequestURL(request, public)
}

func twilioSignedFormRequestURL(request *http.Request, public url.URL) (string, error) {
	if request.URL.RawQuery != public.RawQuery {
		return "", bindingError(core.ErrExchangeContract)
	}
	return public.String(), nil
}

func twilioSignedJSONRequestURL(request *http.Request, public url.URL) (string, error) {
	configured, configuredErr := url.ParseQuery(public.RawQuery)
	observed, observedErr := url.ParseQuery(request.URL.RawQuery)
	if configuredErr != nil || observedErr != nil {
		return "", bindingError(errors.Join(configuredErr, observedErr))
	}
	digests := observed[TwilioBodySHA256QueryName]
	if len(digests) != 1 || !validTwilioBodySHA256(digests[0]) {
		return "", bindingError(core.ErrExchangeContract)
	}
	delete(observed, TwilioBodySHA256QueryName)
	if !sameQueryValues(configured, observed) {
		return "", bindingError(core.ErrExchangeContract)
	}
	public.RawQuery = request.URL.RawQuery
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

func validTwilioBodySHA256(value string) bool {
	var digest core.SHA256Digest
	return digest.UnmarshalText([]byte(value)) == nil
}

var (
	_ core.Validatable = Provider(0)
	_ core.OffWireEnum = Provider(0)
	_ core.Validatable = InboundObservation{}
	_ core.Validatable = StripeWebhookSecret{}
	_ core.Validatable = StripeWebhookReceiver{}
	_ core.Validatable = StripeWebhookReceiveRequest{}
	_ core.Validatable = TwilioWebhookRepresentation(0)
	_ core.OffWireEnum = TwilioWebhookRepresentation(0)
	_ core.Validatable = TwilioAuthToken{}
	_ core.Validatable = TwilioWebhookReceiverRequest{}
	_ core.Validatable = TwilioWebhookReceiver{}
	_ core.Validatable = PlunkWebhookReceiver{}
)
