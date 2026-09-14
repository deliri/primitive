package permit

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Action{}
	_ core.ValidatedJSONMarshaler = Actions{}
	_ core.ValidatedJSONMarshaler = Revision(0)
	_ core.ValidatedJSONMarshaler = Document{}
	_ core.ValidatedJSONMarshaler = RegistrationResponse{}
	_ core.ValidatedJSONMarshaler = CheckInResponse{}
	_ core.ValidatedJSONMarshaler = QuietPeriods{}
	_ core.ValidatedJSONMarshaler = ReportPermissionResponse{}
	_ core.ValidatedJSONMarshaler = ReportProjectID{}
	_ core.ValidatedJSONMarshaler = SignedReport{}
	_ core.ValidatedJSONMarshaler = SignedProjectPermission{}
	_ core.ValidatedJSONMarshaler = SignedReportAcknowledgment{}
	_ core.ValidatedJSONMarshaler = SignedReportAuthorization{}
)
