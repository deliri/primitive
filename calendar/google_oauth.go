package calendar

import (
	"context"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
)

// BeginGoogleAuthorization requests calendar access only. It requests neither
// OpenID nor profile data and cannot authenticate an application account.
func BeginGoogleAuthorization(r AuthorizationRequest) (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	configuration := oauth2.Config{ClientID: r.ClientID.String(), RedirectURL: r.RedirectURI.String(), Endpoint: oauth2.Endpoint{AuthURL: core.CalendarGoogleAuthorizationEndpoint, TokenURL: core.CalendarGoogleTokenEndpoint, AuthStyle: oauth2.AuthStyleInParams}, Scopes: googleScopes(r.Access)}
	options := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("code_challenge", r.Challenge.String()), oauth2.SetAuthURLParam("code_challenge_method", "S256"), oauth2.SetAuthURLParam("include_granted_scopes", "false")}
	if r.Lifetime == LifetimeBusiness {
		options = append(options, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	}
	endpoint, err := core.ParseHTTPEndpoint(configuration.AuthCodeURL(r.State.String(), options...))
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrCalendarContract, err)
	}
	return endpoint, nil
}

// CompleteGoogleAuthorization consumes a code once through the existing
// official SDK transport. State/guest-cookie admission remains application
// policy. An unticked calendar scope never becomes an empty successful grant.
func CompleteGoogleAuthorization(ctx context.Context, r AuthorizationExchange) (Grant, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return Grant{}, errors.Join(core.ErrCalendarContract, err)
	}
	operation, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return Grant{}, err
	}
	defer cancel()
	attempt, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: operation, Duration: r.Policy.AttemptTimeout})
	if err != nil {
		return Grant{}, err
	}
	defer stop()
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationBinary})
	if err != nil {
		return Grant{}, err
	}
	transport, err := r.Client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return Grant{}, err
	}
	client, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return Grant{}, err
	}
	secret, err := r.ClientSecret.CopyBytes()
	if err != nil {
		return Grant{}, err
	}
	defer clear(secret)
	configuration := oauth2.Config{ClientID: r.ClientID.String(), ClientSecret: string(secret), RedirectURL: r.RedirectURI.String(), Endpoint: oauth2.Endpoint{TokenURL: core.CalendarGoogleTokenEndpoint, AuthStyle: oauth2.AuthStyleInParams}}
	token, err := configuration.Exchange(context.WithValue(attempt, oauth2.HTTPClient, client), r.Code.String(), oauth2.VerifierOption(r.Verifier.String()))
	if err != nil {
		return Grant{}, googleOAuthError(err)
	}
	defer func() { token.AccessToken = ""; token.RefreshToken = "" }()
	if token.TokenType != "Bearer" {
		return Grant{}, core.ErrCalendarContract
	}
	scope, ok := token.Extra("scope").(string)
	if !ok || !containsGoogleScopes(scope, r.Access) {
		return Grant{}, core.ErrCalendarConsentMissing
	}
	if r.Lifetime == LifetimeBusiness && token.RefreshToken == "" {
		return Grant{}, core.ErrCalendarConsentMissing
	}
	if r.Lifetime == LifetimeVisitor && token.RefreshToken != "" {
		return Grant{}, core.ErrCalendarContract
	}
	expires, err := temporal.NewInstant(token.Expiry)
	if err != nil {
		return Grant{}, errors.Join(core.ErrCalendarContract, err)
	}
	remaining, err := expires.Since(r.ObservedAt)
	if err != nil || remaining.IsZero() {
		return Grant{}, core.ErrCalendarContract
	}
	access, err := core.NewCalendarCredential(core.CalendarCredentialAccess, []byte(token.AccessToken))
	if err != nil {
		return Grant{}, err
	}
	grant := Grant{AccessToken: access, ExpiresAt: expires}
	if token.RefreshToken != "" {
		refresh, refreshErr := core.NewCalendarCredential(core.CalendarCredentialRefresh, []byte(token.RefreshToken))
		if refreshErr != nil {
			_ = access.Close()
			return Grant{}, refreshErr
		}
		grant.RefreshToken = refresh
	}
	if err := grant.Validate(); err != nil {
		_ = grant.Close()
		return Grant{}, err
	}
	return grant, nil
}
func googleScopes(access Access) []string {
	scopes := []string{core.CalendarGoogleBusyScope, core.CalendarGoogleListScope}
	if access == AccessManageEvents {
		scopes = append(scopes, core.CalendarGoogleWriteScope)
	}
	return scopes
}
func containsGoogleScopes(raw string, access Access) bool {
	if len(raw) > core.CalendarCredentialMaximumBytes {
		return false
	}
	for _, required := range googleScopes(access) {
		found := false
		for scope := range strings.FieldsSeq(raw) {
			if scope == required {
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
func googleOAuthError(err error) error {
	var refusal *oauth2.RetrieveError
	if errors.As(err, &refusal) && refusal.ErrorCode == "invalid_grant" {
		return errors.Join(core.ErrCalendarConsentRevoked, exchange.SanitizeOAuthTokenError(err))
	}
	return errors.Join(core.ErrCalendarContract, exchange.SanitizeOAuthTokenError(err))
}
