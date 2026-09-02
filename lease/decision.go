package lease

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// GrantCanonicalJSONMaximumBytes is the exact compact grant maximum.
	GrantCanonicalJSONMaximumBytes = len(
		`{"not_before":,"contact_after":,"not_after":,"good_until":}`,
	) + 4*temporal.InstantCanonicalJSONMaximumBytes
	grantJSONWhitespaceAllowance = 1 << 10
	// GrantJSONMaximumBytes bounds accepted grant JSON.
	GrantJSONMaximumBytes = GrantCanonicalJSONMaximumBytes +
		grantJSONWhitespaceAllowance

	// RefusalCanonicalJSONMaximumBytes is the exact compact refusal maximum.
	RefusalCanonicalJSONMaximumBytes = len(`{"contact_after":}`) +
		temporal.InstantCanonicalJSONMaximumBytes
	refusalJSONWhitespaceAllowance = 1 << 10
	// RefusalJSONMaximumBytes bounds accepted refusal JSON.
	RefusalJSONMaximumBytes = RefusalCanonicalJSONMaximumBytes +
		refusalJSONWhitespaceAllowance

	// RevocationCanonicalJSONMaximumBytes is the exact compact revocation maximum.
	RevocationCanonicalJSONMaximumBytes = len(`{"reason":}`) +
		RevocationReasonCanonicalJSONMaximumBytes
	revocationJSONWhitespaceAllowance = 256
	// RevocationJSONMaximumBytes bounds accepted revocation JSON.
	RevocationJSONMaximumBytes = RevocationCanonicalJSONMaximumBytes +
		revocationJSONWhitespaceAllowance

	decisionCommonCanonicalJSONBytes = len(
		`{"revision":,"subject":,"generation":,"issued_at":,"outcome":,"body":}`,
	) + RevisionCanonicalJSONMaximumBytes +
		SubjectCanonicalJSONMaximumBytes +
		GenerationCanonicalJSONMaximumBytes +
		temporal.InstantCanonicalJSONMaximumBytes
	decisionRefusalCanonicalJSONMaximumBytes = decisionCommonCanonicalJSONBytes +
		len(`"`+outcomeRefusalToken+`"`) + RefusalCanonicalJSONMaximumBytes
	decisionRevocationCanonicalJSONMaximumBytes = decisionCommonCanonicalJSONBytes +
		len(`"`+outcomeRevocationToken+`"`) + RevocationCanonicalJSONMaximumBytes
	// DecisionCanonicalJSONMaximumBytes is the exact compact decision maximum.
	DecisionCanonicalJSONMaximumBytes = decisionCommonCanonicalJSONBytes +
		len(`"`+outcomeGrantToken+`"`) + GrantCanonicalJSONMaximumBytes
	decisionJSONWhitespaceAllowance = 4 << 10
	// DecisionJSONMaximumBytes bounds accepted decision JSON.
	DecisionJSONMaximumBytes = DecisionCanonicalJSONMaximumBytes +
		decisionJSONWhitespaceAllowance
)

var (
	_ [DecisionCanonicalJSONMaximumBytes - decisionRefusalCanonicalJSONMaximumBytes]struct{}
	_ [DecisionCanonicalJSONMaximumBytes - decisionRevocationCanonicalJSONMaximumBytes]struct{}
)

// Header is the common authenticated identity and sequence of every decision.
type Header struct {
	Subject    Subject          `json:"subject"`
	IssuedAt   temporal.Instant `json:"issued_at"`
	Generation Generation       `json:"generation"`
	Revision   Revision         `json:"revision"`
}

// Validate closes the common decision facts.
func (h Header) Validate() error {
	if err := h.Revision.Validate(); err != nil {
		return contractError(err)
	}
	if err := h.Subject.Validate(); err != nil {
		return contractError(err)
	}
	if err := h.Generation.Validate(); err != nil {
		return contractError(err)
	}
	if err := h.IssuedAt.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Grant is an exact usable lease timeline selected by the issuer.
type Grant struct {
	NotBefore    temporal.Instant `json:"not_before"`
	ContactAfter temporal.Instant `json:"contact_after"`
	NotAfter     temporal.Instant `json:"not_after"`
	GoodUntil    temporal.Instant `json:"good_until"`
}

// Validate owns the grant boundary order.
func (g Grant) Validate() error {
	if err := validateInstants(g.NotBefore, g.ContactAfter, g.NotAfter, g.GoodUntil); err != nil {
		return err
	}
	if err := requireNotAfter(g.NotBefore, g.ContactAfter); err != nil {
		return contractError(errors.New("grant contact precedes activation"), err)
	}
	if err := requireNotAfter(g.ContactAfter, g.NotAfter); err != nil {
		return contractError(errors.New("grant contact follows normal operation"), err)
	}
	if err := requireBefore(g.NotBefore, g.NotAfter); err != nil {
		return contractError(errors.New("grant normal interval is empty"), err)
	}
	if err := requireBefore(g.NotAfter, g.GoodUntil); err != nil {
		return contractError(errors.New("grant continuity interval is empty"), err)
	}
	return nil
}

// MarshalJSON emits exact grant field order.
func (g Grant) MarshalJSON() ([]byte, error) {
	type wire Grant
	return marshalBounded(wire(g), g.Validate, GrantCanonicalJSONMaximumBytes)
}

// UnmarshalJSON accepts one bounded strict grant.
func (g *Grant) UnmarshalJSON(data []byte) error {
	if g == nil {
		return jsonError(errors.New("grant receiver is nil"))
	}
	type wire Grant
	decoded, err := decodeStructure[wire](data, jsonStructureContract{
		maximumBytes: GrantJSONMaximumBytes,
		depth:        1,
		fields:       4,
	})
	if err != nil {
		return err
	}
	candidate := Grant(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*g = candidate
	return nil
}

// Refusal is a recoverable denial and its earliest next contact.
type Refusal struct {
	ContactAfter temporal.Instant `json:"contact_after"`
}

// Validate closes the refusal contact time.
func (r Refusal) Validate() error {
	if err := r.ContactAfter.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// MarshalJSON emits exact refusal field order.
func (r Refusal) MarshalJSON() ([]byte, error) {
	type wire Refusal
	return marshalBounded(wire(r), r.Validate, RefusalCanonicalJSONMaximumBytes)
}

// UnmarshalJSON accepts one bounded strict refusal.
func (r *Refusal) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("refusal receiver is nil"))
	}
	type wire Refusal
	decoded, err := decodeStructure[wire](data, jsonStructureContract{
		maximumBytes: RefusalJSONMaximumBytes,
		depth:        1,
		fields:       1,
	})
	if err != nil {
		return err
	}
	candidate := Refusal(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

// Revocation is one contract-authorized for-cause denial.
type Revocation struct {
	Reason RevocationReason `json:"reason"`
}

// Validate closes the for-cause domain.
func (r Revocation) Validate() error {
	return r.Reason.Validate()
}

// MarshalJSON emits exact revocation field order.
func (r Revocation) MarshalJSON() ([]byte, error) {
	type wire Revocation
	return marshalBounded(wire(r), r.Validate, RevocationCanonicalJSONMaximumBytes)
}

// UnmarshalJSON accepts one bounded strict revocation.
func (r *Revocation) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("revocation receiver is nil"))
	}
	type wire Revocation
	decoded, err := decodeStructure[wire](data, jsonStructureContract{
		maximumBytes: RevocationJSONMaximumBytes,
		depth:        1,
		fields:       1,
	})
	if err != nil {
		return err
	}
	candidate := Revocation(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

// GrantDecisionRequest constructs one signed-grant body.
type GrantDecisionRequest struct {
	Header Header
	Grant  Grant
}

// RefusalDecisionRequest constructs one signed-refusal body.
type RefusalDecisionRequest struct {
	Header  Header
	Refusal Refusal
}

// RevocationDecisionRequest constructs one signed-revocation body.
type RevocationDecisionRequest struct {
	Header     Header
	Revocation Revocation
}

// Decision is one closed grant, refusal, or revocation union.
type Decision struct {
	header     Header
	grant      Grant
	refusal    Refusal
	outcome    Outcome
	revocation Revocation
}

type decisionWire struct {
	Revision   *Revision         `json:"revision"`
	Subject    *Subject          `json:"subject"`
	Generation *Generation       `json:"generation"`
	IssuedAt   *temporal.Instant `json:"issued_at"`
	Outcome    *Outcome          `json:"outcome"`
	// doctrine:local-allowed=external-wire
	Body *jsontext.Value `json:"body"`
}

func (w decisionWire) Validate() error {
	if !w.hasAllFields() {
		return jsonError(errors.New("decision field is missing"))
	}
	header := Header{
		Revision: *w.Revision, Subject: *w.Subject,
		Generation: *w.Generation, IssuedAt: *w.IssuedAt,
	}
	if err := header.Validate(); err != nil {
		return err
	}
	if err := w.Outcome.Validate(); err != nil {
		return err
	}
	if len(*w.Body) == 0 || len(*w.Body) > GrantJSONMaximumBytes {
		return jsonError(errors.New("decision body extent is invalid"))
	}
	return nil
}

func (w decisionWire) hasAllFields() bool {
	return w.Revision != nil &&
		w.Subject != nil &&
		w.Generation != nil &&
		w.IssuedAt != nil &&
		w.Outcome != nil &&
		w.Body != nil
}

// NewGrantDecision constructs one valid grant decision.
func NewGrantDecision(request GrantDecisionRequest) (Decision, error) {
	candidate := Decision{
		header: request.Header, outcome: OutcomeGrant, grant: request.Grant,
	}
	if err := candidate.Validate(); err != nil {
		return Decision{}, err
	}
	return candidate, nil
}

// NewRefusalDecision constructs one valid recoverable refusal.
func NewRefusalDecision(request RefusalDecisionRequest) (Decision, error) {
	candidate := Decision{
		header: request.Header, outcome: OutcomeRefusal, refusal: request.Refusal,
	}
	if err := candidate.Validate(); err != nil {
		return Decision{}, err
	}
	return candidate, nil
}

// NewRevocationDecision constructs one valid for-cause revocation.
func NewRevocationDecision(request RevocationDecisionRequest) (Decision, error) {
	candidate := Decision{
		header: request.Header, outcome: OutcomeRevocation,
		revocation: request.Revocation,
	}
	if err := candidate.Validate(); err != nil {
		return Decision{}, err
	}
	return candidate, nil
}

// Validate proves exactly one selected outcome and every cross-field rule.
func (d Decision) Validate() error {
	if err := d.header.Validate(); err != nil {
		return contractError(err)
	}
	if err := d.outcome.Validate(); err != nil {
		return contractError(err)
	}
	if err := d.validateUnion(); err != nil {
		return err
	}
	return d.validateIssuedAt()
}

func (d Decision) validateUnion() error {
	switch d.outcome {
	case OutcomeGrant:
		if d.refusal != (Refusal{}) || d.revocation != (Revocation{}) {
			return contractError(errors.New("grant decision carries dormant payload"))
		}
		return d.grant.Validate()
	case OutcomeRefusal:
		if d.grant != (Grant{}) || d.revocation != (Revocation{}) {
			return contractError(errors.New("refusal decision carries dormant payload"))
		}
		return d.refusal.Validate()
	case OutcomeRevocation:
		if d.grant != (Grant{}) || d.refusal != (Refusal{}) {
			return contractError(errors.New("revocation decision carries dormant payload"))
		}
		return d.revocation.Validate()
	default:
		return contractError(errors.New(decisionOutcomeUnsupportedText))
	}
}

func (d Decision) validateIssuedAt() error {
	switch d.outcome {
	case OutcomeGrant:
		if err := requireNotAfter(d.header.IssuedAt, d.grant.ContactAfter); err != nil {
			return contractError(errors.New("grant contact precedes issuance"), err)
		}
		return requireNotAfter(d.header.IssuedAt, d.grant.GoodUntil)
	case OutcomeRefusal:
		return requireNotAfter(d.header.IssuedAt, d.refusal.ContactAfter)
	case OutcomeRevocation:
		return nil
	default:
		return contractError(errors.New(decisionOutcomeUnsupportedText))
	}
}

// Header returns a value copy of the common signed facts.
func (d Decision) Header() (Header, error) {
	if err := d.Validate(); err != nil {
		return Header{}, err
	}
	return d.header, nil
}

// Outcome returns the selected variant.
func (d Decision) Outcome() Outcome {
	return d.outcome
}

// Grant returns the grant payload or a contract error for another outcome.
func (d Decision) Grant() (Grant, error) {
	if err := d.Validate(); err != nil {
		return Grant{}, err
	}
	if d.outcome != OutcomeGrant {
		return Grant{}, contractError(errors.New("decision is not a grant"))
	}
	return d.grant, nil
}

// Refusal returns the refusal payload or a contract error for another outcome.
func (d Decision) Refusal() (Refusal, error) {
	if err := d.Validate(); err != nil {
		return Refusal{}, err
	}
	if d.outcome != OutcomeRefusal {
		return Refusal{}, contractError(errors.New("decision is not a refusal"))
	}
	return d.refusal, nil
}

// Revocation returns the revocation payload or a contract error for another outcome.
func (d Decision) Revocation() (Revocation, error) {
	if err := d.Validate(); err != nil {
		return Revocation{}, err
	}
	if d.outcome != OutcomeRevocation {
		return Revocation{}, contractError(errors.New("decision is not a revocation"))
	}
	return d.revocation, nil
}

// AttestationDomain returns Lease's one exact signing domain.
func (Decision) AttestationDomain() Domain {
	return DomainDecisionV1
}

// WriteCanonical writes one fixed-size canonical decision to the supplied
// standard-library writer.
func (d Decision) WriteCanonical(destination io.Writer) error {
	encoded, err := d.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

// MarshalJSON emits one canonical tagged union.
func (d Decision) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	body, err := d.marshalBody()
	if err != nil {
		return nil, err
	}
	revision := d.header.Revision
	subject := d.header.Subject
	generation := d.header.Generation
	issuedAt := d.header.IssuedAt
	outcome := d.outcome
	// doctrine:local-allowed=external-wire
	rawBody := jsontext.Value(body)
	wire := decisionWire{
		Revision: &revision, Subject: &subject,
		Generation: &generation, IssuedAt: &issuedAt,
		Outcome: &outcome, Body: &rawBody,
	}
	encoded, err := json.Marshal(wire)
	if err != nil || len(encoded) > DecisionCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("decision JSON encoding exceeded its contract"), err)
	}
	return encoded, nil
}

func (d Decision) marshalBody() ([]byte, error) {
	switch d.outcome {
	case OutcomeGrant:
		return d.grant.MarshalJSON()
	case OutcomeRefusal:
		return d.refusal.MarshalJSON()
	case OutcomeRevocation:
		return d.revocation.MarshalJSON()
	default:
		return nil, contractError(errors.New(decisionOutcomeUnsupportedText))
	}
}

// UnmarshalJSON accepts one bounded strict tagged union without mutation on
// rejection.
func (d *Decision) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("decision receiver is nil"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: DecisionJSONMaximumBytes,
		depth:        3,
		fields:       6,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSON[decisionWire](bytes.NewReader(data), limits)
	if err != nil {
		return jsonError(err)
	}
	candidate, err := decisionFromWire(wire)
	if err != nil {
		return err
	}
	*d = candidate
	return nil
}

func decisionFromWire(wire decisionWire) (Decision, error) {
	if err := wire.Validate(); err != nil {
		return Decision{}, err
	}
	header := Header{
		Revision: *wire.Revision, Subject: *wire.Subject,
		Generation: *wire.Generation, IssuedAt: *wire.IssuedAt,
	}
	switch *wire.Outcome {
	case OutcomeGrant:
		var body Grant
		if err := json.Unmarshal(*wire.Body, &body); err != nil {
			return Decision{}, err
		}
		return NewGrantDecision(GrantDecisionRequest{Header: header, Grant: body})
	case OutcomeRefusal:
		var body Refusal
		if err := json.Unmarshal(*wire.Body, &body); err != nil {
			return Decision{}, err
		}
		return NewRefusalDecision(RefusalDecisionRequest{Header: header, Refusal: body})
	case OutcomeRevocation:
		var body Revocation
		if err := json.Unmarshal(*wire.Body, &body); err != nil {
			return Decision{}, err
		}
		return NewRevocationDecision(RevocationDecisionRequest{
			Header: header, Revocation: body,
		})
	default:
		return Decision{}, contractError(errors.New(decisionOutcomeUnsupportedText))
	}
}

func marshalBounded[T any](
	value T,
	validate func() error,
	maximum int,
) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > maximum {
		return nil, jsonError(errors.New("lease JSON encoding exceeded its contract"), err)
	}
	return encoded, nil
}

func decodeStructure[T any](
	data []byte,
	contract jsonStructureContract,
) (T, error) {
	var zero T
	limits, err := contract.limits()
	if err != nil {
		return zero, err
	}
	decoded, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return decoded, nil
}

func validateInstants(instants ...temporal.Instant) error {
	for _, instant := range instants {
		if err := instant.Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

func requireBefore(left, right temporal.Instant) error {
	comparison, err := left.Compare(right)
	if err != nil {
		return err
	}
	if comparison != core.ComparisonLess {
		return contractError(errors.New("lease instant is not strictly before its boundary"))
	}
	return nil
}

func requireNotAfter(left, right temporal.Instant) error {
	comparison, err := left.Compare(right)
	if err != nil {
		return err
	}
	if comparison == core.ComparisonGreater {
		return contractError(errors.New("lease instant follows its boundary"))
	}
	return nil
}
