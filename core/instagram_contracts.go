package core

// Instagram's Meta Graph protocol facts are deliberately nominal. Equal values
// in another provider family do not create a shared contract.
const (
	InstagramGraphHost                     = "graph.facebook.com"
	InstagramGraphVersion                  = "v21.0"
	InstagramMediaPageMaximum              = 50
	InstagramMediaIDMaximumBytes           = 64
	InstagramCaptionMaximumBytes           = 2200 * 4
	InstagramCredentialMinimumBytes        = 32
	InstagramCredentialCustodyMaximumBytes = 4 * 1024
)
