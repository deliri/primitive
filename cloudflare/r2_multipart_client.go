package cloudflare

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type R2MultipartUpload struct {
	Bucket   R2Bucket
	Key      R2Key
	UploadID R2UploadID
}

func (u R2MultipartUpload) Validate() error {
	return errors.Join(u.Bucket.Validate(), u.Key.Validate(), u.UploadID.Validate())
}

// R2CompletedPart is a provider-returned part ETag, not a content digest.
type R2CompletedPart struct {
	ETag       string `xml:"ETag"`
	PartNumber uint16 `xml:"PartNumber"`
}

func (p R2CompletedPart) Validate() error {
	if p.PartNumber < 1 || p.PartNumber > core.CloudflareR2MultipartMaximumParts || !r2ETag(p.ETag) {
		return core.ErrCloudflareBinding
	}
	return nil
}
func r2ETag(value string) bool {
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return false
	}
	for _, r := range value[1 : len(value)-1] {
		if r < 0x21 || r > 0x7e || r == '"' {
			return false
		}
	}
	return true
}

type R2MultipartResult struct {
	Bucket            R2Bucket
	Key               R2Key
	ETag              string
	ChecksumCRC64NVME string
}

func (r R2MultipartResult) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Key.Validate()); err != nil {
		return responseError(err)
	}
	if !r2ETag(r.ETag) {
		return core.ErrCloudflareResponse
	}
	if r.ChecksumCRC64NVME != "" && !r2CRC64(r.ChecksumCRC64NVME) {
		return core.ErrCloudflareResponse
	}
	return nil
}

func (c R2Client) CreateMultipart(ctx context.Context, g R2MultipartGrant, policy exchange.StreamPolicy) (R2MultipartUpload, error) {
	if g.intent.Action != R2MultipartCreate {
		return R2MultipartUpload{}, core.ErrCloudflareBinding
	}
	length, err := core.NewByteLength(0)
	if err != nil {
		return R2MultipartUpload{}, err
	}
	fields, err := c.multipartControl(ctx, g, strings.NewReader(""), &length, g.intent.ContentType, policy)
	if err != nil {
		return R2MultipartUpload{}, err
	}
	if fields.Bucket != g.intent.Bucket.value || fields.Key != g.intent.Key.value || fields.ETag != "" || fields.Location != "" {
		return R2MultipartUpload{}, core.ErrCloudflareResponse
	}
	id, err := ParseR2UploadID(fields.UploadID)
	if err != nil {
		return R2MultipartUpload{}, responseError(err)
	}
	return R2MultipartUpload{Bucket: g.intent.Bucket, Key: g.intent.Key, UploadID: id}, nil
}

// UploadPart transfers a single caller-owned source exactly once. The browser
// normally spends Endpoint() directly; this client is also usable by native
// clients and behavioral probes. An uncertain transfer is never auto-replayed.
func (c R2Client) UploadPart(ctx context.Context, g R2MultipartGrant, source io.Reader, length core.ByteLength, policy exchange.StreamPolicy) (R2CompletedPart, error) {
	if err := errors.Join(c.Validate(), g.Validate(), length.Validate(), validatePolicy(policy)); err != nil {
		return R2CompletedPart{}, err
	}
	if g.intent.Action != R2MultipartPart || core.ReaderIsNil(source) || length.Uint64() == 0 || length.Uint64() > core.CloudflareR2MultipartMaximumPartBytes {
		return R2CompletedPart{}, core.ErrCloudflareBinding
	}
	response, err := exchange.Upload(exchange.UploadCall{Context: ctx, Client: c.client, Policy: policy, Request: exchange.UploadRequest{
		Target: g.endpoint, Source: source, ContentLength: &length, ContentType: core.HTTPMediaTypeOctetStream(),
		CaptureHeaders: exchange.HeaderSelection{Names: []core.HTTPHeaderName{cloudflareHeader(core.CloudflareR2ETagHeader)}},
		ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPut, Replay: exchange.ReplaySingleAttempt},
	}})
	if err != nil {
		return R2CompletedPart{}, err
	}
	headers := response.Metadata.Headers
	if len(headers.Values) != 1 || len(headers.Values[0].Values) != 1 || headers.Values[0].Name != cloudflareHeader(core.CloudflareR2ETagHeader) {
		return R2CompletedPart{}, core.ErrCloudflareResponse
	}
	etag, err := headers.Values[0].Values[0].Value()
	if err != nil {
		return R2CompletedPart{}, responseError(err)
	}
	part := R2CompletedPart{PartNumber: g.intent.PartNumber, ETag: etag}
	if err := part.Validate(); err != nil {
		return R2CompletedPart{}, responseError(err)
	}
	return part, nil
}

// R2CompletedParts visits the actual UploadPart receipts in provider order.
// It must call yield synchronously, stop on its first error, and honor ctx.
// The source owns its storage and cleanup; the SDK retains only the current part.
type R2CompletedParts func(ctx context.Context, yield func(R2CompletedPart) error) error

func (c R2Client) CompleteMultipart(ctx context.Context, g R2MultipartGrant, parts R2CompletedParts, policy exchange.StreamPolicy) (R2MultipartResult, error) {
	if err := errors.Join(c.Validate(), g.Validate(), validatePolicy(policy), contextstate.Validate(ctx)); err != nil {
		return R2MultipartResult{}, err
	}
	if g.intent.Action != R2MultipartComplete || parts == nil {
		return R2MultipartResult{}, core.ErrCloudflareBinding
	}
	media, err := core.ParseHTTPMediaType(core.CloudflareR2XMLMediaType)
	if err != nil {
		return R2MultipartResult{}, err
	}
	fields, err := c.multipartCompletionControl(ctx, g, parts, media, policy)
	if err != nil {
		return R2MultipartResult{}, err
	}
	if fields.Bucket != g.intent.Bucket.value || fields.Key != g.intent.Key.value || fields.UploadID != "" {
		return R2MultipartResult{}, core.ErrCloudflareResponse
	}
	result := R2MultipartResult{Bucket: g.intent.Bucket, Key: g.intent.Key, ETag: fields.ETag, ChecksumCRC64NVME: fields.ChecksumCRC64NVME}
	if err := result.Validate(); err != nil {
		return R2MultipartResult{}, err
	}
	return result, nil
}

func (c R2Client) AbortMultipart(ctx context.Context, g R2MultipartGrant, policy exchange.StreamPolicy) error {
	if err := errors.Join(c.Validate(), g.Validate(), validatePolicy(policy)); err != nil {
		return err
	}
	if g.intent.Action != R2MultipartAbort {
		return core.ErrCloudflareBinding
	}
	var status core.HTTPStatusCode
	if err := status.AdmitInt(204); err != nil {
		return err
	}
	_, err := exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.client, Policy: policy, Request: exchange.DownloadRequest{
		Target: g.endpoint, Destination: io.Discard, ExpectedStatus: status, Semantics: exchange.RequestSemantics{Method: exchange.MethodDelete, Replay: exchange.ReplaySingleAttempt},
	}})
	return err
}

func (c R2Client) multipartControl(ctx context.Context, g R2MultipartGrant, source io.Reader, length *core.ByteLength, media core.HTTPMediaType, policy exchange.StreamPolicy) (r2MultipartFields, error) {
	if err := errors.Join(c.Validate(), g.Validate(), validatePolicy(policy)); err != nil {
		return r2MultipartFields{}, err
	}
	root := "CompleteMultipartUploadResult"
	if g.intent.Action == R2MultipartCreate {
		root = "InitiateMultipartUploadResult"
	}
	reader, writer := io.Pipe()
	finished := make(chan r2MultipartDecoded, 1)
	go func() {
		fields, err := decodeR2MultipartXML(reader, root)
		finished <- r2MultipartDecoded{fields: fields, err: errors.Join(err, reader.CloseWithError(err))}
	}()
	_, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.client, Policy: policy, Request: exchange.StreamRoundTripRequest{
		Target: g.endpoint, Source: source, Destination: writer, RequestContentLength: length, RequestContentType: media, Headers: g.intent.CacheControl.headers(),
		ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
	}})
	err = errors.Join(err, writer.CloseWithError(err))
	decoded := <-finished
	err = errors.Join(err, decoded.err)
	if err != nil {
		return r2MultipartFields{}, err
	}
	return decoded.fields, nil
}

type r2MultipartDecoded struct {
	err    error
	fields r2MultipartFields
}

// Control responses admit one flat, exact schema. encoding/xml performs XML
// tokenization; this boundary refuses duplicates, nested values, unknown fields,
// directives, crossed coordinates and a successful HTTP envelope carrying Error.
type r2MultipartFields struct{ Bucket, Key, UploadID, ETag, Location, ChecksumCRC64NVME string }

// R2 documents FULL_OBJECT CRC64NVME. This is the provider's checksum fact;
// it is not silently substituted for the caller's SHA-256 or BLAKE3 identity.
// https://developers.cloudflare.com/r2/api/s3/api/#checksum-types
func r2CRC64(value string) bool {
	if len(value) != 12 {
		return false
	}
	var decoded [9]byte
	n, err := base64.StdEncoding.Decode(decoded[:], []byte(value))
	return err == nil && n == 8 && base64.StdEncoding.EncodeToString(decoded[:n]) == value
}

func decodeR2MultipartXML(source io.Reader, root string) (r2MultipartFields, error) {
	decoder := xml.NewDecoder(source)
	var result r2MultipartFields
	var seen uint8
	depth := 0
	finished := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if !finished || root == "InitiateMultipartUploadResult" && seen != 7 || root == "CompleteMultipartUploadResult" && (seen&11 != 11 || seen&4 != 0) {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
			return result, nil
		}
		if err != nil {
			return r2MultipartFields{}, responseError(err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if finished || !r2XMLNamespace(t.Name.Space) {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
			if depth == 0 {
				if t.Name.Local != root || !r2XMLRootAttributes(t.Attr) {
					return r2MultipartFields{}, core.ErrCloudflareResponse
				}
				depth = 1
				continue
			}
			if len(t.Attr) != 0 {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
			bit := r2MultipartFieldBit(t.Name.Local)
			if bit == 0 || seen&bit != 0 {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
			value, err := r2XMLText(decoder, t.Name)
			if err != nil {
				return r2MultipartFields{}, err
			}
			seen |= bit
			switch t.Name.Local {
			case "Bucket":
				result.Bucket = value
			case "Key":
				result.Key = value
			case "UploadId":
				result.UploadID = value
			case "ETag":
				result.ETag = value
			case "Location":
				result.Location = value
			case "ChecksumCRC64NVME":
				if !r2CRC64(value) {
					return r2MultipartFields{}, core.ErrCloudflareResponse
				}
				result.ChecksumCRC64NVME = value
			default:
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
		case xml.EndElement:
			if depth != 1 || t.Name.Local != root {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
			depth = 0
			finished = true
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
		case xml.ProcInst:
			if depth != 0 || finished || t.Target != "xml" {
				return r2MultipartFields{}, core.ErrCloudflareResponse
			}
		case xml.Comment:
		default:
			return r2MultipartFields{}, core.ErrCloudflareResponse
		}
	}
}
func r2XMLNamespace(value string) bool { return value == "" || value == core.CloudflareR2XMLNamespace }
func r2XMLRootAttributes(attrs []xml.Attr) bool {
	return len(attrs) == 0 || len(attrs) == 1 && attrs[0].Name.Local == "xmlns" && attrs[0].Name.Space == "" && attrs[0].Value == core.CloudflareR2XMLNamespace
}
func r2MultipartFieldBit(name string) uint8 {
	switch name {
	case "Bucket":
		return 1
	case "Key":
		return 2
	case "UploadId":
		return 4
	case "ETag":
		return 8
	case "Location":
		return 16
	case "ChecksumCRC64NVME":
		return 32
	default:
		return 0
	}
}
func r2XMLText(decoder *xml.Decoder, name xml.Name) (string, error) {
	var value strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", responseError(err)
		}
		switch t := token.(type) {
		case xml.CharData:
			value.Write(t)
		case xml.EndElement:
			if t.Name == name {
				return value.String(), nil
			}
			return "", core.ErrCloudflareResponse
		default:
			return "", core.ErrCloudflareResponse
		}
	}
}

func (c R2Client) multipartCompletionControl(ctx context.Context, g R2MultipartGrant, parts R2CompletedParts, media core.HTTPMediaType, policy exchange.StreamPolicy) (r2MultipartFields, error) {
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: policy.OperationTimeout})
	if err != nil {
		return r2MultipartFields{}, err
	}
	defer cancel()
	reader, writer := io.Pipe()
	interrupted := make(chan error, 1)
	stopInterrupt := context.AfterFunc(ctx, func() {
		interrupted <- reader.CloseWithError(ctx.Err())
	})
	defer func() {
		if !stopInterrupt() {
			// io.PipeReader.CloseWithError always returns nil. Receiving still
			// joins the callback before this scope releases the pipe.
			<-interrupted
		}
	}()
	finished := make(chan error, 1)
	go func() {
		err := writeMultipartCompletion(ctx, writer, parts)
		finished <- errors.Join(err, writer.CloseWithError(err))
	}()
	document, err := c.multipartControl(ctx, g, reader, nil, media, policy)
	// Close wakes a producer blocked by peer refusal; cancellation also reaches
	// a caller-owned source. Join before returning custody to the caller.
	cancel()
	err = errors.Join(err, reader.CloseWithError(err), <-finished)
	if err != nil {
		return r2MultipartFields{}, err
	}
	return document, nil
}

func writeMultipartCompletion(ctx context.Context, destination io.Writer, parts R2CompletedParts) error {
	encoder := xml.NewEncoder(destination)
	root := xml.StartElement{Name: xml.Name{Local: core.CloudflareR2MultipartCompletionElement}}
	if err := encoder.EncodeToken(root); err != nil {
		return err
	}
	previous := uint16(0)
	var emitErr error
	err := parts(ctx, func(part R2CompletedPart) error {
		if emitErr != nil {
			return emitErr
		}
		previous, emitErr = writeR2CompletedPart(ctx, encoder, part, previous)
		return emitErr
	})
	err = errors.Join(err, emitErr, contextstate.Validate(ctx))
	if err != nil {
		return err
	}
	if previous == 0 {
		return core.ErrCloudflareBinding
	}
	if err := encoder.EncodeToken(root.End()); err != nil {
		return err
	}
	return encoder.Close()
}

func writeR2CompletedPart(ctx context.Context, encoder *xml.Encoder, part R2CompletedPart, previous uint16) (uint16, error) {
	if err := errors.Join(contextstate.Validate(ctx), part.Validate()); err != nil {
		return previous, err
	}
	if part.PartNumber <= previous {
		return previous, core.ErrCloudflareBinding
	}
	// EncodeElement flushes the completed element through Go's writer, so the
	// producer cannot advance by buffering an unbounded sequence of receipts.
	if err := encoder.EncodeElement(part, xml.StartElement{Name: xml.Name{Local: core.CloudflareR2MultipartPartElement}}); err != nil {
		return previous, err
	}
	return part.PartNumber, nil
}

func (R2MultipartUpload) cloudflareProtocolFact()  {}
func (R2CompletedPart) cloudflareProtocolFact()    {}
func (R2MultipartResult) cloudflareProtocolFact()  {}
func (r2MultipartFields) cloudflareInternalFlow()  {}
func (r2MultipartDecoded) cloudflareInternalFlow() {}
