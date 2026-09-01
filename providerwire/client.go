package providerwire

import (
	"context"
	"errors"
	"path"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// StripeAPIVersion is the exact version compiled into the pinned official SDK.
	StripeAPIVersion = "2026-08-26.dahlia"
	// StripeIdempotencyKeyMaximumBytes is Stripe's published key ceiling.
	StripeIdempotencyKeyMaximumBytes = 255
	// PlunkIdempotencyKeyMaximumBytes is Plunk's published key ceiling.
	PlunkIdempotencyKeyMaximumBytes = 255
	// PayPalRequestIDMaximumBytes is PayPal's published single-byte ceiling.
	PayPalRequestIDMaximumBytes = 38
	// StripeAPIHost is Stripe's canonical API authority.
	StripeAPIHost = "api.stripe.com"
	// TwilioAPIHost is Twilio's canonical 2010 REST API authority.
	TwilioAPIHost = "api.twilio.com"
	// PlunkAPIHost is Plunk's current canonical API authority.
	PlunkAPIHost = "next-api.useplunk.com"
	// PayPalLiveAPIHost is PayPal's canonical live REST authority.
	PayPalLiveAPIHost = "api-m.paypal.com"
	// PayPalSandboxAPIHost is PayPal's canonical sandbox REST authority.
	PayPalSandboxAPIHost = "api-m.sandbox.paypal.com"
	// StripeVersionHeaderName selects Stripe's pinned API contract.
	StripeVersionHeaderName = "Stripe-Version"
	// PayPalRequestIDHeaderName carries PayPal's idempotency identity.
	PayPalRequestIDHeaderName = "PayPal-Request-Id"
)

type stripeClient struct {
	client     exchange.Client
	credential StripeCredential
}

// StripeClient is one Stripe-authorized protocol plug.
type StripeClient struct{ state *stripeClient }

func NewStripeClient(client exchange.Client, credential StripeCredential) (StripeClient, error) {
	if err := errors.Join(client.Validate(), credential.Validate()); err != nil {
		return StripeClient{}, err
	}
	owned, err := ParseStripeCredential(credential.key)
	if err != nil {
		return StripeClient{}, err
	}
	return StripeClient{state: &stripeClient{client: client, credential: owned}}, nil
}

func (c StripeClient) Validate() error {
	if c.state == nil {
		return core.ErrProviderWireContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.credential.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *StripeClient) Close() error {
	if c == nil || c.state == nil {
		return core.ErrProviderWireContract
	}
	err := c.state.credential.Close()
	c.state = nil
	return err
}

func (c StripeClient) RoundTrip(ctx context.Context, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	var zero exchange.StreamRoundTripResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	if err := validateStripeRequest(request); err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.key})
	if err != nil {
		return zero, err
	}
	version, err := providerHeader(StripeVersionHeaderName, StripeAPIVersion)
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization, version}}
	return exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

// Download performs one authenticated Stripe GET into caller-owned custody.
func (c StripeClient) Download(ctx context.Context, request exchange.DownloadRequest, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	var zero exchange.StreamResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	if err := validateProviderDownload(request, StripeAPIHost, "/v"); err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.key})
	if err != nil {
		return zero, err
	}
	version, err := providerHeader(StripeVersionHeaderName, StripeAPIVersion)
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization, version}}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

type plunkClient struct {
	client     exchange.Client
	credential PlunkCredential
}

// PlunkClient is one Plunk-authorized JSON protocol plug.
type PlunkClient struct{ state *plunkClient }

func NewPlunkClient(client exchange.Client, credential PlunkCredential) (PlunkClient, error) {
	if err := errors.Join(client.Validate(), credential.Validate()); err != nil {
		return PlunkClient{}, err
	}
	owned, err := ParsePlunkCredential(credential.key)
	if err != nil {
		return PlunkClient{}, err
	}
	return PlunkClient{state: &plunkClient{client: client, credential: owned}}, nil
}

func (c PlunkClient) Validate() error {
	if c.state == nil {
		return core.ErrProviderWireContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.credential.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *PlunkClient) Close() error {
	if c == nil || c.state == nil {
		return core.ErrProviderWireContract
	}
	err := c.state.credential.Close()
	c.state = nil
	return err
}

func (c PlunkClient) RoundTrip(ctx context.Context, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	var zero exchange.StreamRoundTripResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	if err := validatePlunkRequest(request); err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.key})
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	return exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

// Download performs one authenticated Plunk GET into caller-owned custody.
func (c PlunkClient) Download(ctx context.Context, request exchange.DownloadRequest, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	var zero exchange.StreamResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	if err := validateProviderDownload(request, PlunkAPIHost, "/v1/"); err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.key})
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

type twilioClient struct {
	client     exchange.Client
	credential TwilioCredential
}

// TwilioClient is one Twilio-authorized REST protocol plug.
type TwilioClient struct{ state *twilioClient }

func NewTwilioClient(client exchange.Client, credential TwilioCredential) (TwilioClient, error) {
	if err := errors.Join(client.Validate(), credential.Validate()); err != nil {
		return TwilioClient{}, err
	}
	owned, err := NewTwilioCredential(credential.AccountSID, credential.APIKeySID, credential.secret)
	if err != nil {
		return TwilioClient{}, err
	}
	return TwilioClient{state: &twilioClient{client: client, credential: owned}}, nil
}

func (c TwilioClient) Validate() error {
	if c.state == nil {
		return core.ErrProviderWireContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.credential.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *TwilioClient) Close() error {
	if c == nil || c.state == nil {
		return core.ErrProviderWireContract
	}
	err := c.state.credential.Close()
	c.state = nil
	return err
}

func (c TwilioClient) RoundTrip(ctx context.Context, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	var zero exchange.StreamRoundTripResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	if err := validateTwilioRequest(request, c.state.credential.AccountSID); err != nil {
		return zero, err
	}
	identity, err := exchange.ParseBasicAuthorizationIdentity(c.state.credential.APIKeySID.String())
	if err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{Identity: identity, Secret: c.state.credential.secret})
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	return exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

// Download performs one authenticated Twilio account-bound GET.
func (c TwilioClient) Download(ctx context.Context, request exchange.DownloadRequest, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	var zero exchange.StreamResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	prefix := "/2010-04-01/Accounts/" + c.state.credential.AccountSID.String() + "/"
	if err := validateProviderDownload(request, TwilioAPIHost, prefix); err != nil {
		return zero, err
	}
	identity, err := exchange.ParseBasicAuthorizationIdentity(c.state.credential.APIKeySID.String())
	if err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{Identity: identity, Secret: c.state.credential.secret})
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

type payPalClient struct {
	client exchange.Client
	token  PayPalAccessToken
	test   bool
}

type jsonProviderRequestContract struct {
	request exchange.StreamRoundTripRequest
	host    string
	prefix  string
}

type providerURLContract struct {
	scheme      string
	authority   string
	rawPath     string
	requestPath string
	host        string
}

// PayPalClient is one OAuth-authorized PayPal REST protocol plug.
type PayPalClient struct{ state *payPalClient }

func NewPayPalClient(client exchange.Client, token PayPalAccessToken, sandbox bool) (PayPalClient, error) {
	if err := errors.Join(client.Validate(), token.Validate()); err != nil {
		return PayPalClient{}, err
	}
	owned, err := ParsePayPalAccessToken(token.authorization.Token)
	if err != nil {
		return PayPalClient{}, err
	}
	return PayPalClient{state: &payPalClient{client: client, token: owned, test: sandbox}}, nil
}

func (c PayPalClient) Validate() error {
	if c.state == nil {
		return core.ErrProviderWireContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.token.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *PayPalClient) Close() error {
	if c == nil || c.state == nil {
		return core.ErrProviderWireContract
	}
	err := c.state.token.Close()
	c.state = nil
	return err
}

func (c PayPalClient) RoundTrip(ctx context.Context, request exchange.StreamRoundTripRequest, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	var zero exchange.StreamRoundTripResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	host := PayPalLiveAPIHost
	if c.state.test {
		host = PayPalSandboxAPIHost
	}
	if err := validatePayPalRequest(request, host); err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(c.state.token.authorization)
	if err != nil {
		return zero, err
	}
	headers := []exchange.Header{authorization}
	if request.Semantics.Replay == exchange.ReplaySingleAttemptWithIdempotencyKey {
		replay, headerErr := providerHeader(PayPalRequestIDHeaderName, request.Semantics.IdempotencyKey.String())
		if headerErr != nil {
			return zero, headerErr
		}
		headers = append(headers, replay)
	}
	request.Semantics = exchange.RequestSemantics{Method: request.Semantics.Method, Replay: exchange.ReplaySingleAttempt}
	request.Headers = exchange.Headers{Values: headers}
	return exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

// Download performs one authenticated PayPal GET into caller-owned custody.
func (c PayPalClient) Download(ctx context.Context, request exchange.DownloadRequest, policy exchange.StreamPolicy) (exchange.StreamResponse, error) {
	var zero exchange.StreamResponse
	if err := c.Validate(); err != nil {
		return zero, err
	}
	host := PayPalLiveAPIHost
	if c.state.test {
		host = PayPalSandboxAPIHost
	}
	if err := validateProviderDownload(request, host, "/v"); err != nil {
		return zero, err
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(c.state.token.authorization)
	if err != nil {
		return zero, err
	}
	request.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	return exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: request, Policy: policy})
}

func validateStripeRequest(request exchange.StreamRoundTripRequest) error {
	if err := validateProviderRequestBase(request, StripeAPIHost); err != nil {
		return err
	}
	urlValue := request.Target.HTTPURL()
	wantRequest := "application/x-www-form-urlencoded"
	if strings.HasPrefix(urlValue.Path, "/v2/") {
		wantRequest = "application/json"
	} else if !strings.HasPrefix(urlValue.Path, "/v1/") {
		return core.ErrProviderWireBinding
	}
	if err := requireMediaType(request.RequestContentType, wantRequest); err != nil {
		return err
	}
	if request.Semantics.Method != exchange.MethodPost {
		return core.ErrProviderWireBinding
	}
	return validateOptionalProviderIdempotency(request.Semantics, StripeIdempotencyKeyMaximumBytes)
}

func validatePlunkRequest(request exchange.StreamRoundTripRequest) error {
	if err := validateJSONProviderRequest(jsonProviderRequestContract{request: request, host: PlunkAPIHost, prefix: "/v1/"}); err != nil {
		return err
	}
	return validateOptionalProviderIdempotency(request.Semantics, PlunkIdempotencyKeyMaximumBytes)
}

func validateTwilioRequest(request exchange.StreamRoundTripRequest, account TwilioAccountSID) error {
	if err := validateProviderRequestBase(request, TwilioAPIHost); err != nil {
		return err
	}
	prefix := "/2010-04-01/Accounts/" + account.String() + "/"
	if !strings.HasPrefix(request.Target.HTTPURL().Path, prefix) ||
		request.Semantics.Method != exchange.MethodPost ||
		request.Semantics.Replay != exchange.ReplaySingleAttempt {
		return core.ErrProviderWireBinding
	}
	return requireMediaType(request.RequestContentType, "application/x-www-form-urlencoded")
}

func validateJSONProviderRequest(contract jsonProviderRequestContract) error {
	if err := validateProviderRequestBase(contract.request, contract.host); err != nil {
		return err
	}
	if !strings.HasPrefix(contract.request.Target.HTTPURL().Path, contract.prefix) || contract.request.Semantics.Method != exchange.MethodPost {
		return core.ErrProviderWireBinding
	}
	return requireMediaType(contract.request.RequestContentType, "application/json")
}

func validatePayPalRequest(request exchange.StreamRoundTripRequest, host string) error {
	if err := validateJSONProviderRequest(jsonProviderRequestContract{request: request, host: host, prefix: "/v"}); err != nil {
		return err
	}
	return validateOptionalProviderIdempotency(request.Semantics, PayPalRequestIDMaximumBytes)
}

func validateOptionalProviderIdempotency(semantics exchange.RequestSemantics, maximum int) error {
	switch semantics.Replay {
	case exchange.ReplaySingleAttempt:
		return nil
	case exchange.ReplaySingleAttemptWithIdempotencyKey:
		if validProviderIdempotencyKey(semantics.IdempotencyKey, maximum) {
			return nil
		}
		return core.ErrProviderWireContract
	default:
		return core.ErrProviderWireBinding
	}
}

func validateProviderRequestBase(request exchange.StreamRoundTripRequest, host string) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil {
		return core.ErrProviderWireContract
	}
	urlValue := request.Target.HTTPURL()
	if urlValue.Scheme != core.SchemeHTTPS || urlValue.Host != host || urlValue.RawPath != "" || urlValue.Path == "" || path.Clean(urlValue.Path) != urlValue.Path {
		return core.ErrProviderWireBinding
	}
	if err := requireMediaType(request.ExpectedResponseContentType, "application/json"); err != nil {
		return err
	}
	return nil
}

func validateProviderDownload(request exchange.DownloadRequest, host, prefix string) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil {
		return core.ErrProviderWireContract
	}
	urlValue := request.Target.HTTPURL()
	if !validProviderURL(providerURLContract{scheme: urlValue.Scheme, authority: urlValue.Host, rawPath: urlValue.RawPath, requestPath: urlValue.Path, host: host}) ||
		!strings.HasPrefix(urlValue.Path, prefix) {
		return core.ErrProviderWireBinding
	}
	if request.Semantics.Method != exchange.MethodGet || request.Semantics.Replay != exchange.ReplaySingleAttempt {
		return core.ErrProviderWireBinding
	}
	return requireMediaType(request.ExpectedResponseContentType, "application/json")
}

func validProviderURL(contract providerURLContract) bool {
	return contract.scheme == core.SchemeHTTPS && contract.authority == contract.host && contract.rawPath == "" && contract.requestPath != "" &&
		path.Clean(contract.requestPath) == contract.requestPath
}

func requireMediaType(got core.HTTPMediaType, want string) error {
	wantType, err := core.ParseHTTPMediaType(want)
	if err != nil {
		return err
	}
	matches, err := got.SameBase(wantType)
	if err != nil || !matches {
		return errors.Join(core.ErrProviderWireBinding, core.ErrExchangeContentType, err)
	}
	return nil
}

func providerHeader(nameText, valueText string) (exchange.Header, error) {
	name, err := core.ParseHTTPHeaderName(nameText)
	if err != nil {
		return exchange.Header{}, err
	}
	value, err := exchange.NewHeaderValue(valueText)
	if err != nil {
		return exchange.Header{}, err
	}
	header := exchange.Header{Name: name, Values: []exchange.HeaderValue{value}}
	return header, header.Validate()
}

func validProviderIdempotencyKey(key exchange.IdempotencyKey, maximum int) bool {
	if err := key.Validate(); err != nil {
		return false
	}
	value := key.String()
	return len(value) > 0 && len(value) <= maximum
}

var (
	_ core.Validatable = StripeClient{}
	_ core.Validatable = PlunkClient{}
	_ core.Validatable = TwilioClient{}
	_ core.Validatable = PayPalClient{}
)
