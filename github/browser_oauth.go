package github

import (
	"context"
	"errors"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
)

const githubBrowserAuthorizeURL = "https://github.com/login/oauth/authorize"
const githubBrowserTokenURL = "https://github.com/login/oauth/access_token"
const githubBrowserIssuer = "https://github.com"
const githubBrowserEmailPath = "/user/emails"

// BrowserAuthorizationRequest carries a single S256 server-flow request.
// Source: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#web-application-flow
type BrowserAuthorizationRequest struct {
	ClientID    core.OAuthClientID
	RedirectURI core.OAuthRedirectURI
	State       core.OAuthState
	Challenge   core.OAuthPKCEChallenge
}
type BrowserAuthorization struct{ URL core.HTTPEndpoint }

func (r BrowserAuthorizationRequest) Validate() error {
	if err := errors.Join(r.ClientID.Validate(), r.RedirectURI.Validate(), r.State.Validate(), r.Challenge.Validate()); err != nil {
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
	configuration := oauth2.Config{ClientID: r.ClientID.String(), RedirectURL: r.RedirectURI.String(), Endpoint: oauth2.Endpoint{AuthURL: githubBrowserAuthorizeURL, TokenURL: githubBrowserTokenURL, AuthStyle: oauth2.AuthStyleInParams}, Scopes: []string{"user:email"}}
	target, err := core.ParseHTTPEndpoint(configuration.AuthCodeURL(r.State.String(), oauth2.SetAuthURLParam("code_challenge", r.Challenge.String()), oauth2.SetAuthURLParam("code_challenge_method", "S256")))
	if err != nil {
		return BrowserAuthorization{}, contractError(err)
	}
	return BrowserAuthorization{URL: target}, nil
}

type BrowserAuthorizationCompleteRequest struct {
	Client       exchange.Client
	ClientID     core.OAuthClientID
	ClientSecret core.OAuthClientSecret
	RedirectURI  core.OAuthRedirectURI
	Code         core.OAuthAuthorizationCode
	Verifier     core.OAuthPKCEVerifier
	UserAgent    UserAgent
	Policy       exchange.OperationPolicy
}

func (r BrowserAuthorizationCompleteRequest) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.ClientID.Validate(), r.ClientSecret.Validate(), r.RedirectURI.Validate(), r.Code.Validate(), r.Verifier.Validate(), r.UserAgent.Validate(), r.Policy.Validate()); err != nil {
		return contractError(err)
	}
	if r.Policy.Retry.MaximumAttempts != 1 || r.Policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrGitHubContract
	}
	return nil
}

// BrowserUserID is the provider's stable numeric user identity, never login/email.
type BrowserUserID struct{ value uint64 }

func NewBrowserUserID(value uint64) (BrowserUserID, error) {
	v := BrowserUserID{value}
	if err := v.Validate(); err != nil {
		return BrowserUserID{}, err
	}
	return v, nil
}
func (v BrowserUserID) Validate() error {
	if v.value == 0 {
		return core.ErrGitHubContract
	}
	return nil
}
func (v BrowserUserID) Uint64() (uint64, error) {
	if err := v.Validate(); err != nil {
		return 0, err
	}
	return v.value, nil
}
func (v BrowserUserID) Subject() (core.OAuthSubject, error) {
	if err := v.Validate(); err != nil {
		return core.OAuthSubject{}, err
	}
	return core.ParseOAuthSubject(strconv.FormatUint(v.value, 10))
}

// BrowserVerifiedIdentity contains authenticated subject and optional primary
// email observations. No primary/verified email does not invalidate a subject;
// the caller decides whether registration or account linking is admissible.
type BrowserVerifiedIdentity struct {
	UserID        BrowserUserID
	Email         core.OAuthEmail
	EmailVerified bool
}

func (v BrowserVerifiedIdentity) Validate() error {
	if err := v.UserID.Validate(); err != nil {
		return err
	}
	if v.Email != (core.OAuthEmail{}) {
		if err := v.Email.Validate(); err != nil {
			return contractError(err)
		}
	} else if v.EmailVerified {
		return core.ErrGitHubResponse
	}
	return nil
}
func (v BrowserVerifiedIdentity) PrincipalIdentity() (core.SHA256Digest, error) {
	if err := v.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	subject, err := v.UserID.Subject()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	b, err := core.MarshalCanonicalJSONDocument(browserPrincipal{Issuer: githubBrowserIssuer, Subject: subject.String()})
	if err != nil {
		return core.SHA256Digest{}, contractError(err)
	}
	return core.SHA256Of(b), nil
}

type browserPrincipal struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// CompleteBrowserAuthorization exchanges once through the pinned Go OAuth2
// client, then uses this package's Exchange socket for /user and streamed email
// pages. Tokens never escape this operation. OAuth2 token replies are native SDK
// whole values (its1MiB read bound); email collections are never retained.
// Source: https://docs.github.com/en/rest/users/users#get-the-authenticated-user
// https://docs.github.com/en/rest/users/emails#list-email-addresses-for-the-authenticated-user
func CompleteBrowserAuthorization(ctx context.Context, r BrowserAuthorizationCompleteRequest) (BrowserVerifiedIdentity, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	operation, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	defer cancel()
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationBinary})
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	transport, err := r.Client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	native, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	secret, err := r.ClientSecret.CopyBytes()
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	defer clear(secret)
	configuration := oauth2.Config{ClientID: r.ClientID.String(), ClientSecret: string(secret), RedirectURL: r.RedirectURI.String(), Endpoint: oauth2.Endpoint{TokenURL: githubBrowserTokenURL, AuthStyle: oauth2.AuthStyleInParams}}
	tokenContext, tokenCancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: operation, Duration: r.Policy.AttemptTimeout})
	if err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	defer tokenCancel()
	token, err := configuration.Exchange(context.WithValue(tokenContext, oauth2.HTTPClient, native), r.Code.String(), oauth2.VerifierOption(r.Verifier.String()))
	if err != nil {
		return BrowserVerifiedIdentity{}, authenticationError(exchange.SanitizeOAuthTokenError(err))
	}
	defer func() { token.AccessToken = ""; token.RefreshToken = "" }()
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: []byte(token.AccessToken)})
	if err != nil {
		return BrowserVerifiedIdentity{}, authenticationError(err)
	}
	client, err := NewClient(r.Client, r.UserAgent)
	if err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	defer func() { _ = client.Close() }()
	headers, capture, err := client.headers(operation, true)
	if err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	headers.Values = append(headers.Values, authorization)
	userTarget, err := client.target("/user", nil)
	if err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	var id BrowserUserID
	_, err = client.readBrowserBody(browserBodyRequest{ctx: operation, target: userTarget, headers: headers, capture: capture, policy: r.Policy, decode: func(source io.Reader) error {
		var decodeErr error
		id, decodeErr = decodeBrowserUser(operation, source)
		return decodeErr
	}})
	if err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	primary := browserPrimaryEmail{}
	scope := ""
	if observed := token.Extra("scope"); observed != nil {
		var ok bool
		scope, ok = observed.(string)
		if !ok {
			return BrowserVerifiedIdentity{}, core.ErrGitHubResponse
		}
	}
	if browserEmailScopeGranted(scope) {
		for page := uint64(1); ; {
			target, targetErr := client.target(githubBrowserEmailPath, url.Values{"page": []string{strconv.FormatUint(page, 10)}, "per_page": []string{strconv.Itoa(core.GitHubTagPageMaximumEntries)}})
			if targetErr != nil {
				return BrowserVerifiedIdentity{}, targetErr
			}
			observed, readErr := client.readBrowserBody(browserBodyRequest{ctx: operation, target: target, headers: headers, capture: capture, policy: r.Policy, decode: func(source io.Reader) error { return decodeBrowserEmails(operation, source, &primary) }})
			if readErr != nil {
				return BrowserVerifiedIdentity{}, readErr
			}
			next, nextErr := client.nextBrowserEmailPage(observed.Metadata.Headers, page)
			if nextErr != nil {
				return BrowserVerifiedIdentity{}, nextErr
			}
			if next == 0 {
				break
			}
			page = next
		}
	}
	value := BrowserVerifiedIdentity{UserID: id, Email: primary.email, EmailVerified: primary.verified}
	if err := value.Validate(); err != nil {
		return BrowserVerifiedIdentity{}, err
	}
	if err := contextstate.Validate(operation); err != nil {
		return BrowserVerifiedIdentity{}, contractError(err)
	}
	return value, nil
}

// GitHub normalizes granted scopes as comma-separated tokens. The broader
// user scope includes user:email; unrelated or absent grants cannot authorize
// /user/emails. Subject authentication remains usable without that permission.
// Source: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
func browserEmailScopeGranted(scope string) bool {
	for comma := range strings.SplitSeq(scope, ",") {
		for value := range strings.SplitSeq(comma, " ") {
			if value == "user:email" || value == "user" {
				return true
			}
		}
	}
	return false
}

type browserBodyRequest struct {
	ctx     context.Context
	decode  func(io.Reader) error
	headers exchange.Headers
	capture exchange.HeaderSelection
	target  core.HTTPEndpoint
	policy  exchange.OperationPolicy
}
type browserDownloadResult struct {
	response exchange.StreamResponse
	err      error
}

func (c Client) readBrowserBody(r browserBodyRequest) (exchange.StreamResponse, error) {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	reader, writer := io.Pipe()
	done := make(chan browserDownloadResult, 1)
	go func() {
		response, err := exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: exchange.DownloadRequest{Target: r.target, Destination: writer, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), Headers: r.headers, CaptureHeaders: r.capture, ExpectedStatus: core.HTTPStatusOK()}, Policy: exchange.StreamPolicy{OperationTimeout: r.policy.OperationTimeout, AttemptTimeout: r.policy.AttemptTimeout, Redirect: r.policy.Redirect}})
		closed := writer.CloseWithError(err)
		done <- browserDownloadResult{response: response, err: errors.Join(classifyExchangeError(err), closed)}
	}()
	joined := false
	defer func() {
		cancel()
		// Go's PipeReader.Close always returns nil; cancel and join own cleanup.
		_ = reader.Close()
		if !joined {
			<-done
		}
	}()
	decoded := r.decode(reader)
	if decoded != nil {
		cancel()
		decoded = errors.Join(decoded, reader.CloseWithError(decoded))
	} else {
		decoded = reader.Close()
	}
	observed := <-done
	joined = true
	if err := errors.Join(decoded, observed.err); err != nil {
		return exchange.StreamResponse{}, responseError(err)
	}
	return observed.response, nil
}

type browserUserWire struct {
	ID *uint64 `json:"id"`
}

func decodeBrowserUser(ctx context.Context, source io.Reader) (BrowserUserID, error) {
	cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: source, NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return BrowserUserID{}, responseError(err)
	}
	wire, err := jsonio.DecodeProjectionAt[browserUserWire](ctx, cursor, jsonio.MembersIgnoreExtra)
	if err != nil {
		return BrowserUserID{}, responseError(err)
	}
	if err := cursor.RequireEOF(ctx); err != nil {
		return BrowserUserID{}, responseError(err)
	}
	if wire.ID == nil {
		return BrowserUserID{}, core.ErrGitHubResponse
	}
	id, err := NewBrowserUserID(*wire.ID)
	if err != nil {
		return BrowserUserID{}, responseError(err)
	}
	return id, nil
}

type browserEmailWire struct {
	Email    *string `json:"email"`
	Primary  *bool   `json:"primary"`
	Verified *bool   `json:"verified"`
}
type browserPrimaryEmail struct {
	email    core.OAuthEmail
	verified bool
}

func decodeBrowserEmails(ctx context.Context, source io.Reader, primary *browserPrimaryEmail) error {
	cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: source, NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return responseError(err)
	}
	begin, err := cursor.Decode(ctx)
	if err != nil || begin.Kind != jsonio.TokenArrayStart {
		return responseError(err)
	}
	for {
		kind, peekErr := cursor.PeekKind(ctx)
		if peekErr != nil {
			return responseError(peekErr)
		}
		if kind == jsonio.TokenArrayEnd {
			if _, err := cursor.Decode(ctx); err != nil {
				return responseError(err)
			}
			if err := cursor.RequireEOF(ctx); err != nil {
				return responseError(err)
			}
			return nil
		}
		wire, wireErr := jsonio.DecodeProjectionAt[browserEmailWire](ctx, cursor, jsonio.MembersIgnoreExtra)
		if wireErr != nil {
			return responseError(wireErr)
		}
		if wire.Email == nil || wire.Primary == nil || wire.Verified == nil {
			return core.ErrGitHubResponse
		}
		email, emailErr := core.ParseOAuthEmail(*wire.Email)
		if emailErr != nil {
			return responseError(emailErr)
		}
		if !*wire.Primary {
			continue
		}
		if primary.email != (core.OAuthEmail{}) && (primary.email != email || primary.verified != *wire.Verified) {
			return core.ErrGitHubResponse
		}
		primary.email = email
		primary.verified = *wire.Verified
	}
}
func (c Client) nextBrowserEmailPage(headers exchange.CapturedHeaders, page uint64) (uint64, error) {
	next := uint64(0)
	for _, header := range headers.Values {
		if header.Name.String() != headerLink {
			continue
		}
		if len(header.Values) != 1 {
			return 0, core.ErrGitHubResponse
		}
		raw, err := header.Values[0].Value()
		if err != nil {
			return 0, responseError(err)
		}
		for entry := range strings.SplitSeq(raw, ",") {
			candidate := strings.TrimSpace(entry)
			if !strings.Contains(candidate, githubLinkNextRelation) {
				continue
			}
			if next != 0 || page == math.MaxUint64 {
				return 0, core.ErrGitHubResponse
			}
			const suffix = `>; rel="next"`
			if !strings.HasPrefix(candidate, "<") || !strings.HasSuffix(candidate, suffix) {
				return 0, core.ErrGitHubBinding
			}
			target, targetErr := url.Parse(strings.TrimSuffix(strings.TrimPrefix(candidate, "<"), suffix))
			if targetErr != nil || !c.matchesAuthority(target) || target.Path != githubBrowserEmailPath {
				return 0, bindingError(targetErr)
			}
			query, queryErr := url.ParseQuery(target.RawQuery)
			if queryErr != nil || len(query) != 2 || len(query["page"]) != 1 || len(query["per_page"]) != 1 || query.Get("page") != strconv.FormatUint(page+1, 10) || query.Get("per_page") != strconv.Itoa(core.GitHubTagPageMaximumEntries) {
				return 0, bindingError(queryErr)
			}
			next = page + 1
		}
	}
	return next, nil
}
func (BrowserAuthorizationRequest) githubProtocolFact()         {}
func (BrowserAuthorizationCompleteRequest) githubProtocolFact() {}
func (BrowserAuthorization) githubProtocolFact()                {}
func (BrowserVerifiedIdentity) githubProtocolFact()             {}
func (BrowserUserID) githubSealedValue()                        {}
func (browserPrincipal) githubInternalFlow()                    {}
func (browserBodyRequest) githubInternalFlow()                  {}
func (browserDownloadResult) githubInternalFlow()               {}
func (browserUserWire) githubInternalFlow()                     {}
func (browserEmailWire) githubInternalFlow()                    {}
func (browserPrimaryEmail) githubInternalFlow()                 {}
