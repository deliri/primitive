package permit

import (
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

const ReportDocumentMaximumBytes = 64 << 10

// ReportScope binds every carrier to a tenant, product, project and authorized epoch.
type ReportScope struct {
	Company      id.ULID        `json:"company"`
	Offering     core.Offering  `json:"offering"`
	Project      id.ULID        `json:"project"`
	Installation lease.DeviceID `json:"installation"`
	Epoch        id.ULID        `json:"epoch"`
}

func (s ReportScope) Validate() error {
	if err := errors.Join(s.Company.Validate(), s.Offering.Validate(), s.Project.Validate(), s.Installation.Validate(), s.Epoch.Validate()); err != nil {
		return errors.Join(core.ErrReportBinding, err)
	}
	return nil
}

// InitialReportDigest makes sequence zero explicit and bound to the epoch.
func InitialReportDigest(scope ReportScope) (core.SHA256Digest, error) {
	if err := scope.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(scope)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(append([]byte("primitive-report-predecessor-v1\x00"), encoded...)), nil
}

type ReportEvidence struct {
	Digest core.SHA256Digest `json:"digest"`
	Bytes  uint64            `json:"bytes"`
}

func (e ReportEvidence) Validate() error {
	if e.Bytes == 0 || e.Bytes > math.MaxInt64 {
		return core.ErrReportContract
	}
	return e.Digest.Validate()
}

// ReportPayload is a delta. UsageWindow is its evidence interval, not its slot.
type ReportPayload struct {
	Scope    ReportScope              `json:"scope"`
	Sequence uint64                   `json:"sequence"`
	Previous core.SHA256Digest        `json:"previous"`
	Window   controlplane.UsageWindow `json:"window"`
	Evidence ReportEvidence           `json:"evidence"`
	Policy   controlwire.PolicyCursor `json:"policy"`
}

func (p ReportPayload) Validate() error {
	if err := errors.Join(p.Scope.Validate(), p.Previous.Validate(), p.Window.Validate(), p.Evidence.Validate(), p.Policy.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if p.Sequence == 0 || p.Sequence > math.MaxInt64 {
		return core.ErrReportContract
	}
	return nil
}

type ProjectPermission struct {
	Scope     ReportScope      `json:"scope"`
	IssuedAt  temporal.Instant `json:"issued_at"`
	NotBefore temporal.Instant `json:"not_before"`
	ExpiresAt temporal.Instant `json:"expires_at"`
	Schedule  ReportSchedule   `json:"schedule"`
}

func (p ProjectPermission) Validate() error {
	if err := errors.Join(p.Scope.Validate(), p.IssuedAt.Validate(), p.NotBefore.Validate(), p.ExpiresAt.Validate(), p.Schedule.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	i, _ := p.IssuedAt.Nanoseconds()
	n, _ := p.NotBefore.Nanoseconds()
	e, _ := p.ExpiresAt.Nanoseconds()
	o, _ := p.Schedule.NextReportAt.Nanoseconds()
	if n >= e || i >= e || o < i || o >= e {
		return core.ErrReportSchedule
	}
	return nil
}

type ReportAcknowledgment struct {
	Scope           ReportScope       `json:"scope"`
	Sequence        uint64            `json:"sequence"`
	ReportDigest    core.SHA256Digest `json:"report_digest"`
	ProjectRevision uint64            `json:"project_revision"`
	AcceptedAt      temporal.Instant  `json:"accepted_at"`
	Schedule        ReportSchedule    `json:"schedule"`
}

func (a ReportAcknowledgment) Validate() error {
	if err := errors.Join(a.Scope.Validate(), a.ReportDigest.Validate(), a.AcceptedAt.Validate(), a.Schedule.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if a.Sequence == 0 || a.Sequence > math.MaxInt64 || a.ProjectRevision == 0 || a.ProjectRevision > math.MaxInt64 {
		return core.ErrReportContract
	}
	order, err := a.Schedule.NextReportAt.Compare(a.AcceptedAt)
	if err != nil || order != core.ComparisonGreater {
		return errors.Join(core.ErrReportSchedule, err)
	}
	return nil
}

func reportLimits() core.StrictJSONLimits {
	limits := core.DefaultStrictJSONLimits()
	maximum, _ := core.NewByteCount(ReportDocumentMaximumBytes)
	limits.DocumentMaximumBytes = maximum
	return limits
}
func writeReportBytes(w io.Writer, encoded []byte) error {
	if len(encoded) == 0 || len(encoded) > ReportDocumentMaximumBytes {
		return core.ErrReportContract
	}
	if core.WriterIsNil(w) {
		return errors.Join(core.ErrReportContract, io.ErrClosedPipe)
	}
	n, err := w.Write(encoded)
	if n != len(encoded) {
		return errors.Join(core.ErrReportContract, io.ErrShortWrite, err)
	}
	return err
}
