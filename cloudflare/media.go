package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// MediaUpload borrows source and destination. Bytes is the exact file extent,
// excluding multipart framing. The response remains an unclassified provider
// stream; callers must decode it before claiming image/video readiness.
type MediaUpload struct {
	Source   io.Reader
	Response io.Writer
	Filename string
	Bytes    core.ByteLength
}

func (m MediaUpload) Validate() error {
	if core.ReaderIsNil(m.Source) || core.WriterIsNil(m.Response) || m.Filename == "" || len(m.Filename) > core.CloudflareMultipartFilenameMaximumBytes || strings.ContainsAny(m.Filename, "\x00\r\n/\\") {
		return core.ErrCloudflareContract
	}
	return m.Bytes.Validate()
}
func (m MediaUpload) validateMaximum(maximum uint64) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if m.Bytes.Uint64() > maximum {
		return core.ErrCloudflareContract
	}
	return nil
}

// Multipart framing is bounded by the filename and Go's boundary. Only the
// framing is retained; the file flows directly from its reader under backpressure.
func (source MediaUpload) transfer(ctx context.Context, client exchange.Client, target core.HTTPEndpoint, policy exchange.StreamPolicy) (exchange.StreamRoundTripResponse, error) {
	if err := validatePolicy(policy); err != nil {
		return exchange.StreamRoundTripResponse{}, err
	}
	head, tail, media, err := mediaFraming(source.Filename)
	if err != nil {
		return exchange.StreamRoundTripResponse{}, err
	}
	length, err := source.Bytes.Int64()
	if err != nil {
		return exchange.StreamRoundTripResponse{}, err
	}
	reader := &exactSource{source: source.Source, remaining: length}
	body := io.MultiReader(bytes.NewReader(head), reader, bytes.NewReader(tail))
	result, transferErr := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: client, Policy: policy,
		Request: exchange.StreamRoundTripRequest{Target: target, Source: body, Destination: source.Response,
			RequestContentType: media, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK(),
			Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}}})
	if transferErr == nil && !reader.complete.Load() {
		transferErr = core.ErrCloudflareResponse
	}
	return result, transferErr
}

func mediaFraming(filename string) ([]byte, []byte, core.HTTPMediaType, error) {
	var buffer bytes.Buffer
	form := multipart.NewWriter(&buffer)
	if _, err := form.CreateFormFile(core.CloudflareMultipartFileField, filename); err != nil {
		return nil, nil, core.HTTPMediaType{}, err
	}
	head := bytes.Clone(buffer.Bytes())
	buffer.Reset()
	if err := form.Close(); err != nil {
		return nil, nil, core.HTTPMediaType{}, err
	}
	media, err := core.ParseHTTPMediaType(form.FormDataContentType())
	return head, buffer.Bytes(), media, err
}

// exactSource preserves underflow, overflow and source errors. Its final EOF
// probe prevents an oversized file from being silently truncated into success.
type exactSource struct {
	source    io.Reader
	remaining int64
	complete  atomic.Bool
}

func (r *exactSource) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.complete.Load() {
		return 0, io.EOF
	}
	if r.remaining == 0 {
		return r.finish()
	}
	n, err := r.source.Read(p[:min(int64(len(p)), r.remaining)])
	if n < 0 || n > len(p) || int64(n) > r.remaining {
		return 0, core.ErrCloudflareContract
	}
	r.remaining -= int64(n)
	if errors.Is(err, io.EOF) {
		if r.remaining != 0 {
			return n, io.ErrUnexpectedEOF
		}
		r.complete.Store(true)
	}
	return n, err
}
func (r *exactSource) finish() (int, error) {
	var probe [1]byte
	n, err := r.source.Read(probe[:])
	if n != 0 {
		return 0, core.ErrCloudflareContract
	}
	if errors.Is(err, io.EOF) {
		r.complete.Store(true)
	}
	return 0, err
}
