package sourceclaim

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Emit receives one validated atomic claim. Repository-owned claim packages
// use this signature so claim count is streamed rather than capped by a
// Primitive-invented project size.
type Emit func(Claim) error

// Stream is the compiler-visible entrypoint shape implemented by an offline
// repository claims package.
type Stream func(Emit) error

// Summary retains exact streaming accounting without retaining the claims.
type Summary struct {
	Digest        core.SHA256Digest `json:"digest"`
	Bytes         core.ByteLength   `json:"bytes"`
	Projects      uint64            `json:"projects"`
	Packages      uint64            `json:"packages"`
	Files         uint64            `json:"files"`
	Subjects      uint64            `json:"subjects"`
	ProjectClaims uint64            `json:"project_claims"`
	PackageClaims uint64            `json:"package_claims"`
	FileClaims    uint64            `json:"file_claims"`
	Claims        uint64            `json:"claims"`
}

// Consume validates canonical subject/claim order and forwards each claim.
// Canonical ordering makes duplicates detectable with O(1) retained state.
// Primitive reports cardinality; it does not impose a repository-size or
// per-subject claim quota.
func Consume(stream Stream, destination Emit) (Summary, error) {
	if stream == nil || destination == nil {
		return Summary{}, contractError(errors.New("source claim stream or destination is nil"))
	}
	consumer := claimConsumer{destination: destination, digest: core.NewDigestWriter()}
	streamErr := stream(consumer.accept)
	if err := errors.Join(streamErr, consumer.err); err != nil {
		return Summary{}, err
	}
	if !consumer.seen {
		return Summary{}, contractError(errors.New("source claim stream is empty"))
	}
	digest, length, err := consumer.digest.Seal()
	if err != nil {
		return Summary{}, contractError(err)
	}
	consumer.summary.Digest = digest
	consumer.summary.Bytes = length
	if err := consumer.summary.Validate(); err != nil {
		return Summary{}, err
	}
	return consumer.summary, nil
}

type claimConsumer struct {
	destination Emit
	digest      *core.DigestWriter
	previous    Claim
	summary     Summary
	err         error
	seen        bool
}

func (c *claimConsumer) accept(claim Claim) error {
	if c.err != nil {
		return c.err
	}
	c.err = c.acceptOne(claim)
	return c.err
}

func (c *claimConsumer) acceptOne(claim Claim) error {
	if err := claim.Validate(); err != nil {
		return err
	}
	newSubject, err := c.advanceSubject(claim)
	if err != nil {
		return err
	}
	if newSubject {
		if err := incrementSubjectKind(&c.summary, claim.Subject.Kind); err != nil {
			return err
		}
	}
	if err := writeClaimRecord(c.digest, claim); err != nil {
		return err
	}
	if c.summary.Claims == ^uint64(0) {
		return contractError(errors.New("source claim count overflows uint64"))
	}
	c.summary.Claims++
	if err := incrementClaimKind(&c.summary, claim.Subject.Kind); err != nil {
		return err
	}
	c.previous = claim
	return c.destination(claim)
}

// Validate proves exact subject, scope, digest, and byte accounting for one
// complete canonical claim stream.
func (s Summary) Validate() error {
	if err := errors.Join(s.Digest.Validate(), s.Bytes.Validate()); err != nil {
		return contractError(err)
	}
	if s.Claims == 0 || s.Bytes.Uint64() == 0 || !claimSumEquals(s.Subjects, s.Projects, s.Packages, s.Files) ||
		!claimSumEquals(s.Claims, s.ProjectClaims, s.PackageClaims, s.FileClaims) {
		return conflictError(errors.New("source claim summary accounting does not close"))
	}
	return nil
}

func writeClaimRecord(destination *core.DigestWriter, claim Claim) error {
	encoded, err := claim.MarshalJSON()
	if err != nil {
		return err
	}
	if _, err := destination.Write(encoded); err != nil {
		return contractError(err)
	}
	if _, err := destination.Write([]byte{'\n'}); err != nil {
		return contractError(err)
	}
	return nil
}

func claimSumEquals(want uint64, values ...uint64) bool {
	var total uint64
	for _, value := range values {
		if total > ^uint64(0)-value {
			return false
		}
		total += value
	}
	return total == want
}

func (c *claimConsumer) advanceSubject(claim Claim) (bool, error) {
	if !c.seen {
		c.seen = true
		c.summary.Subjects = 1
		return true, nil
	}
	order := core.CompareSourceSubjects(c.previous.Subject, claim.Subject)
	if order > 0 || order == 0 && c.previous.ID.String() >= claim.ID.String() {
		return false, conflictError(errors.New("source claims are duplicated or not in canonical order"))
	}
	if order == 0 {
		return false, nil
	}
	if c.summary.Subjects == ^uint64(0) {
		return false, contractError(errors.New("source claim subject count overflows uint64"))
	}
	c.summary.Subjects++
	return true, nil
}

func incrementSubjectKind(summary *Summary, kind core.SourceSubjectKind) error {
	counter, err := subjectKindCounter(summary, kind)
	if err != nil {
		return err
	}
	if *counter == ^uint64(0) {
		return contractError(errors.New("source claim subject-kind count overflows uint64"))
	}
	*counter++
	return nil
}

func subjectKindCounter(summary *Summary, kind core.SourceSubjectKind) (*uint64, error) {
	switch kind {
	case core.SourceSubjectProject:
		return &summary.Projects, nil
	case core.SourceSubjectPackage:
		return &summary.Packages, nil
	case core.SourceSubjectFile:
		return &summary.Files, nil
	default:
		return nil, contractError(errors.New("source claim subject kind is invalid"))
	}
}

func incrementClaimKind(summary *Summary, kind core.SourceSubjectKind) error {
	counter, err := claimKindCounter(summary, kind)
	if err != nil {
		return err
	}
	if *counter == ^uint64(0) {
		return contractError(errors.New("source claim scope count overflows uint64"))
	}
	*counter++
	return nil
}

func claimKindCounter(summary *Summary, kind core.SourceSubjectKind) (*uint64, error) {
	switch kind {
	case core.SourceSubjectProject:
		return &summary.ProjectClaims, nil
	case core.SourceSubjectPackage:
		return &summary.PackageClaims, nil
	case core.SourceSubjectFile:
		return &summary.FileClaims, nil
	default:
		return nil, contractError(errors.New("source claim subject kind is invalid"))
	}
}
