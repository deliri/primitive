package cloudflare

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// R2WriteConditions are signed, provider-enforced conditions. ContentMD5 is
// transport integrity, not a cryptographic identity or an authentication key.
// CreateOnly prevents a still-valid grant from overwriting an existing object.
// https://developers.cloudflare.com/r2/api/s3/api/
type R2WriteConditions struct {
	ContentMD5 string
	CreateOnly bool
}

func (c R2WriteConditions) Validate() error {
	if c.ContentMD5 != "" {
		if len(c.ContentMD5) != 24 {
			return core.ErrCloudflareBinding
		}
		var decoded [18]byte
		n, err := base64.StdEncoding.Decode(decoded[:], []byte(c.ContentMD5))
		if err != nil || n != 16 || base64.StdEncoding.EncodeToString(decoded[:n]) != c.ContentMD5 {
			return core.ErrCloudflareBinding
		}
	}
	return nil
}

func (c R2WriteConditions) validateMethod(method exchange.Method) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if method != exchange.MethodPut && (c.CreateOnly || c.ContentMD5 != "") {
		return core.ErrCloudflareBinding
	}
	return nil
}

func (c R2WriteConditions) headers() exchange.Headers {
	var headers exchange.Headers
	if c.ContentMD5 != "" {
		headers.Values = append(headers.Values, cloudflareRequestHeader(core.CloudflareR2ContentMD5Header, c.ContentMD5))
	}
	if c.CreateOnly {
		headers.Values = append(headers.Values, cloudflareRequestHeader(core.CloudflareR2IfNoneMatchHeader, core.CloudflareR2CreateOnlyValue))
	}
	return headers
}

func cloudflareRequestHeader(name, value string) exchange.Header {
	field, err := exchange.NewHeaderValue(value)
	if err != nil {
		panic(err)
	}
	return exchange.Header{Name: cloudflareHeader(name), Values: []exchange.HeaderValue{field}}
}

func cloudflareHeader(value string) core.HTTPHeaderName {
	// This is exclusively called with compiler-owned constant header names.
	name, err := core.ParseHTTPHeaderName(value)
	if err != nil {
		panic(err)
	}
	return name
}

// Headers projects the exact signed request fields for the governed upload
// transport. Content-Type is independently carried by the grant.
func (g R2Grant) Headers() (exchange.Headers, error) {
	if err := g.Validate(); err != nil {
		return exchange.Headers{}, err
	}
	return r2WriteHeaders(g.conditions, g.cacheControl), nil
}

// R2ObjectMetadata contains only facts independently returned by HEAD. ETag is
// opaque: multipart ETags must never be reinterpreted as a content checksum.
type R2ObjectMetadata struct {
	ContentType core.HTTPMediaType
	ETag        string
	Bytes       core.ByteLength
}

func (m R2ObjectMetadata) Validate() error {
	if err := errors.Join(m.ContentType.Validate(), m.Bytes.Validate()); err != nil {
		return responseError(err)
	}
	if len(m.ETag) < 3 || m.ETag[0] != '"' || m.ETag[len(m.ETag)-1] != '"' || strings.ContainsAny(m.ETag[1:len(m.ETag)-1], "\"\r\n") {
		return core.ErrCloudflareResponse
	}
	return nil
}

// Head verifies one exact object using a HEAD grant. No media bytes traverse
// the application server and no listing or history query is performed.
func (c R2Client) Head(ctx context.Context, grant R2Grant, policy exchange.StreamPolicy) (R2ObjectMetadata, error) {
	if err := errors.Join(c.Validate(), grant.Validate(), validatePolicy(policy)); err != nil {
		return R2ObjectMetadata{}, err
	}
	if grant.method != exchange.MethodHead {
		return R2ObjectMetadata{}, core.ErrCloudflareBinding
	}
	selection := exchange.HeaderSelection{Names: []core.HTTPHeaderName{core.HTTPHeaderContentType(), core.HTTPHeaderContentLength(), cloudflareHeader(core.CloudflareR2ETagHeader)}}
	response, err := exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.client, Policy: policy, Request: exchange.DownloadRequest{Target: grant.endpoint, Destination: io.Discard, CaptureHeaders: selection, ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodHead, Replay: exchange.ReplaySingleAttempt}}})
	if err != nil {
		return R2ObjectMetadata{}, err
	}
	return r2Metadata(response.Metadata.Headers)
}

func r2Metadata(headers exchange.CapturedHeaders) (R2ObjectMetadata, error) {
	if err := headers.Validate(); err != nil || len(headers.Values) != 3 {
		return R2ObjectMetadata{}, responseError(err)
	}
	var result R2ObjectMetadata
	var err error
	for _, h := range headers.Values {
		if len(h.Values) != 1 {
			return R2ObjectMetadata{}, core.ErrCloudflareResponse
		}
		value, valueErr := h.Values[0].Value()
		if valueErr != nil {
			return R2ObjectMetadata{}, responseError(valueErr)
		}
		switch h.Name {
		case core.HTTPHeaderContentType():
			result.ContentType, err = core.ParseHTTPMediaType(value)
		case core.HTTPHeaderContentLength():
			var length uint64
			length, err = strconv.ParseUint(value, 10, 64)
			if err == nil {
				result.Bytes, err = core.NewByteLength(length)
			}
		case cloudflareHeader(core.CloudflareR2ETagHeader):
			result.ETag = value
		default:
			return R2ObjectMetadata{}, core.ErrCloudflareResponse
		}
		if err != nil {
			return R2ObjectMetadata{}, responseError(err)
		}
	}
	if err := result.Validate(); err != nil {
		return R2ObjectMetadata{}, err
	}
	return result, nil
}

func (R2WriteConditions) cloudflareProtocolFact() {}
func (R2ObjectMetadata) cloudflareProtocolFact()  {}
