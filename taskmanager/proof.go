package taskmanager

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

// EvidenceSummary is one bounded human explanation of a referenced proof.
type EvidenceSummary string

func ParseEvidenceSummary(value string) (EvidenceSummary, error) {
	parsed := EvidenceSummary(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (s EvidenceSummary) Validate() error {
	return validateText(string(s), EvidenceSummaryMaximumRunes, false)
}

func (s EvidenceSummary) String() string { return string(s) }

// RepositoryIdentity names the exact repository associated with a Git proof.
type RepositoryIdentity string

func ParseRepositoryIdentity(value string) (RepositoryIdentity, error) {
	parsed := RepositoryIdentity(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (r RepositoryIdentity) Validate() error {
	return validateText(string(r), RepositoryMaximumRunes, false)
}

func (r RepositoryIdentity) String() string { return string(r) }

// CommitSummary is one bounded truthful Git change summary.
type CommitSummary string

func ParseCommitSummary(value string) (CommitSummary, error) {
	parsed := CommitSummary(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (s CommitSummary) Validate() error {
	return validateText(string(s), CommitSummaryMaximumRunes, false)
}

func (s CommitSummary) String() string { return string(s) }

// EvidenceRecord is one immutable reference to externally stored proof.
type EvidenceRecord struct {
	Summary      EvidenceSummary         `json:"summary"`
	Location     core.HTTPEndpoint       `json:"location"`
	CreatedAt    temporal.NumericInstant `json:"created_at"`
	TaskRevision Revision                `json:"task_revision"`
	Digest       core.SHA256Digest       `json:"digest"`
	ID           id.UUIDv7               `json:"id"`
	ProjectID    id.UUIDv7               `json:"project_id"`
	TaskID       id.UUIDv7               `json:"task_id"`
	Kind         EvidenceKind            `json:"kind"`
}

// EvidenceCursor is one stable evidence continuation bound to one task.
type EvidenceCursor struct {
	ProjectID id.UUIDv7               `json:"project_id"`
	TaskID    id.UUIDv7               `json:"task_id"`
	CreatedAt temporal.NumericInstant `json:"created_at"`
	ID        id.UUIDv7               `json:"id"`
}

func (c EvidenceCursor) Validate() error {
	return validateJoined(c.ProjectID.Validate(), c.TaskID.Validate(), c.CreatedAt.Validate(), c.ID.Validate())
}

// ListEvidenceRequest selects one bounded proof page for one task.
type ListEvidenceRequest struct {
	After     *EvidenceCursor `json:"after,omitempty"`
	Limit     PageLimit       `json:"limit"`
	ProjectID id.UUIDv7       `json:"project_id"`
	TaskID    id.UUIDv7       `json:"task_id"`
	Order     PageOrder       `json:"order"`
}

func (r ListEvidenceRequest) Validate() error {
	if err := validateJoined(r.ProjectID.Validate(), r.TaskID.Validate(), r.Order.Validate(), r.Limit.Validate()); err != nil {
		return err
	}
	if r.After == nil {
		return nil
	}
	if err := r.After.Validate(); err != nil || r.After.ProjectID != r.ProjectID || r.After.TaskID != r.TaskID {
		return contractError(err)
	}
	return nil
}

func (r ListEvidenceRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ListEvidenceRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// EvidencePage is one bounded proof-history projection for one task.
type EvidencePage struct {
	Next      *EvidenceCursor  `json:"next,omitempty"`
	Items     []EvidenceRecord `json:"items"`
	ProjectID id.UUIDv7        `json:"project_id"`
	TaskID    id.UUIDv7        `json:"task_id"`
	Order     PageOrder        `json:"order"`
}

func (p EvidencePage) Validate() error {
	if err := (proofPageHeader{ProjectID: p.ProjectID, TaskID: p.TaskID, Order: p.Order, ItemCount: len(p.Items)}).Validate(); err != nil {
		return err
	}
	if err := p.validateItems(); err != nil {
		return err
	}
	return validateEvidencePageCursor(p)
}

func (p EvidencePage) validateItems() error {
	for _, item := range p.Items {
		if err := item.Validate(); err != nil || item.ProjectID != p.ProjectID || item.TaskID != p.TaskID {
			return contractError(err)
		}
	}
	return nil
}

func validateEvidencePageCursor(page EvidencePage) error {
	if page.Next == nil {
		return nil
	}
	if err := page.Next.Validate(); err != nil || len(page.Items) == 0 ||
		page.Next.ProjectID != page.ProjectID || page.Next.TaskID != page.TaskID {
		return contractError(err)
	}
	last := page.Items[len(page.Items)-1]
	if page.Next.CreatedAt != last.CreatedAt || page.Next.ID != last.ID {
		return contractError()
	}
	return nil
}

func (p EvidencePage) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire EvidencePage
	return core.MarshalCanonicalJSONDocument(wire(p))
}

func (r EvidenceRecord) Validate() error {
	url := r.Location.HTTPURL()
	if url.Scheme != core.SchemeHTTPS {
		return contractError()
	}
	return validateJoined(
		r.ID.Validate(), r.ProjectID.Validate(), r.TaskID.Validate(), r.Kind.Validate(),
		r.Summary.Validate(), r.Location.Validate(), r.Digest.Validate(),
		r.TaskRevision.Validate(), r.CreatedAt.Validate(),
	)
}

func (r EvidenceRecord) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire EvidenceRecord
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// AppendEvidenceRequest appends one proof reference against the task's exact
// current revision without embedding proof bytes in the task document.
type AppendEvidenceRequest struct {
	Summary          EvidenceSummary   `json:"summary"`
	Location         core.HTTPEndpoint `json:"location"`
	ExpectedRevision Revision          `json:"expected_revision"`
	Digest           core.SHA256Digest `json:"digest"`
	ID               id.UUIDv7         `json:"id"`
	ProjectID        id.UUIDv7         `json:"project_id"`
	TaskID           id.UUIDv7         `json:"task_id"`
	MutationID       id.UUIDv7         `json:"mutation_id"`
	Kind             EvidenceKind      `json:"kind"`
}

func (r AppendEvidenceRequest) Validate() error {
	url := r.Location.HTTPURL()
	if url.Scheme != core.SchemeHTTPS {
		return contractError()
	}
	return validateJoined(
		r.ID.Validate(), r.ProjectID.Validate(), r.TaskID.Validate(), r.MutationID.Validate(),
		r.Kind.Validate(), r.Summary.Validate(), r.Location.Validate(), r.Digest.Validate(),
		r.ExpectedRevision.Validate(),
	)
}

func (r AppendEvidenceRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire AppendEvidenceRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (r AppendEvidenceRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return mutationKey(r, r.MutationID)
}

// GitCommitRecord binds one repository's parent and resulting object names to
// the task revision that admitted the proof.
type GitCommitRecord struct {
	Repository   RepositoryIdentity      `json:"repository"`
	Summary      CommitSummary           `json:"summary"`
	CreatedAt    temporal.NumericInstant `json:"created_at"`
	TaskRevision Revision                `json:"task_revision"`
	Parent       core.BuildCommit        `json:"parent"`
	Result       core.BuildCommit        `json:"result"`
	ID           id.UUIDv7               `json:"id"`
	ProjectID    id.UUIDv7               `json:"project_id"`
	TaskID       id.UUIDv7               `json:"task_id"`
}

// GitCommitCursor is one stable commit continuation bound to one task.
type GitCommitCursor struct {
	ProjectID id.UUIDv7               `json:"project_id"`
	TaskID    id.UUIDv7               `json:"task_id"`
	CreatedAt temporal.NumericInstant `json:"created_at"`
	ID        id.UUIDv7               `json:"id"`
}

func (c GitCommitCursor) Validate() error {
	return validateJoined(c.ProjectID.Validate(), c.TaskID.Validate(), c.CreatedAt.Validate(), c.ID.Validate())
}

// ListGitCommitsRequest selects one bounded Git proof page for one task.
type ListGitCommitsRequest struct {
	After     *GitCommitCursor `json:"after,omitempty"`
	Limit     PageLimit        `json:"limit"`
	ProjectID id.UUIDv7        `json:"project_id"`
	TaskID    id.UUIDv7        `json:"task_id"`
	Order     PageOrder        `json:"order"`
}

func (r ListGitCommitsRequest) Validate() error {
	if err := validateJoined(r.ProjectID.Validate(), r.TaskID.Validate(), r.Order.Validate(), r.Limit.Validate()); err != nil {
		return err
	}
	if r.After == nil {
		return nil
	}
	if err := r.After.Validate(); err != nil || r.After.ProjectID != r.ProjectID || r.After.TaskID != r.TaskID {
		return contractError(err)
	}
	return nil
}

func (r ListGitCommitsRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ListGitCommitsRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// GitCommitPage is one bounded Git-history projection for one task.
type GitCommitPage struct {
	Next      *GitCommitCursor  `json:"next,omitempty"`
	Items     []GitCommitRecord `json:"items"`
	ProjectID id.UUIDv7         `json:"project_id"`
	TaskID    id.UUIDv7         `json:"task_id"`
	Order     PageOrder         `json:"order"`
}

func (p GitCommitPage) Validate() error {
	if err := (proofPageHeader{ProjectID: p.ProjectID, TaskID: p.TaskID, Order: p.Order, ItemCount: len(p.Items)}).Validate(); err != nil {
		return err
	}
	if err := p.validateItems(); err != nil {
		return err
	}
	return validateGitCommitPageCursor(p)
}

func (p GitCommitPage) validateItems() error {
	for _, item := range p.Items {
		if err := item.Validate(); err != nil || item.ProjectID != p.ProjectID || item.TaskID != p.TaskID {
			return contractError(err)
		}
	}
	return nil
}

type proofPageHeader struct {
	ProjectID id.UUIDv7
	TaskID    id.UUIDv7
	Order     PageOrder
	ItemCount int
}

func (h proofPageHeader) Validate() error {
	if err := validateJoined(h.ProjectID.Validate(), h.TaskID.Validate(), h.Order.Validate()); err != nil || h.ItemCount > int(PageLimitMaximum) {
		return contractError(err)
	}
	return nil
}

func validateGitCommitPageCursor(page GitCommitPage) error {
	if page.Next == nil {
		return nil
	}
	if err := page.Next.Validate(); err != nil || len(page.Items) == 0 ||
		page.Next.ProjectID != page.ProjectID || page.Next.TaskID != page.TaskID {
		return contractError(err)
	}
	last := page.Items[len(page.Items)-1]
	if page.Next.CreatedAt != last.CreatedAt || page.Next.ID != last.ID {
		return contractError()
	}
	return nil
}

func (p GitCommitPage) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire GitCommitPage
	return core.MarshalCanonicalJSONDocument(wire(p))
}

func (r GitCommitRecord) Validate() error {
	if r.Parent == r.Result {
		return contractError()
	}
	return validateJoined(
		r.ID.Validate(), r.ProjectID.Validate(), r.TaskID.Validate(), r.Repository.Validate(),
		r.Parent.Validate(), r.Result.Validate(), r.Summary.Validate(),
		r.TaskRevision.Validate(), r.CreatedAt.Validate(),
	)
}

func (r GitCommitRecord) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire GitCommitRecord
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// AppendGitCommitRequest appends exact Git identity against one task revision.
type AppendGitCommitRequest struct {
	Repository       RepositoryIdentity `json:"repository"`
	Summary          CommitSummary      `json:"summary"`
	ExpectedRevision Revision           `json:"expected_revision"`
	Parent           core.BuildCommit   `json:"parent"`
	Result           core.BuildCommit   `json:"result"`
	ID               id.UUIDv7          `json:"id"`
	ProjectID        id.UUIDv7          `json:"project_id"`
	TaskID           id.UUIDv7          `json:"task_id"`
	MutationID       id.UUIDv7          `json:"mutation_id"`
}

func (r AppendGitCommitRequest) Validate() error {
	if r.Parent == r.Result {
		return contractError()
	}
	return validateJoined(
		r.ID.Validate(), r.ProjectID.Validate(), r.TaskID.Validate(), r.MutationID.Validate(),
		r.Repository.Validate(), r.Parent.Validate(), r.Result.Validate(), r.Summary.Validate(),
		r.ExpectedRevision.Validate(),
	)
}

func (r AppendGitCommitRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire AppendGitCommitRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (r AppendGitCommitRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return mutationKey(r, r.MutationID)
}

var (
	_ core.Validatable            = EvidenceSummary("")
	_ core.Validatable            = RepositoryIdentity("")
	_ core.Validatable            = CommitSummary("")
	_ core.ValidatedJSONMarshaler = EvidenceRecord{}
	_ core.ValidatedJSONMarshaler = ListEvidenceRequest{}
	_ core.ValidatedJSONMarshaler = EvidencePage{}
	_ core.ValidatedJSONMarshaler = AppendEvidenceRequest{}
	_ core.ValidatedJSONMarshaler = GitCommitRecord{}
	_ core.ValidatedJSONMarshaler = ListGitCommitsRequest{}
	_ core.ValidatedJSONMarshaler = GitCommitPage{}
	_ core.ValidatedJSONMarshaler = AppendGitCommitRequest{}
	_ exchange.IdempotencyBound   = AppendEvidenceRequest{}
	_ exchange.IdempotencyBound   = AppendGitCommitRequest{}
)
