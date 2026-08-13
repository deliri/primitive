package controlplane

// The exact member names this package's documents use.
//
// They are spelled once because they are protocol values: a member name is part
// of the bytes a signature covers, and the emitter and the field-name vocabulary
// that reports a binding failure must agree on it. Two literals could be
// corrected one at a time and nothing in the build would notice.
//
// Account and offering are not restated here. They name facts other packages
// also carry, so Core owns them and this package borrows the same value rather
// than writing a second copy that happens to match today.
const (
	protocolMemberBuild             = "build"
	protocolMemberRevision          = "revision"
	protocolMemberRouteFamily       = "route_family"
	protocolMemberRequestNonce      = "request_nonce"
	protocolMemberDevicePublicKey   = "device_public_key"
	protocolMemberRegistrationToken = "registration_token"
	protocolMemberInstallation      = "installation"
)
