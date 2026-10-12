package hostfacts

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"strings"
	"unicode"
	"unicode/utf8"
)

// UserName is the native account database label for the effective process user.
// It is an observed label, without an authentication or remote-identity claim.
type UserName struct{ value string }

func (n UserName) String() string { return n.value }
func (n UserName) Validate() error {
	if n.value == "" || !utf8.ValidString(n.value) || strings.ContainsFunc(n.value, unicode.IsControl) {
		return core.ErrHostFactsContract
	}
	return nil
}

func ObserveEffectiveUser(ctx context.Context) (UserName, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return UserName{}, err
	}
	name, err := observeEffectiveUser()
	if err := errors.Join(err, contextstate.Validate(ctx)); err != nil {
		identity := core.ErrHostFactsObservation
		if errors.Is(err, core.ErrHostFactsUnsupported) {
			identity = core.ErrHostFactsUnsupported
		}
		return UserName{}, fail(OperationEffectiveUser, identity, err)
	}
	observed := UserName{value: name}
	if err := observed.Validate(); err != nil {
		return UserName{}, fail(OperationEffectiveUser, core.ErrHostFactsObservation, err)
	}
	return observed, nil
}
