package timematch

import (
	"context"
	jsontext "encoding/json/jsontext"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

// Google OAuth web-server flow and Calendar scopes.
// https://developers.google.com/identity/protocols/oauth2/web-server
// https://developers.google.com/workspace/calendar/api/auth
// PKCE parameters (code_challenge, S256, code_verifier) are documented at
// https://developers.google.com/identity/protocols/oauth2/native-app; the
// web-server page does not mention them. This package always sends them.
const (
	googleAuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL         = "https://oauth2.googleapis.com/token"
	googleRevokeURL        = "https://oauth2.googleapis.com/revoke"
	// "See the availability on Google calendars you have access to." Accepted by freeBusy.query.
	googleScopeFreeBusy = "https://www.googleapis.com/auth/calendar.events.freebusy"
	// "See the list of Google calendars you're subscribed to." Accepted by calendarList.list.
	googleScopeCalendarList = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
	// "See, create, change, and delete events on Google calendars you own." Accepted by events insert/update/delete.
	googleScopeOwnedEvents = "https://www.googleapis.com/auth/calendar.events.owned"
	googleScopeOpenID      = "openid"
	googleScopeEmail       = "email"
	googleScopeProfile     = "profile"
)

// Microsoft identity platform v2 auth code flow and Graph permissions.
// https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow
// Calendars.ReadBasic is the least-privileged delegated permission for list
// calendars and calendarView for work/school and personal accounts;
// Calendars.ReadWrite is required for event create/update/delete.
// A refresh token is "only provided if offline_access scope was requested".
const (
	microsoftLoginURL        = "https://login.microsoftonline.com/"
	microsoftScopeReadBasic  = "https://graph.microsoft.com/Calendars.ReadBasic"
	microsoftScopeReadWrite  = "https://graph.microsoft.com/Calendars.ReadWrite"
	microsoftScopeUserRead   = "https://graph.microsoft.com/User.Read"
	microsoftScopeOffline    = "offline_access"
	microsoftGraphScopeShort = "https://graph.microsoft.com/"
)

const bearerTokenType = "bearer"

func scopes(provider Provider, access Access, lifetime Lifetime) ([]string, error) {
	switch provider {
	case ProviderGoogle:
		// A booking visitor grants profile, free/busy and event creation once.
		// The client reads no appointment content and never requests offline
		// access. Read-only availability remains a separate SDK capability.
		if lifetime == LifetimeVisitor {
			if access == AccessManageEvents {
				return []string{googleScopeFreeBusy, googleScopeOwnedEvents, googleScopeOpenID, googleScopeEmail, googleScopeProfile, googleidentity.BrowserPhoneScope}, nil
			}
			return []string{googleScopeFreeBusy}, nil
		}
		values := []string{googleScopeFreeBusy, googleScopeCalendarList}
		if access == AccessManageEvents {
			values = append(values, googleScopeOwnedEvents)
		}
		return values, nil
	case ProviderMicrosoft:
		values := []string{microsoftScopeReadBasic}
		if access == AccessManageEvents {
			values = []string{microsoftScopeReadWrite}
			if lifetime == LifetimeVisitor {
				values = append(values, microsoftScopeUserRead)
			}
		}
		if lifetime == LifetimeBusiness {
			values = append(values, microsoftScopeOffline)
		}
		return values, nil
	default:
		return nil, core.ErrTimeMatchContract
	}
}

// AuthorizationEndpoint builds the provider consent URL. A booking visitor
// authorizes contact profile, free/busy and the chosen event in one redirect.
// This is booking contact data, never an application login or account link.
// A visitor grant is online-only.
func AuthorizationEndpoint(r AuthorizationRequest) (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	scope, err := scopes(r.Application.Provider, r.Access, r.Lifetime)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	query := url.Values{}
	query.Set("client_id", r.Application.ClientID.String())
	query.Set("redirect_uri", r.Application.RedirectURI.String())
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scope, " "))
	query.Set("state", r.State.String())
	query.Set("code_challenge", r.Challenge.String())
	query.Set("code_challenge_method", "S256")
	base := googleAuthorizationURL
	switch r.Application.Provider {
	case ProviderGoogle:
		// access_type=offline returns a refresh token; prompt=consent makes
		// Google issue it again on a repeated business connection.
		if r.Lifetime == LifetimeBusiness {
			query.Set("access_type", "offline")
			query.Set("prompt", "consent")
		} else {
			query.Set("access_type", "online")
		}
	case ProviderMicrosoft:
		base = microsoftLoginURL + r.Application.Tenant.String() + "/oauth2/v2.0/authorize"
		query.Set("response_mode", "query")
	default:
		return core.HTTPEndpoint{}, core.ErrTimeMatchContract
	}
	return endpoint(base + "?" + query.Encode())
}

func tokenURL(application Application) string {
	if application.Provider == ProviderMicrosoft {
		return microsoftLoginURL + application.Tenant.String() + "/oauth2/v2.0/token"
	}
	return googleTokenURL
}

// ExchangeCode redeems one authorization code. A grant missing any requested
// scope (Google granular consent lets users untick scopes) fails with
// ErrTimeMatchConsent and its credentials are destroyed.
func (c Client) ExchangeCode(ctx context.Context, r CodeExchange) (Grant, error) {
	if err := c.begin(ctx, r); err != nil {
		return Grant{}, err
	}
	scope, err := scopes(r.Application.Provider, r.Access, r.Lifetime)
	if err != nil {
		return Grant{}, err
	}
	secret := r.Application.ClientSecret.value
	form := formBody(
		formPair{key: "grant_type", value: []byte("authorization_code")},
		formPair{key: "code", value: []byte(r.Code.String())},
		formPair{key: "redirect_uri", value: []byte(r.Application.RedirectURI.String())},
		formPair{key: "client_id", value: []byte(r.Application.ClientID.String())},
		formPair{key: "client_secret", value: secret},
		formPair{key: "code_verifier", value: []byte(r.Verifier.String())},
		formPair{key: "scope", value: []byte(strings.Join(scope, " ")), omit: r.Application.Provider != ProviderMicrosoft},
	)
	token, err := c.token(ctx, r.Application, form, r.Policy)
	if err != nil {
		return Grant{}, err
	}
	defer token.clear()
	grant, err := admitExchange(token, r)
	if err != nil && r.Application.Provider == ProviderGoogle {
		// A refused grant must not stay live at the provider. Revoking the
		// access token also revokes its refresh token (Google OAuth web-server
		// guide). Microsoft documents no revocation door: zeroing is all.
		_, revokeErr := c.revokeGoogle(ctx, token.access, r.Policy)
		return Grant{}, errors.Join(err, revokeErr)
	}
	return grant, err
}

// admitExchange applies the lifetime contract to an issued token: a visitor
// never consented to retained access, and a business grant requires it.
func admitExchange(token tokenWire, r CodeExchange) (Grant, error) {
	if r.Lifetime == LifetimeVisitor && len(token.refresh) != 0 {
		return Grant{}, core.ErrTimeMatchResponse
	}
	if r.Lifetime == LifetimeBusiness && len(token.refresh) == 0 {
		return Grant{}, core.ErrTimeMatchConsent
	}
	return token.grant(grantShape{provider: r.Application.Provider, access: r.Access, lifetime: r.Lifetime, observed: r.ObservedAt})
}

// Refresh renews a business grant. Microsoft rotates the refresh token on
// every use ("You're expected to discard the old refresh token"); the
// returned Grant then carries the replacement. Google returns none.
func (c Client) Refresh(ctx context.Context, r RefreshRequest) (Grant, error) {
	if err := c.begin(ctx, r); err != nil {
		return Grant{}, err
	}
	scope, err := scopes(r.Application.Provider, r.Access, LifetimeBusiness)
	if err != nil {
		return Grant{}, err
	}
	form := formBody(
		formPair{key: "grant_type", value: []byte("refresh_token")},
		formPair{key: "refresh_token", value: r.RefreshToken.value},
		formPair{key: "client_id", value: []byte(r.Application.ClientID.String())},
		formPair{key: "client_secret", value: r.Application.ClientSecret.value},
		formPair{key: "scope", value: []byte(strings.Join(scope, " ")), omit: r.Application.Provider != ProviderMicrosoft},
	)
	token, err := c.token(ctx, r.Application, form, r.Policy)
	if err != nil {
		return Grant{}, err
	}
	defer token.clear()
	return token.grant(grantShape{provider: r.Application.Provider, access: r.Access, lifetime: LifetimeBusiness, observed: r.ObservedAt})
}

// Revoke invalidates the submitted token at Google. Submit the refresh token
// when retained, so access-token expiry cannot leave that permission live.
// RevokeTokenAlreadyInvalid describes only the submitted credential, never
// proves that a different refresh token was revoked. Microsoft documents no delegated-token revocation
// endpoint and Apple app-specific passwords are revoked by the account owner
// at account.apple.com; both return RevokeProviderHasNoRevocation without HTTP.
func (c Client) Revoke(ctx context.Context, r RevokeRequest) (RevokeOutcome, error) {
	if err := c.begin(ctx, r); err != nil {
		return RevokeInvalid, err
	}
	if r.Provider != ProviderGoogle {
		return RevokeProviderHasNoRevocation, nil
	}
	return c.revokeGoogle(ctx, r.Token.value, r.Policy)
}

func (c Client) revokeGoogle(ctx context.Context, token []byte, policy exchange.OperationPolicy) (RevokeOutcome, error) {
	target, err := endpoint(googleRevokeURL)
	if err != nil {
		return RevokeInvalid, err
	}
	form := formBody(formPair{key: "token", value: token})
	defer clear(form)
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPost, body: form, requestType: core.HTTPMediaTypeFormURLEncoded(), policy: policy, expected: http.StatusOK})
	defer clear(result.body)
	if err != nil {
		// Google documents invalid_token for an expired/already revoked token.
		// Other 400s, malformed bodies and unavailable endpoints remain errors.
		// https://developers.google.com/identity/oauth2/web/reference/js-reference#revocationresponse
		var failure struct {
			Error string `json:"error"`
		}
		if result.status == http.StatusBadRequest && unmarshalExtensible(result.body, &failure) == nil && failure.Error == "invalid_token" {
			return RevokeTokenAlreadyInvalid, nil
		}
		return RevokeInvalid, oauthFailure(result, err)
	}
	return RevokedAtProvider, nil
}

func (c Client) token(ctx context.Context, application Application, form []byte, policy exchange.OperationPolicy) (tokenWire, error) {
	defer clear(form)
	target, err := endpoint(tokenURL(application))
	if err != nil {
		return tokenWire{}, err
	}
	result, err := c.send(ctx, call{target: target, method: exchange.MethodPost, body: form, requestType: core.HTTPMediaTypeFormURLEncoded(), responseType: core.HTTPMediaTypeJSON(), policy: policy, expected: http.StatusOK})
	if err != nil {
		return tokenWire{}, oauthFailure(result, err)
	}
	defer clear(result.body)
	token, err := decodeJSON[tokenWire](result.body)
	if err != nil {
		return tokenWire{}, err
	}
	return token, nil
}

// oauthFailure classifies RFC 6749 section 5.2 error codes without retaining
// or echoing the provider body. invalid_grant covers an expired or revoked
// grant; consent_required and interaction_required are Microsoft's
// documented consent refusals.
func oauthFailure(result outcome, err error) error {
	defer clear(result.body)
	if result.status == http.StatusBadRequest || result.status == http.StatusUnauthorized {
		var document struct {
			Error string `json:"error"`
		}
		if unmarshalExtensible(result.body, &document) == nil {
			switch document.Error {
			case "invalid_grant", "consent_required", "interaction_required":
				return errors.Join(core.ErrTimeMatchConsent, err)
			}
		}
	}
	return errors.Join(core.ErrTimeMatchResponse, err)
}

// tokenWire is the RFC 6749 section 5.1 response. Credentials are copied into
// SDK-owned byte slices that are cleared after projection.
type tokenWire struct {
	access       []byte
	refresh      []byte
	scope        string
	scopePresent bool
	tokenType    string
	expiresIn    int64
}

// wireScope records whether the RFC6749 optional scope member was supplied.
// Omission means the originally requested scope; an explicitly empty or wrong
// scope never gains that permission by default.
// Source: https://www.rfc-editor.org/rfc/rfc6749#section-5.1
type wireScope struct {
	value   string
	present bool
}

func (s *wireScope) UnmarshalJSON(raw []byte) error {
	value, err := core.DecodeJSONStringToken(raw)
	if err != nil {
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
	*s = wireScope{value: value, present: true}
	return nil
}
func (wireScope) timematchInternalFlow() {}

type wireSecret struct{ value []byte }

func (s *wireSecret) UnmarshalJSON(raw []byte) error {
	value, err := jsontext.AppendUnquote(nil, raw)
	if err != nil {
		return errors.Join(core.ErrTimeMatchResponse, err)
	}
	s.value = value
	return nil
}

func (t *tokenWire) UnmarshalJSON(raw []byte) error {
	var document struct {
		AccessToken  wireSecret `json:"access_token"`
		RefreshToken wireSecret `json:"refresh_token"`
		Scope        wireScope  `json:"scope"`
		TokenType    string     `json:"token_type"`
		ExpiresIn    int64      `json:"expires_in"`
	}
	if err := unmarshalExtensible(raw, &document); err != nil {
		clear(document.AccessToken.value)
		clear(document.RefreshToken.value)
		return err
	}
	*t = tokenWire{access: document.AccessToken.value, refresh: document.RefreshToken.value, scope: document.Scope.value, scopePresent: document.Scope.present, tokenType: document.TokenType, expiresIn: document.ExpiresIn}
	return nil
}

func (t tokenWire) Validate() error {
	if !visibleASCII(t.access) || t.expiresIn <= 0 || !strings.EqualFold(t.tokenType, bearerTokenType) {
		t.clear()
		return core.ErrTimeMatchResponse
	}
	if len(t.refresh) != 0 && !visibleASCII(t.refresh) {
		t.clear()
		return core.ErrTimeMatchResponse
	}
	return nil
}

func (t tokenWire) clear() {
	clear(t.access)
	clear(t.refresh)
}

// grantShape is the requested grant a token response is projected into.
type grantShape struct {
	observed temporal.Instant
	provider Provider
	access   Access
	lifetime Lifetime
}

func (t tokenWire) grant(shape grantShape) (Grant, error) {
	provider, access, lifetime, observed := shape.provider, shape.access, shape.lifetime, shape.observed
	if (t.scopePresent || t.scope != "") && !grantedScopes(shape, t.scope) {
		return Grant{}, core.ErrTimeMatchConsent
	}
	expires, err := expiry(observed, t.expiresIn)
	if err != nil {
		return Grant{}, err
	}
	grant := Grant{Provider: provider, Access: access, Lifetime: lifetime, ExpiresAt: expires}
	if grant.AccessToken, err = ParseSecret(t.access); err != nil {
		return Grant{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	if len(t.refresh) != 0 {
		if grant.RefreshToken, err = ParseSecret(t.refresh); err != nil {
			_ = grant.Close()
			return Grant{}, errors.Join(core.ErrTimeMatchResponse, err)
		}
	}
	if err := grant.Validate(); err != nil {
		_ = grant.Close()
		return Grant{}, errors.Join(core.ErrTimeMatchResponse, err)
	}
	return grant, nil
}

// grantedScopes requires every requested calendar scope in the token's
// space-delimited scope field. Microsoft may return Graph scopes without the
// resource prefix; offline_access is evidenced by the refresh token itself.
func grantedScopes(shape grantShape, raw string) bool {
	requested, err := scopes(shape.provider, shape.access, shape.lifetime)
	if err != nil {
		return false
	}
	for _, required := range requested {
		if required == microsoftScopeOffline || required == googleidentity.BrowserPhoneScope {
			continue
		}
		found := false
		for granted := range strings.FieldsSeq(raw) {
			if granted == required || shape.provider == ProviderMicrosoft && microsoftGraphScopeShort+granted == required {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type formPair struct {
	key   string
	value []byte
	omit  bool
}

// formBody encodes application/x-www-form-urlencoded bytes directly from the
// secret slices so no immutable string copy of a credential is created here.
func formBody(pairs ...formPair) []byte {
	const hex = "0123456789ABCDEF"
	body := make([]byte, 0, 256)
	for _, pair := range pairs {
		if pair.omit {
			continue
		}
		if len(body) != 0 {
			body = append(body, '&')
		}
		body = append(body, url.QueryEscape(pair.key)...)
		body = append(body, '=')
		for _, b := range pair.value {
			switch {
			case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '-', b == '.', b == '_', b == '~':
				body = append(body, b)
			case b == ' ':
				body = append(body, '+')
			default:
				body = append(body, '%', hex[b>>4], hex[b&0x0f])
			}
		}
	}
	return body
}
