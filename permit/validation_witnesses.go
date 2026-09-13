package permit

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Action{}
	_ core.ValidatedJSONMarshaler = Actions{}
	_ core.ValidatedJSONMarshaler = Revision(0)
	_ core.ValidatedJSONMarshaler = Document{}
	_ core.ValidatedJSONMarshaler = RegistrationResponse{}
	_ core.ValidatedJSONMarshaler = CheckInResponse{}
)
