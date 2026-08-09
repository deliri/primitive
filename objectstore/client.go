package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const cloudflareImagesFormField = "file"

// Client is an immutable capability over one caller-owned Exchange client.
type Client struct {
	exchange exchange.Client
}

// NewClient constructs one Objectstore client.
func NewClient(client exchange.Client) (Client, error) {
	value := Client{exchange: client}
	if err := value.Validate(); err != nil {
		return Client{}, err
	}
	return value, nil
}

// Validate rejects an unset Exchange capability.
func (c Client) Validate() error {
	if err := c.exchange.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// Transfer is immutable evidence from one attempted provider operation.
type Transfer struct {
	version    ProviderVersion
	bytes      core.ByteLength
	crc32c     core.CRC32C
	status     core.HTTPStatusCode
	sha256     core.SHA256Digest
	provider   Provider
	direction  Direction
	commitment Commitment
}

// Provider returns the selected vendor.
func (t Transfer) Provider() Provider { return t.provider }

// Direction returns the attempted operation.
func (t Transfer) Direction() Direction { return t.direction }

// Commitment returns the remote acceptance classification.
func (t Transfer) Commitment() Commitment { return t.commitment }

// Bytes returns bytes read from or written to the object stream.
func (t Transfer) Bytes() core.ByteLength { return t.bytes }

// SHA256 returns the verified digest on confirmed transfers.
func (t Transfer) SHA256() core.SHA256Digest { return t.sha256 }

// CRC32C returns the verified checksum on confirmed transfers.
func (t Transfer) CRC32C() core.CRC32C { return t.crc32c }

// Status returns the provider status when one was received.
func (t Transfer) Status() (core.HTTPStatusCode, bool) {
	return t.status, t.status.Validate() == nil
}

// Version returns the typed provider generation or version when supplied.
func (t Transfer) Version() (ProviderVersion, bool) {
	return t.version, !t.version.IsZero()
}

// Validate proves one confirmed exact transfer.
func (t Transfer) Validate() error {
	if err := t.provider.Validate(); err != nil {
		return err
	}
	if err := t.direction.Validate(); err != nil {
		return err
	}
	if t.commitment != CommitmentConfirmed {
		return core.ErrObjectStoreContract
	}
	if err := t.sha256.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := t.crc32c.Validate(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := t.status.Validate(); err != nil || !t.status.IsSuccess() {
		return core.ErrObjectStoreContract
	}
	if err := t.validateVersion(); err != nil {
		return err
	}
	return nil
}

func (t Transfer) validateVersion() error {
	if t.version.IsZero() {
		return nil
	}
	if t.version.Provider() != t.provider {
		return core.ErrObjectStoreContract
	}
	return t.version.Validate()
}

// UploadS3 performs one exact, single-attempt Amazon S3 PutObject.
func UploadS3(
	ctx context.Context,
	client Client,
	request UploadRequest,
) (Transfer, error) {
	return client.upload(ctx, request, ProviderAmazonS3)
}

// UploadGCS performs one exact, single-attempt Cloud Storage XML API upload.
func UploadGCS(
	ctx context.Context,
	client Client,
	request UploadRequest,
) (Transfer, error) {
	return client.upload(ctx, request, ProviderGoogleCloudStorage)
}

// UploadCloudflareImages performs one exact, single-attempt direct image
// upload.
func UploadCloudflareImages(
	ctx context.Context,
	client Client,
	request UploadRequest,
) (Transfer, error) {
	return client.upload(ctx, request, ProviderCloudflareImages)
}

func (c Client) upload(
	ctx context.Context,
	request UploadRequest,
	provider Provider,
) (Transfer, error) {
	result := newTransfer(provider, DirectionUpload)
	if err := request.validateCall(ctx, c, provider); err != nil {
		return result, err
	}
	if err := rejectExpired(request.Target.ExpiresAt); err != nil {
		return result, err
	}
	prepared, err := prepareUpload(request, provider)
	if err != nil {
		return result, err
	}
	response, transferErr := exchange.Upload(exchange.UploadCall{
		Context: ctx, Client: c.exchange,
		Request: prepared.request,
		Policy:  request.Policy.exchange(),
	})
	result = resultFromResponse(result, response)
	delivered, lengthErr := core.NewByteLength(prepared.exact.delivered)
	result.bytes = delivered
	if lengthErr != nil {
		return result, projectFailure(lengthErr, DirectionUpload)
	}
	if transferErr != nil {
		result.commitment = uploadFailureCommitment(transferErr, prepared.exact)
		return result, projectFailure(transferErr, DirectionUpload)
	}
	return confirmUpload(
		result,
		request.Integrity,
		prepared,
		response.Metadata.Headers,
	)
}

// DownloadS3 performs one exact, single-attempt whole-object Amazon S3
// GetObject.
func DownloadS3(
	ctx context.Context,
	client Client,
	request DownloadRequest,
) (Transfer, error) {
	return client.download(ctx, request, ProviderAmazonS3)
}

// DownloadGCS performs one exact, single-attempt whole-object Cloud Storage
// XML API download.
func DownloadGCS(
	ctx context.Context,
	client Client,
	request DownloadRequest,
) (Transfer, error) {
	return client.download(ctx, request, ProviderGoogleCloudStorage)
}

func (c Client) download(
	ctx context.Context,
	request DownloadRequest,
	provider Provider,
) (Transfer, error) {
	result := newTransfer(provider, DirectionDownload)
	if err := request.validateCall(ctx, c, provider); err != nil {
		return result, err
	}
	if err := rejectExpired(request.Target.ExpiresAt); err != nil {
		return result, err
	}
	prepared, err := prepareDownload(request, provider)
	if err != nil {
		return result, err
	}
	response, transferErr := exchange.Download(exchange.DownloadCall{
		Context: ctx, Client: c.exchange,
		Request: prepared.request,
		Policy:  request.Policy.exchange(),
	})
	result = resultFromResponse(result, response)
	if transferErr != nil {
		result.commitment = CommitmentRejected
		return result, projectFailure(transferErr, DirectionDownload)
	}
	if err := rejectPartialContent(response.Metadata.Headers); err != nil {
		result.commitment = CommitmentRejected
		return result, err
	}
	return confirmDownload(
		result,
		request.Integrity,
		prepared.digests,
		response.Metadata.Headers,
	)
}

type preparedDownload struct {
	digests streamDigests
	request exchange.DownloadRequest
}

func prepareDownload(
	request DownloadRequest,
	provider Provider,
) (preparedDownload, error) {
	headers, err := downloadHeaders(provider, request.Target)
	if err != nil {
		return preparedDownload{}, err
	}
	selection, err := responseSelection(
		provider,
		DirectionDownload,
	)
	if err != nil {
		return preparedDownload{}, err
	}
	digests := newDigests()
	return preparedDownload{
		digests: digests,
		request: exchange.DownloadRequest{
			Target:                      exchangeTarget{url: request.Target.URL.value},
			Destination:                 io.MultiWriter(request.Destination, digests.writer()),
			Semantics:                   singleAttempt(exchange.MethodGet),
			ExpectedResponseContentType: request.ContentType,
			Headers:                     headers,
			CaptureHeaders:              selection,
			ResponseBodyLimit:           downloadLimit(request.Integrity.Length),
			ExpectedStatus:              core.HTTPStatusOK(),
		},
	}, nil
}

func rejectPartialContent(headers exchange.CapturedHeaders) error {
	name, err := headerName(headerContentRange)
	if err != nil {
		return err
	}
	if capturedHeaderPresent(headers, name) {
		return core.ErrObjectStoreIntegrity
	}
	return nil
}

type preparedUpload struct {
	digests streamDigests
	exact   *ExactReader
	request exchange.UploadRequest
}

func prepareUpload(
	request UploadRequest,
	provider Provider,
) (preparedUpload, error) {
	length, err := request.Integrity.Length.Int64()
	if err != nil {
		return preparedUpload{}, errors.Join(core.ErrObjectStoreSize, err)
	}
	exact := NewExactReader(request.Source, length)
	if length == 0 {
		if proveErr := exact.ProveEmpty(); proveErr != nil {
			return preparedUpload{}, proveErr
		}
	}
	digests := newDigests()
	source := io.TeeReader(exact, digests.writer())
	spec, err := Spec(provider)
	if err != nil {
		return preparedUpload{}, err
	}
	body, err := uploadBody(spec, request, source)
	if err != nil {
		return preparedUpload{}, err
	}
	headers, err := uploadHeaders(provider, request.Target, request.Integrity)
	if err != nil {
		return preparedUpload{}, err
	}
	selection, err := responseSelection(
		provider,
		DirectionUpload,
	)
	if err != nil {
		return preparedUpload{}, err
	}
	return preparedUpload{
		exact: exact, digests: digests,
		request: exchange.UploadRequest{
			Target:         exchangeTarget{url: request.Target.URL.value},
			Source:         body.source,
			Semantics:      singleAttempt(spec.UploadMethod),
			ContentType:    body.contentType,
			Headers:        headers,
			CaptureHeaders: selection,
			ContentLength:  body.length,
			ExpectedStatus: core.HTTPStatusOK(),
		},
	}, nil
}

// requestBody is the exact request body one vendor encoding requires.
type requestBody struct {
	source      io.Reader
	contentType core.HTTPMediaType
	length      core.ByteLength
}

func uploadBody(
	spec VendorSpec,
	request UploadRequest,
	source io.Reader,
) (requestBody, error) {
	switch spec.UploadEncoding {
	case UploadEncodingRawObject:
		return requestBody{
			source:      source,
			contentType: request.ContentType,
			length:      request.Integrity.Length,
		}, nil
	case UploadEncodingMultipartFile:
		return multipartUpload(
			source,
			request.ContentType,
			request.Integrity.Length,
		)
	case UploadEncodingUnknown, uploadEncodingLimit:
		return requestBody{}, core.ErrObjectStoreContract
	}
	return requestBody{}, core.ErrObjectStoreContract
}

func confirmUpload(
	result Transfer,
	expected Integrity,
	prepared preparedUpload,
	headers exchange.CapturedHeaders,
) (Transfer, error) {
	if !prepared.exact.verified {
		result.commitment = CommitmentRejected
		return result, errors.Join(
			core.ErrObjectStoreContract,
			coreSourceIntegrity(),
		)
	}
	return confirmTransfer(result, expected, prepared.digests, headers)
}

// confirmDownload compares the local streaming CRC32C with the checksum the
// provider computed over the object it served, then applies the same exact
// extent and digest proof every direction shares. The caller-supplied expected
// digests and Objectstore's local digests come from one algorithm, so the
// provider value is the only independent witness available.
func confirmDownload(
	result Transfer,
	expected Integrity,
	digests streamDigests,
	headers exchange.CapturedHeaders,
) (Transfer, error) {
	_, local, err := digests.values()
	if err != nil {
		result.commitment = CommitmentRejected
		return result, err
	}
	remote, present, err := providerDownloadCRC32C(headers, result.provider)
	if err != nil {
		result.commitment = CommitmentRejected
		return result, err
	}
	if present && remote != local {
		result.commitment = CommitmentRejected
		return result, core.ErrObjectStoreIntegrity
	}
	return confirmTransfer(result, expected, digests, headers)
}

func confirmTransfer(
	result Transfer,
	expected Integrity,
	digests streamDigests,
	headers exchange.CapturedHeaders,
) (Transfer, error) {
	shaValue, crcValue, err := digests.values()
	if err != nil {
		result.commitment = CommitmentRejected
		return result, err
	}
	if result.bytes != expected.Length ||
		shaValue != expected.SHA256 ||
		crcValue != expected.CRC32C {
		result.commitment = CommitmentRejected
		return result, core.ErrObjectStoreIntegrity
	}
	version, present, err := capturedVersion(headers, result.provider)
	if err != nil {
		result.commitment = invalidResponseCommitment(result.direction)
		return result, err
	}
	if present {
		result.version = version
	}
	result.sha256 = shaValue
	result.crc32c = crcValue
	result.commitment = CommitmentConfirmed
	if err := result.Validate(); err != nil {
		return result, err
	}
	return result, nil
}

func (r UploadRequest) validateCall(
	ctx context.Context,
	client Client,
	provider Provider,
) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := client.Validate(); err != nil {
		return err
	}
	return r.validateFor(provider)
}

func (r DownloadRequest) validateCall(
	ctx context.Context,
	client Client,
	provider Provider,
) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := client.Validate(); err != nil {
		return err
	}
	return r.validateFor(provider)
}

func rejectExpired(expiresAt temporal.Instant) error {
	observation, err := temporal.Observe()
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	now, err := observation.Instant()
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	order, err := now.Compare(expiresAt)
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if order != core.ComparisonLess {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreExpired)
	}
	return nil
}

type exchangeTarget struct {
	url url.URL
}

func (t exchangeTarget) Validate() error {
	endpoint, err := core.ParseHTTPEndpoint(t.url.String())
	if err != nil || endpoint.HTTPURL().Scheme != httpsScheme {
		return core.ErrObjectStoreContract
	}
	return nil
}

func (t exchangeTarget) HTTPURL() url.URL { return t.url }

func (exchangeTarget) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type streamDigests struct {
	sha256 *core.DigestWriter
	crc32c hash.Hash32
}

func newDigests() streamDigests {
	return streamDigests{
		sha256: core.NewDigestWriter(),
		crc32c: crc32.New(crc32.MakeTable(crc32.Castagnoli)),
	}
}

func (d streamDigests) writer() io.Writer {
	return io.MultiWriter(d.sha256, d.crc32c)
}

// values peeks the running digests without ending the stream, because the
// download path reads them once for the provider comparison and again for the
// caller's expected integrity. The refusal branch guards a writer this
// package never constructs: transfers are bounded far below the count domain
// and nothing here seals, so a failure is a contract breach to surface, not a
// state to explain away.
func (d streamDigests) values() (core.SHA256Digest, core.CRC32C, error) {
	digest, _, err := d.sha256.Digest()
	if err != nil {
		return core.SHA256Digest{}, core.CRC32C{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return digest, core.NewCRC32C(d.crc32c.Sum32()), nil
}

// multipartUpload frames the object stream as one multipart file field without
// buffering it. The caller's media type becomes the part's own Content-Type so
// the declared object type is carried to the provider rather than discarded.
func multipartUpload(
	source io.Reader,
	partType core.HTTPMediaType,
	length core.ByteLength,
) (requestBody, error) {
	partHeader, err := multipartFileHeader(partType)
	if err != nil {
		return requestBody{}, err
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if _, err := writer.CreatePart(partHeader); err != nil {
		return requestBody{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	prefix := bytes.Clone(buffer.Bytes())
	buffer.Reset()
	if err := writer.Close(); err != nil {
		return requestBody{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	suffix := bytes.Clone(buffer.Bytes())
	contentType, err := core.ParseHTTPMediaType(writer.FormDataContentType())
	if err != nil {
		return requestBody{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	overhead := uint64(len(prefix) + len(suffix))
	if length.Uint64() > ^uint64(0)-overhead {
		return requestBody{}, core.ErrObjectStoreSize
	}
	framedLength, err := core.NewByteLength(length.Uint64() + overhead)
	if err != nil {
		return requestBody{}, errors.Join(core.ErrObjectStoreSize, err)
	}
	return requestBody{
		source: io.MultiReader(
			bytes.NewReader(prefix), source, bytes.NewReader(suffix),
		),
		contentType: contentType,
		length:      framedLength,
	}, nil
}

func multipartFileHeader(
	partType core.HTTPMediaType,
) (textproto.MIMEHeader, error) {
	disposition, err := headerName(headerContentDisposition)
	if err != nil {
		return nil, err
	}
	header := make(textproto.MIMEHeader, 2)
	header.Set(disposition.String(), multipartFileDisposition())
	header.Set(core.HTTPHeaderContentType().String(), partType.String())
	return header, nil
}

func newTransfer(provider Provider, direction Direction) Transfer {
	return Transfer{
		provider: provider, direction: direction,
		commitment: CommitmentNotAttempted,
	}
}

func resultFromResponse(
	result Transfer,
	response exchange.StreamResponse,
) Transfer {
	result.bytes = response.Metadata.Bytes
	result.status = response.Metadata.Status
	return result
}

func invalidResponseCommitment(direction Direction) Commitment {
	if direction == DirectionUpload {
		return CommitmentIndeterminate
	}
	return CommitmentRejected
}

func uploadFailureCommitment(err error, exact *ExactReader) Commitment {
	_, hasStatus := errors.AsType[exchange.StatusError](err)
	if hasStatus || errors.Is(err, core.ErrObjectStoreSource) ||
		(exact != nil && exact.Failure() != nil) {
		return CommitmentRejected
	}
	return CommitmentIndeterminate
}

func projectFailure(cause error, direction Direction) error {
	projected := []error{core.ErrObjectStoreContract}
	projected = append(projected, directionFailure(cause, direction)...)
	projected = append(projected, statusFailure(cause, direction)...)
	projected = append(projected, sanitizedExchangeFailure(cause))
	return errors.Join(projected...)
}

func directionFailure(cause error, direction Direction) []error {
	if !errors.Is(cause, core.ErrExchangeWrite) {
		if direction == DirectionDownload &&
			errors.Is(cause, core.ErrExchangeBodyLimit) {
			return []error{core.ErrObjectStoreIntegrity}
		}
		return nil
	}
	if direction == DirectionUpload {
		return []error{core.ErrObjectStoreSource}
	}
	return []error{core.ErrObjectStoreDestination}
}

func statusFailure(cause error, direction Direction) []error {
	status, ok := errors.AsType[exchange.StatusError](cause)
	if !ok {
		return nil
	}
	projected := []error{status}
	code := statusCode(status)
	if code == http.StatusNotFound && direction == DirectionDownload {
		projected = append(projected, core.ErrObjectStoreAbsent)
	}
	if (code == http.StatusConflict || code == http.StatusPreconditionFailed) &&
		direction == DirectionUpload {
		projected = append(projected, core.ErrObjectStoreConflict)
	}
	return projected
}

func sanitizedExchangeFailure(cause error) error {
	urlFailure, ok := errors.AsType[*url.Error](cause)
	if !ok {
		return cause
	}
	return errors.Join(
		stableExchangeIdentities(cause),
		&url.Error{
			Op: urlFailure.Op, URL: core.RedactedValueText, Err: urlFailure.Err,
		},
	)
}

func stableExchangeIdentities(cause error) error {
	var matched []error
	for _, identity := range [...]error{
		core.ErrExchangeContract,
		core.ErrExchangeRequest,
		core.ErrExchangeTransport,
		core.ErrExchangeResponse,
		core.ErrExchangeCancelled,
		core.ErrExchangeRedirect,
		core.ErrExchangeBodyLimit,
		core.ErrExchangeContentType,
		core.ErrExchangeWrite,
		context.Canceled,
		context.DeadlineExceeded,
		io.ErrUnexpectedEOF,
		io.ErrShortWrite,
		io.ErrNoProgress,
	} {
		if errors.Is(cause, identity) {
			matched = append(matched, identity)
		}
	}
	return errors.Join(matched...)
}

func coreSourceIntegrity() error {
	return errors.Join(core.ErrObjectStoreSource, core.ErrObjectStoreIntegrity)
}

func statusCode(status exchange.StatusError) int {
	value, _ := status.Status().Int()
	return value
}

func singleAttempt(method exchange.Method) exchange.RequestSemantics {
	return exchange.RequestSemantics{
		Method: method, Replay: exchange.ReplaySingleAttempt,
	}
}

func downloadLimit(length core.ByteLength) core.ByteCount {
	value := length.Uint64()
	if value == 0 {
		value = 1
	}
	limit, _ := core.NewByteCount(value)
	return limit
}
