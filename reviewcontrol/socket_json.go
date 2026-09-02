package reviewcontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/proofledger"
)

const SocketDocumentMaximumBytes = 1 << 20

const (
	eventEnvelopeFramingMaximumBytes      = 24 << 10
	pageFramingMaximumBytes               = 16 << 10
	_                                uint = proofledger.EventJSONMaximumBytes - EventPayloadJSONMaximumBytes - eventEnvelopeFramingMaximumBytes
	_                                uint = SocketDocumentMaximumBytes - proofledger.PageJSONMaximumBytes - pageFramingMaximumBytes
	_                                uint = core.JSONDocumentMaximumBytes - SocketDocumentMaximumBytes
)

type issueReviewRequestWire IssueReviewRequest
type issueReviewResponseWire IssueReviewResponse
type readReviewRequestWire ReadReviewRequest
type readReviewResponseWire ReadReviewResponse
type recordObservationRequestWire RecordObservationRequest
type recordObservationResponseWire RecordObservationResponse
type recordDecisionRequestWire RecordDecisionRequest
type recordDecisionResponseWire RecordDecisionResponse
type readEventsRequestWire ReadEventsRequest
type readEventsResponseWire ReadEventsResponse
type readProjectionRequestWire ReadProjectionRequest
type readProjectionResponseWire ReadProjectionResponse

func decodeSocketDocument[W any, D core.Validatable](data []byte, convert func(W) D) (D, error) {
	var zero D
	wire, err := core.DecodeStrictJSONStructure[W](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return zero, jsonError(err)
	}
	candidate := convert(wire)
	if err := candidate.Validate(); err != nil {
		return zero, jsonError(err)
	}
	return candidate, nil
}

func validateSocketDocument[W any](wire W, causes ...error) error {
	if err := errors.Join(causes...); err != nil {
		return validateContract(err)
	}
	return validateContract(validateEncodedDocument(wire, SocketDocumentMaximumBytes))
}

func (r IssueReviewRequest) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, issueReviewRequestWire(r), SocketDocumentMaximumBytes)
}
func (r IssueReviewResponse) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, issueReviewResponseWire(r), SocketDocumentMaximumBytes)
}
func (r ReadReviewRequest) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, readReviewRequestWire(r), SocketDocumentMaximumBytes)
}
func (r ReadReviewResponse) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, readReviewResponseWire(r), SocketDocumentMaximumBytes)
}
func (r RecordObservationRequest) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, recordObservationRequestWire(r), SocketDocumentMaximumBytes)
}
func (r RecordObservationResponse) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, recordObservationResponseWire(r), SocketDocumentMaximumBytes)
}
func (r RecordDecisionRequest) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, recordDecisionRequestWire(r), SocketDocumentMaximumBytes)
}
func (r RecordDecisionResponse) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, recordDecisionResponseWire(r), SocketDocumentMaximumBytes)
}
func (r ReadEventsRequest) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, readEventsRequestWire(r), SocketDocumentMaximumBytes)
}
func (r ReadEventsResponse) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, readEventsResponseWire(r), SocketDocumentMaximumBytes)
}
func (r ReadProjectionRequest) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, readProjectionRequestWire(r), SocketDocumentMaximumBytes)
}
func (r ReadProjectionResponse) MarshalJSON() ([]byte, error) {
	return marshalDocument(r, readProjectionResponseWire(r), SocketDocumentMaximumBytes)
}

func (r *IssueReviewRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w issueReviewRequestWire) IssueReviewRequest { return IssueReviewRequest(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *IssueReviewResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w issueReviewResponseWire) IssueReviewResponse { return IssueReviewResponse(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *ReadReviewRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w readReviewRequestWire) ReadReviewRequest { return ReadReviewRequest(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *ReadReviewResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w readReviewResponseWire) ReadReviewResponse { return ReadReviewResponse(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *RecordObservationRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w recordObservationRequestWire) RecordObservationRequest { return RecordObservationRequest(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *RecordObservationResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w recordObservationResponseWire) RecordObservationResponse { return RecordObservationResponse(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *RecordDecisionRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w recordDecisionRequestWire) RecordDecisionRequest { return RecordDecisionRequest(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *RecordDecisionResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w recordDecisionResponseWire) RecordDecisionResponse { return RecordDecisionResponse(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *ReadEventsRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w readEventsRequestWire) ReadEventsRequest { return ReadEventsRequest(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *ReadEventsResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w readEventsResponseWire) ReadEventsResponse { return ReadEventsResponse(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *ReadProjectionRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w readProjectionRequestWire) ReadProjectionRequest { return ReadProjectionRequest(w) })
	if e == nil {
		*r = c
	}
	return e
}
func (r *ReadProjectionResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	c, e := decodeSocketDocument(data, func(w readProjectionResponseWire) ReadProjectionResponse { return ReadProjectionResponse(w) })
	if e == nil {
		*r = c
	}
	return e
}
