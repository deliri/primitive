package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

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

type WebhookReceiveRequest struct {
	// ScratchParent is caller-owned placement for provisional bytes. Filestore
	// creates and removes a private child namespace; no body extent is limited.
	ScratchParent core.AbsolutePath
	Call          exchange.SocketServerCall
	Destination   io.Writer
	ObservedAt    temporal.Instant
	Tolerance     temporal.Duration
}

func (r WebhookReceiveRequest) Validate() error {
	if r.ScratchParent.Validate() != nil || r.Call.Validate() != nil || core.WriterIsNil(r.Destination) || r.ObservedAt.Validate() != nil || r.Tolerance.Validate() != nil || r.Tolerance.IsZero() {
		return core.ErrStripeContract
	}
	return nil
}

type webhookReceiverState struct {
	secret WebhookSecret
}

type WebhookReceiver struct{ state *webhookReceiverState }

func NewWebhookReceiver(secret WebhookSecret) (WebhookReceiver, error) {
	if err := secret.Validate(); err != nil {
		return WebhookReceiver{}, contractError(err)
	}
	owned, err := ParseWebhookSecret(secret.value)
	if err != nil {
		return WebhookReceiver{}, err
	}
	return WebhookReceiver{state: &webhookReceiverState{secret: owned}}, nil
}

func (r WebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrStripeContract
	}
	if err := r.state.secret.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (r *WebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrStripeContract
	}
	err := r.state.secret.Close()
	r.state = nil
	return err
}

func (r WebhookReceiver) Receive(request WebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	media, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	signature, err := uniqueHeader(request.Call, core.StripeWebhookSignatureHeaderName, core.StripeWebhookSignatureMaximumBytes)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	seconds, err := signatureSeconds(signature)
	if err != nil {
		return InboundObservation{}, verificationError(err)
	}
	if err := validateSignatureTime(signature, request.ObservedAt, request.Tolerance); err != nil {
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
			path, err := core.ParseRelativePath(core.StripeWebhookScratchFilename)
			if err != nil {
				return err
			}
			result, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{
				Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600},
				Use: func(ctx context.Context, file filestore.ScratchReplayFile) error {
					mac := hmac.New(sha256.New, r.state.secret.value)
					var timestamp [20]byte
					_, _ = mac.Write(strconv.AppendInt(timestamp[:0], seconds, 10))
					_, _ = mac.Write([]byte{core.StripeWebhookSignedPayloadSeparator})
					var buffer [exchange.TransferBufferBytes]byte
					received, err := exchange.ReceiveStream(exchange.StreamReceiveCall{
						Call: request.Call, ExpectedContentType: media, Buffer: buffer[:],
						Destination: io.MultiWriter(file, mac),
						Route:       exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
					})
					if err != nil {
						return err
					}
					var digest [sha256.Size]byte
					if err := verifySignatureDigest(signature, mac.Sum(digest[:0])); err != nil {
						return err
					}
					extent, err := received.Bytes.Int64()
					if err != nil {
						return err
					}
					written, err := writeVerifiedBody(ctx, request.Destination, io.NewSectionReader(file, 0, extent), extent)
					observation = InboundObservation{Bytes: written, ObservedAt: request.ObservedAt}
					return err
				},
			})
			return errors.Join(err, result.OperationError(), result.CleanupError())
		},
	})
	if err != nil {
		return observation, err
	}
	return observation, observation.Validate()
}

func uniqueHeader(call exchange.SocketServerCall, name string, maximum uint64) (string, error) {
	header, err := core.ParseHTTPHeaderName(name)
	if err != nil {
		return "", authenticationError(err)
	}
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return "", authenticationError(err)
	}
	value, err := call.UniqueHeader(header, limit)
	if err != nil {
		return "", core.ErrStripeAuthentication
	}
	text, err := value.Value()
	if err != nil {
		return "", authenticationError(err)
	}
	return text, nil
}

func validateSignatureTime(header string, observed temporal.Instant, tolerance temporal.Duration) error {
	signed, err := signatureInstant(header)
	if err != nil {
		return err
	}
	comparison, err := observed.Compare(signed)
	if err != nil {
		return err
	}
	var age temporal.Duration
	if comparison == core.ComparisonLess {
		age, err = signed.Since(observed)
	} else {
		age, err = observed.Since(signed)
	}
	if err != nil {
		return err
	}
	order, err := age.Compare(tolerance)
	if err != nil || order == core.ComparisonGreater {
		return errors.Join(core.ErrStripeVerification, err)
	}
	return nil
}

func signatureInstant(header string) (temporal.Instant, error) {
	seconds, err := signatureSeconds(header)
	if err != nil {
		return temporal.Instant{}, err
	}
	nanosecondsPerSecond := int64(temporal.NanosecondsPerSecond)
	if seconds > math.MaxInt64/nanosecondsPerSecond || seconds < math.MinInt64/nanosecondsPerSecond {
		return temporal.Instant{}, core.ErrStripeVerification
	}
	return temporal.InstantFromNanoseconds(seconds * nanosecondsPerSecond), nil
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
)
