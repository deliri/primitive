package twilio

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

type WebhookRepresentation uint8

const (
	WebhookRepresentationUnknown WebhookRepresentation = iota
	WebhookRepresentationForm
	WebhookRepresentationJSON
	webhookRepresentationLimit
)

func (r WebhookRepresentation) Validate() error {
	if r <= WebhookRepresentationUnknown || r >= webhookRepresentationLimit {
		return core.ErrTwilioContract
	}
	return nil
}
func (r WebhookRepresentation) IsValid() bool { return r.Validate() == nil }
func (r WebhookRepresentation) String() string {
	if !r.IsValid() {
		return ""
	}
	return [...]string{"", "form", "json"}[r]
}
func (WebhookRepresentation) OffWireEnum() {}
func (r WebhookRepresentation) mediaType() (core.HTTPMediaType, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	if r == WebhookRepresentationForm {
		return core.HTTPMediaTypeFormURLEncoded(), nil
	}
	return core.HTTPMediaTypeJSON(), nil
}

type InboundObservation struct {
	Bytes      core.ByteLength
	ObservedAt temporal.Instant
}

func (o InboundObservation) Validate() error {
	if err := o.ObservedAt.Validate(); err != nil {
		return contractError(err)
	}
	_, err := o.Bytes.Int64()
	if err != nil {
		return contractError(err)
	}
	return nil
}

type WebhookReceiverRequest struct {
	Token          AuthToken
	PublicEndpoint core.HTTPEndpoint
	Representation WebhookRepresentation
}

func (r WebhookReceiverRequest) Validate() error {
	if err := errors.Join(r.Token.Validate(), r.PublicEndpoint.Validate(), r.Representation.Validate()); err != nil {
		return contractError(err)
	}
	if r.PublicEndpoint.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.ErrTwilioBinding
	}
	_, err := validateWebhookQuery(context.Background(), r.PublicEndpoint.HTTPURL().RawQuery, false)
	if err != nil {
		return bindingError(err)
	}
	return nil
}

type WebhookReceiveRequest struct {
	ScratchParent core.AbsolutePath
	Call          exchange.SocketServerCall
	Destination   io.Writer
	ObservedAt    temporal.Instant
}

func (r WebhookReceiveRequest) Validate() error {
	if r.ScratchParent.Validate() != nil || r.Call.Validate() != nil || core.WriterIsNil(r.Destination) || r.ObservedAt.Validate() != nil {
		return core.ErrTwilioContract
	}
	return nil
}

type webhookReceiverState struct {
	token          AuthToken
	publicEndpoint core.HTTPEndpoint
	representation WebhookRepresentation
}
type WebhookReceiver struct{ state *webhookReceiverState }

func NewWebhookReceiver(request WebhookReceiverRequest) (WebhookReceiver, error) {
	if err := request.Validate(); err != nil {
		return WebhookReceiver{}, err
	}
	owned, err := ParseAuthToken(request.Token.value)
	if err != nil {
		return WebhookReceiver{}, err
	}
	return WebhookReceiver{state: &webhookReceiverState{token: owned, publicEndpoint: request.PublicEndpoint, representation: request.Representation}}, nil
}
func (r WebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrTwilioContract
	}
	if err := errors.Join(r.state.token.Validate(), r.state.publicEndpoint.Validate(), r.state.representation.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}
func (r *WebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrTwilioContract
	}
	err := r.state.token.Close()
	r.state = nil
	return err
}

func (r WebhookReceiver) Receive(request WebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	signedURL, expectedBodyDigest, err := signedRequestURL(request.Call, r.state.publicEndpoint, r.state.representation)
	if err != nil {
		return InboundObservation{}, err
	}
	signature, err := webhookSignature(request.Call)
	if err != nil {
		return InboundObservation{}, err
	}
	hashes, err := newWebhookSignatureHashes(signedURL, r.state.token.value)
	if err != nil {
		return InboundObservation{}, err
	}
	media, err := r.state.representation.mediaType()
	if err != nil {
		return InboundObservation{}, err
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return InboundObservation{}, err
	}
	var observation InboundObservation
	err = filestore.WithScratchScope(ctx, filestore.ScratchScopeRequest{Parent: request.ScratchParent, Use: func(ctx context.Context, root *os.Root) error {
		path, err := core.ParseRelativePath(core.TwilioWebhookScratchFilename)
		if err != nil {
			return err
		}
		result, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600}, Use: func(ctx context.Context, file filestore.ScratchReplayFile) error {
			bodyHash := sha256.New()
			var buffer [exchange.TransferBufferBytes]byte
			received, err := exchange.ReceiveStream(exchange.StreamReceiveCall{Call: request.Call, ExpectedContentType: media, Buffer: buffer[:], Destination: io.MultiWriter(file, bodyHash), Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}})
			if err != nil {
				return err
			}
			extent, err := received.Bytes.Int64()
			if err != nil {
				return err
			}
			switch r.state.representation {
			case WebhookRepresentationForm:
				if err := writeSortedWebhookForm(ctx, request.ScratchParent, file, extent, io.MultiWriter(hashes.WithPort, hashes.WithoutPort)); err != nil {
					return verificationError(err)
				}
			case WebhookRepresentationJSON:
				var actual [sha256.Size]byte
				var expected [sha256.Size]byte
				expected, err = expectedBodyDigest.Bytes()
				if err != nil {
					return err
				}
				if subtle.ConstantTimeCompare(bodyHash.Sum(actual[:0]), expected[:]) != 1 {
					return core.ErrTwilioVerification
				}
			default:
				return core.ErrTwilioContract
			}
			if err := hashes.Verify(signature); err != nil {
				return err
			}
			written, err := writeVerifiedWebhookBody(ctx, request.Destination, io.NewSectionReader(file, 0, extent), extent)
			observation = InboundObservation{Bytes: written, ObservedAt: request.ObservedAt}
			return err
		}})
		return errors.Join(err, result.OperationError(), result.CleanupError())
	}})
	if err != nil {
		return observation, err
	}
	return observation, observation.Validate()
}

func webhookSignature(call exchange.SocketServerCall) (string, error) {
	headerName, err := core.ParseHTTPHeaderName(core.TwilioWebhookSignatureHeaderName)
	if err != nil {
		return "", contractError(err)
	}
	headerMaximum, err := core.NewByteCount(core.TwilioWebhookSignatureBytes)
	if err != nil {
		return "", contractError(err)
	}
	header, err := call.UniqueHeader(headerName, headerMaximum)
	if err != nil {
		return "", core.ErrTwilioAuthentication
	}
	signature, err := header.Value()
	if err != nil {
		return "", authenticationError(err)
	}
	if len(signature) != core.TwilioWebhookSignatureBytes {
		return "", core.ErrTwilioAuthentication
	}
	return signature, nil
}

func signedRequestURL(call exchange.SocketServerCall, endpoint core.HTTPEndpoint, representation WebhookRepresentation) (string, core.SHA256Digest, error) {
	public := endpoint.HTTPURL()
	matches, err := call.MatchesEndpointPath(endpoint)
	if err != nil || !matches {
		return "", core.SHA256Digest{}, bindingError(err)
	}
	rawQuery, err := call.RawQuery()
	if err != nil {
		return "", core.SHA256Digest{}, bindingError(err)
	}
	if representation == WebhookRepresentationForm {
		if rawQuery != public.RawQuery {
			return "", core.SHA256Digest{}, core.ErrTwilioBinding
		}
		return public.String(), core.SHA256Digest{}, nil
	}
	if representation != WebhookRepresentationJSON {
		return "", core.SHA256Digest{}, core.ErrTwilioContract
	}
	ctx, err := call.Context()
	if err != nil {
		return "", core.SHA256Digest{}, bindingError(err)
	}
	digest, err := validateWebhookQuery(ctx, rawQuery, true)
	if err != nil {
		return "", core.SHA256Digest{}, err
	}
	matches, err = sameWebhookQueries(ctx, public.RawQuery, rawQuery)
	if err != nil || !matches {
		return "", core.SHA256Digest{}, bindingError(err)
	}
	public.RawQuery = rawQuery
	return public.String(), digest, nil
}

func writeVerifiedWebhookBody(ctx context.Context, destination io.Writer, source io.Reader, extent int64) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	var buffer [exchange.TransferBufferBytes]byte
	written, err := io.CopyBuffer(destination, source, buffer[:])
	count, countErr := core.CheckedUint64FromInt64(written)
	length, lengthErr := core.NewByteLength(count)
	if joined := errors.Join(err, countErr, lengthErr, contextstate.Validate(ctx)); joined != nil {
		return length, joined
	}
	if written != extent {
		return length, io.ErrShortWrite
	}
	return length, nil
}

var (
	_ core.Validatable = WebhookRepresentation(0)
	_ core.OffWireEnum = WebhookRepresentation(0)
	_ core.Validatable = InboundObservation{}
	_ core.Validatable = WebhookReceiverRequest{}
	_ core.Validatable = WebhookReceiveRequest{}
	_ core.Validatable = WebhookReceiver{}
)
