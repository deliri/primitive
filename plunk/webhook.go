package plunk

import (
	"crypto/subtle"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
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
	Call        exchange.SocketServerCall
	Destination io.Writer
	ObservedAt  temporal.Instant
}

func (r WebhookReceiveRequest) Validate() error {
	if r.Call.Validate() != nil || r.Destination == nil || r.ObservedAt.Validate() != nil {
		return core.ErrPlunkContract
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
		return core.ErrPlunkContract
	}
	if err := errors.Join(r.state.secret.Validate(), validateWebhookMaximum(r.state.maximum)); err != nil {
		return contractError(err)
	}
	return nil
}

func (r *WebhookReceiver) Close() error {
	if r == nil || r.state == nil {
		return core.ErrPlunkContract
	}
	err := r.state.secret.Close()
	r.state = nil
	return err
}

func (r WebhookReceiver) Receive(request WebhookReceiveRequest) (InboundObservation, error) {
	if err := errors.Join(r.Validate(), request.Validate()); err != nil {
		return InboundObservation{}, contractError(err)
	}
	received, err := exchange.ReceiveBearerAuthorization(request.Call)
	if err != nil {
		return InboundObservation{}, authenticationError(err)
	}
	defer clear(received.Token)
	if subtle.ConstantTimeCompare(received.Token, r.state.secret.value) != 1 {
		return InboundObservation{}, verificationError(core.ErrExchangeContract)
	}
	media, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return InboundObservation{}, contractError(err)
	}
	stream, err := exchange.ReceiveStream(exchange.StreamReceiveCall{
		Destination:         request.Destination,
		Call:                request.Call,
		ExpectedContentType: media,
		Policy:              exchange.ServerStreamPolicy{RequestBodyLimit: r.state.maximum},
		Route:               exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
	})
	observation := InboundObservation{Bytes: stream.Bytes, ObservedAt: request.ObservedAt}
	if err != nil {
		return observation, err
	}
	return observation, observation.Validate()
}

func validateWebhookMaximum(maximum core.ByteCount) error {
	value, err := maximum.Uint64()
	if err != nil || value == 0 || value > core.PlunkWebhookCustodyMaximumBytes {
		return errors.Join(core.ErrPlunkContract, err)
	}
	return nil
}

var (
	_ core.Validatable = InboundObservation{}
	_ core.Validatable = WebhookReceiveRequest{}
	_ core.Validatable = WebhookReceiver{}
)
