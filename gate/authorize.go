package gate

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// AuthorizeRequest carries one already-authentic Lease assessment to a
// new-work boundary.
type AuthorizeRequest struct {
	Assessment lease.Assessment
}

// Validate rejects an unset or malformed Lease assessment.
func (r AuthorizeRequest) Validate() error {
	if err := r.Assessment.Validate(); err != nil {
		return contractError(ContractBoundaryAuthorizeRequest, err)
	}
	return nil
}

// NewWorkPermit is an unforgeable in-process capability to begin immediate
// paid work under one exact Lease assessment.
type NewWorkPermit struct {
	assessment lease.Assessment
}

// Validate rejects the zero value and any assessment that does not currently
// permit new work.
func (p NewWorkPermit) Validate() error {
	if err := p.assessment.Validate(); err != nil {
		return contractError(ContractBoundaryNewWorkPermit, err)
	}
	disposition, err := dispositionForState(p.assessment.State())
	if err != nil {
		return contractError(ContractBoundaryNewWorkPermit, err)
	}
	if disposition != authorizationDispositionPermit {
		return contractError(ContractBoundaryNewWorkPermit)
	}
	return nil
}

// Assessment returns the exact Lease assessment that authorized the permit.
func (p NewWorkPermit) Assessment() (lease.Assessment, error) {
	if err := p.Validate(); err != nil {
		return lease.Assessment{}, err
	}
	return p.assessment, nil
}

// DenialError is a sealed typed refusal to begin new paid work.
//
// Only Gate can implement this interface. Its package-private implementation
// is a value that always carries the exact assessment that produced it, so an
// unset or typed-nil carrier cannot masquerade as an authentic refusal.
type DenialError interface {
	error
	Validate() error
	Assessment() (lease.Assessment, error)
	State() (lease.State, error)
	ContactState() (lease.ContactState, error)
	gateDenial()
}

type denialError struct {
	assessment lease.Assessment
}

// Validate rejects malformed or permission-bearing denials.
func (e denialError) Validate() error {
	if err := e.assessment.Validate(); err != nil {
		return contractError(ContractBoundaryDenial, err)
	}
	disposition, err := dispositionForState(e.assessment.State())
	if err != nil {
		return contractError(ContractBoundaryDenial, err)
	}
	if disposition != authorizationDispositionDeny {
		return contractError(ContractBoundaryDenial)
	}
	return nil
}

// Error returns the stable Core-owned denial diagnostic.
func (denialError) Error() string { return core.ErrGateDenied.Error() }

// Unwrap preserves Gate's stable denial identity.
func (denialError) Unwrap() error { return core.ErrGateDenied }

// Assessment returns the exact Lease assessment that caused denial.
func (e denialError) Assessment() (lease.Assessment, error) {
	if err := e.Validate(); err != nil {
		return lease.Assessment{}, err
	}
	return e.assessment, nil
}

// State returns the exact denied Lease state.
func (e denialError) State() (lease.State, error) {
	if err := e.Validate(); err != nil {
		return lease.StateUnknown, err
	}
	return e.assessment.State(), nil
}

// ContactState returns the signed contact posture accompanying denial.
func (e denialError) ContactState() (lease.ContactState, error) {
	if err := e.Validate(); err != nil {
		return lease.ContactStateUnknown, err
	}
	return e.assessment.ContactState(), nil
}

func (denialError) gateDenial() {}

// AuthorizeNewWork returns an unforgeable permit only for a current or
// continuity Lease assessment. Every other authentic state returns a typed
// denial and a zero permit, and every rejected contract returns a zero permit
// with the exact rejected boundary.
func AuthorizeNewWork(request AuthorizeRequest) (NewWorkPermit, error) {
	if err := request.Validate(); err != nil {
		return NewWorkPermit{}, err
	}
	disposition, err := dispositionForState(request.Assessment.State())
	if err != nil {
		return NewWorkPermit{}, contractError(
			ContractBoundaryAuthorizeRequest,
			err,
		)
	}
	switch disposition {
	case authorizationDispositionPermit:
		permit := NewWorkPermit{assessment: request.Assessment}
		if permitErr := permit.Validate(); permitErr != nil {
			return NewWorkPermit{}, permitErr
		}
		return permit, nil
	case authorizationDispositionDeny:
		denial := denialError{assessment: request.Assessment}
		if denialErr := denial.Validate(); denialErr != nil {
			return NewWorkPermit{}, denialErr
		}
		return NewWorkPermit{}, denial
	default:
		return NewWorkPermit{}, contractError(ContractBoundaryAuthorizeRequest)
	}
}

var _ DenialError = denialError{}

type authorizationDisposition uint8

const (
	authorizationDispositionUnknown authorizationDisposition = iota
	authorizationDispositionPermit
	authorizationDispositionDeny
)

func dispositionForState(
	state lease.State,
) (authorizationDisposition, error) {
	if err := state.Validate(); err != nil {
		return authorizationDispositionUnknown,
			errors.Join(core.ErrGateContract, err)
	}
	switch state {
	case lease.StateCurrent, lease.StateContinuity:
		return authorizationDispositionPermit, nil
	case lease.StateNotYetValid, lease.StateExpired,
		lease.StateRefused, lease.StateRevoked:
		return authorizationDispositionDeny, nil
	default:
		return authorizationDispositionUnknown, core.ErrGateContract
	}
}
