package googleidentity

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/oauth2"
)

// BrowserProfileReadRequest borrows an already acquired access credential.
// Subject must come from the provider's authenticated identity response. This
// operation reads selected profile observations; it never exchanges a code.
type BrowserProfileReadRequest struct {
	Client      exchange.Client
	AccessToken exchange.SecretHeader
	Subject     core.OAuthSubject
	Profile     BrowserProfileRequest
	Policy      exchange.OperationPolicy
}

func (r BrowserProfileReadRequest) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.AccessToken.Validate(), r.Subject.Validate(), r.Profile.Validate(), r.Policy.Validate()); err != nil {
		return contractError(err)
	}
	if r.AccessToken.Scheme != exchange.SecretHeaderBearer || r.Policy.Retry.MaximumAttempts != 1 || r.Policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrGoogleIdentityContract
	}
	return nil
}

func ReadBrowserProfile(ctx context.Context, r BrowserProfileReadRequest) (BrowserProfile, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return BrowserProfile{}, contractError(err)
	}
	operation, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return BrowserProfile{}, contractError(err)
	}
	defer cancel()
	token := &oauth2.Token{AccessToken: string(r.AccessToken.Value), TokenType: "Bearer"}
	defer func() { token.AccessToken = "" }()
	return browserPeopleProfile(operation, BrowserAuthorizationCompleteRequest{Client: r.Client, Profile: r.Profile}, browserProfileCredential{Token: token, Subject: r.Subject})
}

func (BrowserProfileReadRequest) googleIdentityProtocolFact() {}
