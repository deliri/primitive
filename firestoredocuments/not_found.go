package firestoredocuments

import (
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsNotFound reports an unambiguous native missing-document refusal. It permits
// transparent wrapping and joins with ErrFirestoreDocumentsOperation, but any
// other leaf withdraws absence. A marker alone is never a native observation.
// The supplied error and all its causes remain unchanged. Foreign wrappers
// which panic, contain typed nils or contradict their native status refuse.
func IsNotFound(err error) (missing bool) {
	defer func() {
		if recover() != nil {
			missing = false
		}
	}()
	allowed, observed := inspectNotFound(err)
	return allowed && observed
}

func inspectNotFound(err error) (bool, bool) {
	if core.IsNil(err) {
		return false, false
	}
	if err == core.ErrFirestoreDocumentsOperation {
		return true, false
	}
	wrapped, observed := false, false
	switch value := err.(type) {
	case interface{ Unwrap() []error }:
		children := value.Unwrap()
		if len(children) == 0 {
			return false, false
		}
		wrapped = true
		for _, child := range children {
			allowed, native := inspectNotFound(child)
			if !allowed {
				return false, false
			}
			observed = observed || native
		}
	case interface{ Unwrap() error }:
		allowed, native := inspectNotFound(value.Unwrap())
		if !allowed {
			return false, false
		}
		wrapped, observed = true, native
	}
	// Inspect children first: an outer GRPCStatus cannot conceal a sibling
	// failure. When a wrapper exposes its own status it must also agree.
	native, hasStatus := err.(interface{ GRPCStatus() *status.Status })
	if hasStatus {
		value := native.GRPCStatus()
		if value == nil || value.Code() != codes.NotFound {
			return false, false
		}
		if !wrapped {
			observed = true
		}
	}
	return wrapped || hasStatus, observed
}
