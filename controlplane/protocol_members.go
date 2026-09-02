package controlplane

// The exact member names this package's documents use.
//
// They are spelled once because they are protocol values: a member name is part
// of the bytes a signature covers, and the emitter and the field-name vocabulary
// that reports a binding failure must agree on it. Two literals could be
// corrected one at a time and nothing in the build would notice.
//
// Offering is not restated here. It names the same fact in another package, so
// Core owns it and this package borrows that value. Account currently belongs
// only to this control-plane protocol and remains local rather than pretending
// to be a shared contract.
const (
	protocolMemberAccount           = "account"
	protocolMemberBuild             = "build"
	protocolMemberRevision          = "revision"
	protocolMemberRouteFamily       = "route_family"
	protocolMemberRequestNonce      = "request_nonce"
	protocolMemberDevicePublicKey   = "device_public_key"
	protocolMemberRegistrationToken = "registration_token"
	protocolMemberInstallation      = "installation"
)
