package runnercontrol

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

// IsGoEventGrammarError reports an owned cmd/go grammar refusal with no
// unrelated native cause. A bare JSON contract, nominal consumer refusal or
// joined I/O/cancellation error cannot acquire grammar authority from a sibling.
func IsGoEventGrammarError(err error) bool {
	if core.IsNil(err) || !errors.Is(err, core.ErrGoEventGrammar) {
		return false
	}
	return core.ErrorBranchesMatch(core.ErrorBranchesRequest{Error: err, Accept: func(node core.ErrorBranchObservation) bool {
		if _, owned := node.Error.(goEventGrammarCause); owned {
			return true
		}
		if node.Unwrapped != nil {
			return false
		}
		identity, ok := node.Error.(core.ErrorIdentity)
		return ok && (identity == core.ErrGoEventGrammar || identity == core.ErrJSONContract || identity == core.ErrPrimitiveContract)
	}})
}

// Only the parser's pure native unquote operation creates this cause. Its
// private type prevents a foreign JSON codec error from lending I/O authority.
type goEventGrammarCause struct{ cause error }

func (e goEventGrammarCause) Error() string {
	return fmt.Sprintf("%v: %v", core.ErrGoEventGrammar, e.cause)
}
func (e goEventGrammarCause) Unwrap() error            { return e.cause }
func (goEventGrammarCause) Is(target error) bool       { return errors.Is(core.ErrGoEventGrammar, target) }
func (goEventGrammarCause) runnerControlInternalFlow() {}
