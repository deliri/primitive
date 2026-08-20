package cloudidentity

import (
	"context"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	googleMetadataAccessTokenPath = "/computeMetadata/v1/instance/service-accounts/default/token"
	googleMetadataAccessTokenURL  = googleMetadataBaseURL + googleMetadataAccessTokenPath
	googleAccessTokenTypeBearer   = "Bearer"

	// GoogleCloudAccessTokenLifetimeMaximumSeconds is the largest whole-second
	// provider lifetime representable by temporal.Duration.
	GoogleCloudAccessTokenLifetimeMaximumSeconds = uint64(math.MaxInt64) / temporal.NanosecondsPerSecond
	// GoogleCloudAccessTokenResponseWhitespaceMaximumBytes admits bounded JSON
	// formatting without making provider whitespace an implicit protocol.
	GoogleCloudAccessTokenResponseWhitespaceMaximumBytes = 1024
	// googleAccessTokenResponseSyntaxMaximumBytes is the exact canonical JSON
	// framing extent around an empty token and the largest admitted lifetime.
	// The hostile typed-marshaler ratchet owns its agreement with the wire tags.
	googleAccessTokenResponseSyntaxMaximumBytes = 65
	// GoogleCloudAccessTokenResponseMaximumBytes bounds the complete metadata
	// response before strict decoding and token construction.
	GoogleCloudAccessTokenResponseMaximumBytes = TokenMaximumBytes + googleAccessTokenResponseSyntaxMaximumBytes + GoogleCloudAccessTokenResponseWhitespaceMaximumBytes
)

// GoogleCloudAccessTokenRequest is one Google Cloud access-token acquisition
// intent. It has no audience because the attached service account and its OAuth
// scopes are metadata-service owned facts.
type GoogleCloudAccessTokenRequest struct {
	Policy Policy
}

// Validate checks the complete single-attempt operation policy.
func (r GoogleCloudAccessTokenRequest) Validate() error { return r.Policy.Validate() }

// googleAccessTokenLifetimeSeconds owns Google's documented whole-second wire
// magnitude before it is raised into temporal.Duration.
type googleAccessTokenLifetimeSeconds uint64

func (s googleAccessTokenLifetimeSeconds) Validate() error {
	if s == 0 || uint64(s) > GoogleCloudAccessTokenLifetimeMaximumSeconds {
		return core.ErrCloudIdentityContract
	}
	return nil
}

type googleAccessTokenResponse struct {
	AccessToken string                           `json:"access_token"`
	TokenType   string                           `json:"token_type"`
	ExpiresIn   googleAccessTokenLifetimeSeconds `json:"expires_in"`
}

func (r googleAccessTokenResponse) Validate() error {
	if r.TokenType != googleAccessTokenTypeBearer {
		return core.ErrCloudIdentityContract
	}
	if err := r.ExpiresIn.Validate(); err != nil {
		return err
	}
	return validateBearerTokenValue(r.AccessToken)
}

func (r googleAccessTokenResponse) token() (AccessToken, error) {
	if err := r.Validate(); err != nil {
		return AccessToken{}, err
	}
	lifetime, err := temporal.DurationFromSeconds(uint64(r.ExpiresIn))
	if err != nil {
		return AccessToken{}, contractError(err)
	}
	return newGoogleAccessToken(r.AccessToken, lifetime)
}

type googleAccessTokenContracts struct {
	endpoint      core.HTTPEndpoint
	responseLimit core.ByteCount
}

func googleAccessContracts() (googleAccessTokenContracts, error) {
	endpoint, err := core.ParseHTTPEndpoint(googleMetadataAccessTokenURL)
	if err != nil {
		return googleAccessTokenContracts{}, contractError(err)
	}
	limit, err := core.NewByteCount(uint64(GoogleCloudAccessTokenResponseMaximumBytes))
	if err != nil {
		return googleAccessTokenContracts{}, contractError(err)
	}
	return googleAccessTokenContracts{endpoint: endpoint, responseLimit: limit}, nil
}

// AcquireGoogleCloudAccessToken obtains one OAuth access bearer from Google
// Cloud's documented metadata protocol through Primitive Exchange. It does not
// discover credentials, cache, refresh, or authorize a consumer operation.
func AcquireGoogleCloudAccessToken(
	ctx context.Context,
	client Client,
	request GoogleCloudAccessTokenRequest,
) (AccessToken, error) {
	if err := validateAcquisition(client, request); err != nil {
		return AccessToken{}, err
	}
	contracts, err := googleAccessContracts()
	if err != nil {
		return AccessToken{}, err
	}
	header, err := core.ParseHTTPHeaderName(googleMetadataHeaderName)
	if err != nil {
		return AccessToken{}, contractError(err)
	}
	headers, err := googleMetadataHeaders(header)
	if err != nil {
		return AccessToken{}, err
	}
	response, err := acquire(acquisitionCall{
		context:       ctx,
		client:        client,
		target:        contracts.endpoint,
		headers:       headers,
		responseLimit: contracts.responseLimit,
		policy:        request.Policy,
	})
	if err != nil {
		return AccessToken{}, err
	}
	wire, err := decodeGoogleAccessTokenResponse(response.Body)
	if err != nil {
		return AccessToken{}, err
	}
	return wire.token()
}

func decodeGoogleAccessTokenResponse(body []byte) (googleAccessTokenResponse, error) {
	maximum, err := core.NewByteCount(uint64(GoogleCloudAccessTokenResponseMaximumBytes))
	if err != nil {
		return googleAccessTokenResponse{}, contractError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	limits.NestingDepthMaximum = 1
	limits.ObjectFieldMaximum = 3
	limits.ArrayItemMaximum = 1
	wire, err := core.DecodeStrictJSONStructure[googleAccessTokenResponse](body, limits)
	if err != nil {
		return googleAccessTokenResponse{}, contractError(err)
	}
	if err := wire.Validate(); err != nil {
		return googleAccessTokenResponse{}, contractError(err)
	}
	return wire, nil
}

var (
	_ core.Validatable = GoogleCloudAccessTokenRequest{}
	_ core.Validatable = googleAccessTokenResponse{}
)
