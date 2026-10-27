package plunk

import (
	"bytes"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// WebhookJSONRequest borrows one HTTP ingress and selects the caller's complete
// JSON extent policy. The receiver and request remain the caller's capabilities.
type WebhookJSONRequest struct {
	Call       exchange.SocketServerCall
	Limits     core.StrictJSONLimits
	ObservedAt temporal.Instant
}

func (r WebhookJSONRequest) Validate() error {
	if err := r.Call.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.Limits.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.ObservedAt.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// WebhookDocument carries an authenticated structural document, not an accepted
// application event. Plunk workflow custom bodies replace the default envelope;
// their schema, business validation and side effects belong to the caller.
type WebhookDocument[Payload any] struct {
	Body        Payload
	Observation InboundObservation
}

// ReceiveWebhookJSON authenticates before reading or decoding the body. It
// borrows receiver without closing or changing it. After authentication, the
// existing Exchange receive owns exactly one body close, including failures.
// Authentication refusal leaves the unread body with its existing caller.
// Every refusal returns zero facts; typed parser/IO/context causes survive.
func ReceiveWebhookJSON[Payload any](receiver WebhookReceiver, request WebhookJSONRequest) (WebhookDocument[Payload], error) {
	if err := request.Validate(); err != nil {
		return WebhookDocument[Payload]{}, err
	}
	ctx, err := request.Call.Context()
	if err != nil {
		return WebhookDocument[Payload]{}, contractError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return WebhookDocument[Payload]{}, contractError(err)
	}
	destination := webhookJSONBuffer{maximum: request.Limits.DocumentMaximumBytes}
	observation, err := receiver.Receive(WebhookReceiveRequest{Call: request.Call, Destination: &destination, ObservedAt: request.ObservedAt})
	if err != nil {
		return WebhookDocument[Payload]{}, wireDocumentError(err)
	}
	payload, err := core.DecodeStrictJSONStructure[Payload](destination.buffer.Bytes(), request.Limits)
	if err != nil {
		return WebhookDocument[Payload]{}, wireDocumentError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return WebhookDocument[Payload]{}, contractError(err)
	}
	return WebhookDocument[Payload]{Body: payload, Observation: observation}, nil
}

// webhookJSONBuffer aggregates only the one caller-bounded document requested
// by a typed JSON API; it never sets a product transfer quota.
type webhookJSONBuffer struct {
	buffer  bytes.Buffer
	maximum core.ByteCount
}

func (b *webhookJSONBuffer) Write(raw []byte) (int, error) {
	if uint64(len(raw)) > b.maximum.Uint64()-uint64(b.buffer.Len()) {
		return 0, core.ErrExchangeBodyLimit
	}
	return b.buffer.Write(raw)
}

// wireDocumentRefusal keeps native causes inspectable without rendering custom
// body member names, provider values, contact data or other private content.
type wireDocumentRefusal struct{ cause error }

func wireDocumentError(cause error) error      { return &wireDocumentRefusal{cause: cause} }
func (e *wireDocumentRefusal) Error() string   { return core.ErrPlunkContract.Error() }
func (e *wireDocumentRefusal) Unwrap() []error { return []error{core.ErrPlunkContract, e.cause} }
