package stripe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	stripesdk "github.com/stripe/stripe-go/v86"
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
	Call        exchange.SocketServerCall
	Destination io.Writer
	ObservedAt  temporal.Instant
	Tolerance   temporal.Duration
}

func (r WebhookReceiveRequest) Validate() error {
	if r.Call.Validate() != nil || r.Destination == nil || r.ObservedAt.Validate() != nil || r.Tolerance.Validate() != nil || r.Tolerance.IsZero() {
		return core.ErrStripeContract
	}
	return nil
}

type webhookReceiverState struct {
	secret  WebhookSecret
	maximum core.ByteCount
}

type WebhookReceiver struct{ state *webhookReceiverState }

func NewWebhookReceiver(secret WebhookSecret, maximum core.ByteCount) (WebhookReceiver, error) {
	if err := errors.Join(secret.Validate(), validateWebhookMaximum(maximum)); err != nil {
		return WebhookReceiver{}, contractError(err)
	}
	owned, err := ParseWebhookSecret(secret.value)
	if err != nil {
		return WebhookReceiver{}, err
	}
	return WebhookReceiver{state: &webhookReceiverState{secret: owned, maximum: maximum}}, nil
}

func (r WebhookReceiver) Validate() error {
	if r.state == nil {
		return core.ErrStripeContract
	}
	if err := errors.Join(r.state.secret.Validate(), validateWebhookMaximum(r.state.maximum)); err != nil {
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
	received, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{
		Call:                request.Call,
		ExpectedContentType: media,
		Policy:              exchange.ServerBoundedPolicy{RequestBodyLimit: r.state.maximum},
		Route:               exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
	})
	if err != nil {
		return InboundObservation{}, err
	}
	signature, err := uniqueHeader(request.Call, core.StripeWebhookSignatureHeaderName, core.StripeWebhookSignatureMaximumBytes)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	if err := stripesdk.ValidatePayload(received.Body, signature, string(r.state.secret.value), stripesdk.WithIgnoreTolerance()); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	if err := validateSignatureTime(signature, request.ObservedAt, request.Tolerance); err != nil {
		return InboundObservation{}, verificationError(err)
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	written, err := writeBody(ctx, request.Destination, received.Body)
	observation := InboundObservation{Bytes: written, ObservedAt: request.ObservedAt}
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
	nanosecondsPerSecond := int64(temporal.NanosecondsPerSecond)
	var seconds int64
	found := false
	for member := range strings.SplitSeq(header, ",") {
		if !strings.HasPrefix(member, "t=") {
			continue
		}
		if found {
			return temporal.Instant{}, core.ErrStripeVerification
		}
		parsed, err := strconv.ParseInt(strings.TrimPrefix(member, "t="), 10, 64)
		if err != nil {
			return temporal.Instant{}, verificationError(err)
		}
		if parsed > math.MaxInt64/nanosecondsPerSecond || parsed < math.MinInt64/nanosecondsPerSecond {
			return temporal.Instant{}, core.ErrStripeVerification
		}
		seconds = parsed
		found = true
	}
	if !found {
		return temporal.Instant{}, core.ErrStripeVerification
	}
	return temporal.InstantFromNanoseconds(seconds * nanosecondsPerSecond), nil
}

func writeBody(ctx context.Context, destination io.Writer, body []byte) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	var buffer [exchange.TransferBufferBytes]byte
	written, err := io.CopyBuffer(destination, bytes.NewReader(body), buffer[:])
	count, countErr := core.CheckedUint64FromInt64(written)
	length, lengthErr := core.NewByteLength(count)
	if joined := errors.Join(err, countErr, lengthErr, contextstate.Validate(ctx)); joined != nil {
		return length, joined
	}
	if written != int64(len(body)) {
		return length, io.ErrShortWrite
	}
	return length, nil
}

func validateWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value == 0 || value > core.StripeWebhookCustodyMaximumBytes {
		return errors.Join(core.ErrStripeContract, err)
	}
	return nil
}

var (
	_ core.Validatable = InboundObservation{}
	_ core.Validatable = WebhookReceiveRequest{}
	_ core.Validatable = WebhookReceiver{}
)
