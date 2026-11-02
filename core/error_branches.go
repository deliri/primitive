package core

import "errors"

// ErrorBranchObservation borrows one non-joined native error and its immediate
// single Unwrap result. Neither value is a persisted classification or proof.
type ErrorBranchObservation struct {
	// Error is the observed native error; it is never nil in a valid observation.
	Error error
	// Unwrapped is the single errors.Unwrap result of Error, or nil at a terminal cause.
	Unwrapped error
}

// Validate refuses an observation without an error.
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
	// Error is the root whose every joined branch must reach an accepted cause.
	Error error
	// Accept decides whether one observed wrapper or terminal cause is admitted.
	Accept func(ErrorBranchObservation) bool
}

// Validate refuses a request without an Accept policy.
func (r ErrorBranchesRequest) Validate() error {
	if r.Accept == nil {
		return ErrPrimitiveContract
	}
	return nil
}

// ErrorBranchesMatch requires every joined branch to reach an accepted native
// cause. Nil roots, nil branches, empty joins and invalid requests refuse with
// false. It follows Go's Unwrap contracts without collecting a second tree.
// Linear wrappers use a loop; only active joined ancestry uses an explicit stack.
// As with errors.Is, caller-supplied Unwrap methods must describe a finite tree.
func ErrorBranchesMatch(request ErrorBranchesRequest) bool {
	if request.Validate() != nil {
		return false
	}
	return errorBranchesMatch(request.Error, request.Accept)
}

func errorBranchesMatch(current error, accept func(ErrorBranchObservation) bool) bool {
	// Borrow the remaining sibling slices instead of copying joined populations.
	// The inline window covers ordinary nesting; deeper valid trees continue.
	var window [16][]error
	pending := window[:0]
	for {
		matched := false
		for !IsNil(current) {
			if joined, ok := current.(interface{ Unwrap() []error }); ok {
				branches := joined.Unwrap()
				if len(branches) == 0 {
					return false
				}
				if len(branches) > 1 {
					pending = append(pending, branches[1:])
				}
				current = branches[0]
				continue
			}
			cause := errors.Unwrap(current)
			if accept(ErrorBranchObservation{Error: current, Unwrapped: cause}) {
				matched = true
				break
			}
			current = cause
		}
		if !matched {
			return false
		}
		if len(pending) == 0 {
			return true
		}
		top := len(pending) - 1
		current = pending[top][0]
		pending[top] = pending[top][1:]
		if len(pending[top]) == 0 {
			pending = pending[:top]
		}
	}
}
