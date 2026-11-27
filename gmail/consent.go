package gmail

import (
	"context"
	"errors"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// ConsentRequest exchanges one code collected from Google Identity Services.
// The product authenticates the owner and verifies the browser origin before
// passing the code and its server-configured origin to Primitive.
// https://developers.google.com/identity/oauth2/web/guides/use-code-model
type ConsentRequest struct {
	Mailbox      core.OAuthEmail
	ClientID     core.OAuthClientID
	ClientSecret core.OAuthClientSecret
	Code         core.OAuthAuthorizationCode
	Origin       core.OAuthRedirectURI
	Policy       exchange.OperationPolicy
}

func (r ConsentRequest) Validate() error {
	if err := errors.Join(r.Mailbox.Validate(), r.ClientID.Validate(), r.ClientSecret.Validate(), r.Code.Validate(),
		r.Origin.Validate(), validatePolicy(r.Policy)); err != nil {
		return err
	}
	origin, err := url.Parse(r.Origin.String())
	if err != nil || origin == nil || origin.User != nil || origin.Path != "" || origin.RawQuery != "" ||
		origin.ForceQuery || origin.Fragment != "" || len(r.Code.String()) > credentialMaximumBytes {
		return core.ErrGmailContract
	}
	return nil
}

func (c Client) EnrollWorkspaceGrant(ctx context.Context, r ConsentRequest) (RefreshToken, error) {
	if err := c.admit(ctx, r); err != nil {
		return RefreshToken{}, err
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return RefreshToken{}, errors.Join(core.ErrGmailContract, err)
	}
	defer cancel()
	form, err := consentForm(r)
	if err != nil {
		return RefreshToken{}, err
	}
	wire, err := c.exchangeGrant(ctx, r.Policy, form)
	if err != nil {
		return RefreshToken{}, err
	}
	defer clear(wire)
	access, refresh, err := admitWorkspaceGrant(wire, r.Mailbox)
	if err != nil {
		return RefreshToken{}, err
	}
	binding := c.verifyMailbox(ctx, access.token, mailboxVerification{policy: r.Policy, mailbox: r.Mailbox})
	if err := errors.Join(binding, access.Close()); err != nil {
		return RefreshToken{}, errors.Join(err, refresh.Close())
	}
	return refresh, nil
}

func admitWorkspaceGrant(data []byte, mailbox core.OAuthEmail) (Access, RefreshToken, error) {
	var doc tokenWire
	defer doc.clear()
	if err := decodeTokenWire(data, &doc); err != nil {
		return Access{}, RefreshToken{}, err
	}
	if doc.RefreshExpires != 0 {
		return Access{}, RefreshToken{}, core.ErrGmailConsent
	}
	refresh, err := NewRefreshToken(doc.Refresh)
	if err != nil {
		return Access{}, RefreshToken{}, errors.Join(core.ErrGmailConsent, err)
	}
	access, err := doc.access(mailbox, GrantReadAndSend)
	if err != nil {
		return Access{}, RefreshToken{}, errors.Join(err, refresh.Close())
	}
	return access, refresh, nil
}

func consentForm(r ConsentRequest) ([]byte, error) {
	clientSecret, err := r.ClientSecret.CopyBytes()
	if err != nil {
		return nil, errors.Join(core.ErrGmailContract, err)
	}
	defer clear(clientSecret)
	if len(clientSecret) > credentialMaximumBytes {
		return nil, core.ErrGmailContract
	}
	prefix := "grant_type=authorization_code&client_id=" + url.QueryEscape(r.ClientID.String()) +
		"&redirect_uri=" + url.QueryEscape(r.Origin.String()) + "&client_secret="
	code := r.Code.String()
	form := make([]byte, 0, len(prefix)+len("&code=")+3*(len(clientSecret)+len(code)))
	form = append(form, prefix...)
	form = appendFormValue(form, clientSecret)
	form = append(form, "&code="...)
	return appendFormValue(form, []byte(code)), nil
}

func (ConsentRequest) gmailProtocolFact() {}
