package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	sendEndpoint             = "https://gmail.googleapis.com/upload/gmail/v1/users/"
	sendDigestHeader         = "X-Primitive-Content-SHA256"
	sendResponseMaximumBytes = 16 << 10
)

func (c Client) SendMessage(ctx context.Context, r MessageSendRequest) (SendAcceptance, error) {
	if err := c.admit(ctx, r); err != nil {
		return SendAcceptance{}, err
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: r.Policy.OperationTimeout})
	if err != nil {
		return SendAcceptance{}, err
	}
	defer cancel()
	return c.sendPrepared(ctx, r)
}

type sendMetadata struct {
	Thread ThreadID `json:"threadId,omitempty"`
}

type sendAcceptanceWire struct {
	ID     MessageID        `json:"id"`
	Thread ThreadID         `json:"threadId"`
	Date   internalDateWire `json:"internalDate"`
}

func (w sendAcceptanceWire) project(expected ThreadID) (SendAcceptance, error) {
	if expected != "" && w.Thread != expected {
		return SendAcceptance{}, errors.Join(core.ErrGmailSendUnconfirmed, core.ErrGmailBinding)
	}
	got := SendAcceptance{Message: w.ID, Thread: w.Thread}
	if w.Date != "" {
		at, err := projectDate(w.Date)
		if err != nil {
			return SendAcceptance{}, errors.Join(core.ErrGmailSendUnconfirmed, err)
		}
		got.At = at
	}
	if err := got.Validate(); err != nil {
		return SendAcceptance{}, errors.Join(core.ErrGmailSendUnconfirmed, err)
	}
	return got, nil
}

func (c Client) sendPrepared(ctx context.Context, r MessageSendRequest) (SendAcceptance, error) {
	body, extent, media, err := sendMedia(r.Prepared)
	if err != nil {
		return SendAcceptance{}, err
	}
	target, err := core.ParseHTTPEndpoint(sendEndpoint + "me/messages/send?uploadType=multipart&fields=id,threadId,internalDate")
	if err != nil {
		return SendAcceptance{}, core.ErrGmailContract
	}
	authorization, err := exchange.StandardHeaderAuthorization.Name()
	if err != nil {
		return SendAcceptance{}, core.ErrGmailContract
	}
	bearer, err := r.Access.access.token.CopyBytes()
	if err != nil {
		return SendAcceptance{}, errors.Join(core.ErrGmailConsent, err)
	}
	defer clear(bearer)
	success := responseBuffer{limit: sendResponseMaximumBytes}
	failure := responseBuffer{limit: sendResponseMaximumBytes}
	defer success.clear()
	defer failure.clear()
	observation, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{
		Context: ctx, Client: c.network, Policy: streamPolicy(r.Policy),
		Request: exchange.StreamRoundTripRequest{
			Target: target, Source: body, RequestContentLength: &extent, RequestContentType: media,
			Semantics:      exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
			SecretHeaders:  []exchange.SecretHeader{{Name: authorization, Value: bearer, Scheme: exchange.SecretHeaderBearer}},
			ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(),
			Destination: &success, FailureDestination: &failure,
		},
	})
	if err != nil {
		code, _ := observation.Metadata.Status.Int()
		return SendAcceptance{}, classifySendFailure(code, failure.data, err)
	}
	return decodeSendAcceptance(success.data, r.Prepared.thread)
}

func decodeSendAcceptance(data []byte, thread ThreadID) (SendAcceptance, error) {
	if len(data) > sendResponseMaximumBytes {
		return SendAcceptance{}, errors.Join(core.ErrGmailSendUnconfirmed, core.ErrExchangeBodyLimit)
	}
	wire, err := decodeProvider[sendAcceptanceWire](data)
	if err != nil {
		return SendAcceptance{}, errors.Join(core.ErrGmailSendUnconfirmed, err)
	}
	return wire.project(thread)
}

func sendMedia(message PreparedMessage) (io.Reader, core.ByteLength, core.HTTPMediaType, error) {
	digest, err := message.digest.Hex()
	if err != nil {
		return nil, core.ByteLength{}, core.HTTPMediaType{}, err
	}
	// Materialize only the small envelope. Exchange borrows the prepared
	// RFC message directly, including an optional PDF, without copying it.
	var envelope bytes.Buffer
	writer := multipart.NewWriter(&envelope)
	if err := writer.SetBoundary("gmail-" + digest[:32]); err != nil {
		return nil, core.ByteLength{}, core.HTTPMediaType{}, err
	}
	media, err := core.ParseHTTPMediaType(mime.FormatMediaType("multipart/related", map[string]string{"boundary": writer.Boundary()}))
	if err != nil {
		return nil, core.ByteLength{}, core.HTTPMediaType{}, err
	}
	if err := writeSendEnvelope(writer, message.thread); err != nil {
		return nil, core.ByteLength{}, core.HTTPMediaType{}, err
	}
	separator := envelope.Len()
	if err := writer.Close(); err != nil {
		return nil, core.ByteLength{}, core.HTTPMediaType{}, err
	}
	length, err := core.NewByteLength(uint64(envelope.Len() + len(message.raw)))
	if err != nil {
		return nil, core.ByteLength{}, core.HTTPMediaType{}, err
	}
	return io.MultiReader(bytes.NewReader(envelope.Bytes()[:separator]), bytes.NewReader(message.raw), bytes.NewReader(envelope.Bytes()[separator:])), length, media, nil
}

func writeSendEnvelope(writer *multipart.Writer, thread ThreadID) error {
	metadata, err := writer.CreatePart(mimeHeaders("application/json; charset=UTF-8", "", ""))
	if err != nil {
		return err
	}
	if err := json.NewEncoder(metadata).Encode(sendMetadata{Thread: thread}); err != nil {
		return err
	}
	_, err = writer.CreatePart(mimeHeaders("message/rfc822", "", ""))
	return err
}

type sendFaultReason string

const (
	sendFaultRateLimit    sendFaultReason = "rateLimitExceeded"
	sendFaultUserRate     sendFaultReason = "userRateLimitExceeded"
	sendFaultDailyLimit   sendFaultReason = "dailyLimitExceeded"
	sendFaultQuota        sendFaultReason = "quotaExceeded"
	sendFaultBadRecipient sendFaultReason = "invalidRecipient"
	sendFaultDomainPolicy sendFaultReason = "domainPolicy"
)

type sendFaultWire struct {
	Error sendFaultDetails `json:"error"`
}

type sendFaultDetails struct {
	Code    int             `json:"code"`
	Reasons []sendFaultItem `json:"errors"`
}

type sendFaultItem struct {
	Reason sendFaultReason `json:"reason"`
}

func (f sendFaultDetails) classify(status int) error {
	if f.Code != status || len(f.Reasons) > 16 {
		return core.ErrGmailSendRefused
	}
	for _, item := range f.Reasons {
		switch item.Reason {
		case sendFaultRateLimit, sendFaultUserRate, sendFaultDailyLimit, sendFaultQuota:
			return errors.Join(core.ErrGmailSendRefused, core.ErrExchangeRateLimited)
		case sendFaultDomainPolicy:
			if status == http.StatusForbidden {
				return errors.Join(core.ErrGmailSendRefused, core.ErrGmailPolicy)
			}
		case sendFaultBadRecipient:
			if status == http.StatusBadRequest {
				return errors.Join(core.ErrGmailSendRefused, core.ErrGmailRecipient)
			}
		default:
			continue
		}
	}
	return core.ErrGmailSendRefused
}

func classifySendFailure(status int, payload []byte, cause error) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.Join(core.ErrGmailSendRefused, core.ErrGmailConsent)
	case http.StatusTooManyRequests:
		return errors.Join(core.ErrGmailSendRefused, core.ErrExchangeRateLimited)
	case http.StatusRequestEntityTooLarge:
		return errors.Join(core.ErrGmailSendRefused, core.ErrExchangeBodyLimit)
	case http.StatusBadRequest, http.StatusForbidden:
		wire, _ := decodeProvider[sendFaultWire](payload)
		err := wire.Error.classify(status)
		if status == http.StatusForbidden && !errors.Is(err, core.ErrExchangeRateLimited) && !errors.Is(err, core.ErrGmailPolicy) {
			return errors.Join(err, core.ErrGmailConsent)
		}
		return err
	case http.StatusNotFound:
		return errors.Join(core.ErrGmailSendRefused, core.ErrGmailBinding)
	default:
		return errors.Join(core.ErrGmailSendUnconfirmed, cancellationOnly(cause))
	}
}

func cancellationOnly(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		return context.Canceled
	default:
		return nil
	}
}

func (sendMetadata) gmailInternalFlow()       {}
func (sendAcceptanceWire) gmailInternalFlow() {}
func (sendFaultWire) gmailInternalFlow()      {}
func (sendFaultDetails) gmailInternalFlow()   {}
func (sendFaultItem) gmailInternalFlow()      {}
