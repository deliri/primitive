package gmail

import (
	"bytes"
	"context"
	jsontext "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	tokenEndpoint          = "https://oauth2.googleapis.com/token"
	credentialMaximumBytes = 4096
	tokenResponseMaximum   = 16 * 1024
	ReadOnlyScope          = "https://www.googleapis.com/auth/gmail.readonly"
)

// RefreshToken is distinct from a bearer access credential. Copies share
// destruction. The caller owns closing the token supplied to Refresh.
type RefreshToken struct{ value core.OAuthClientSecret }

func NewRefreshToken(value []byte) (RefreshToken, error) {
	if len(value) > credentialMaximumBytes {
		return RefreshToken{}, core.ErrGmailContract
	}
	secret, err := core.NewOAuthClientSecret(value)
	if err != nil {
		return RefreshToken{}, errors.Join(core.ErrGmailContract, err)
	}
	return RefreshToken{value: secret}, nil
}

func (t RefreshToken) Validate() error { return t.value.Validate() }
func (t *RefreshToken) Close() error {
	if t == nil {
		return core.ErrGmailContract
	}
	return t.value.Close()
}
func (RefreshToken) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, core.RedactedValueText)
}

func (RefreshToken) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(core.ErrGmailConsent, core.ErrJSONContract)
}

type RefreshRequest struct {
	Mailbox      core.OAuthEmail
	ClientID     core.OAuthClientID
	ClientSecret core.OAuthClientSecret
	RefreshToken RefreshToken
	Policy       exchange.OperationPolicy
}

func (r RefreshRequest) Validate() error {
	return errors.Join(r.Mailbox.Validate(), r.ClientID.Validate(), r.ClientSecret.Validate(), r.RefreshToken.Validate(), validatePolicy(r.Policy))
}

// Access is an account-bound Gmail bearer. Refresh and Delegate are its issuing capabilities.
// A copied handle shares its secret lifetime, and formatting always redacts it.
type Access struct {
	mailbox   core.OAuthEmail
	token     core.OAuthClientSecret
	lifetime  temporal.Duration
	delegated *delegatedMailbox
}

func (a Access) Mailbox() core.OAuthEmail { return a.mailbox }

// Lifetime is the acquired bearer lifetime. A fresh delegated handle has not
// acquired a bearer and reports zero until its first successful renewal.
func (a Access) Lifetime() temporal.Duration {
	if a.delegated != nil {
		a.delegated.mu.Lock()
		defer a.delegated.mu.Unlock()
		return a.delegated.current.Lifetime()
	}
	return a.lifetime
}
func (a Access) Validate() error {
	if a.delegated != nil {
		return errors.Join(a.mailbox.Validate(), a.delegated.validate())
	}
	if a.lifetime.IsZero() {
		return core.ErrGmailConsent
	}
	return errors.Join(a.mailbox.Validate(), a.token.Validate(), a.lifetime.Validate())
}

func (a *Access) Close() error {
	if a == nil {
		return core.ErrGmailConsent
	}
	if a.delegated != nil {
		return a.delegated.close()
	}
	return a.token.Close()
}
func (Access) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, core.RedactedValueText)
}

func (Access) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(core.ErrGmailConsent, core.ErrJSONContract)
}

// Google’s refresh endpoint returns the access token, bearer type, lifetime
// and granted scopes. Only the exact read scope is admitted here.
// https://developers.google.com/identity/protocols/oauth2/web-server#offline
type tokenWire struct {
	Token   secretWire    `json:"access_token"`
	Type    tokenTypeWire `json:"token_type"`
	Expires uint64        `json:"expires_in"`
	Scope   scopeWire     `json:"scope"`
}

type oauthErrorWire struct {
	Code oauthErrorCodeWire `json:"error"`
}

type mailboxWire struct {
	Email mailAddressWire `json:"emailAddress"`
}

type secretWire []byte
type tokenTypeWire string
type scopeWire string
type oauthErrorCodeWire string
type mailAddressWire string

func (s *secretWire) UnmarshalJSON(data []byte) error {
	value, err := jsontext.AppendUnquote(nil, data)
	if err != nil {
		return core.ErrGmailResponse
	}
	*s = value
	return nil
}

// Refresh exchanges an offline grant for one short-lived bearer, then
// verifies the Google profile. No credential escapes a failed identity check.
func (c Client) Refresh(ctx context.Context, r RefreshRequest) (Access, error) {
	if err := c.admit(ctx, r); err != nil {
		return Access{}, err
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return Access{}, errors.Join(core.ErrGmailContract, err)
	}
	defer cancel()
	access, err := c.refreshToken(ctx, r)
	if err != nil {
		return Access{}, err
	}
	if err := c.verifyMailbox(ctx, access.token, mailboxVerification{policy: r.Policy, mailbox: r.Mailbox}); err != nil {
		return Access{}, errors.Join(err, access.Close())
	}
	return access, nil
}

type mailboxVerification struct {
	policy  exchange.OperationPolicy
	mailbox core.OAuthEmail
}

func (mailboxVerification) gmailInternalFlow() {}

func (c Client) verifyMailbox(ctx context.Context, token core.OAuthClientSecret, request mailboxVerification) error {
	profile, err := readJSON[mailboxWire](ctx, token, mailboxRead{client: c, policy: request.policy, path: mailboxEndpoint + "/profile?fields=emailAddress"})
	if err != nil {
		return err
	}
	got, err := core.ParseOAuthEmail(string(profile.Email))
	if err != nil || !strings.EqualFold(got.String(), request.mailbox.String()) {
		return core.ErrGmailBinding
	}
	return nil
}

func (c Client) refreshToken(ctx context.Context, r RefreshRequest) (Access, error) {
	form, err := refreshForm(r)
	if err != nil {
		return Access{}, err
	}
	defer clear(form)
	target, err := core.ParseHTTPEndpoint(tokenEndpoint)
	if err != nil {
		return Access{}, errors.Join(core.ErrGmailContract, err)
	}
	body := responseBuffer{data: make([]byte, 0, tokenResponseMaximum), limit: tokenResponseMaximum}
	faultBody := responseBuffer{data: make([]byte, 0, tokenResponseMaximum), limit: tokenResponseMaximum}
	defer body.clear()
	defer faultBody.clear()
	length, err := core.NewByteLength(uint64(len(form)))
	if err != nil {
		return Access{}, errors.Join(core.ErrGmailContract, err)
	}
	response, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{
		Context: ctx, Client: c.network, Policy: streamPolicy(r.Policy),
		Request: exchange.StreamRoundTripRequest{
			Target: target, Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
			Source: bytes.NewReader(form), RequestContentLength: &length, Destination: &body, FailureDestination: &faultBody,
			RequestContentType: core.HTTPMediaTypeFormURLEncoded(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(),
			ExpectedStatus: core.HTTPStatusOK(),
		},
	})
	if err != nil {
		status, _ := response.Metadata.Status.Int()
		if status == http.StatusBadRequest {
			fault, decodeErr := decodeProvider[oauthErrorWire](faultBody.data)
			if decodeErr == nil && fault.Code == "invalid_grant" {
				return Access{}, errors.Join(core.ErrGmailConsent, err)
			}
		}
		return Access{}, responseFailure(response.Metadata, err)
	}
	return admitAccess(body.data, r.Mailbox)
}

func admitAccess(data []byte, mailbox core.OAuthEmail) (Access, error) {
	if len(data) > tokenResponseMaximum {
		return Access{}, errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
	}
	if err := mailbox.Validate(); err != nil {
		return Access{}, errors.Join(core.ErrGmailBinding, err)
	}
	var doc tokenWire
	defer func() { clear(doc.Token) }()
	if err := jsonio.ValidateBytes(data); err != nil {
		return Access{}, errors.Join(core.ErrGmailResponse, core.ErrJSONContract)
	}
	if err := jsonv2.Unmarshal(data, &doc); err != nil {
		return Access{}, errors.Join(core.ErrGmailResponse, core.ErrJSONContract)
	}
	if doc.Scope != ReadOnlyScope {
		return Access{}, core.ErrGmailConsent
	}
	if len(doc.Token) == 0 || len(doc.Token) > credentialMaximumBytes {
		return Access{}, core.ErrGmailResponse
	}
	if !strings.EqualFold(string(doc.Type), "Bearer") {
		return Access{}, core.ErrGmailConsent
	}
	lifetime, err := temporal.DurationFromSeconds(doc.Expires)
	if err != nil || lifetime.IsZero() {
		return Access{}, errors.Join(core.ErrGmailResponse, err)
	}
	if err := (exchange.BearerAuthorization{Token: doc.Token}).Validate(); err != nil {
		return Access{}, core.ErrGmailResponse
	}
	token, err := core.NewOAuthClientSecret(doc.Token)
	if err != nil {
		return Access{}, errors.Join(core.ErrGmailResponse, err)
	}
	return Access{mailbox: mailbox, token: token, lifetime: lifetime}, nil
}

func refreshForm(r RefreshRequest) ([]byte, error) {
	clientSecret, err := r.ClientSecret.CopyBytes()
	if err != nil {
		return nil, errors.Join(core.ErrGmailContract, err)
	}
	defer clear(clientSecret)
	if len(clientSecret) > credentialMaximumBytes {
		return nil, core.ErrGmailContract
	}
	refresh, err := r.RefreshToken.value.CopyBytes()
	if err != nil {
		return nil, errors.Join(core.ErrGmailContract, err)
	}
	defer clear(refresh)
	prefix := "grant_type=refresh_token&client_id=" + url.QueryEscape(r.ClientID.String()) + "&client_secret="
	form := make([]byte, 0, len(prefix)+len("&refresh_token=")+3*(len(clientSecret)+len(refresh)))
	form = append(form, prefix...)
	form = appendFormValue(form, clientSecret)
	form = append(form, "&refresh_token="...)
	return appendFormValue(form, refresh), nil
}

// appendFormValue encodes confidential bytes without making immutable string
// copies. The form is allocated for the worst case and erased by its caller.
func appendFormValue(out, value []byte) []byte {
	const hex = "0123456789ABCDEF"
	for _, c := range value {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			out = append(out, c)
		} else {
			out = append(out, '%', hex[c>>4], hex[c&15])
		}
	}
	return out
}
