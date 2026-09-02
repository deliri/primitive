package submission

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	DecisionDocumentJSONMaximumBytes uint64 = uint64(
		GrantDocumentJSONMaximumBytes + receipt.EvidenceDocumentJSONMaximumBytes,
	)
	decisionTokenUpload = "upload"
	decisionTokenReuse  = "reuse"
)

// DecisionKind closes the only two authority outcomes for one declaration.
type DecisionKind uint8

const (
	DecisionUnknown DecisionKind = iota
	DecisionUpload
	DecisionReuse
	decisionLimit
)

func decisionTokens() [decisionLimit]string {
	return [...]string{"", decisionTokenUpload, decisionTokenReuse}
}

func (k DecisionKind) Validate() error {
	if k <= DecisionUnknown || k >= decisionLimit || decisionTokens()[k] == "" {
		return contractError(errors.New("submission decision kind is invalid"))
	}
	return nil
}

// IsValid reports whether k is one published submission decision kind.
func (k DecisionKind) IsValid() bool { return k.Validate() == nil }

func (k DecisionKind) String() string {
	if k >= decisionLimit {
		return ""
	}
	return decisionTokens()[k]
}

func (k DecisionKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *DecisionKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil submission decision kind receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	for candidate := DecisionUnknown + 1; candidate < decisionLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return jsonError(errors.New("submission decision kind text is unsupported"))
}

// DecisionDocument is the receive-only union. Upload carries a bearer grant;
// reuse carries the authority's already-accepted object evidence.
type DecisionDocument struct {
	Grant    *GrantDocument            `json:"grant,omitempty"`
	Evidence *receipt.EvidenceDocument `json:"evidence,omitempty"`
	Kind     DecisionKind              `json:"kind"`
}

func (d DecisionDocument) Validate() error {
	if err := d.Kind.Validate(); err != nil {
		return err
	}
	switch d.Kind {
	case DecisionUpload:
		if d.Grant == nil || d.Evidence != nil {
			return bindingError(errors.New("upload decision carries reuse evidence"))
		}
		return d.Grant.Validate()
	case DecisionReuse:
		if d.Grant != nil || d.Evidence == nil {
			return bindingError(errors.New("reuse decision carries an upload grant"))
		}
		return d.Evidence.Validate()
	default:
		return contractError(errors.New("submission decision escaped its domain"))
	}
}

func (d *DecisionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil submission decision receiver"))
	}
	type wire DecisionDocument
	decoded, err := decodeStrict[wire](data, DecisionDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := DecisionDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// DecisionProjection is the issue-only union matching DecisionDocument.
type DecisionProjection struct {
	grant    *GrantProjection
	evidence *receipt.EvidenceDocument
	kind     DecisionKind
}

type (
	uploadDecisionProjectionWire struct {
		Grant GrantProjection `json:"grant"`
		Kind  DecisionKind    `json:"kind"`
	}
	reuseDecisionProjectionWire struct {
		Evidence receipt.EvidenceDocument `json:"evidence"`
		Kind     DecisionKind             `json:"kind"`
	}
)

func UploadDecision(grant GrantProjection) (DecisionProjection, error) {
	candidate := DecisionProjection{kind: DecisionUpload, grant: &grant}
	return candidate, candidate.Validate()
}

// ReuseDecisionRequest supplies the exact authenticated tenant scope and
// declaration an authority must prove before it may disclose accepted-object
// evidence instead of issuing a fresh upload grant.
type ReuseDecisionRequest struct {
	Scope       receipt.Scope
	Declaration Declaration
	Evidence    receipt.EvidenceDocument
	TrustedKeys attest.TrustedKeys
}

func (r ReuseDecisionRequest) Validate() error {
	if err := errors.Join(
		r.Evidence.Validate(), r.Declaration.Validate(), r.TrustedKeys.Validate(),
		r.Scope.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func ReuseDecision(request ReuseDecisionRequest) (DecisionProjection, error) {
	verified, err := verifyReuseEvidence(request)
	if err != nil {
		return DecisionProjection{}, err
	}
	evidence, err := verified.Document()
	if err != nil {
		return DecisionProjection{}, bindingError(err)
	}
	candidate := DecisionProjection{kind: DecisionReuse, evidence: &evidence}
	return candidate, candidate.Validate()
}

func (p DecisionProjection) Validate() error {
	if err := p.kind.Validate(); err != nil {
		return err
	}
	switch p.kind {
	case DecisionUpload:
		if p.grant == nil || p.evidence != nil {
			return bindingError(errors.New("upload projection carries reuse evidence"))
		}
		return p.grant.Validate()
	case DecisionReuse:
		if p.grant != nil || p.evidence == nil {
			return bindingError(errors.New("reuse projection carries an upload grant"))
		}
		return p.evidence.Validate()
	default:
		return contractError(errors.New("submission projection escaped its domain"))
	}
}

func (p DecisionProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := p.marshalJSON()
	if err != nil || uint64(len(encoded)) > DecisionDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p DecisionProjection) ValidateJSONProjection(encoded []byte, limits core.StrictJSONLimits) error {
	return core.ValidateReceiveOnlyJSONProjection[DecisionProjection, DecisionDocument, *DecisionDocument](
		p, encoded, limits,
	)
}

func (p DecisionProjection) marshalJSON() ([]byte, error) {
	switch p.kind {
	case DecisionUpload:
		return core.MarshalCanonicalJSONDocument(uploadDecisionProjectionWire{
			Grant: *p.grant,
			Kind:  p.kind,
		})
	case DecisionReuse:
		return core.MarshalCanonicalJSONDocument(reuseDecisionProjectionWire{
			Evidence: *p.evidence,
			Kind:     p.kind,
		})
	default:
		return nil, contractError(errors.New("submission projection escaped encoding"))
	}
}

// DecisionExpectation supplies the authenticated tenant scope separately from
// the content declaration, preventing dedup from becoming a cross-tenant
// existence oracle.
type DecisionExpectation struct {
	Decision    DecisionDocument
	Scope       receipt.Scope
	Request     RequestPayload
	TrustedKeys attest.TrustedKeys
	ObservedAt  temporal.Instant
}

func (e DecisionExpectation) Validate() error {
	if err := errors.Join(
		e.Decision.Validate(), e.Request.Validate(), e.Scope.Validate(),
		e.ObservedAt.Validate(), e.TrustedKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

// VerifiedDecision is a sealed upload-or-reuse proof.
type VerifiedDecision struct {
	grant    *VerifiedGrant
	evidence *receipt.VerifiedEvidence
	kind     DecisionKind
}

func VerifyDecision(expectation DecisionExpectation) (VerifiedDecision, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedDecision{}, err
	}
	switch expectation.Decision.Kind {
	case DecisionUpload:
		grant, err := VerifyGrant(GrantExpectation{
			Request: expectation.Request, Document: *expectation.Decision.Grant,
			ObservedAt: expectation.ObservedAt, TrustedKeys: expectation.TrustedKeys,
		})
		if err != nil {
			return VerifiedDecision{}, err
		}
		verified := VerifiedDecision{kind: DecisionUpload, grant: &grant}
		return verified, verified.Validate()
	case DecisionReuse:
		return verifyReuseDecision(expectation)
	default:
		return VerifiedDecision{}, contractError(errors.New("submission decision escaped verification"))
	}
}

func verifyReuseDecision(expectation DecisionExpectation) (VerifiedDecision, error) {
	evidence := *expectation.Decision.Evidence
	verifiedEvidence, err := verifyReuseEvidence(ReuseDecisionRequest{
		Evidence: evidence, Declaration: expectation.Request.Declaration,
		TrustedKeys: expectation.TrustedKeys, Scope: expectation.Scope,
	})
	if err != nil {
		return VerifiedDecision{}, err
	}
	verified := VerifiedDecision{kind: DecisionReuse, evidence: &verifiedEvidence}
	return verified, verified.Validate()
}

func verifyReuseEvidence(request ReuseDecisionRequest) (receipt.VerifiedEvidence, error) {
	if err := request.Validate(); err != nil {
		return receipt.VerifiedEvidence{}, err
	}
	body := request.Evidence.Payload.Body
	verified, err := receipt.VerifyEvidence(receipt.VerifyEvidenceRequest{
		Document: request.Evidence, TrustedKeys: request.TrustedKeys,
		Expected: receipt.EvidenceExpectation{
			Principal: request.Scope.Principal, Offering: request.Scope.Offering, Body: body,
		},
	})
	if err != nil {
		return receipt.VerifiedEvidence{}, bindingError(err)
	}
	declaration := request.Declaration
	if body.Extent != declaration.Extent || body.SHA256 != declaration.SHA256 ||
		body.CRC32C != declaration.CRC32C {
		return receipt.VerifiedEvidence{}, bindingError(errors.New("reuse evidence differs from declaration"))
	}
	return verified, nil
}

func (v VerifiedDecision) Validate() error {
	if err := v.kind.Validate(); err != nil {
		return err
	}
	switch v.kind {
	case DecisionUpload:
		if v.grant == nil || v.evidence != nil {
			return bindingError(errors.New("verified upload carries reuse proof"))
		}
		return v.grant.Validate()
	case DecisionReuse:
		if v.grant != nil || v.evidence == nil {
			return bindingError(errors.New("verified reuse carries upload proof"))
		}
		return v.evidence.Validate()
	default:
		return contractError(errors.New("verified decision escaped its domain"))
	}
}

func (v VerifiedDecision) Kind() (DecisionKind, error) {
	if err := v.Validate(); err != nil {
		return DecisionUnknown, err
	}
	return v.kind, nil
}

func (v VerifiedDecision) Grant() (VerifiedGrant, bool) {
	if v.kind != DecisionUpload || v.Validate() != nil {
		return VerifiedGrant{}, false
	}
	return *v.grant, true
}

func (v VerifiedDecision) Evidence() (receipt.VerifiedEvidence, bool) {
	if v.kind != DecisionReuse || v.Validate() != nil {
		return receipt.VerifiedEvidence{}, false
	}
	return *v.evidence, true
}

var (
	_ core.Validatable             = DecisionUnknown
	_ core.Validatable             = DecisionDocument{}
	_ core.Validatable             = DecisionProjection{}
	_ core.Validatable             = DecisionExpectation{}
	_ core.Validatable             = ReuseDecisionRequest{}
	_ core.Validatable             = VerifiedDecision{}
	_ core.ValidatedJSONMarshaler  = DecisionKind(0)
	_ core.ValidatedJSONMarshaler  = DecisionProjection{}
	_ core.ValidatedJSONProjection = DecisionProjection{}
	_ json.Unmarshaler             = (*DecisionDocument)(nil)
)
