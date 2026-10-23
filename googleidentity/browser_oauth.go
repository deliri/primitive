package googleidentity

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

const googleBrowserAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
const googleBrowserTokenURL = "https://oauth2.googleapis.com/token"

// BrowserAuthorizationRequest carries exactly one server-flow authorization.
// State custody, callback admission and account policy belong to the caller.
// Source: https://developers.google.com/identity/openid-connect/openid-connect#authenticationuriparameters
// PKCE: https://developers.google.com/identity/protocols/oauth2/native-app#step1-code-verifier
// The same S256 OAuth protocol is used here with confidential web clients.
type BrowserAuthorizationRequest struct {
	ClientID    core.OAuthClientID
	RedirectURI core.OAuthRedirectURI
	State       core.OAuthState
	Challenge   core.OAuthPKCEChallenge
	Nonce       core.OAuthNonce
}
type BrowserAuthorization struct{ URL core.HTTPEndpoint }

func (r BrowserAuthorizationRequest) Validate() error {
	if err := errors.Join(r.ClientID.Validate(), r.RedirectURI.Validate(), r.State.Validate(), r.Challenge.Validate(), r.Nonce.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}
func (r BrowserAuthorization) Validate() error {
	if err := r.URL.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}
func BeginBrowserAuthorization(r BrowserAuthorizationRequest) (BrowserAuthorization, error) {
	if err := r.Validate(); err != nil {
		return BrowserAuthorization{}, err
	}
	configuration := oauth2.Config{ClientID: r.ClientID.String(), RedirectURL: r.RedirectURI.String(), Endpoint: oauth2.Endpoint{AuthURL: googleBrowserAuthorizeURL, TokenURL: googleBrowserTokenURL, AuthStyle: oauth2.AuthStyleInParams}, Scopes: []string{"openid", "email"}}
	endpoint, err := core.ParseHTTPEndpoint(configuration.AuthCodeURL(r.State.String(), oauth2.SetAuthURLParam("code_challenge", r.Challenge.String()), oauth2.SetAuthURLParam("code_challenge_method", "S256"), oauth2.SetAuthURLParam("nonce", r.Nonce.String())))
	if err != nil {
		return BrowserAuthorization{}, contractError(err)
	}
	return BrowserAuthorization{URL: endpoint}, nil
}

// BrowserAuthorizationCompleteRequest consumes one code. It owns no refresh,
// credential discovery, browser cookie, nonce generation or application session.
type BrowserAuthorizationCompleteRequest struct {
	IdentityVerifier BrowserVerifier
	ClockSkew        temporal.Duration
	Client           exchange.Client
	ClientID         core.OAuthClientID
	ClientSecret     core.OAuthClientSecret
	RedirectURI      core.OAuthRedirectURI
	Code             core.OAuthAuthorizationCode
	Verifier         core.OAuthPKCEVerifier
	Nonce            core.OAuthNonce
	Policy           exchange.OperationPolicy
}

func (r BrowserAuthorizationCompleteRequest) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.ClientID.Validate(), r.ClientSecret.Validate(), r.RedirectURI.Validate(), r.Code.Validate(), r.Verifier.Validate(), r.Nonce.Validate(), r.Policy.Validate(), r.IdentityVerifier.Validate(), r.ClockSkew.Validate()); err != nil {
		return contractError(err)
	}
	if r.IdentityVerifier.native.audience.String() != r.ClientID.String() || r.ClockSkew.Nanoseconds() > int64(core.OAuthClockSkewMaximumSeconds)*int64(temporal.NanosecondsPerSecond) || r.Policy.Retry.MaximumAttempts != 1 || r.Policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrGoogleIdentityContract
	}
	return nil
}

// HostedDomain is the signed optional hd claim. Unset explicitly means absent.
// This is a provider observation, not permission to join an application tenant.
type HostedDomain struct{ value string }

func ParseHostedDomain(value string) (HostedDomain, error) {
	v := HostedDomain{value}
	if err := v.Validate(); err != nil {
		return HostedDomain{}, err
	}
	return v, nil
}
func (v HostedDomain) Validate() error {
	if v.value == "" {
		return nil
	}
	for label := range strings.SplitSeq(v.value, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return core.ErrGoogleIdentityContract
		}
		for i := range len(label) {
			b := label[i]
			if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-') {
				return core.ErrGoogleIdentityContract
			}
		}
	}
	return nil
}
func (v HostedDomain) String() string { return v.value }
func (v HostedDomain) Present() bool  { return v.value != "" }

// BrowserVerifiedIdentity publishes signature-verified provider observations.
// Subject with canonical Issuer is the stable principal; email is not its key.
// EmailVerified does not imply that Google remains authoritative for a third-
// party email address. The application retains account/link/role admission.
type BrowserVerifiedIdentity struct {
	Issuer        core.HTTPEndpoint
	Subject       core.OAuthSubject
	Email         core.OAuthEmail
	HostedDomain  HostedDomain
	IssuedAt      temporal.Instant
	Expires       temporal.Instant
	EmailVerified bool
}

func (v BrowserVerifiedIdentity) Validate() error {
	if err := errors.Join(v.Issuer.Validate(), v.Subject.Validate(), v.HostedDomain.Validate(), v.IssuedAt.Validate(), v.Expires.Validate()); err != nil {
		return contractError(err)
	}
	if v.Email != (core.OAuthEmail{}) {
		if err := v.Email.Validate(); err != nil {
			return contractError(err)
		}
	} else if v.EmailVerified {
		return core.ErrGoogleIdentityContract
	}
	order, err := v.IssuedAt.Compare(v.Expires)
	if err != nil || order != core.ComparisonLess || v.Issuer.String() != googleCloudIdentityIssuer {
		return contractError(err)
	}
	return nil
}

// AuthoritativeEmail derives Google's documented email-authority observation;
// it grants no right to link an existing application account.
// Source: https://developers.google.com/identity/openid-connect/openid-connect#obtainuserinfo
func (v BrowserVerifiedIdentity) AuthoritativeEmail() bool {
	if v.Validate() != nil || !v.EmailVerified {
		return false
	}
	_, domain, ok := strings.Cut(v.Email.String(), "@")
	return ok && (strings.EqualFold(domain, "gmail.com") || v.HostedDomain.Present())
}
func (v BrowserVerifiedIdentity) PrincipalIdentity() (core.SHA256Digest, error) {
	if err := v.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	b, err := core.MarshalCanonicalJSONDocument(browserPrincipal{Issuer: v.Issuer.String(), Subject: v.Subject.String()})
	if err != nil {
		return core.SHA256Digest{}, contractError(err)
	}
	return core.SHA256Of(b), nil
}

type browserPrincipal struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// CompleteBrowserAuthorization performs Google's documented code exchange once
// through the pinned Go OAuth2 client and authenticates the returned ID token
// using Google's existing official idtoken verifier behind Exchange transport.
// OAuth2's token response is a native SDK whole-value operation (SDK1MiB read
// bound); no full-stream/O(1) token-decoding claim is made here.
// Source: https://developers.google.com/identity/openid-connect/openid-connect#exchangecode
func CompleteBrowserAuthorization(ctx context.Context, r BrowserAuthorizationCompleteRequest) (BrowserVerifiedIdentity, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	operation, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	defer cancel()
	attempt, attemptCancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: operation, Duration: r.Policy.AttemptTimeout})
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	defer attemptCancel()
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationBinary})
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	transport, err := r.Client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	client, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	secret, err := r.ClientSecret.CopyBytes()
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	defer clear(secret)
	configuration := oauth2.Config{ClientID: r.ClientID.String(), ClientSecret: string(secret), RedirectURL: r.RedirectURI.String(), Endpoint: oauth2.Endpoint{TokenURL: googleBrowserTokenURL, AuthStyle: oauth2.AuthStyleInParams}}
	token, err := configuration.Exchange(context.WithValue(attempt, oauth2.HTTPClient, client), r.Code.String(), oauth2.VerifierOption(r.Verifier.String()))
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(exchange.SanitizeOAuthTokenError(err))
	}
	defer func() { token.AccessToken = ""; token.RefreshToken = "" }()
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
	}
	// Google OpenID discovery advertises RS256 only. Reuse the existing
	// JOSE admission before the SDK; IAP ES256 is a different protocol and
	// malformed signatures must never reach its native fixed-width slicing.
	// Source: https://accounts.google.com/.well-known/openid-configuration
	if err := validateGoogleCloudJWTHeader(raw); err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	payload, err := r.IdentityVerifier.native.validator.Validate(attempt, raw, r.ClientID.String())
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	return browserIdentityFromPayload(attempt, r, payload)
}

func browserIdentityFromPayload(attempt context.Context, r BrowserAuthorizationCompleteRequest, payload *idtoken.Payload) (BrowserVerifiedIdentity, error) {
	var err error
	if payload.Issuer != googleCloudIdentityIssuer && payload.Issuer != "accounts.google.com" {
		return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
	}
	nonce, ok := payload.Claims["nonce"].(string)
	if !ok || subtle.ConstantTimeCompare([]byte(nonce), []byte(r.Nonce.String())) != 1 {
		return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
	}
	if azp, present := payload.Claims["azp"]; present {
		party, ok := azp.(string)
		if !ok || party != r.ClientID.String() {
			return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
		}
	}
	email := ""
	if claim, present := payload.Claims["email"]; present {
		var ok bool
		email, ok = claim.(string)
		if !ok || email == "" {
			return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
		}
	}
	verified := false
	if claim, present := payload.Claims["email_verified"]; present {
		var ok bool
		verified, ok = claim.(bool)
		if !ok {
			return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
		}
	}
	domain := HostedDomain{}
	if hd, present := payload.Claims["hd"]; present {
		value, ok := hd.(string)
		if !ok || value == "" {
			return BrowserVerifiedIdentity{}, core.ErrGoogleIdentityContract
		}
		domain, err = ParseHostedDomain(value)
		if err != nil {
			return BrowserVerifiedIdentity{}, err
		}
	}
	subject, subjectErr := core.ParseOAuthSubject(payload.Subject)
	address := core.OAuthEmail{}
	var emailErr error
	if email != "" {
		address, emailErr = core.ParseOAuthEmail(email)
	}
	issuer, issuerErr := core.ParseHTTPEndpoint(googleCloudIdentityIssuer)
	issued, issuedErr := temporal.InstantFromUnixSeconds(payload.IssuedAt)
	expires, expiresErr := temporal.InstantFromUnixSeconds(payload.Expires)
	if err := errors.Join(subjectErr, emailErr, issuerErr, issuedErr, expiresErr); err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	observation, err := temporal.Observe()
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	now, err := observation.Instant()
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	issuedBoundary, err := now.Add(r.ClockSkew)
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	issuedOrder, err := issued.Compare(issuedBoundary)
	if err != nil || issuedOrder == core.ComparisonGreater {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	expiryOrder, err := expires.Compare(now)
	if err != nil || expiryOrder != core.ComparisonGreater {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	value := BrowserVerifiedIdentity{Issuer: issuer, Subject: subject, Email: address, HostedDomain: domain, IssuedAt: issued, Expires: expires, EmailVerified: verified}
	if err := value.Validate(); err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	if err := contextstate.Validate(attempt); err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	return value, nil
}

func (BrowserAuthorizationRequest) googleIdentityProtocolFact()         {}
func (BrowserAuthorizationCompleteRequest) googleIdentityProtocolFact() {}
func (BrowserAuthorization) googleIdentitySealedProjection()            {}
func (BrowserVerifiedIdentity) googleIdentitySealedProjection()         {}
func (HostedDomain) googleIdentityProtocolFact()                        {}
func (browserPrincipal) googleIdentityInternalFlow()                    {}
