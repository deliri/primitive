package providerwire

import (
	"bytes"
	"context"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// PayPalWebhookMaximumBytes leaves fixed framing space beneath Core's
	// strict one-MiB JSON document ceiling for the verification envelope.
	PayPalWebhookMaximumBytes                     = core.JSONDocumentMaximumBytes - 64*1024
	payPalWebhookVerificationResponseMaximumBytes = 256
	PayPalWebhookIDMaximumBytes                   = 50
	PayPalAuthAlgorithmMaximumBytes               = 100
	PayPalCertificateURLMaximumBytes              = 500
	PayPalTransmissionIDMaximumBytes              = 50
	PayPalTransmissionSignatureMaximumBytes       = 500
	PayPalTransmissionTimeMaximumBytes            = 100
	PayPalAuthAlgorithmHeaderName                 = "PAYPAL-AUTH-ALGO"
	PayPalCertificateURLHeaderName                = "PAYPAL-CERT-URL"
	PayPalTransmissionIDHeaderName                = "PAYPAL-TRANSMISSION-ID"
	PayPalTransmissionSignatureHeaderName         = "PAYPAL-TRANSMISSION-SIG"
	PayPalTransmissionTimeHeaderName              = "PAYPAL-TRANSMISSION-TIME"
	PayPalLiveCertificateHost                     = "api.paypal.com"
	PayPalSandboxCertificateHost                  = "api.sandbox.paypal.com"
	payPalWebhookVerificationPath                 = "/v1/notifications/verify-webhook-signature"
	payPalWebhookCertificatePathPrefix            = "/v1/notifications/certs/"
	payPalWebhookVerificationSuccessText          = "SUCCESS"
	payPalWebhookVerificationFailureText          = "FAILURE"
)

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
	if !payPalASCIIAlphanumeric(string(i), PayPalWebhookIDMaximumBytes) {
		return core.ErrProviderWireContract
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
		return core.ErrProviderWireContract
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
		return errors.Join(core.ErrJSONContract, core.ErrProviderWireContract)
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
	if len(p.WebhookEvent) == 0 || len(p.WebhookEvent) > PayPalWebhookMaximumBytes ||
		!p.WebhookEvent.IsValid() || p.WebhookEvent.Kind() != jsontext.KindBeginObject {
		return core.ErrProviderWireContract
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
		return core.ErrProviderWireContract
	}
	return decoded.Validate()
}

type payPalWebhookReceiver struct {
	client    PayPalClient
	webhookID PayPalWebhookID
	maximum   core.ByteCount
}

// PayPalWebhookReceiver verifies one exact raw callback through PayPal's
// documented verification endpoint before releasing it to caller custody.
type PayPalWebhookReceiver struct{ state *payPalWebhookReceiver }

// PayPalWebhookReceiveRequest supplies explicit wall time, replay tolerance,
// transfer policy, and caller custody for one callback.
type PayPalWebhookReceiveRequest struct {
	Request     *http.Request
	Destination io.Writer
	ObservedAt  temporal.Instant
	Tolerance   temporal.Duration
	Policy      exchange.OperationPolicy
}

func (r PayPalWebhookReceiveRequest) Validate() error {
	if r.Request == nil || r.Destination == nil || r.ObservedAt.Validate() != nil ||
		r.Tolerance.Validate() != nil || r.Tolerance.IsZero() || r.Policy.Validate() != nil {
		return core.ErrProviderWireContract
	}
	return nil
}

func NewPayPalWebhookReceiver(client PayPalClient, webhookID PayPalWebhookID, maximum core.ByteCount) (PayPalWebhookReceiver, error) {
	if err := errors.Join(client.Validate(), webhookID.Validate(), validatePayPalWebhookMaximum(maximum)); err != nil {
		return PayPalWebhookReceiver{}, contractError(err)
	}
	owned, err := NewPayPalClient(client.state.client, client.state.token, client.state.test)
	if err != nil {
		return PayPalWebhookReceiver{}, err
	}
	return PayPalWebhookReceiver{state: &payPalWebhookReceiver{client: owned, webhookID: webhookID, maximum: maximum}}, nil
}

func (r PayPalWebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrProviderWireContract
	}
	return errors.Join(r.state.client.Validate(), r.state.webhookID.Validate(), validatePayPalWebhookMaximum(r.state.maximum))
}

func (r *PayPalWebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrProviderWireContract
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
	body, err := receiveWebhookBody(request.Request, r.state.maximum, media)
	if err != nil {
		return InboundObservation{}, err
	}
	projection, err := r.verificationProjection(request.Request, body)
	if err != nil {
		return InboundObservation{}, err
	}
	if err := validatePayPalTransmissionTime(projection.TransmissionAt, request.ObservedAt, request.Tolerance); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	status, err := r.state.client.verifyWebhook(request.Request.Context(), projection, request.Policy)
	if err != nil {
		return InboundObservation{}, err
	}
	if status != PayPalWebhookVerificationSuccess {
		return InboundObservation{}, core.ErrProviderWireVerification
	}
	return writeAuthenticatedBody(request.Request.Context(), ProviderPayPal, request.Destination, body)
}

func (r PayPalWebhookReceiver) verificationProjection(request *http.Request, body []byte) (payPalWebhookVerificationProjection, error) {
	authAlgorithm, err := payPalAuthAlgorithmHeader(request)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	certificateURL, err := payPalCertificateURLHeader(request)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	if err := r.validateCertificateURL(certificateURL); err != nil {
		return payPalWebhookVerificationProjection{}, bindingError(err)
	}
	transmissionID, err := payPalTransmissionIDHeader(request)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	signature, err := payPalTransmissionSignatureHeader(request)
	if err != nil {
		return payPalWebhookVerificationProjection{}, err
	}
	transmissionAt, err := payPalTransmissionTimeHeader(request)
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

func payPalAuthAlgorithmHeader(request *http.Request) (PayPalAuthAlgorithm, error) {
	value, err := uniqueHeader(request, PayPalAuthAlgorithmHeaderName, PayPalAuthAlgorithmMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalAuthAlgorithm(value)
}

func payPalCertificateURLHeader(request *http.Request) (PayPalCertificateURL, error) {
	value, err := uniqueHeader(request, PayPalCertificateURLHeaderName, PayPalCertificateURLMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalCertificateURL(value)
}

func payPalTransmissionIDHeader(request *http.Request) (PayPalTransmissionID, error) {
	value, err := uniqueHeader(request, PayPalTransmissionIDHeaderName, PayPalTransmissionIDMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalTransmissionID(value)
}

func payPalTransmissionSignatureHeader(request *http.Request) (PayPalTransmissionSignature, error) {
	value, err := uniqueHeader(request, PayPalTransmissionSignatureHeaderName, PayPalTransmissionSignatureMaximumBytes)
	if err != nil {
		return "", authenticationError(err)
	}
	return ParsePayPalTransmissionSignature(value)
}

func payPalTransmissionTimeHeader(request *http.Request) (PayPalTransmissionTime, error) {
	value, err := uniqueHeader(request, PayPalTransmissionTimeHeaderName, PayPalTransmissionTimeMaximumBytes)
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
	wantHost := PayPalLiveCertificateHost
	if r.state.client.state.test {
		wantHost = PayPalSandboxCertificateHost
	}
	urlValue := endpoint.HTTPURL()
	if urlValue.Scheme != core.SchemeHTTPS || urlValue.Host != wantHost || urlValue.RawPath != "" ||
		urlValue.RawQuery != "" || path.Clean(urlValue.Path) != urlValue.Path ||
		!strings.HasPrefix(urlValue.Path, payPalWebhookCertificatePathPrefix) {
		return core.ErrProviderWireBinding
	}
	return nil
}

func (c PayPalClient) verifyWebhook(ctx context.Context, projection payPalWebhookVerificationProjection, policy exchange.OperationPolicy) (PayPalWebhookVerificationStatus, error) {
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
	host := PayPalLiveAPIHost
	if c.state.test {
		host = PayPalSandboxAPIHost
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
	if err != nil || value > PayPalWebhookMaximumBytes {
		return errors.Join(core.ErrProviderWireContract, err)
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
	if !payPalASCIIAlphanumeric(string(v), PayPalAuthAlgorithmMaximumBytes) {
		return core.ErrProviderWireContract
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
	if len(v) == 0 || len(v) > PayPalCertificateURLMaximumBytes {
		return core.ErrProviderWireContract
	}
	_, err := core.ParseHTTPEndpoint(string(v))
	if err != nil {
		return errors.Join(core.ErrProviderWireContract, err)
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
	return validatePayPalTransmissionMember(string(v), PayPalTransmissionIDMaximumBytes)
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
	return validatePayPalTransmissionMember(string(v), PayPalTransmissionSignatureMaximumBytes)
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
	if len(v) == 0 || len(v) > PayPalTransmissionTimeMaximumBytes {
		return core.ErrProviderWireContract
	}
	if _, err := temporal.ParseRFC3339(string(v)); err != nil {
		return errors.Join(core.ErrProviderWireContract, err)
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
	age, err := stripeSignatureAge(observed, instant, comparison)
	if err != nil {
		return err
	}
	order, err := age.Compare(tolerance)
	if err != nil || order == core.ComparisonGreater {
		return errors.Join(core.ErrProviderWireVerification, err)
	}
	return nil
}

func validPayPalScalarString(value string, err error) string {
	if err != nil {
		return ""
	}
	return value
}

func validatePayPalTransmissionMember(value string, maximum int) error {
	if len(value) < 2 || len(value) > maximum || !payPalWordByte(value[0]) {
		return core.ErrProviderWireContract
	}
	nondecimal := false
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return core.ErrProviderWireContract
		}
		nondecimal = nondecimal || value[index] < '0' || value[index] > '9'
	}
	if !nondecimal {
		return core.ErrProviderWireContract
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
