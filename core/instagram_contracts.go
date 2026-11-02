package core

// Instagram's Meta Graph protocol facts are deliberately nominal. Equal values
// in another provider family do not create a shared contract.
//
// Documented by Meta (read 2026-11-02):
//   - Host for Instagram API with Facebook Login: graph.facebook.com
//     https://developers.facebook.com/docs/instagram-platform/reference/instagram-media
//   - v26.0 is the latest Graph API version, expiry TBD; v21.0 expires 2027-01-21
//     https://developers.facebook.com/docs/graph-api/changelog/versions/
//   - Captions: "Maximum 2200 characters"
//     https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/media
//   - Error code 190: "Access token has expired"
//     https://developers.facebook.com/docs/graph-api/guides/error-handling
//
// Primitive's own custody bounds (Meta documents none): media page items,
// media identity bytes and credential bytes.
const (
	InstagramGraphHost                     = "graph.facebook.com"
	InstagramGraphVersion                  = "v26.0"
	InstagramCaptionMaximumCharacters      = 2200
	InstagramAccessTokenErrorCode          = 190
	InstagramMediaPageCustodyMaximum       = 100
	InstagramMediaIDCustodyMaximumBytes    = 64
	InstagramCredentialMinimumBytes        = 32
	InstagramCredentialCustodyMaximumBytes = 4 * 1024
)
