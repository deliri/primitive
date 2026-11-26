package gmail

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/jsonio"
)

const (
	mailboxEndpoint      = "https://gmail.googleapis.com/gmail/v1/users/me"
	threadListFields     = "threads(id),nextPageToken"
	threadMetadataFields = "id,messages(id,threadId,internalDate,labelIds,payload(headers(name,value)))"
	mimeLeafFields       = "mimeType,filename,body(data)"
	messageFields        = "id,threadId,labelIds,payload(" + mimeLeafFields + ",parts(" + mimeLeafFields + ",parts(" + mimeLeafFields + ",parts(" + mimeLeafFields + "))))"
	responseMaximumBytes = 256 * 1024
)

// Client borrows Exchange. Each call lends its own bound access and deadline.
type Client struct{ network exchange.Client }

func NewClient(network exchange.Client) (Client, error) {
	c := Client{network: network}
	if err := c.Validate(); err != nil {
		return Client{}, err
	}
	return c, nil
}

func (c Client) Validate() error {
	if err := c.network.Validate(); err != nil {
		return errors.Join(core.ErrGmailContract, err)
	}
	return nil
}

func (c Client) admit(ctx context.Context, request core.Validatable) error {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return errors.Join(core.ErrGmailContract, err)
	}
	return nil
}

func (c Client) ListThreads(ctx context.Context, r ThreadListRequest) (ThreadPage, error) {
	if err := c.admit(ctx, r); err != nil {
		return ThreadPage{}, err
	}
	path := mailboxEndpoint + "/threads?fields=" + url.QueryEscape(threadListFields) + "&labelIds=INBOX&maxResults=" + strconv.Itoa(int(r.Limit))
	if r.Filter == ThreadFilterUnread {
		path += "&labelIds=UNREAD"
	}
	if r.After != "" {
		path += "&pageToken=" + url.QueryEscape(string(r.After))
	}
	wire, err := readJSON[threadPageWire](ctx, c, r.Access.token, r.Policy, path)
	if err != nil {
		return ThreadPage{}, err
	}
	return wire.project(r)
}

func (c Client) ReadThread(ctx context.Context, r ThreadReadRequest) (Thread, error) {
	if err := c.admit(ctx, r); err != nil {
		return Thread{}, err
	}
	path := mailboxEndpoint + "/threads/" + string(r.Thread) + "?format=metadata&metadataHeaders=From&metadataHeaders=Subject&fields=" + url.QueryEscape(threadMetadataFields)
	wire, err := readJSON[threadWire](ctx, c, r.Access.token, r.Policy, path)
	if err != nil {
		return Thread{}, err
	}
	return wire.project(r.Thread)
}

func (c Client) ReadMessage(ctx context.Context, r MessageReadRequest) (Message, error) {
	if err := c.admit(ctx, r); err != nil {
		return Message{}, err
	}
	path := mailboxEndpoint + "/messages/" + string(r.Message) + "?format=full&fields=" + url.QueryEscape(messageFields)
	wire, err := readJSON[messageWire](ctx, c, r.Access.token, r.Policy, path)
	if err != nil {
		return Message{}, err
	}
	return wire.project(r)
}

func readJSON[T any](ctx context.Context, c Client, token core.OAuthClientSecret, policy exchange.OperationPolicy, path string) (T, error) {
	var zero T
	target, err := core.ParseHTTPEndpoint(path)
	if err != nil {
		return zero, errors.Join(core.ErrGmailContract, err)
	}
	name, err := core.ParseHTTPHeaderName("Authorization")
	if err != nil {
		return zero, errors.Join(core.ErrGmailContract, err)
	}
	bearer, err := token.CopyBytes()
	if err != nil {
		return zero, errors.Join(core.ErrGmailConsent, err)
	}
	defer clear(bearer)
	body := responseBuffer{limit: responseMaximumBytes}
	defer body.clear()
	response, err := exchange.Download(exchange.DownloadCall{
		Context: ctx, Client: c.network, Policy: streamPolicy(policy),
		Request: exchange.DownloadRequest{
			Target: target, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			SecretHeaders:  []exchange.SecretHeader{{Name: name, Value: bearer, Scheme: exchange.SecretHeaderBearer}},
			ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(),
			Destination: &body,
		},
	})
	if err != nil {
		return zero, responseFailure(response.Metadata, err)
	}
	return decodeProvider[T](body.data)
}

func streamPolicy(p exchange.OperationPolicy) exchange.StreamPolicy {
	return exchange.StreamPolicy{OperationTimeout: p.OperationTimeout, AttemptTimeout: p.AttemptTimeout, Redirect: p.Redirect}
}

type responseBuffer struct {
	data  []byte
	limit int
}

func (b *responseBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-len(b.data) {
		return 0, core.ErrExchangeBodyLimit
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *responseBuffer) clear() { clear(b.data) }

func decodeProvider[T any](data []byte) (T, error) {
	var zero T
	if len(data) > responseMaximumBytes {
		return zero, errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
	}
	value, err := jsonio.DecodeValue[T](data, jsonio.ProjectionReadPolicy{Members: jsonio.MembersIgnoreExtra, Root: jsonio.RootDeclaredFields, RootNulls: jsonio.RootNullRefused})
	if err != nil {
		// Provider syntax errors may include an offending field or value.
		return zero, errors.Join(core.ErrGmailResponse, core.ErrJSONContract)
	}
	return value, nil
}

func responseFailure(metadata exchange.ResponseMetadata, err error) error {
	code, _ := metadata.Status.Int()
	if code == http.StatusUnauthorized {
		return errors.Join(core.ErrGmailConsent, err)
	}
	return errors.Join(core.ErrGmailResponse, err)
}
