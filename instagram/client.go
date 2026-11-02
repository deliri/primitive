package instagram

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const authorizationHeader = "Authorization"

// Client borrows the caller's transport and holds one account credential. It
// owns no cache, schedule or retry policy beyond the caller's operation policy.
type Client struct {
	network    exchange.Client
	credential Credential
}

func NewClient(network exchange.Client, credential Credential) (Client, error) {
	client := Client{network: network, credential: credential}
	if err := client.Validate(); err != nil {
		return Client{}, err
	}
	return client, nil
}

func (c Client) Validate() error {
	if err := errors.Join(c.network.Validate(), c.credential.Validate()); err != nil {
		return errors.Join(core.ErrInstagramContract, err)
	}
	return nil
}

// ListMedia reads one page of the account's newest media. The token travels
// only as a redacted bearer header, never in the URL.
func (c Client) ListMedia(ctx context.Context, request MediaListRequest) (MediaPage, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return MediaPage{}, errors.Join(core.ErrInstagramContract, err)
	}
	target, err := request.Endpoint()
	if err != nil {
		return MediaPage{}, err
	}
	name, err := core.ParseHTTPHeaderName(authorizationHeader)
	if err != nil {
		return MediaPage{}, errors.Join(core.ErrInstagramContract, err)
	}
	response, err := exchange.SendNoBodyJSON[mediaListWire](exchange.NoBodyJSONCall{
		Context: ctx, Client: c.network,
		Request: exchange.NoBodyRequest{
			Target:         target,
			Semantics:      exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySafe},
			SecretHeaders:  []exchange.SecretHeader{{Name: name, Value: c.credential.value, Scheme: exchange.SecretHeaderBearer}},
			ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: exchange.NoBodyJSONPolicy{Operation: request.Policy},
	})
	if err != nil {
		return MediaPage{}, errors.Join(failureIdentity(response.Metadata.Status, response.FailureBody), err)
	}
	return response.Body.page(request.Limit)
}

// metaAccessTokenCode is Meta's OAuthException code for an expired, revoked
// or otherwise invalid access token.
// https://developers.facebook.com/docs/graph-api/guides/error-handling
const metaAccessTokenCode = 190

type metaErrorWire struct {
	Error metaErrorDetailWire `json:"error"`
}

type metaErrorDetailWire struct {
	Message     string `json:"message"`
	Type        string `json:"type"`
	UserTitle   string `json:"error_user_title"`
	UserMessage string `json:"error_user_msg"`
	TraceID     string `json:"fbtrace_id"`
	Code        int    `json:"code"`
	Subcode     int    `json:"error_subcode"`
	IsTransient bool   `json:"is_transient"`
}

func (w metaErrorWire) Validate() error { return nil }

// failureIdentity separates a token Meta refuses, which an owner must renew,
// from every other failure. An unreadable failure body is never authentication.
func failureIdentity(status core.HTTPStatusCode, body []byte) error {
	code, err := status.Int()
	if err == nil && (code == 401 || code == 403) {
		return core.ErrInstagramAuthentication
	}
	if len(body) == 0 {
		return core.ErrInstagramResponse
	}
	failure, err := core.DecodeStrictJSONBytes[metaErrorWire](body, core.ExtensibleJSONLimits())
	if err == nil && failure.Error.Code == metaAccessTokenCode {
		return core.ErrInstagramAuthentication
	}
	return core.ErrInstagramResponse
}
