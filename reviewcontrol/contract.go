package reviewcontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	RelatedCodeMaximum           = 64
	ContextReferenceMaximum      = 64
	CheckRequirementMaximum      = 64
	ProofRequirementMaximum      = 64
	FindingMaximum               = 128
	EvidenceReferenceMaximum     = 64
	PacketJSONMaximumBytes       = 64 << 10
	ObservationJSONMaximumBytes  = 64 << 10
	DecisionJSONMaximumBytes     = 16 << 10
	EventPayloadJSONMaximumBytes = 72 << 10
)

type CheckRequirement struct {
	Kind  CheckKind                       `json:"kind"`
	Scope projectstandards.SourcePath     `json:"scope"`
	Probe *projectstandards.ProbeIdentity `json:"probe,omitempty"`
}

type ProofRequirement struct {
	Kind    ProofKind                   `json:"kind"`
	Rule    projectstandards.Identifier `json:"rule"`
	Subject Subject                     `json:"subject"`
}

type Contract struct {
	Identity       ContractIdentity                 `json:"identity"`
	Title          ContractTitle                    `json:"title"`
	Problem        ProblemStatement                 `json:"problem"`
	Completion     CompletionStatement              `json:"completion"`
	RequiredChecks []CheckRequirement               `json:"required_checks"`
	RequiredProof  []ProofRequirement               `json:"required_proof"`
	RelatedCode    []projectstandards.CodeReference `json:"related_code"`
}

type ContextReference struct {
	Path   projectstandards.SourcePath `json:"path"`
	SHA256 core.SHA256Digest           `json:"sha256"`
	Bytes  core.ByteCount              `json:"bytes"`
	Reason projectstandards.Text       `json:"reason"`
}

type Packet struct {
	Identity ReviewIdentity                     `json:"identity"`
	Subject  Subject                            `json:"subject"`
	Contract Contract                           `json:"contract"`
	Context  []ContextReference                 `json:"context"`
	IssuedBy projectstandards.EvidenceAuthority `json:"issued_by"`
	IssuedAt temporal.Instant                   `json:"issued_at"`
}

func (r CheckRequirement) Validate() error {
	if err := errors.Join(r.Kind.Validate(), r.Scope.Validate()); err != nil {
		return contractError(err)
	}
	if r.Probe != nil {
		return validateContract(r.Probe.Validate())
	}
	if r.Kind != CheckManualInspection {
		return contractError()
	}
	return nil
}

func (r ProofRequirement) Validate() error {
	return validateContract(r.Kind.Validate(), r.Rule.Validate(), r.Subject.Validate())
}

func (c Contract) Validate() error {
	if err := errors.Join(c.Identity.Validate(), c.Title.Validate(), c.Problem.Validate(), c.Completion.Validate()); err != nil {
		return contractError(err)
	}
	if len(c.RequiredChecks) == 0 || len(c.RequiredChecks) > CheckRequirementMaximum ||
		len(c.RequiredProof) == 0 || len(c.RequiredProof) > ProofRequirementMaximum || len(c.RelatedCode) > RelatedCodeMaximum {
		return contractError()
	}
	return c.validateMembers()
}

func (c Contract) validateMembers() error {
	for index := range c.RequiredChecks {
		if err := c.RequiredChecks[index].Validate(); err != nil {
			return err
		}
	}
	for index := range c.RequiredProof {
		if err := c.RequiredProof[index].Validate(); err != nil {
			return err
		}
		for prior := range index {
			if c.RequiredProof[prior].Rule == c.RequiredProof[index].Rule {
				return contractError(errors.New("review control proof requirement rule is duplicated"))
			}
		}
	}
	for index := range c.RelatedCode {
		if err := c.RelatedCode[index].Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

func (r ContextReference) Validate() error {
	return validateContract(r.Path.Validate(), r.SHA256.Validate(), r.Bytes.Validate(), r.Reason.Validate())
}

func (p Packet) Validate() error {
	if err := errors.Join(p.Identity.Validate(), p.Subject.Validate(), p.Contract.Validate(), p.IssuedBy.Validate(), p.IssuedAt.Validate()); err != nil {
		return contractError(err)
	}
	if len(p.Context) > ContextReferenceMaximum {
		return contractError()
	}
	for index := range p.Contract.RequiredProof {
		if !SameSubject(p.Subject, p.Contract.RequiredProof[index].Subject) {
			return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
		}
	}
	for index := range p.Context {
		if err := p.Context[index].Validate(); err != nil {
			return err
		}
	}
	return validateEncodedDocument(packetWire(p), PacketJSONMaximumBytes)
}

func (p Packet) Digest() (core.SHA256Digest, error) {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type Reviewer struct {
	Identity ReviewerIdentity             `json:"identity"`
	Kind     ReviewerKind                 `json:"kind"`
	Producer projectstandards.Identifier  `json:"producer"`
	Model    *projectstandards.Identifier `json:"model,omitempty"`
}

func (r Reviewer) Validate() error {
	if err := errors.Join(r.Identity.Validate(), r.Kind.Validate(), r.Producer.Validate()); err != nil {
		return contractError(err)
	}
	if r.Model != nil {
		return validateContract(r.Model.Validate())
	}
	return nil
}

type SourceLocation struct {
	Path        projectstandards.SourcePath `json:"path"`
	Line        uint32                      `json:"line,omitempty"`
	Column      uint32                      `json:"column,omitempty"`
	EndLine     uint32                      `json:"end_line,omitempty"`
	EndColumn   uint32                      `json:"end_column,omitempty"`
	Approximate bool                        `json:"approximate,omitempty"`
}

func (l SourceLocation) Validate() error {
	if err := l.Path.Validate(); err != nil {
		return contractError(err)
	}
	if err := l.validateStart(); err != nil {
		return err
	}
	return l.validateEnd()
}

func (l SourceLocation) validateStart() error {
	if l.Line == 0 {
		return l.validateWholeFile()
	}
	if l.EndLine == 0 {
		return l.validatePoint()
	}
	if l.Column == 0 || l.EndColumn == 0 || l.EndLine < l.Line || l.EndLine == l.Line && l.EndColumn < l.Column {
		return contractError()
	}
	return nil
}

func (l SourceLocation) validateWholeFile() error {
	if l.Column != 0 || l.EndLine != 0 || l.EndColumn != 0 {
		return contractError()
	}
	return nil
}

func (l SourceLocation) validatePoint() error {
	if l.EndColumn != 0 {
		return contractError()
	}
	return nil
}

func (l SourceLocation) validateEnd() error {
	if l.EndLine == 0 && l.EndColumn != 0 || l.EndLine != 0 && l.EndColumn == 0 {
		return contractError()
	}
	return nil
}

type Finding struct {
	Identity FindingIdentity             `json:"identity"`
	Rule     projectstandards.Identifier `json:"rule"`
	Severity FindingSeverity             `json:"severity"`
	Location *SourceLocation             `json:"location,omitempty"`
	Summary  FindingSummary              `json:"summary"`
	Detail   FindingDetail               `json:"detail"`
}

func (f Finding) Validate() error {
	if err := validateContract(f.Identity.Validate(), f.Rule.Validate(), f.Severity.Validate(), f.Summary.Validate(), f.Detail.Validate()); err != nil {
		return err
	}
	if f.Location != nil {
		return validateContract(f.Location.Validate())
	}
	return nil
}

type EvidenceReference struct {
	Requirement projectstandards.Identifier              `json:"requirement"`
	Observation projectstandards.ObservationID           `json:"observation"`
	Receipt     runnercontrol.ObservationDeliveryReceipt `json:"receipt"`
}

func (r EvidenceReference) Validate() error {
	if err := errors.Join(r.Requirement.Validate(), r.Observation.Validate(), r.Receipt.Validate()); err != nil {
		return contractError(err)
	}
	if !r.Receipt.Published {
		return errors.Join(core.ErrReviewControlMissingEvidence, contractError())
	}
	return nil
}

type Observation struct {
	Identity   ObservationIdentity `json:"identity"`
	Review     ReviewIdentity      `json:"review"`
	Subject    Subject             `json:"subject"`
	Reviewer   Reviewer            `json:"reviewer"`
	Verdict    Verdict             `json:"verdict"`
	Findings   []Finding           `json:"findings"`
	Evidence   []EvidenceReference `json:"evidence"`
	ObservedAt temporal.Instant    `json:"observed_at"`
}

func (o Observation) Validate() error {
	if err := errors.Join(o.Identity.Validate(), o.Review.Validate(), o.Subject.Validate(), o.Reviewer.Validate(), o.Verdict.Validate(), o.ObservedAt.Validate()); err != nil {
		return contractError(err)
	}
	if len(o.Findings) > FindingMaximum || len(o.Evidence) > EvidenceReferenceMaximum {
		return contractError()
	}
	if o.Verdict == VerdictChangesRequired && len(o.Findings) == 0 {
		return contractError()
	}
	for index := range o.Findings {
		if err := o.Findings[index].Validate(); err != nil {
			return err
		}
	}
	for index := range o.Evidence {
		if err := o.Evidence[index].Validate(); err != nil {
			return err
		}
	}
	return validateEncodedDocument(observationWire(o), ObservationJSONMaximumBytes)
}

func (o Observation) Digest() (core.SHA256Digest, error) {
	encoded, err := o.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}
