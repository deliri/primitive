// Package upgradereport authenticates a machine's observation of an exact
// candidate attempt and the authority's durable acknowledgment. It does not run
// product tests, interpret their evidence, decide eligibility, or accept claims
// as independent proof that an upgrade is correct.
package upgradereport

import (
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

// Outcome records the caller's observation, never an inferred success. The
// product decides which observations constitute completion of its upgrade.
type Outcome uint8

const (
	OutcomeUnknown Outcome = iota
	Succeeded
	Failed
	Cancelled
	Interrupted
)

func (o Outcome) Validate() error {
	if o < Succeeded || o > Interrupted {
		return core.ErrReportContract
	}
	return nil
}
func (o Outcome) IsValid() bool { return o.Validate() == nil }
func (o Outcome) String() string {
	if !o.IsValid() {
		return ""
	}
	return [...]string{"", "succeeded", failedOutcomeText, "cancelled", "interrupted"}[o]
}
func (o Outcome) MarshalText() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return []byte(o.String()), nil
}
func (Outcome) ParseCanonicalText(b []byte) (Outcome, error) {
	for o := Succeeded; o <= Interrupted; o++ {
		if string(b) == o.String() {
			return o, nil
		}
	}
	return OutcomeUnknown, core.ErrReportContract
}
func (o Outcome) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(o.String())
}
func (o *Outcome) UnmarshalJSON(b []byte) error {
	if o == nil {
		return core.ErrReportContract
	}
	s, err := core.DecodeJSONStringToken(b)
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	v, err := (OutcomeUnknown).ParseCanonicalText([]byte(s))
	if err != nil {
		return err
	}
	*o = v
	return nil
}

// Stage names only the mechanical boundary observed by the caller.
type Stage uint8

const (
	StageUnknown Stage = iota
	Bootstrap
	Capacity
	Download
	Verification
	Trial
	Promotion
	Persistence
	Cleanup
	Complete
)

func (s Stage) Validate() error {
	if s < Bootstrap || s > Complete {
		return core.ErrReportContract
	}
	return nil
}
func (s Stage) IsValid() bool { return s.Validate() == nil }
func (s Stage) String() string {
	if !s.IsValid() {
		return ""
	}
	return [...]string{"", "bootstrap", "capacity", downloadStageText, "verification", "trial", "promotion", "persistence", "cleanup", completeStageText}[s]
}
func (s Stage) MarshalText() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return []byte(s.String()), nil
}
func (Stage) ParseCanonicalText(b []byte) (Stage, error) {
	for s := Bootstrap; s <= Complete; s++ {
		if string(b) == s.String() {
			return s, nil
		}
	}
	return StageUnknown, core.ErrReportContract
}
func (s Stage) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(s.String())
}
func (s *Stage) UnmarshalJSON(b []byte) error {
	if s == nil {
		return core.ErrReportContract
	}
	v, err := core.DecodeJSONStringToken(b)
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got, err := (StageUnknown).ParseCanonicalText([]byte(v))
	if err != nil {
		return err
	}
	*s = got
	return nil
}

// Payload contains no logs, paths, credentials or product-specific test model.
// Evidence is an authority-signed exact-object receipt, not a claimed URL.
type Payload struct {
	Installed  core.BuildIdentity       `json:"installed"`
	Candidate  core.BuildIdentity       `json:"candidate"`
	Evidence   receipt.EvidenceDocument `json:"evidence"`
	ObservedAt temporal.Instant         `json:"observed_at"`
	Attempt    controlwire.RequestNonce `json:"attempt"`
	Revision   controlwire.Revision     `json:"revision"`
	Outcome    Outcome                  `json:"outcome"`
	Stage      Stage                    `json:"stage"`
}

func (p Payload) Validate() error {
	if err := errors.Join(p.Attempt.Validate(), p.Installed.Validate(), p.Candidate.Validate(), p.Evidence.Validate(), p.ObservedAt.Validate(), p.Revision.Validate(), p.Outcome.Validate(), p.Stage.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if p.Installed.Offering() != p.Candidate.Offering() || p.Installed.Platform() != p.Candidate.Platform() || p.Installed == p.Candidate || p.Evidence.Payload.Header.Offering != p.Installed.Offering() {
		return core.ErrReportBinding
	}
	return nil
}

// Acknowledgment binds exactly one retained signed request. It acknowledges
// durable recording, not independent acceptance of the machine's observation.
type Acknowledgment struct {
	Attempt       controlwire.RequestNonce `json:"attempt"`
	RequestDigest core.SHA256Digest        `json:"request_digest"`
	RecordedAt    temporal.Instant         `json:"recorded_at"`
}

func (p Acknowledgment) Validate() error {
	return errors.Join(p.Attempt.Validate(), p.RequestDigest.Validate(), p.RecordedAt.Validate())
}

const failedOutcomeText = "failed"

const completeStageText = "complete"

const downloadStageText = "download"
