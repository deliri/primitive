package cloudflare

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// InboundObservation reports acknowledged destination bytes. Authentication
// does not imply JSON validity, processing readiness, novelty, or acceptance by
// application policy. A transfer error may accompany an authenticated prefix.
type InboundObservation struct {
	Bytes      core.ByteLength
	ObservedAt temporal.Instant
}

func (o InboundObservation) Validate() error {
	if err := errors.Join(o.Bytes.Validate(), o.ObservedAt.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// WebhookReceiveRequest binds an incoming call to one configured route.
// The caller owns the destination and context; Exchange closes a consumed body.
type WebhookReceiveRequest struct {
	Call        exchange.SocketServerCall
	Destination io.Writer
	Endpoint    core.HTTPEndpoint
	ObservedAt  temporal.Instant
	BodyMaximum core.ByteCount
}

func (r WebhookReceiveRequest) Validate() error {
	if core.WriterIsNil(r.Destination) {
		return core.ErrCloudflareContract
	}
	if err := errors.Join(r.Call.Validate(), r.Endpoint.Validate(), r.ObservedAt.Validate(), r.BodyMaximum.Validate()); err != nil {
		return contractError(err)
	}
	matches, err := r.Call.MatchesEndpointPath(r.Endpoint)
	query, queryErr := r.Call.RawQuery()
	if err != nil || queryErr != nil || !matches || query != r.Endpoint.HTTPURL().RawQuery {
		return core.ErrCloudflareBinding
	}
	return nil
}

// ImagesWebhookReceiver authenticates Cloudflare Notifications' documented
// cf-webhook-auth header. It does not implement the unrelated Stream HMAC scheme.
type ImagesWebhookReceiver struct{ secret NotificationSecret }

func NewImagesWebhookReceiver(secret NotificationSecret) (ImagesWebhookReceiver, error) {
	if err := secret.Validate(); err != nil {
		return ImagesWebhookReceiver{}, err
	}
	owned, err := ParseNotificationSecret(secret.value)
	return ImagesWebhookReceiver{secret: owned}, err
}

func (r ImagesWebhookReceiver) Validate() error { return r.secret.Validate() }
func (r *ImagesWebhookReceiver) Close() error {
	if r == nil {
		return core.ErrCloudflareContract
	}
	return r.secret.Close()
}

func (r ImagesWebhookReceiver) Receive(request WebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, err
	}
	value, err := readHeader(request.Call, core.CloudflareNotificationAuthenticationHeader, core.CloudflareSecretCustodyMaximumBytes)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	if subtle.ConstantTimeCompare([]byte(value), r.secret.value) != 1 {
		return InboundObservation{}, core.ErrCloudflareVerification
	}
	stream, err := receiveWebhook(request, request.Destination)
	return InboundObservation{Bytes: stream.Bytes, ObservedAt: request.ObservedAt}, err
}

// StreamWebhookReceiveRequest uses exclusively owned seekable scratch so the
// body can be authenticated without retaining it in memory or publishing an
// unauthenticated prefix. Obtain file scratch through Filestore. The caller
// closes/removes scratch; its contents remain untrusted until Receive succeeds.
// MaximumAge and FutureTolerance are application policy, not Cloudflare limits.
type StreamWebhookReceiveRequest struct {
	Scratch io.ReadWriteSeeker
	WebhookReceiveRequest
	MaximumAge      temporal.Duration
	FutureTolerance temporal.Duration
}

func (r StreamWebhookReceiveRequest) Validate() error {
	if r.Scratch == nil || core.ReaderIsNil(r.Scratch) || core.WriterIsNil(r.Scratch) {
		return core.ErrCloudflareContract
	}
	return errors.Join(r.WebhookReceiveRequest.Validate(), r.MaximumAge.Validate(), r.FutureTolerance.Validate())
}

type StreamWebhookReceiver struct{ secret StreamWebhookSecret }

func NewStreamWebhookReceiver(secret StreamWebhookSecret) (StreamWebhookReceiver, error) {
	if err := secret.Validate(); err != nil {
		return StreamWebhookReceiver{}, err
	}
	material, err := secret.value.CopyBytes()
	if err != nil {
		return StreamWebhookReceiver{}, authenticationError(err)
	}
	defer clear(material)
	owned, err := ParseStreamWebhookSecret(material)
	return StreamWebhookReceiver{secret: owned}, err
}

func (r StreamWebhookReceiver) Validate() error { return r.secret.Validate() }
func (r *StreamWebhookReceiver) Close() error {
	if r == nil {
		return core.ErrCloudflareContract
	}
	return r.secret.Close()
}

type streamSignature struct {
	timestamp string
	digest    [sha256.Size]byte
	instant   temporal.Instant
}

func parseStreamSignature(value string) (streamSignature, error) {
	if len(value) > core.CloudflareStreamSignatureMaximumBytes {
		return streamSignature{}, core.ErrCloudflareVerification
	}
	first, second, ok := strings.Cut(value, ",")
	if !ok {
		return streamSignature{}, core.ErrCloudflareVerification
	}
	// Fields are identified by name, as specified by Cloudflare, rather than position.
	// https://developers.cloudflare.com/stream/manage-video-library/using-webhooks/
	if strings.HasPrefix(first, "sig1=") {
		first, second = second, first
	}
	stamp, ok := strings.CutPrefix(first, "time=")
	if !ok {
		return streamSignature{}, core.ErrCloudflareVerification
	}
	encoded, ok := strings.CutPrefix(second, "sig1=")
	if !ok || len(encoded) != sha256.Size*2 {
		return streamSignature{}, core.ErrCloudflareVerification
	}
	return decodeStreamSignature(stamp, encoded)
}

func decodeStreamSignature(stamp, encoded string) (streamSignature, error) {
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || seconds < 0 || strconv.FormatInt(seconds, 10) != stamp {
		return streamSignature{}, core.ErrCloudflareVerification
	}
	instant, err := temporal.InstantFromUnixSeconds(seconds)
	if err != nil {
		return streamSignature{}, verificationError(err)
	}
	result := streamSignature{timestamp: stamp, instant: instant}
	if _, err := hex.Decode(result.digest[:], []byte(encoded)); err != nil {
		return streamSignature{}, verificationError(err)
	}
	return result, nil
}

func (r StreamWebhookReceiver) Receive(request StreamWebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, err
	}
	value, err := readHeader(request.Call, core.CloudflareStreamSignatureHeader, uint64(core.CloudflareStreamSignatureMaximumBytes))
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	signature, err := parseStreamSignature(value)
	if err != nil {
		return InboundObservation{}, err
	}
	if err := signature.validateFreshness(request); err != nil {
		return InboundObservation{}, err
	}
	return r.receiveAuthenticated(request, signature)
}

func (s streamSignature) validateFreshness(request StreamWebhookReceiveRequest) error {
	earliest, err := request.ObservedAt.Subtract(request.MaximumAge)
	latest, latestErr := request.ObservedAt.Add(request.FutureTolerance)
	if err != nil || latestErr != nil {
		return verificationError(errors.Join(err, latestErr))
	}
	before, err := s.instant.Compare(earliest)
	after, afterErr := s.instant.Compare(latest)
	if err != nil || afterErr != nil || before == core.ComparisonLess || after == core.ComparisonGreater {
		return core.ErrCloudflareVerification
	}
	return nil
}

func (r StreamWebhookReceiver) receiveAuthenticated(request StreamWebhookReceiveRequest, signature streamSignature) (InboundObservation, error) {
	if _, err := request.Scratch.Seek(0, io.SeekStart); err != nil {
		return InboundObservation{}, contractError(err)
	}
	material, err := r.secret.value.CopyBytes()
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	defer clear(material)
	mac := hmac.New(sha256.New, material)
	if _, err := io.WriteString(mac, signature.timestamp+"."); err != nil {
		return InboundObservation{}, contractError(err)
	}
	stream, err := receiveWebhook(request.WebhookReceiveRequest, io.MultiWriter(request.Scratch, mac))
	if err != nil {
		return InboundObservation{}, err
	}
	if !hmac.Equal(signature.digest[:], mac.Sum(nil)) {
		return InboundObservation{}, core.ErrCloudflareVerification
	}
	return request.replayAuthenticated(stream.Bytes)
}

func (request StreamWebhookReceiveRequest) replayAuthenticated(extent core.ByteLength) (InboundObservation, error) {
	length, err := extent.Int64()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	if _, err := request.Scratch.Seek(0, io.SeekStart); err != nil {
		return InboundObservation{}, contractError(err)
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	// The exact authenticated extent excludes old trailing bytes in reused scratch.
	n, err := io.Copy(request.Destination, &contextReader{ctx: ctx, source: io.LimitReader(request.Scratch, length)})
	count, countErr := core.CheckedUint64FromInt64(n)
	bytes, lengthErr := core.NewByteLength(count)
	if n != length && err == nil {
		err = io.ErrUnexpectedEOF
	}
	return InboundObservation{Bytes: bytes, ObservedAt: request.ObservedAt}, errors.Join(err, countErr, lengthErr)
}

type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := contextstate.Validate(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.source.Read(p)
	if canceled := contextstate.Validate(r.ctx); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}

func readHeader(call exchange.SocketServerCall, name string, maximum uint64) (string, error) {
	header, err := core.ParseHTTPHeaderName(name)
	if err != nil {
		return "", err
	}
	bound, err := core.NewByteCount(maximum)
	if err != nil {
		return "", err
	}
	value, err := call.UniqueHeader(header, bound)
	if err != nil {
		return "", err
	}
	return value.Value()
}

func receive(call exchange.SocketServerCall, destination io.Writer) (exchange.ReceivedStream, error) {
	return exchange.ReceiveStream(exchange.StreamReceiveCall{Call: call, Destination: destination,
		ExpectedContentType: core.HTTPMediaTypeJSON(), Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}})
}

// webhookDestination bounds total retained bytes independently of Go's copy
// window. It never allocates in proportion to input or writes over budget.
type webhookDestination struct {
	destination io.Writer
	remaining   uint64
}

func (w *webhookDestination) Write(data []byte) (int, error) {
	if uint64(len(data)) > w.remaining {
		return 0, core.ErrCloudflareContract
	}
	n, err := w.destination.Write(data)
	if n < 0 || n > len(data) {
		return 0, contractError(err)
	}
	w.remaining -= uint64(n)
	if n != len(data) && err == nil {
		err = io.ErrShortWrite
	}
	return n, err
}
func receiveWebhook(request WebhookReceiveRequest, destination io.Writer) (exchange.ReceivedStream, error) {
	maximum, err := request.BodyMaximum.Uint64()
	if err != nil {
		return exchange.ReceivedStream{}, contractError(err)
	}
	bounded := webhookDestination{destination: destination, remaining: maximum}
	return receive(request.Call, &bounded)
}
