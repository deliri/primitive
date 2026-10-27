package cloudbilling

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
	"github.com/deliri/primitive/v2026/exchange"
)

type ServerOptions struct {
	Endpoint core.HTTPEndpoint
	Currency currency.Code
	PageSize int32
	Limits   PageLimits
}

func (o ServerOptions) Validate() error {
	if err := errors.Join(o.Endpoint.Validate(), o.Currency.Validate(), o.Limits.Validate()); err != nil {
		return errors.Join(core.ErrCloudBillingContract, err)
	}
	u := o.Endpoint.HTTPURL()
	if o.PageSize <= 0 || u.RawQuery != "" || u.ForceQuery || !strings.HasPrefix(u.Path, "/v1/services/") || !strings.HasSuffix(u.Path, "/skus") {
		return core.ErrCloudBillingContract
	}
	service := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/v1/services/"), "/skus")
	if service == "" || strings.ContainsAny(service, "/\\") || u.RawPath != "" {
		return core.ErrCloudBillingContract
	}
	return nil
}

// Server retains a caller-owned Exchange capability and one service agreement.
// NewServer does not own or close the supplied client's transport.
type Server struct {
	client  exchange.Client
	options ServerOptions
}

func NewServer(client exchange.Client, options ServerOptions) (Server, error) {
	if err := errors.Join(client.Validate(), options.Validate()); err != nil {
		return Server{}, errors.Join(core.ErrCloudBillingContract, err)
	}
	return Server{client: client, options: options}, nil
}
func (s Server) Validate() error {
	if err := errors.Join(s.client.Validate(), s.options.Validate()); err != nil {
		return errors.Join(core.ErrCloudBillingContract, err)
	}
	return nil
}

type ListSKUsRequest struct {
	Token PageToken
	Proof io.Writer
}

// SourceReceipt binds the body bytes, excluding the single separating newline.
type SourceReceipt struct {
	URL    core.HTTPEndpoint
	SHA256 core.SHA256Digest
	Bytes  core.ByteLength
}
type PageReceipt struct {
	Page   Page
	Source SourceReceipt
}

func (s Server) ListSKUs(ctx context.Context, request ListSKUsRequest, policy exchange.StreamPolicy) (PageReceipt, error) {
	if err := errors.Join(contextstate.Validate(ctx), s.Validate(), policy.Validate()); err != nil {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingContract, err)
	}
	maximum, _ := s.options.Limits.TokenMaximumBytes.Uint64()
	if core.WriterIsNil(request.Proof) || !utf8.ValidString(string(request.Token)) || uint64(len(request.Token)) > maximum || policy.Redirect.Mode != exchange.RedirectReject {
		return PageReceipt{}, core.ErrCloudBillingContract
	}
	u := s.options.Endpoint.HTTPURL()
	query := u.Query()
	query.Set("currencyCode", s.options.Currency.String())
	query.Set("pageSize", strconv.FormatInt(int64(s.options.PageSize), 10))
	if request.Token != "" {
		query.Set("pageToken", string(request.Token))
	}
	u.RawQuery = query.Encode()
	target, err := core.ParseHTTPEndpoint(u.String())
	if err != nil {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingContract, err)
	}
	limit, _ := s.options.Limits.JSON.DocumentMaximumBytes.Int64()
	body := pageBuffer{maximum: limit}
	_, err = exchange.Download(exchange.DownloadCall{Context: ctx, Client: s.client, Request: exchange.DownloadRequest{Target: target, Destination: &body, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()}, Policy: policy})
	if err != nil {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingResponse, err)
	}
	// Retain original complete 200 bytes even if the provider decoder refuses
	// them. The caller's ordinary file remains the authoritative source proof.
	if n, err := request.Proof.Write(body.Bytes()); err != nil {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingResponse, err)
	} else if n != body.Len() {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingResponse, io.ErrShortWrite)
	}
	if n, err := io.WriteString(request.Proof, "\n"); err != nil {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingResponse, err)
	} else if n != 1 {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingResponse, io.ErrShortWrite)
	}
	page, err := DecodePage(body.Bytes(), s.options.Limits)
	if err != nil {
		return PageReceipt{}, err
	}
	extent, err := core.NewByteLength(uint64(body.Len()))
	if err != nil {
		return PageReceipt{}, errors.Join(core.ErrCloudBillingResponse, err)
	}
	return PageReceipt{Page: page, Source: SourceReceipt{URL: target, SHA256: core.SHA256Of(body.Bytes()), Bytes: extent}}, nil
}

// One aggregate page is explicitly caller bounded; memory grows only to its
// actual extent, never preallocated from the caller's numerical maximum.
type pageBuffer struct {
	bytes.Buffer
	maximum int64
}

func (b *pageBuffer) Write(data []byte) (int, error) {
	if int64(len(data)) > b.maximum-int64(b.Len()) {
		return 0, core.ErrCloudBillingResponse
	}
	return b.Buffer.Write(data)
}
