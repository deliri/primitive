package loops

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

// InboundObservation reports one verified delivery: the provider delivery
// identity, the signed instant, the released byte count and the caller's
// observation instant.
type InboundObservation struct {
	DeliveryID DeliveryID
	SignedAt   temporal.Instant
	Bytes      core.ByteLength
	ObservedAt temporal.Instant
}

func (o InboundObservation) Validate() error {
	if err := errors.Join(o.DeliveryID.Validate(), o.SignedAt.Validate(), o.ObservedAt.Validate()); err != nil {
		return contractError(err)
	}
	_, err := o.Bytes.Int64()
	if err != nil {
		return contractError(err)
	}
	return nil
}

type WebhookReceiveRequest struct {
	Call exchange.SocketServerCall
	// Destination receives the body only after the complete signed content
	// verifies. A refused delivery writes no byte to Destination.
	Destination io.Writer
	// ScratchParent is caller-owned placement for provisional bytes. Filestore
	// creates and removes a private child namespace; no body extent is limited.
	ScratchParent core.AbsolutePath
	// ObservedAt is the caller's receipt instant; the tolerance is measured
	// against it, never against a clock read inside this package.
	ObservedAt temporal.Instant
}

func (r WebhookReceiveRequest) Validate() error {
	if r.ScratchParent.Validate() != nil || r.Call.Validate() != nil || core.WriterIsNil(r.Destination) || r.ObservedAt.Validate() != nil {
		return core.ErrLoopsContract
	}
	return nil
}

type webhookReceiverState struct {
	secret WebhookSecret
}

type WebhookReceiver struct{ state *webhookReceiverState }

func NewWebhookReceiver(secret WebhookSecret) (WebhookReceiver, error) {
	owned, err := secret.ownedCopy()
	if err != nil {
		return WebhookReceiver{}, contractError(err)
	}
	return WebhookReceiver{state: &webhookReceiverState{secret: owned}}, nil
}

func (r WebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrLoopsContract
	}
	if err := r.state.secret.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (r *WebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrLoopsContract
	}
	err := r.state.secret.Close()
	r.state = nil
	return err
}

// Receive verifies one Standard Webhooks delivery. Header grammar and the
// timestamp tolerance are checked before any body byte is read. The body then
// streams once into private scratch while HMAC-SHA256 hashes
// "{webhook-id}.{webhook-timestamp}.{raw body}"; only after any v1 signature
// matches are the exact scratch bytes released to Destination. Missing or
// repeated headers are authentication failures; malformed, stale, future or
// unmatched evidence is a verification failure.
// https://loops.so/docs/webhooks
func (r WebhookReceiver) Receive(request WebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	signed, err := receiveSignedHeaders(request.Call)
	if err != nil {
		return InboundObservation{}, err
	}
	if err := errors.Join(validateSignatureHeader(signed.signature), validateSignatureTime(signed.signedAt, request.ObservedAt)); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	var observation InboundObservation
	err = filestore.WithScratchScope(ctx, filestore.ScratchScopeRequest{
		Parent: request.ScratchParent,
		Use: func(ctx context.Context, root *os.Root) error {
			path, pathErr := core.ParseRelativePath(core.LoopsWebhookScratchFilename)
			if pathErr != nil {
				return pathErr
			}
			result, scopeErr := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{
				Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600},
				Use: func(ctx context.Context, file filestore.ScratchReplayFile) error {
					var releaseErr error
					observation, releaseErr = r.verifyThenRelease(ctx, request, signed, file)
					return releaseErr
				},
			})
			return errors.Join(scopeErr, result.OperationError(), result.CleanupError())
		},
	})
	if err != nil {
		return observation, err
	}
	return observation, observation.Validate()
}

// signedHeaders is the admitted Standard Webhooks header triple. The
// timestamp keeps its exact header spelling because it is signed content.
type signedHeaders struct {
	delivery      DeliveryID
	timestampText string
	signature     string
	signedAt      temporal.Instant
}

func receiveSignedHeaders(call exchange.SocketServerCall) (signedHeaders, error) {
	idText, err := uniqueHeader(call, core.LoopsWebhookIDHeaderName, core.LoopsWebhookIDMaximumBytes)
	if err != nil {
		return signedHeaders{}, err
	}
	timestampText, err := uniqueHeader(call, core.LoopsWebhookTimestampHeaderName, core.LoopsWebhookTimestampMaximumBytes)
	if err != nil {
		return signedHeaders{}, err
	}
	signature, err := uniqueHeader(call, core.LoopsWebhookSignatureHeaderName, core.LoopsWebhookSignatureMaximumBytes)
	if err != nil {
		return signedHeaders{}, err
	}
	delivery, err := ParseDeliveryID(idText)
	if err != nil {
		return signedHeaders{}, err
	}
	signedAt, err := parseSignatureTimestamp(timestampText)
	if err != nil {
		return signedHeaders{}, err
	}
	return signedHeaders{delivery: delivery, timestampText: timestampText, signedAt: signedAt, signature: signature}, nil
}

// verifyThenRelease streams the body once into scratch and the HMAC, verifies
// the complete signed content, and only then copies the exact scratch bytes to
// the caller's destination.
func (r WebhookReceiver) verifyThenRelease(ctx context.Context, request WebhookReceiveRequest, signed signedHeaders, file filestore.ScratchReplayFile) (InboundObservation, error) {
	media, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	mac := hmac.New(sha256.New, r.state.secret.key)
	_, _ = io.WriteString(mac, signed.delivery.value)
	_, _ = io.WriteString(mac, core.LoopsWebhookSignedContentSeparator)
	_, _ = io.WriteString(mac, signed.timestampText)
	_, _ = io.WriteString(mac, core.LoopsWebhookSignedContentSeparator)
	var buffer [exchange.TransferBufferBytes]byte
	received, err := exchange.ReceiveStream(exchange.StreamReceiveCall{
		Call: request.Call, ExpectedContentType: media, Buffer: buffer[:],
		Destination: io.MultiWriter(file, mac),
		Route:       exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
	})
	if err != nil {
		return InboundObservation{}, err
	}
	var digest [sha256.Size]byte
	if err := verifySignatureDigest(signed.signature, mac.Sum(digest[:0])); err != nil {
		return InboundObservation{}, err
	}
	extent, err := received.Bytes.Int64()
	if err != nil {
		return InboundObservation{}, err
	}
	written, err := writeVerifiedBody(ctx, request.Destination, io.NewSectionReader(file, 0, extent), extent)
	return InboundObservation{DeliveryID: signed.delivery, SignedAt: signed.signedAt, Bytes: written, ObservedAt: request.ObservedAt}, err
}

func (signedHeaders) loopsInternalFlow() {}

func uniqueHeader(call exchange.SocketServerCall, name string, maximum uint64) (string, error) {
	header, err := core.ParseHTTPHeaderName(name)
	if err != nil {
		return "", contractError(err)
	}
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return "", contractError(err)
	}
	value, err := call.UniqueHeader(header, limit)
	if err != nil {
		return "", authenticationError(err)
	}
	text, err := value.Value()
	if err != nil {
		return "", authenticationError(err)
	}
	return text, nil
}

func writeVerifiedBody(ctx context.Context, destination io.Writer, source io.Reader, extent int64) (core.ByteLength, error) {
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
	_ core.Validatable = InboundObservation{}
	_ core.Validatable = WebhookReceiveRequest{}
	_ core.Validatable = WebhookReceiver{}
	_ internalFlow     = signedHeaders{}
)
