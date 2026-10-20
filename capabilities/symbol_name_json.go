package capabilities

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// MarshalJSON owns one identifier scalar's output allocation. Native Go lexical
// admission remains ParseSymbolName's contract; no JSON extent quota applies.
func (n SymbolName) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, err
	}
	data, err := core.MarshalCanonicalJSONString(n.value)
	if err != nil {
		return nil, errors.Join(core.ErrCapabilitiesContract, err)
	}
	return data, nil
}

// UnmarshalJSON admits one native identifier and commits only after both JSON
// and Go lexical admission succeed. A rejected wire value preserves the receiver.
func (n *SymbolName) UnmarshalJSON(data []byte) error {
	if n == nil {
		return core.ErrCapabilitiesContract
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	candidate, err := ParseSymbolName(value)
	if err != nil {
		return err
	}
	*n = candidate
	return nil
}

var _ core.ValidatedJSONMarshaler = SymbolName{}
