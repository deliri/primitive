package googleidentity

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	googleMetadataBaseURL      = "http://metadata.google.internal"
	googleMetadataIdentityPath = "/computeMetadata/v1/instance/service-accounts/default/identity"
	googleMetadataIdentityURL  = googleMetadataBaseURL + googleMetadataIdentityPath
	googleMetadataHeaderName   = "Metadata-Flavor"
	googleMetadataHeaderValue  = "Google"
	googleAudienceQueryName    = "audience"
	googleFormatQueryName      = "format"
	googleFormatStandardValue  = "standard"
)

type googleProtocolContracts struct {
	metadataHeader core.HTTPHeaderName
	endpoint       core.HTTPEndpoint
	responseLimit  core.ByteCount
}

func googleContracts() (googleProtocolContracts, error) {
	endpoint, err := core.ParseHTTPEndpoint(googleMetadataIdentityURL)
	if err != nil {
		return googleProtocolContracts{}, contractError(err)
	}
	header, err := core.ParseHTTPHeaderName(googleMetadataHeaderName)
	if err != nil {
		return googleProtocolContracts{}, contractError(err)
	}
	// Google answers with the bare token and no envelope, so its transport
	// bound is the common token bound itself.
	limit, err := core.NewByteCount(TokenMaximumBytes)
	if err != nil {
		return googleProtocolContracts{}, contractError(err)
	}
	return googleProtocolContracts{
		endpoint:       endpoint,
		metadataHeader: header,
		responseLimit:  limit,
	}, nil
}

// AcquireGoogleCloud obtains one opaque identity-token bearer from Google
// Cloud's metadata service for the exact requested audience. It explicitly
// requests the standard format: the common contract identifies the attached
// service account and does not promise Google-specific VM or project claims.
func AcquireGoogleCloud(
	ctx context.Context,
	client Client,
	request IdentityTokenRequest,
) (Token, error) {
	if err := validateAcquisition(client, request); err != nil {
		return Token{}, err
	}
	contracts, err := googleContracts()
	if err != nil {
		return Token{}, err
	}
	target, err := googleCloudTarget(contracts.endpoint, request.Audience)
	if err != nil {
		return Token{}, err
	}
	headers, err := googleMetadataHeaders(contracts.metadataHeader)
	if err != nil {
		return Token{}, err
	}
	response, err := acquire(acquisitionCall{
		context:        ctx,
		client:         client,
		target:         target,
		headers:        headers,
		responseHeader: contracts.metadataHeader,
		responseLimit:  contracts.responseLimit,
		policy:         request.Policy,
	})
	if err != nil {
		return Token{}, err
	}
	return newToken(string(response.Body))
}

func googleMetadataHeaders(name core.HTTPHeaderName) (exchange.Headers, error) {
	value, err := exchange.NewHeaderValue(googleMetadataHeaderValue)
	if err != nil {
		return exchange.Headers{}, contractError(err)
	}
	headers := exchange.Headers{Values: []exchange.Header{{
		Name: name, Values: []exchange.HeaderValue{value},
	}}}
	return headers, headers.Validate()
}

func googleCloudTarget(
	base core.HTTPEndpoint,
	audience Audience,
) (core.HTTPEndpoint, error) {
	target := base.HTTPURL()
	query := target.Query()
	query.Set(googleAudienceQueryName, audience.String())
	query.Set(googleFormatQueryName, googleFormatStandardValue)
	target.RawQuery = query.Encode()
	endpoint, err := core.ParseHTTPEndpoint(target.String())
	if err != nil {
		return core.HTTPEndpoint{}, contractError(err)
	}
	return endpoint, nil
}
