package providerwire

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// PayPalClientIDMaximumBytes is Primitive's PayPal-specific OAuth client
	// identity ceiling where PayPal publishes no smaller wire maximum.
	PayPalClientIDMaximumBytes = 255
	// PayPalClientSecretMaximumBytes is Primitive's PayPal-specific OAuth
	// client-secret custody ceiling where PayPal publishes no smaller maximum.
	PayPalClientSecretMaximumBytes      = 1024
	payPalOAuthPath                     = "/v1/oauth2/token"
	payPalOAuthGrantBody                = "grant_type=client_credentials"
	payPalOAuthResponseMaximumBytes     = 64 * 1024
	payPalOAuthResponseFieldMaximum     = 6
	payPalOAuthTokenTypeBearer          = "Bearer"
	payPalOAuthResponseNestingMaximum   = 1
	payPalOAuthResponseArrayItemMaximum = 1
)

// PayPalClientCredential is one copied, redacted OAuth client capability.
type PayPalClientCredential struct {
	ClientID exchange.BasicAuthorizationIdentity
	secret   []byte
}

func NewPayPalClientCredential(clientID exchange.BasicAuthorizationIdentity, secret []byte) (PayPalClientCredential, error) {
	candidate := PayPalClientCredential{ClientID: clientID, secret: append([]byte(nil), secret...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.secret)
		return PayPalClientCredential{}, err
	}
	return candidate, nil
}

func (c PayPalClientCredential) Validate() error {
	if err := c.ClientID.Validate(); err != nil || len(c.ClientID.String()) == 0 || len(c.ClientID.String()) > PayPalClientIDMaximumBytes ||
		len(c.secret) == 0 || len(c.secret) > PayPalClientSecretMaximumBytes {
		return contractError(err)
	}
	request := exchange.BasicAuthorizationRequest{Identity: c.ClientID, Secret: c.secret}
	if err := request.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *PayPalClientCredential) Close() error {
	if c == nil {
		return core.ErrProviderWireContract
	}
	clear(c.secret)
	*c = PayPalClientCredential{}
	return nil
}

func (PayPalClientCredential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// PayPalAccessGrant is one provider-observed bearer and positive lifetime.
type PayPalAccessGrant struct {
	Token     PayPalAccessToken
	ExpiresIn temporal.Duration
}

// PayPalAccessGrantRequest owns one OAuth acquisition intent.
type PayPalAccessGrantRequest struct {
	Client     exchange.Client
	Credential PayPalClientCredential
	Policy     exchange.OperationPolicy
	Sandbox    bool
}

func (r PayPalAccessGrantRequest) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.Credential.Validate(), r.Policy.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (g PayPalAccessGrant) Validate() error {
	if err := errors.Join(g.Token.Validate(), g.ExpiresIn.Validate()); err != nil || g.ExpiresIn.IsZero() {
		return contractError(err)
	}
	return nil
}

func (g *PayPalAccessGrant) Close() error {
	if g == nil {
		return core.ErrProviderWireContract
	}
	err := g.Token.Close()
	*g = PayPalAccessGrant{}
	return err
}

func (PayPalAccessGrant) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type payPalOAuthLifetimeSeconds uint64

func (s payPalOAuthLifetimeSeconds) Validate() error {
	if s == 0 || uint64(s) > uint64(math.MaxInt64)/temporal.NanosecondsPerSecond {
		return core.ErrProviderWireContract
	}
	return nil
}

type payPalOAuthResponse struct {
	Scope       string                     `json:"scope"`
	AccessToken string                     `json:"access_token"`
	TokenType   string                     `json:"token_type"`
	AppID       string                     `json:"app_id"`
	ExpiresIn   payPalOAuthLifetimeSeconds `json:"expires_in"`
	Nonce       string                     `json:"nonce"`
}

func (r payPalOAuthResponse) Validate() error {
	if r.TokenType != payPalOAuthTokenTypeBearer {
		return core.ErrProviderWireContract
	}
	if err := r.ExpiresIn.Validate(); err != nil {
		return err
	}
	authorization := exchange.BearerAuthorization{Token: []byte(r.AccessToken)}
	if err := authorization.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (r payPalOAuthResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	type wire payPalOAuthResponse
	return json.Marshal(wire(r))
}

func (r payPalOAuthResponse) grant() (PayPalAccessGrant, error) {
	if err := r.Validate(); err != nil {
		return PayPalAccessGrant{}, err
	}
	token, err := ParsePayPalAccessToken([]byte(r.AccessToken))
	if err != nil {
		return PayPalAccessGrant{}, err
	}
	lifetime, err := temporal.DurationFromSeconds(uint64(r.ExpiresIn))
	if err != nil {
		return PayPalAccessGrant{}, errors.Join(contractError(err), token.Close())
	}
	grant := PayPalAccessGrant{Token: token, ExpiresIn: lifetime}
	return grant, grant.Validate()
}

// AcquirePayPalAccessGrant performs PayPal's documented OAuth client-credentials
// exchange. It does not cache, refresh, or decide how the token may be used.
func AcquirePayPalAccessGrant(ctx context.Context, intent PayPalAccessGrantRequest) (PayPalAccessGrant, error) {
	request, err := newPayPalOAuthRequest(ctx, intent)
	if err != nil {
		return PayPalAccessGrant{}, err
	}
	response, err := sendPayPalOAuthRequest(request)
	if err != nil {
		return PayPalAccessGrant{}, err
	}
	wire, err := decodePayPalOAuthResponse(response.Body, request.responseLimit)
	if err != nil {
		return PayPalAccessGrant{}, err
	}
	return wire.grant()
}

func newPayPalOAuthRequest(ctx context.Context, intent PayPalAccessGrantRequest) (payPalOAuthRequest, error) {
	if err := intent.Validate(); err != nil {
		return payPalOAuthRequest{}, err
	}
	host := PayPalLiveAPIHost
	if intent.Sandbox {
		host = PayPalSandboxAPIHost
	}
	target, err := core.ParseHTTPEndpoint("https://" + host + payPalOAuthPath)
	if err != nil {
		return payPalOAuthRequest{}, bindingError(err)
	}
	authorization, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{
		Identity: intent.Credential.ClientID, Secret: intent.Credential.secret,
	})
	if err != nil {
		return payPalOAuthRequest{}, contractError(err)
	}
	requestMedia, err := core.ParseHTTPMediaType("application/x-www-form-urlencoded")
	if err != nil {
		return payPalOAuthRequest{}, contractError(err)
	}
	responseMedia, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		return payPalOAuthRequest{}, contractError(err)
	}
	requestLimit, err := core.NewByteCount(uint64(len(payPalOAuthGrantBody)))
	if err != nil {
		return payPalOAuthRequest{}, contractError(err)
	}
	responseLimit, err := core.NewByteCount(payPalOAuthResponseMaximumBytes)
	if err != nil {
		return payPalOAuthRequest{}, contractError(err)
	}
	return payPalOAuthRequest{
		context: ctx, client: intent.Client, target: target, authorization: authorization,
		requestMedia: requestMedia, responseMedia: responseMedia,
		requestLimit: requestLimit, responseLimit: responseLimit, policy: intent.Policy,
	}, nil
}

type payPalOAuthRequest struct {
	context       context.Context
	client        exchange.Client
	target        core.HTTPEndpoint
	authorization exchange.Header
	requestMedia  core.HTTPMediaType
	responseMedia core.HTTPMediaType
	requestLimit  core.ByteCount
	responseLimit core.ByteCount
	policy        exchange.OperationPolicy
}

func sendPayPalOAuthRequest(request payPalOAuthRequest) (exchange.BoundedResponse, error) {
	return exchange.SendBounded(exchange.BoundedCall{
		Context: request.context,
		Client:  request.client,
		Request: exchange.BoundedRequest{
			Target: request.target,
			Semantics: exchange.RequestSemantics{
				Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt,
			},
			RequestContentType:          request.requestMedia,
			ExpectedResponseContentType: request.responseMedia,
			Body:                        []byte(payPalOAuthGrantBody),
			Headers:                     exchange.Headers{Values: []exchange.Header{request.authorization}},
			ExpectedStatus:              core.HTTPStatusOK(),
		},
		Policy: exchange.BoundedPolicy{
			Operation: request.policy, RequestBodyLimit: request.requestLimit, ResponseBodyLimit: request.responseLimit,
		},
	})
}

func decodePayPalOAuthResponse(body []byte, maximum core.ByteCount) (payPalOAuthResponse, error) {
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	limits.NestingDepthMaximum = payPalOAuthResponseNestingMaximum
	limits.ObjectFieldMaximum = payPalOAuthResponseFieldMaximum
	limits.ArrayItemMaximum = payPalOAuthResponseArrayItemMaximum
	wire, err := core.DecodeStrictJSONStructure[payPalOAuthResponse](body, limits)
	if err != nil {
		return payPalOAuthResponse{}, contractError(err)
	}
	if err := wire.Validate(); err != nil {
		return payPalOAuthResponse{}, err
	}
	return wire, nil
}

var (
	_ core.Validatable            = PayPalClientCredential{}
	_ core.Validatable            = PayPalAccessGrant{}
	_ core.Validatable            = PayPalAccessGrantRequest{}
	_ core.Validatable            = payPalOAuthResponse{}
	_ core.ValidatedJSONMarshaler = payPalOAuthResponse{}
)
