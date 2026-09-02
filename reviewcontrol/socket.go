package reviewcontrol

import (
	"context"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/proofledger"
)

type Operation uint8

const (
	OperationUnknown Operation = iota
	OperationIssueReview
	OperationReadReview
	OperationRecordObservation
	OperationRecordDecision
	OperationReadEvents
	OperationReadProjection
	operationLimit
)

func (o Operation) Validate() error {
	if o <= OperationUnknown || o >= operationLimit {
		return contractError()
	}
	return nil
}

func (o Operation) IsValid() bool { return o.Validate() == nil }
func (o Operation) String() string {
	labels := []string{"", "issue_review", "read_review", "record_observation", "record_decision", "read_events", "read_projection"}
	if !o.IsValid() {
		return ""
	}
	return labels[o]
}
func (o Operation) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(o.String())
}
func (o *Operation) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonError()
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	for candidate := OperationIssueReview; candidate < operationLimit; candidate++ {
		if candidate.String() == value {
			*o = candidate
			return nil
		}
	}
	return jsonError()
}

type IssueReviewRequest struct {
	Request controlwire.RequestNonce `json:"request"`
	Packet  Packet                   `json:"packet"`
}
type IssueReviewResponse struct {
	Receipt proofledger.ReceiptDocument `json:"receipt"`
}
type ReadReviewRequest struct {
	Identity ReviewIdentity `json:"identity"`
}
type ReadReviewResponse struct {
	Packet Packet `json:"packet"`
}
type RecordObservationRequest struct {
	Request     controlwire.RequestNonce `json:"request"`
	Observation Observation              `json:"observation"`
}
type RecordObservationResponse struct {
	Receipt proofledger.ReceiptDocument `json:"receipt"`
}
type RecordDecisionRequest struct {
	Intent DecisionIntent `json:"intent"`
}
type RecordDecisionResponse struct {
	Receipt proofledger.ReceiptDocument `json:"receipt"`
}
type ReadEventsRequest struct {
	Page proofledger.PageRequest `json:"page"`
}
type ReadEventsResponse struct {
	Page proofledger.Page[EventPayload] `json:"page"`
}
type ReadProjectionRequest struct {
	Review ReviewIdentity `json:"review"`
}
type ReadProjectionResponse struct {
	Projection Projection `json:"projection"`
}

func (r IssueReviewRequest) Validate() error {
	return validateSocketDocument(issueReviewRequestWire(r), r.Request.Validate(), r.Packet.Validate())
}
func (r IssueReviewResponse) Validate() error {
	return validateSocketDocument(issueReviewResponseWire(r), r.Receipt.Validate())
}
func (r ReadReviewRequest) Validate() error {
	return validateSocketDocument(readReviewRequestWire(r), r.Identity.Validate())
}
func (r ReadReviewResponse) Validate() error {
	return validateSocketDocument(readReviewResponseWire(r), r.Packet.Validate())
}
func (r RecordObservationRequest) Validate() error {
	return validateSocketDocument(recordObservationRequestWire(r), r.Request.Validate(), r.Observation.Validate())
}
func (r RecordObservationResponse) Validate() error {
	return validateSocketDocument(recordObservationResponseWire(r), r.Receipt.Validate())
}
func (r RecordDecisionRequest) Validate() error {
	return validateSocketDocument(recordDecisionRequestWire(r), r.Intent.Validate())
}
func (r RecordDecisionResponse) Validate() error {
	return validateSocketDocument(recordDecisionResponseWire(r), r.Receipt.Validate())
}
func (r ReadEventsRequest) Validate() error {
	return validateSocketDocument(readEventsRequestWire(r), r.Page.Validate())
}
func (r ReadEventsResponse) Validate() error {
	return validateSocketDocument(readEventsResponseWire(r), r.Page.Validate())
}
func (r ReadProjectionRequest) Validate() error {
	return validateSocketDocument(readProjectionRequestWire(r), r.Review.Validate())
}
func (r ReadProjectionResponse) Validate() error {
	return validateSocketDocument(readProjectionResponseWire(r), r.Projection.Validate())
}

func (r IssueReviewRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := r.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return r.Request.IdempotencyKey()
}

func (r RecordObservationRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := r.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return r.Request.IdempotencyKey()
}

func (r RecordDecisionRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := r.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return r.Intent.Request.IdempotencyKey()
}

type ReviewIssuer interface {
	IssueReview(context.Context, IssueReviewRequest) (IssueReviewResponse, error)
}
type ReviewReader interface {
	ReadReview(context.Context, ReadReviewRequest) (ReadReviewResponse, error)
}
type ObservationRecorder interface {
	RecordObservation(context.Context, RecordObservationRequest) (RecordObservationResponse, error)
}
type HumanDecisionRecorder interface {
	RecordDecision(context.Context, VerifiedHumanAuthority, RecordDecisionRequest) (RecordDecisionResponse, error)
}
type EventReader interface {
	ReadEvents(context.Context, ReadEventsRequest) (ReadEventsResponse, error)
}
type ProjectionReader interface {
	ReadProjection(context.Context, ReadProjectionRequest) (ReadProjectionResponse, error)
}
