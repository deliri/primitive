package paypal

import (
	"bytes"
	"context"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// The verification response is one documented status field; this is
	// Primitive's bounded envelope custody, not a PayPal-published extent.
	// Source: https://developer.paypal.com/api/webhooks/v1/verify-webhook-signature-post/
	payPalWebhookVerificationResponseMaximumBytes = 256
	// The following exact field limits are published by PayPal's verification schema.
	// Source: https://developer.paypal.com/api/webhooks/v1/verify-webhook-signature-post/
	payPalWebhookVerificationPath        = "/v1/notifications/verify-webhook-signature"
	payPalWebhookCertificatePathPrefix   = "/v1/notifications/certs/"
	payPalWebhookVerificationSuccessText = "SUCCESS"
	payPalWebhookVerificationFailureText = "FAILURE"
)

// InboundObservation proves that one provider-verified webhook body crossed
// into caller-owned custody at an explicit observed instant.
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

// PayPalWebhookID is one configured webhook authority binding.
type PayPalWebhookID string

func ParsePayPalWebhookID(value string) (PayPalWebhookID, error) {
	identifier := PayPalWebhookID(value)
	if err := identifier.Validate(); err != nil {
		return "", err
	}
	return identifier, nil
}

func (i PayPalWebhookID) Validate() error {
	if !payPalASCIIAlphanumeric(string(i), core.PayPalWebhookIDMaximumBytes) {
		return core.ErrPayPalContract
	}
	return nil
}

func (i PayPalWebhookID) String() string {
	if err := i.Validate(); err != nil {
		return ""
	}
	return string(i)
}

// PayPalWebhookVerificationStatus is PayPal's closed verification response.
type PayPalWebhookVerificationStatus uint8

const (
	PayPalWebhookVerificationUnknown PayPalWebhookVerificationStatus = iota
	PayPalWebhookVerificationSuccess
	PayPalWebhookVerificationFailure
	payPalWebhookVerificationStatusLimit
)

type payPalWebhookVerificationStatusFact struct{ wire string }

func payPalWebhookVerificationStatusFacts() [payPalWebhookVerificationStatusLimit]payPalWebhookVerificationStatusFact {
	return [...]payPalWebhookVerificationStatusFact{
		PayPalWebhookVerificationUnknown: {},
		PayPalWebhookVerificationSuccess: {wire: payPalWebhookVerificationSuccessText},
		PayPalWebhookVerificationFailure: {wire: payPalWebhookVerificationFailureText},
	}
}

func (s PayPalWebhookVerificationStatus) Validate() error {
	if s <= PayPalWebhookVerificationUnknown || s >= payPalWebhookVerificationStatusLimit || payPalWebhookVerificationStatusFacts()[s].wire == "" {
		return core.ErrPayPalContract
	}
	return nil
}

func (s PayPalWebhookVerificationStatus) IsValid() bool { return s.Validate() == nil }

func (s PayPalWebhookVerificationStatus) String() string {
	if err := s.Validate(); err != nil {
		return ""
	}
	return payPalWebhookVerificationStatusFacts()[s].wire
}

func (s PayPalWebhookVerificationStatus) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(s.String())
}

func (s *PayPalWebhookVerificationStatus) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPayPalContract)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed := PayPalWebhookVerificationUnknown
	for candidate := PayPalWebhookVerificationSuccess; candidate < payPalWebhookVerificationStatusLimit; candidate++ {
		if payPalWebhookVerificationStatusFacts()[candidate].wire == text {
			parsed = candidate
			break
		}
	}
	if err := parsed.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = parsed
	return nil
}

type payPalWebhookVerificationResponse struct {
	Status PayPalWebhookVerificationStatus `json:"verification_status"`
}

func (r payPalWebhookVerificationResponse) Validate() error { return r.Status.Validate() }

func (r payPalWebhookVerificationResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	type wire payPalWebhookVerificationResponse
	return json.Marshal(wire(r))
}

type payPalWebhookVerificationProjection struct {
	AuthAlgorithm  PayPalAuthAlgorithm         `json:"auth_algo"`
	CertificateURL PayPalCertificateURL        `json:"cert_url"`
	TransmissionID PayPalTransmissionID        `json:"transmission_id"`
	Signature      PayPalTransmissionSignature `json:"transmission_sig"`
	TransmissionAt PayPalTransmissionTime      `json:"transmission_time"`
	WebhookID      PayPalWebhookID             `json:"webhook_id"`
	// doctrine:local-allowed=external-wire
	WebhookEvent jsontext.Value `json:"webhook_event"`
}

func (p payPalWebhookVerificationProjection) Validate() error {
	if err := validatePayPalWebhookFields(p); err != nil {
		return err
	}
	if err := p.WebhookID.Validate(); err != nil {
		return err
	}
	if len(p.WebhookEvent) == 0 || len(p.WebhookEvent) > core.PayPalWebhookEventCustodyMaximumBytes ||
		!p.WebhookEvent.IsValid() || p.WebhookEvent.Kind() != jsontext.KindBeginObject {
		return core.ErrPayPalContract
	}
	return nil
}

func validatePayPalWebhookFields(p payPalWebhookVerificationProjection) error {
	return errors.Join(p.AuthAlgorithm.Validate(), p.CertificateURL.Validate(), p.TransmissionID.Validate(),
		p.Signature.Validate(), p.TransmissionAt.Validate())
}

func (p payPalWebhookVerificationProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	type wire payPalWebhookVerificationProjection
	return json.Marshal(wire(p))
}

func (p payPalWebhookVerificationProjection) ValidateJSONProjection(encoded []byte, limits core.StrictJSONLimits) error {
	decoded, err := core.DecodeStrictJSONStructure[payPalWebhookVerificationProjection](encoded, limits)
	if err != nil {
		return err
	}
	if decoded.AuthAlgorithm != p.AuthAlgorithm || decoded.CertificateURL != p.CertificateURL ||
		decoded.TransmissionID != p.TransmissionID || decoded.Signature != p.Signature ||
		decoded.TransmissionAt != p.TransmissionAt || decoded.WebhookID != p.WebhookID ||
		!bytes.Equal(decoded.WebhookEvent, p.WebhookEvent) {
		return core.ErrPayPalContract
	}
	return decoded.Validate()
}

type payPalWebhookReceiver struct {
	client    Client
	webhookID PayPalWebhookID
	maximum   core.ByteCount
}

// PayPalWebhookReceiver verifies one exact raw callback through PayPal's
// documented verification endpoint before releasing it to caller custody.
type PayPalWebhookReceiver struct{ state *payPalWebhookReceiver }

// PayPalWebhookReceiveRequest supplies explicit wall time, replay tolerance,
// transfer policy, and caller custody for one callback.
type PayPalWebhookReceiveRequest struct {
	Call        exchange.SocketServerCall
	Destination io.Writer
	ObservedAt  temporal.Instant
	Tolerance   temporal.Duration
	Policy      exchange.OperationPolicy
}

func (r PayPalWebhookReceiveRequest) Validate() error {
	if r.Call.Validate() != nil || r.Destination == nil || r.ObservedAt.Validate() != nil ||
		r.Tolerance.Validate() != nil || r.Tolerance.IsZero() || r.Policy.Validate() != nil {
		return core.ErrPayPalContract
	}
	return nil
}

func NewPayPalWebhookReceiver(client Client, webhookID PayPalWebhookID, maximum core.ByteCount) (PayPalWebhookReceiver, error) {
	if err := errors.Join(client.Validate(), webhookID.Validate(), validatePayPalWebhookMaximum(maximum)); err != nil {
		return PayPalWebhookReceiver{}, contractError(err)
	}
	owned, err := NewClient(client.state.client, client.state.token, client.state.sandbox)
	if err != nil {
		return PayPalWebhookReceiver{}, err
	}
	return PayPalWebhookReceiver{state: &payPalWebhookReceiver{client: owned, webhookID: webhookID, maximum: maximum}}, nil
}

func (r PayPalWebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrPayPalContract
	}
	return errors.Join(r.state.client.Validate(), r.state.webhookID.Validate(), validatePayPalWebhookMaximum(r.state.maximum))
}

func (r *PayPalWebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrPayPalContract
	}
	err := r.state.client.Close()
	r.state = nil
	return err
}

func (r PayPalWebhookReceiver) Receive(request PayPalWebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	media, err := jsonMediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	body, err := receiveWebhookBody(request.Call, r.state.maximum, media)
	if err != nil {
		return InboundObservation{}, err
	}
	projection, err := r.verificationProjection(request.Call, body)
	if err != nil {
		return InboundObservation{}, err
	}
	if err := validatePayPalTransmissionTime(projection.TransmissionAt, request.ObservedAt, request.Tolerance); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	status, err := r.state.client.verifyWebhook(ctx, projection, request.Policy)
	if err != nil {
		return InboundObservation{}, err
	}
	if status != PayPalWebhookVerificationSuccess {
		return InboundObservation{}, core.ErrPayPalVerification
	}
	written, err := writeAuthenticatedBody(ctx, request.Destination, body)
	observation := InboundObservation{Bytes: written, ObservedAt: request.ObservedAt}
	if err != nil {
		return observation, err
	}
	return observation, observation.Validate()
}

func (r PayPalWebhookReceiver) verificationProjection(call exchange.SocketServerCall, body []byte) (payPalWebhookVerificationProjection, error) {
	authAlgorithm, err := payPalAuthAlgorithmHeader(call)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	certificateURL, err := payPalCertificateURLHeader(call)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	if err := r.validateCertificateURL(certificateURL); err != nil {
		return payPalWebhookVerificationProjection{}, bindingError(err)
	}
	transmissionID, err := payPalTransmissionIDHeader(call)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	signature, err := payPalTransmissionSignatureHeader(call)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	transmissionAt, err := payPalTransmissionTimeHeader(call)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	projection := payPalWebhookVerificationProjection{
		AuthAlgorithm: authAlgorithm, CertificateURL: certificateURL,
		TransmissionID: transmissionID, Signature: signature,
		TransmissionAt: transmissionAt, WebhookID: r.state.webhookID,
		// doctrine:local-allowed=external-wire
		WebhookEvent: jsontext.Value(append([]byte(nil), body...)),
	}
	return projection, projection.Validate()
}

func payPalAuthAlgorithmHeader(call exchange.SocketServerCall) (PayPalAuthAlgorithm, error) {
	value, err := uniqueHeader(call, core.PayPalAuthAlgorithmHeaderName, core.PayPalAuthAlgorithmMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalAuthAlgorithm(value)
}

func payPalCertificateURLHeader(call exchange.SocketServerCall) (PayPalCertificateURL, error) {
	value, err := uniqueHeader(call, core.PayPalCertificateURLHeaderName, core.PayPalCertificateURLMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalCertificateURL(value)
}

func payPalTransmissionIDHeader(call exchange.SocketServerCall) (PayPalTransmissionID, error) {
	value, err := uniqueHeader(call, core.PayPalTransmissionIDHeaderName, core.PayPalTransmissionIDMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalTransmissionID(value)
}

func payPalTransmissionSignatureHeader(call exchange.SocketServerCall) (PayPalTransmissionSignature, error) {
	value, err := uniqueHeader(call, core.PayPalTransmissionSignatureHeaderName, core.PayPalTransmissionSignatureMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalTransmissionSignature(value)
}

func payPalTransmissionTimeHeader(call exchange.SocketServerCall) (PayPalTransmissionTime, error) {
	value, err := uniqueHeader(call, core.PayPalTransmissionTimeHeaderName, core.PayPalTransmissionTimeMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalTransmissionTime(value)
}

func (r PayPalWebhookReceiver) validateCertificateURL(value PayPalCertificateURL) error {
	if err := value.Validate(); err != nil {
		return err
	}
	endpoint, err := core.ParseHTTPEndpoint(string(value))
	if err != nil {
		return err
	}
	wantHost := core.PayPalLiveCertificateHost
	if r.state.client.state.sandbox {
		wantHost = core.PayPalSandboxCertificateHost
	}
	urlValue := endpoint.HTTPURL()
	if urlValue.Scheme != core.SchemeHTTPS || urlValue.Host != wantHost || urlValue.RawPath != "" ||
		urlValue.RawQuery != "" || path.Clean(urlValue.Path) != urlValue.Path ||
		!strings.HasPrefix(urlValue.Path, payPalWebhookCertificatePathPrefix) {
		return core.ErrPayPalBinding
	}
	return nil
}

func (c Client) verifyWebhook(ctx context.Context, projection payPalWebhookVerificationProjection, policy exchange.OperationPolicy) (PayPalWebhookVerificationStatus, error) {
	if err := errors.Join(c.Validate(), projection.Validate(), policy.Validate()); err != nil {
		return PayPalWebhookVerificationUnknown, contractError(err)
	}
	requestLimit, err := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if err != nil {
		return PayPalWebhookVerificationUnknown, contractError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = requestLimit
	limits.NestingDepthMaximum = core.JSONNestingDepthMaximum
	limits.ObjectFieldMaximum = core.JSONObjectFieldCountMaximum
	body, err := core.EncodeValidatedJSON(projection, limits)
	if err != nil {
		return PayPalWebhookVerificationUnknown, contractError(err)
	}
	host := core.PayPalLiveAPIHost
	if c.state.sandbox {
		host = core.PayPalSandboxAPIHost
	}
	target, err := core.ParseHTTPEndpoint("https://" + host + payPalWebhookVerificationPath)
	if err != nil {
		return PayPalWebhookVerificationUnknown, bindingError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(c.state.token.authorization)
	if err != nil {
		return PayPalWebhookVerificationUnknown, contractError(err)
	}
	media, err := jsonMediaType()
	if err != nil {
		return PayPalWebhookVerificationUnknown, contractError(err)
	}
	responseLimit, err := core.NewByteCount(payPalWebhookVerificationResponseMaximumBytes)
	if err != nil {
		return PayPalWebhookVerificationUnknown, contractError(err)
	}
	response, err := exchange.SendBounded(exchange.BoundedCall{
		Context: ctx, Client: c.state.client,
		Request: exchange.BoundedRequest{
			Target: target, Body: body,
			Semantics:          exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
			RequestContentType: media, ExpectedResponseContentType: media,
			Headers: exchange.Headers{Values: []exchange.Header{authorization}}, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: exchange.BoundedPolicy{Operation: policy, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit},
	})
	if err != nil {
		return PayPalWebhookVerificationUnknown, err
	}
	return decodePayPalVerificationResponse(response.Body, responseLimit)
}

func decodePayPalVerificationResponse(body []byte, maximum core.ByteCount) (PayPalWebhookVerificationStatus, error) {
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	limits.NestingDepthMaximum = 1
	limits.ObjectFieldMaximum = 1
	limits.ArrayItemMaximum = 1
	wire, err := core.DecodeStrictJSONStructure[payPalWebhookVerificationResponse](body, limits)
	if err != nil {
		return PayPalWebhookVerificationUnknown, verificationError(err)
	}
	if err := wire.Validate(); err != nil {
		return PayPalWebhookVerificationUnknown, verificationError(err)
	}
	return wire.Status, nil
}

func validatePayPalWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value > core.PayPalWebhookEventCustodyMaximumBytes {
		return errors.Join(core.ErrPayPalContract, err)
	}
	return nil
}

type PayPalAuthAlgorithm string
type PayPalCertificateURL string
type PayPalTransmissionID string
type PayPalTransmissionSignature string
type PayPalTransmissionTime string

func ParsePayPalAuthAlgorithm(value string) (PayPalAuthAlgorithm, error) {
	parsed := PayPalAuthAlgorithm(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (v PayPalAuthAlgorithm) Validate() error {
	if !payPalASCIIAlphanumeric(string(v), core.PayPalAuthAlgorithmMaximumBytes) {
		return core.ErrPayPalContract
	}
	return nil
}

func (v PayPalAuthAlgorithm) String() string { return validPayPalScalarString(string(v), v.Validate()) }

func ParsePayPalCertificateURL(value string) (PayPalCertificateURL, error) {
	parsed := PayPalCertificateURL(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (v PayPalCertificateURL) Validate() error {
	if len(v) == 0 || len(v) > core.PayPalCertificateURLMaximumBytes {
		return core.ErrPayPalContract
	}
	_, err := core.ParseHTTPEndpoint(string(v))
	if err != nil {
		return errors.Join(core.ErrPayPalContract, err)
	}
	return nil
}

func (v PayPalCertificateURL) String() string {
	return validPayPalScalarString(string(v), v.Validate())
}

func ParsePayPalTransmissionID(value string) (PayPalTransmissionID, error) {
	parsed := PayPalTransmissionID(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (v PayPalTransmissionID) Validate() error {
	return validatePayPalTransmissionMember(string(v), core.PayPalTransmissionIDMaximumBytes)
}

func (v PayPalTransmissionID) String() string {
	return validPayPalScalarString(string(v), v.Validate())
}

func ParsePayPalTransmissionSignature(value string) (PayPalTransmissionSignature, error) {
	parsed := PayPalTransmissionSignature(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (v PayPalTransmissionSignature) Validate() error {
	return validatePayPalTransmissionMember(string(v), core.PayPalTransmissionSignatureMaximumBytes)
}

func (v PayPalTransmissionSignature) String() string {
	return validPayPalScalarString(string(v), v.Validate())
}

func ParsePayPalTransmissionTime(value string) (PayPalTransmissionTime, error) {
	parsed := PayPalTransmissionTime(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (v PayPalTransmissionTime) Validate() error {
	if len(v) == 0 || len(v) > core.PayPalTransmissionTimeMaximumBytes {
		return core.ErrPayPalContract
	}
	if _, err := temporal.ParseRFC3339(string(v)); err != nil {
		return errors.Join(core.ErrPayPalContract, err)
	}
	return nil
}

func (v PayPalTransmissionTime) String() string {
	return validPayPalScalarString(string(v), v.Validate())
}

func validatePayPalTransmissionTime(
	signed PayPalTransmissionTime,
	observed temporal.Instant,
	tolerance temporal.Duration,
) error {
	if err := signed.Validate(); err != nil {
		return err
	}
	instant, err := temporal.ParseRFC3339(signed.String())
	if err != nil {
		return err
	}
	comparison, err := observed.Compare(instant)
	if err != nil {
		return err
	}
	age, err := signatureAge(observed, instant, comparison)
	if err != nil {
		return err
	}
	order, err := age.Compare(tolerance)
	if err != nil || order == core.ComparisonGreater {
		return errors.Join(core.ErrPayPalVerification, err)
	}
	return nil
}

func signatureAge(observed, signed temporal.Instant, comparison core.Comparison) (temporal.Duration, error) {
	if comparison == core.ComparisonLess {
		return signed.Since(observed)
	}
	return observed.Since(signed)
}

func jsonMediaType() (core.HTTPMediaType, error) {
	return exchange.StandardMediaTypeJSON.HTTPMediaType()
}

func receiveWebhookBody(call exchange.SocketServerCall, maximum core.ByteCount, media core.HTTPMediaType) ([]byte, error) {
	received, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{
		Call:                call,
		ExpectedContentType: media,
		Policy:              exchange.ServerBoundedPolicy{RequestBodyLimit: maximum},
		Route:               exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
	})
	if err != nil {
		return nil, err
	}
	return received.Body, nil
}

func uniqueHeader(call exchange.SocketServerCall, name string, maximum uint64) (string, error) {
	header, err := core.ParseHTTPHeaderName(name)
	if err != nil {
		return "", authenticationError(err)
	}
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return "", authenticationError(err)
	}
	value, err := call.UniqueHeader(header, limit)
	if err != nil {
		return "", core.ErrPayPalAuthentication
	}
	text, err := value.Value()
	if err != nil {
		return "", authenticationError(err)
	}
	return text, nil
}

func writeAuthenticatedBody(ctx context.Context, destination io.Writer, body []byte) (core.ByteLength, error) {
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

func validPayPalScalarString(value string, err error) string {
	if err != nil {
		return ""
	}
	return value
}

func validatePayPalTransmissionMember(value string, maximum int) error {
	if len(value) < 2 || len(value) > maximum || !payPalWordByte(value[0]) {
		return core.ErrPayPalContract
	}
	nondecimal := false
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return core.ErrPayPalContract
		}
		nondecimal = nondecimal || value[index] < '0' || value[index] > '9'
	}
	if !nondecimal {
		return core.ErrPayPalContract
	}
	return nil
}

func payPalASCIIAlphanumeric(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for index := range len(value) {
		if !payPalASCIIAlphanumericByte(value[index]) {
			return false
		}
	}
	return true
}

func payPalWordByte(value byte) bool {
	return payPalASCIIAlphanumericByte(value) || value == '_'
}

func payPalASCIIAlphanumericByte(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

var (
	_ core.Validatable             = PayPalWebhookID("")
	_ core.Validatable             = PayPalAuthAlgorithm("")
	_ core.Validatable             = PayPalCertificateURL("")
	_ core.Validatable             = PayPalTransmissionID("")
	_ core.Validatable             = PayPalTransmissionSignature("")
	_ core.Validatable             = PayPalTransmissionTime("")
	_ core.Validatable             = PayPalWebhookVerificationStatus(0)
	_ core.ValidatedJSONMarshaler  = PayPalWebhookVerificationStatus(0)
	_ core.Validatable             = payPalWebhookVerificationResponse{}
	_ core.ValidatedJSONMarshaler  = payPalWebhookVerificationResponse{}
	_ core.ValidatedJSONMarshaler  = payPalWebhookVerificationProjection{}
	_ core.ValidatedJSONProjection = payPalWebhookVerificationProjection{}
	_ core.Validatable             = PayPalWebhookReceiver{}
	_ core.Validatable             = PayPalWebhookReceiveRequest{}
)
