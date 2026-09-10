package tailnet

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	tailscale "tailscale.com/client/tailscale/v2"
)

const (
	tokenExchangePath   = "/api/v2/oauth/token-exchange"
	createKeyPath       = "/api/v2/tailnet/-/keys"
	enrollmentAPIHost   = "api.tailscale.com"
	authKeyMaximumBytes = 1024
	authKeyPrefix       = "tskey-auth-"
)

// enrollmentTransport belongs to one synchronous enrollment call. The provider
// token source creates background requests; binding them to this call prevents
// it from dropping cancellation. No transport escapes the owning call.
type enrollmentTransport struct {
	context context.Context
	base    http.RoundTripper
}

func (t enrollmentTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || t.context == nil || t.base == nil {
		return nil, core.ErrTailnetContract
	}
	if err := errors.Join(t.context.Err(), validateEnrollmentRequest(request)); err != nil {
		if request.Body != nil {
			err = errors.Join(err, request.Body.Close())
		}
		return nil, err
	}
	return t.base.RoundTrip(request.Clone(t.context))
}

func enrollmentHTTPClient(ctx context.Context, client exchange.Client) (*http.Client, error) {
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{
		Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationJSON,
	})
	if err != nil {
		return nil, err
	}
	transport, err := client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return nil, err
	}
	return exchange.NewOfficialSDKHTTPClient(enrollmentTransport{context: ctx, base: transport})
}

// acquireAuthKey uses the official provider client for both identity federation
// and key creation. The key is ephemeral, single-use, and scoped to the exact tag.
func acquireAuthKey(ctx context.Context, client exchange.Client, configuration tailnetconfig.Configuration, token googleidentity.Token) (string, error) {
	if ctx == nil {
		return "", core.ErrTailnetContract
	}
	if err := errors.Join(configuration.Validate(), token.Validate(), client.Validate(), ctx.Err()); err != nil {
		return "", errors.Join(core.ErrTailnetEnrollment, err)
	}
	httpClient, err := enrollmentHTTPClient(ctx, client)
	if err != nil {
		return "", errors.Join(core.ErrTailnetEnrollment, err)
	}
	bearer, err := token.BearerValue()
	if err != nil {
		return "", errors.Join(core.ErrTailnetEnrollment, err)
	}
	identity, found := strings.CutPrefix(bearer, exchange.BearerAuthorizationScheme+" ")
	if !found {
		return "", core.ErrTailnetEnrollment
	}
	sdk := tailscale.Client{HTTP: httpClient, Auth: &tailscale.IdentityFederation{
		ClientID: configuration.ClientID.String(), IDTokenFunc: func() (string, error) { return identity, nil },
	}}
	request := tailscale.CreateKeyRequest{}
	request.Capabilities.Devices.Create.Ephemeral = true
	request.Capabilities.Devices.Create.Tags = []string{configuration.Tag.String()}
	key, err := sdk.Keys().CreateAuthKey(ctx, request)
	if err != nil {
		return "", errors.Join(core.ErrTailnetEnrollment, err)
	}
	if err := errors.Join(ctx.Err(), validateReturnedKey(key, configuration.Tag)); err != nil {
		return "", errors.Join(core.ErrTailnetEnrollment, err)
	}
	return key.Key, nil
}

func validateReturnedKey(key *tailscale.Key, tag tailnetconfig.Tag) error {
	if key == nil {
		return core.ErrTailnetEnrollment
	}
	create := key.Capabilities.Devices.Create
	if key.Invalid || !key.Revoked.IsZero() || create.Reusable || !create.Ephemeral || create.Preauthorized || !slices.Equal(create.Tags, []string{tag.String()}) {
		return core.ErrTailnetEnrollment
	}
	return validateAuthKey(key.Key)
}

func validateAuthKey(value string) error {
	if len(value) > authKeyMaximumBytes || !strings.HasPrefix(value, authKeyPrefix) || len(value) == len(authKeyPrefix) {
		return core.ErrTailnetEnrollment
	}
	for _, character := range value[len(authKeyPrefix):] {
		if authKeyCharacter(character) {
			continue
		}
		return core.ErrTailnetEnrollment
	}
	return nil
}

// validateEnrollmentRequest prevents SDK credential omissions and protocol drift
// from escaping as an unauthenticated or differently addressed effect.
func validateEnrollmentRequest(request *http.Request) error {
	if request.URL == nil || request.URL.Scheme != "https" || request.URL.Host != enrollmentAPIHost || request.Method != http.MethodPost || request.URL.RawQuery != "" {
		return core.ErrTailnetEnrollment
	}
	switch request.URL.Path {
	case tokenExchangePath:
		return nil
	case createKeyPath:
		return validateEnrollmentAuthorization(request.Header.Get("Authorization"))
	default:
		return core.ErrTailnetEnrollment
	}
}

func authKeyCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-'
}

func validateEnrollmentAuthorization(authorization string) error {
	value, found := strings.CutPrefix(authorization, exchange.BearerAuthorizationScheme+" ")
	if !found || value == "" || strings.ContainsAny(value, " \t\r\n") {
		return core.ErrTailnetEnrollment
	}
	return nil
}
