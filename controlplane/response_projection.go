package controlplane

import (
	"bytes"

	"github.com/deliri/primitive/v2026/core"
)

// validateTypedResponseProjection closes a bidirectional signed response at
// egress. These responses already own one complete envelope, so wrapping them
// in ResponseProjection would create a second signature instead of improving
// the agreement.
func validateTypedResponseProjection[T core.ValidatedJSONMarshaler](
	value T,
	encoded []byte,
	limits core.StrictJSONLimits,
) error {
	decoded, err := core.DecodeStrictJSON[T](encoded, limits)
	if err != nil {
		return err
	}
	canonical, err := decoded.MarshalJSON()
	if err != nil {
		return err
	}
	if !bytes.Equal(encoded, canonical) {
		return core.ErrJSONContract
	}
	want, err := value.MarshalJSON()
	if err != nil {
		return err
	}
	if !bytes.Equal(encoded, want) {
		return core.ErrJSONContract
	}
	return nil
}
