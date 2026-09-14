package permit

import (
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ReportAuthorization contains current server-selected restrictions, without a
// second schedule. A refresh can replace this signed authorization while the
// last committed schedule carrier remains byte-for-byte unchanged.
type ReportAuthorization struct {
	Scope        ReportScope              `json:"scope"`
	Enabled      bool                     `json:"enabled"`
	ObservedAt   temporal.Instant         `json:"observed_at"`
	NotBefore    temporal.Instant         `json:"not_before"`
	ExpiresAt    temporal.Instant         `json:"expires_at"`
	Policy       controlwire.PolicyCursor `json:"policy"`
	Transmission TransmissionPolicy       `json:"transmission"`
}

func (a ReportAuthorization) Validate() error {
	if err := errors.Join(a.Scope.Validate(), a.ObservedAt.Validate(), a.NotBefore.Validate(), a.ExpiresAt.Validate(), a.Policy.Validate(), a.Transmission.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	order, _ := a.NotBefore.Compare(a.ExpiresAt)
	if order != core.ComparisonLess {
		return core.ErrReportContract
	}
	return nil
}

// ReportPermissionResponse has exactly one schedule carrier: initial permission
// or committed acknowledgment. Authorization refresh cannot manufacture an ack.
type ReportPermissionResponse struct {
	Authorization     SignedReportAuthorization   `json:"authorization"`
	InitialPermission *SignedProjectPermission    `json:"initial_permission,omitempty"`
	Acknowledgment    *SignedReportAcknowledgment `json:"acknowledgment,omitempty"`
}

func (r ReportPermissionResponse) Validate() error {
	if err := r.Authorization.Validate(); err != nil {
		return err
	}
	if (r.InitialPermission == nil) == (r.Acknowledgment == nil) {
		return core.ErrReportSchedule
	}
	scope := r.Authorization.Payload.Scope
	if r.InitialPermission != nil {
		if err := r.InitialPermission.Validate(); err != nil {
			return err
		}
		if r.InitialPermission.Payload.Scope != scope {
			return core.ErrReportBinding
		}
		return nil
	}
	if err := r.Acknowledgment.Validate(); err != nil {
		return err
	}
	if r.Acknowledgment.Payload.Scope != scope {
		return core.ErrReportBinding
	}
	return nil
}
func (r ReportPermissionResponse) Schedule() (ReportSchedule, error) {
	if err := r.Validate(); err != nil {
		return ReportSchedule{}, err
	}
	if r.InitialPermission != nil {
		return r.InitialPermission.Payload.Schedule, nil
	}
	return r.Acknowledgment.Payload.Schedule, nil
}
func (r ReportPermissionResponse) Timing(at temporal.Instant) (ReportTiming, error) {
	if err := errors.Join(r.Validate(), at.Validate()); err != nil {
		return ReportTiming{}, err
	}
	if !r.Authorization.Payload.Enabled {
		return ReportTiming{}, core.ErrPermitAction
	}
	a := r.Authorization.Payload
	nb, _ := a.NotBefore.Nanoseconds()
	expiry, _ := a.ExpiresAt.Nanoseconds()
	if r.InitialPermission != nil {
		initialNB, _ := r.InitialPermission.Payload.NotBefore.Nanoseconds()
		initialExpiry, _ := r.InitialPermission.Payload.ExpiresAt.Nanoseconds()
		nb = max(nb, initialNB)
		expiry = min(expiry, initialExpiry)
	}
	now, _ := at.Nanoseconds()
	if now >= expiry {
		return ReportTiming{}, core.ErrPermitValidity
	}
	t := ReportTiming{ObservedAt: at, NotBefore: temporal.InstantFromNanoseconds(nb), ExpiresAt: temporal.InstantFromNanoseconds(expiry)}
	return t, t.Validate()
}
func (r ReportPermissionResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire ReportPermissionResponse
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r *ReportPermissionResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrReportContract
	}
	type wire ReportPermissionResponse
	got, err := core.DecodeStrictJSONStructure[wire](data, reportLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	candidate := ReportPermissionResponse(got)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*r = candidate
	return nil
}
