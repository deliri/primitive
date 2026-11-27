package googleidentity

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json/jsontext"
	"errors"
	"net/http"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	delegationIAMBase      = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/"
	delegationWireMaximum  = 16 * 1024
	delegationJWTMaximum   = 8 * 1024
	delegationKeyIDMaximum = 128
	delegationGrant        = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

type delegationJWTText []byte
type delegationBearerText []byte

type delegationTokenType uint8

const (
	delegationTokenInvalid delegationTokenType = iota
	delegationTokenBearer
)

func (t delegationTokenType) IsValid() bool { return t == delegationTokenBearer }
func (t delegationTokenType) String() string {
	if t.IsValid() {
		return googleAccessTokenTypeBearer
	}
	return ""
}
func (t delegationTokenType) Validate() error {
	if !t.IsValid() {
		return core.ErrGoogleDelegationResponse
	}
	return nil
}
func (t delegationTokenType) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(t.String())
}
func (t *delegationTokenType) UnmarshalJSON(data []byte) error {
	text, err := core.DecodeJSONStringToken(data)
	if err != nil || text != googleAccessTokenTypeBearer {
		return core.ErrGoogleDelegationResponse
	}
	*t = delegationTokenBearer
	return nil
}

// OAuth refusal details are projected to a closed operational category.
type delegationFaultCode uint8

const (
	delegationFaultUnknown delegationFaultCode = iota
	delegationFaultUnauthorizedClient
	delegationFaultAccessDenied
	delegationFaultAdminPolicy
)

func (c delegationFaultCode) IsValid() bool { return c <= delegationFaultAdminPolicy }
func (c delegationFaultCode) Validate() error {
	if !c.IsValid() {
		return core.ErrGoogleDelegationResponse
	}
	return nil
}
func (c delegationFaultCode) String() string {
	if !c.IsValid() {
		return ""
	}
	return [...]string{"", "unauthorized_client", "access_denied", "admin_policy_enforced"}[c]
}
func (c delegationFaultCode) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(c.String())
}
func (c *delegationFaultCode) UnmarshalJSON(data []byte) error {
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return core.ErrGoogleDelegationResponse
	}
	switch text {
	case delegationFaultUnauthorizedClient.String():
		*c = delegationFaultUnauthorizedClient
	case delegationFaultAccessDenied.String():
		*c = delegationFaultAccessDenied
	case delegationFaultAdminPolicy.String():
		*c = delegationFaultAdminPolicy
	default:
		*c = delegationFaultUnknown
	}
	return nil
}

type optionalDelegationScope struct {
	value   string
	present bool
}

func (s *optionalDelegationScope) UnmarshalJSON(data []byte) error {
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return core.ErrGoogleDelegationResponse
	}
	*s = optionalDelegationScope{value: text, present: true}
	return nil
}

type delegationClaims struct {
	Issuer           string `json:"iss"`
	Subject          string `json:"sub,omitempty"`
	Audience         string `json:"aud"`
	Scope            string `json:"scope"`
	IssuedAtSeconds  int64  `json:"iat"`
	ExpiresAtSeconds int64  `json:"exp"`
}

type delegatedSignRequest struct {
	Payload string `json:"payload"`
}

type delegatedSignReply struct {
	KeyID  string            `json:"keyId"`
	Signed delegationJWTText `json:"signedJwt"`
}

// signedDelegationAssertion is the exact compact credential returned by the
// bound IAM signer. Its mutable bytes are owned by a single exchange.
type signedDelegationAssertion struct{ bytes []byte }

type delegatedTokenReply struct {
	Token            delegationBearerText    `json:"access_token"`
	Type             delegationTokenType     `json:"token_type"`
	ExpiresInSeconds uint64                  `json:"expires_in"`
	Scope            optionalDelegationScope `json:"scope,omitempty"`
}

type delegatedFaultReply struct {
	Code        delegationFaultCode `json:"error"`
	Description string              `json:"error_description,omitempty"`
}

type delegationPOST struct {
	path    string
	media   core.HTTPMediaType
	headers []exchange.SecretHeader
	body    []byte
}

type delegationBuffer struct{ bytes []byte }

func (b *delegationBuffer) Write(p []byte) (int, error) {
	if len(p) > delegationWireMaximum-len(b.bytes) {
		return 0, core.ErrExchangeBodyLimit
	}
	b.bytes = append(b.bytes, p...)
	return len(p), nil
}

func (s *delegationJWTText) UnmarshalJSON(data []byte) error {
	raw, err := jsontext.AppendUnquote(nil, data)
	if err != nil || len(raw) == 0 || len(raw) > delegationJWTMaximum {
		clear(raw)
		return core.ErrGoogleDelegationResponse
	}
	*s = raw
	return nil
}
func (s *delegationBearerText) UnmarshalJSON(data []byte) error {
	raw, err := jsontext.AppendUnquote(nil, data)
	if err != nil {
		clear(raw)
		return core.ErrGoogleDelegationResponse
	}
	*s = raw
	return nil
}

func (r DelegatedAccessRequest) claims() (delegationClaims, error) {
	if err := r.Validate(); err != nil {
		return delegationClaims{}, err
	}
	nanos, err := r.ObservedAt.Nanoseconds()
	if err != nil {
		return delegationClaims{}, err
	}
	scope, err := r.Scope.wire()
	if err != nil {
		return delegationClaims{}, err
	}
	issued := nanos / int64(temporal.NanosecondsPerSecond)
	return delegationClaims{
		Issuer: r.Signer.String(), Subject: r.Subject.Email().String(), Audience: googleServiceAccountTokenURL,
		Scope: scope, IssuedAtSeconds: issued, ExpiresAtSeconds: issued + delegationAssertionSeconds,
	}, nil
}

func signDelegatedAssertion(ctx context.Context, network exchange.Client, request DelegatedAccessRequest, runtime AccessToken) (signedDelegationAssertion, error) {
	claims, err := request.claims()
	if err != nil {
		return signedDelegationAssertion{}, err
	}
	raw, err := core.MarshalCanonicalJSONDocument(claims)
	if err != nil {
		return signedDelegationAssertion{}, contractError(err)
	}
	body, err := core.MarshalCanonicalJSONDocument(delegatedSignRequest{Payload: string(raw)})
	if err != nil {
		return signedDelegationAssertion{}, contractError(err)
	}
	name, err := core.ParseHTTPHeaderName("Authorization")
	if err != nil || runtime.Validate() != nil {
		return signedDelegationAssertion{}, core.ErrGoogleIdentityContract
	}
	bearer := []byte(*runtime.value)
	defer clear(bearer)
	reply, err := runDelegationPOST(ctx, network, request.Policy, delegationPOST{
		path: delegationIAMBase + url.PathEscape(request.Signer.String()) + ":signJwt", media: core.HTTPMediaTypeJSON(),
		headers: []exchange.SecretHeader{{Name: name, Value: bearer, Scheme: exchange.SecretHeaderBearer}}, body: body,
	})
	defer clear(reply)
	if err != nil {
		if delegationHTTPStatus(err) == http.StatusForbidden {
			return signedDelegationAssertion{}, errors.Join(core.ErrGoogleDelegationDenied, err)
		}
		return signedDelegationAssertion{}, errors.Join(core.ErrGoogleDelegationResponse, err)
	}
	return decodeDelegatedAssertion(reply, claims)
}

func decodeDelegatedAssertion(data []byte, want delegationClaims) (signedDelegationAssertion, error) {
	reply, err := decodeDelegationDocument[delegatedSignReply](data, 2)
	if err != nil {
		clear(reply.Signed)
		return signedDelegationAssertion{}, err
	}
	if err := bindDelegatedAssertion(reply, want); err != nil {
		clear(reply.Signed)
		return signedDelegationAssertion{}, err
	}
	return signedDelegationAssertion{bytes: reply.Signed}, nil
}

func bindDelegatedAssertion(reply delegatedSignReply, want delegationClaims) error {
	if len(reply.KeyID) == 0 || len(reply.KeyID) > delegationKeyIDMaximum || len(reply.Signed) == 0 {
		return core.ErrGoogleDelegationResponse
	}
	header, tail, headerOK := bytes.Cut(reply.Signed, []byte{'.'})
	payload, signature, payloadOK := bytes.Cut(tail, []byte{'.'})
	if !headerOK || !payloadOK || len(signature) == 0 {
		return core.ErrGoogleDelegationResponse
	}
	headerJSON, hErr := decodeJWTPart(header)
	payloadJSON, pErr := decodeJWTPart(payload)
	sig, sErr := decodeJWTPart(signature)
	defer clear(sig)
	if err := errors.Join(hErr, pErr, sErr); err != nil {
		return err
	}
	if len(sig) == 0 {
		return core.ErrGoogleDelegationResponse
	}
	typedHeader, err := decodeDelegationDocument[googleCloudJWTHeader](headerJSON, 3)
	if err != nil {
		return err
	}
	if typedHeader.Validate() != nil || typedHeader.KeyID != reply.KeyID {
		return core.ErrGoogleDelegationBinding
	}
	got, err := decodeDelegationDocument[delegationClaims](payloadJSON, 6)
	if err != nil {
		return err
	}
	if got != want {
		return core.ErrGoogleDelegationBinding
	}
	// IAM's validated HTTPS connection and requested resource authenticate
	// the signer. OAuth verifies the signature against the asserted issuer.
	return nil
}

func decodeJWTPart(part []byte) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.Strict().AppendDecode(nil, part)
	if err != nil || len(decoded) == 0 {
		clear(decoded)
		return nil, core.ErrGoogleDelegationResponse
	}
	// The standard base64 decoder ignores newlines. Compact JWS cannot.
	if !bytes.Equal(base64.RawURLEncoding.AppendEncode(nil, decoded), part) {
		clear(decoded)
		return nil, core.ErrGoogleDelegationResponse
	}
	return decoded, nil
}

func exchangeDelegatedAssertion(ctx context.Context, network exchange.Client, request DelegatedAccessRequest, assertion signedDelegationAssertion) (DelegatedAccessToken, error) {
	// Valid compact JWS contains only base64url and dot, which are form-safe.
	body := make([]byte, 0, len(assertion.bytes)+96)
	body = append(body, "grant_type="+url.QueryEscape(delegationGrant)+"&assertion="...)
	body = append(body, assertion.bytes...)
	defer clear(body)
	reply, err := runDelegationPOST(ctx, network, request.Policy, delegationPOST{
		path: googleServiceAccountTokenURL, media: core.HTTPMediaTypeFormURLEncoded(), body: body,
	})
	defer clear(reply)
	if err != nil {
		return DelegatedAccessToken{}, classifyDelegationFault(reply, err)
	}
	scope, err := request.Scope.wire()
	if err != nil {
		return DelegatedAccessToken{}, err
	}
	return decodeDelegatedToken(reply, request.ObservedAt, scope)
}

func decodeDelegatedToken(data []byte, start temporal.Instant, scope string) (DelegatedAccessToken, error) {
	reply, err := decodeDelegationDocument[delegatedTokenReply](data, 4)
	defer clear(reply.Token)
	if err != nil {
		return DelegatedAccessToken{}, err
	}
	// OAuth permits omitting scope when the grant exactly matches the request.
	if reply.Scope.present && reply.Scope.value != scope {
		return DelegatedAccessToken{}, core.ErrGoogleDelegationBinding
	}
	if reply.Type != delegationTokenBearer || reply.ExpiresInSeconds == 0 || reply.ExpiresInSeconds > delegationLifetimeMaximum {
		return DelegatedAccessToken{}, core.ErrGoogleDelegationResponse
	}
	if err := (exchange.BearerAuthorization{Token: reply.Token}).Validate(); err != nil {
		return DelegatedAccessToken{}, errors.Join(core.ErrGoogleDelegationResponse, err)
	}
	lifetime, err := temporal.DurationFromSeconds(reply.ExpiresInSeconds)
	if err != nil {
		return DelegatedAccessToken{}, errors.Join(core.ErrGoogleDelegationResponse, err)
	}
	expires, err := start.Add(lifetime)
	if err != nil {
		return DelegatedAccessToken{}, errors.Join(core.ErrGoogleDelegationResponse, err)
	}
	secret, err := core.NewOAuthClientSecret(reply.Token)
	if err != nil {
		return DelegatedAccessToken{}, errors.Join(core.ErrGoogleDelegationResponse, err)
	}
	return DelegatedAccessToken{secret: secret, start: start, expiry: expires, lifetime: lifetime}, nil
}

func classifyDelegationFault(data []byte, cause error) error {
	status := delegationHTTPStatus(cause)
	if status == http.StatusForbidden && !errors.Is(cause, core.ErrExchangeBodyLimit) {
		return errors.Join(core.ErrGoogleDelegationDenied, cause)
	}
	if status == http.StatusBadRequest || status == http.StatusUnauthorized {
		fault, err := decodeDelegationDocument[delegatedFaultReply](data, 2)
		if err == nil && fault.Code != delegationFaultUnknown {
			return errors.Join(core.ErrGoogleDelegationDenied, cause)
		}
	}
	return errors.Join(core.ErrGoogleDelegationResponse, cause)
}

func delegationHTTPStatus(err error) int {
	if status, ok := errors.AsType[exchange.StatusError](err); ok {
		code, _ := status.Status().Int()
		return code
	}
	return 0
}

func decodeDelegationDocument[T any](data []byte, fieldLimit uint16) (T, error) {
	var zero T
	if len(data) > delegationWireMaximum {
		return zero, errors.Join(core.ErrGoogleDelegationResponse, core.ErrExchangeBodyLimit)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.NestingDepthMaximum = 1
	limits.ObjectFieldMaximum = fieldLimit
	limits.ArrayItemMaximum = 1
	result, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		// The provider can put assertion or bearer material in malformed fields.
		return zero, errors.Join(core.ErrGoogleDelegationResponse, core.ErrJSONContract)
	}
	return result, nil
}

func runDelegationPOST(ctx context.Context, network exchange.Client, policy Policy, input delegationPOST) ([]byte, error) {
	target, targetErr := core.ParseHTTPEndpoint(input.path)
	length, lengthErr := core.NewByteLength(uint64(len(input.body)))
	if err := errors.Join(targetErr, lengthErr); err != nil {
		return nil, contractError(err)
	}
	var receive delegationBuffer
	_, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{
		Context: ctx, Client: network, Policy: exchange.StreamPolicy{
			OperationTimeout: policy.OperationTimeout, AttemptTimeout: policy.AttemptTimeout,
			Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
		Request: exchange.StreamRoundTripRequest{
			Target: target, Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
			SecretHeaders: input.headers, Source: bytes.NewReader(input.body), RequestContentLength: &length,
			Destination: &receive, FailureDestination: &receive, RequestContentType: input.media,
			ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK(),
		},
	})
	return receive.bytes, err
}

func (delegationClaims) googleIdentityProtocolFact()               {}
func (delegatedSignRequest) googleIdentityProtocolFact()           {}
func (delegatedSignReply) googleIdentityInternalFlow()             {}
func (signedDelegationAssertion) googleIdentityCapabilityWrapper() {}
func (delegatedTokenReply) googleIdentityInternalFlow()            {}
func (delegatedFaultReply) googleIdentityInternalFlow()            {}
func (delegationPOST) googleIdentityInternalFlow()                 {}
func (delegationBuffer) googleIdentityInternalFlow()               {}

func (optionalDelegationScope) googleIdentityInternalFlow() {}
