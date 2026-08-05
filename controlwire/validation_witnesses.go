package controlwire

import (
	"encoding/json"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

var (
	_ core.Validatable = Revision(0)
	_ core.Validatable = RequestNonce{}
	_ core.Validatable = RegistrationToken{}
	_ core.Validatable = RegistrationTokenVerifier{}

	_ core.ValidatedJSONMarshaler = Revision(0)
	_ core.ValidatedJSONMarshaler = RequestNonce{}
	_ core.ValidatedJSONMarshaler = RegistrationToken{}
	_ core.ValidatedJSONMarshaler = RegistrationTokenVerifier{}

	_ json.Unmarshaler = (*Revision)(nil)
	_ json.Unmarshaler = (*RequestNonce)(nil)
	_ json.Unmarshaler = (*RegistrationToken)(nil)
	_ json.Unmarshaler = (*RegistrationTokenVerifier)(nil)

	// The registration token is the only secret this package holds. Its
	// redaction is a compiler-visible obligation, not a convention.
	_ fmt.Formatter = RegistrationToken{}
)
