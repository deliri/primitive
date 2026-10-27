package plunk

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"net/mail"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// SendRequest is the documented single-recipient POST /v1/send wire document.
// Data remains the caller's compiler-visible application metadata, rather
// than an untyped payload bag. The caller owns its content and byte policy.
type SendRequest[Data any] struct {
	Data     *Data  `json:"data,omitempty"`
	Template string `json:"template,omitempty"`
	From     string `json:"from,omitempty"`
	Reply    string `json:"reply,omitempty"`
	To       string `json:"to"`
	Subject  string `json:"subject,omitempty"`
	Body     string `json:"body,omitempty"`
}

func (r SendRequest[Data]) Validate() error {
	if _, err := mail.ParseAddress(r.To); err != nil {
		return contractError(err)
	}
	for _, address := range [...]string{r.From, r.Reply} {
		if address != "" {
			if _, err := mail.ParseAddress(address); err != nil {
				return contractError(err)
			}
		}
	}
	if r.Template == "" && (strings.TrimSpace(r.Subject) == "" || strings.TrimSpace(r.Body) == "") {
		return core.ErrPlunkContract
	}
	return nil
}

// EncodeSendRequest emits provider wire bytes. It adds no delivery policy,
// retry, implicit subscription or transfer ceiling.
func EncodeSendRequest[Data any](request SendRequest[Data]) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, contractError(err)
	}
	return encoded, nil
}

// SendStatus is Plunk's optional per-recipient outcome. An absent member is
// Unreported; a present member admits only the provider's three spellings.
type SendStatus uint8

const (
	SendStatusUnreported SendStatus = iota
	SendStatusQueued
	SendStatusScheduled
	SendStatusThrottled
	sendStatusLimit
)

func (s SendStatus) Validate() error {
	if s >= sendStatusLimit {
		return core.ErrPlunkContract
	}
	return nil
}

func (s SendStatus) String() string {
	switch s {
	case SendStatusUnreported:
		return "unreported"
	case SendStatusQueued:
		return "queued"
	case SendStatusScheduled:
		return "scheduled"
	case SendStatusThrottled:
		return "throttled"
	default:
		return "invalid"
	}
}

func (s SendStatus) MarshalJSON() ([]byte, error) {
	if s == SendStatusUnreported || s.Validate() != nil {
		return nil, core.ErrPlunkContract
	}
	return core.MarshalCanonicalJSONString(s.String())
}

func (s *SendStatus) UnmarshalJSON(raw []byte) error {
	if s == nil {
		return core.ErrPlunkContract
	}
	text, err := core.DecodeJSONStringToken(raw)
	if err != nil {
		return contractError(err)
	}
	for value := SendStatusQueued; value < sendStatusLimit; value++ {
		if value.String() == text {
			*s = value
			return nil
		}
	}
	return core.ErrPlunkContract
}

// SendTimestamp owns Plunk's RFC3339 JSON spelling and retains the parsed
// Temporal instant. The zero value is absence/null, never an observed instant.
type SendTimestamp struct{ instant temporal.Instant }

func NewSendTimestamp(instant temporal.Instant) (SendTimestamp, error) {
	if err := instant.Validate(); err != nil {
		return SendTimestamp{}, contractError(err)
	}
	return SendTimestamp{instant: instant}, nil
}

func (t SendTimestamp) Instant() temporal.Instant { return t.instant }
func (t SendTimestamp) IsZero() bool              { return t.instant == (temporal.Instant{}) }

func (t SendTimestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	text, err := t.instant.RFC3339Nano()
	if err != nil {
		return nil, contractError(err)
	}
	return core.MarshalCanonicalJSONString(text)
}

func (t *SendTimestamp) UnmarshalJSON(raw []byte) error {
	if t == nil {
		return core.ErrPlunkContract
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		*t = SendTimestamp{}
		return nil
	}
	text, err := core.DecodeJSONStringToken(raw)
	if err != nil {
		return contractError(err)
	}
	instant, err := temporal.ParseRFC3339(text)
	if err != nil {
		return contractError(err)
	}
	*t = SendTimestamp{instant: instant}
	return nil
}

type SendResponse struct {
	Error   SendError `json:"error"`
	Data    SendData  `json:"data"`
	Success bool      `json:"success"`
}

type SendData struct {
	Timestamp SendTimestamp `json:"timestamp"`
	Emails    []SentEmail   `json:"emails,format:emitnull"`
}

type SentEmail struct {
	Contact       SendContact   `json:"contact"`
	EmailID       string        `json:"email"`
	ScheduledFor  SendTimestamp `json:"scheduledFor,omitzero"`
	NextAllowedAt SendTimestamp `json:"nextAllowedAt,omitzero"`
	Status        SendStatus    `json:"status,omitzero"`
}

type SendContact struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type SendError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r SendResponse) Validate() error {
	if !r.Success {
		return nil
	}
	if err := r.Data.Timestamp.Instant().Validate(); err != nil {
		return contractError(err)
	}
	for _, item := range r.Data.Emails {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (e SentEmail) Validate() error {
	switch e.Status {
	case SendStatusUnreported, SendStatusQueued:
		if e.EmailID == "" || !e.ScheduledFor.IsZero() || !e.NextAllowedAt.IsZero() {
			return core.ErrPlunkContract
		}
	case SendStatusScheduled:
		if e.EmailID == "" || e.ScheduledFor.IsZero() || !e.NextAllowedAt.IsZero() {
			return core.ErrPlunkContract
		}
	case SendStatusThrottled:
		if e.EmailID != "" || !e.ScheduledFor.IsZero() {
			return core.ErrPlunkContract
		}
	default:
		return core.ErrPlunkContract
	}
	return nil
}

// DecodeSendResponse owns strict provider grammar, statuses, RFC3339 and safe
// shape diagnostics. The caller already owns the whole input value and its
// allocation/extent policy; no SDK transfer ceiling or recipient quota exists.
// Recipient/cardinality acceptance and durable identifiers remain application
// policy. Every refusal returns zero provider facts and preserves native cause.
func DecodeSendResponse(ctx context.Context, raw []byte, limits core.StrictJSONLimits) (SendResponse, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return SendResponse{}, contractError(err)
	}
	response, err := core.DecodeStrictJSONStructure[SendResponse](raw, limits)
	if err != nil {
		cause := contractError(err)
		if semantic, ok := errors.AsType[*json.SemanticError](err); ok {
			return SendResponse{}, newSendResponseShapeError(ctx, raw, semantic, cause)
		}
		return SendResponse{}, cause
	}
	if err := response.Validate(); err != nil {
		return SendResponse{}, err
	}
	return response, nil
}
