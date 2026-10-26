package core

import "errors"

// ErrorBranchObservation borrows one non-joined native error and its immediate
// single Unwrap result. Neither value is a persisted classification or proof.
type ErrorBranchObservation struct {
	Error     error
	Unwrapped error
}

func (o ErrorBranchObservation) Validate() error {
	if IsNil(o.Error) {
		return ErrPrimitiveContract
	}
	return nil
}

// ErrorBranchesRequest supplies policy over native causes. Accept may recognize
// an owned atomic wrapper, or a terminal cause with Unwrapped == nil. Accepting
// a wrapper deliberately accepts that wrapper's subtree; joined nodes are always
// expanded first, so one matching branch cannot authorize a different branch.
type ErrorBranchesRequest struct {
	Error  error
	Accept func(ErrorBranchObservation) bool
}

func (r ErrorBranchesRequest) Validate() error {
	if r.Accept == nil {
		return ErrPrimitiveContract
	}
	return nil
}

// ErrorBranchesMatch requires every joined branch to reach an accepted native
// cause. Nil roots, nil branches, empty joins and invalid requests refuse with
// false. It follows Go's Unwrap contracts without collecting a second tree.
// Linear wrappers use a loop; only active joined ancestry uses stack space.
// As with errors.Is, caller-supplied Unwrap methods must describe a finite tree.
func ErrorBranchesMatch(request ErrorBranchesRequest) bool {
	if request.Validate() != nil {
		return false
	}
	return errorBranchesMatch(request.Error, request.Accept)
}

func errorBranchesMatch(current error, accept func(ErrorBranchObservation) bool) bool {
	for !IsNil(current) {
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			branches := joined.Unwrap()
			if len(branches) == 0 {
				return false
			}
			for _, branch := range branches {
				if !errorBranchesMatch(branch, accept) {
					return false
				}
			}
			return true
		}
		cause := errors.Unwrap(current)
		if accept(ErrorBranchObservation{Error: current, Unwrapped: cause}) {
			return true
		}
		current = cause
	}
	return false
}
