package googleidentity

import (
	"context"
	"net/http"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/credentials/impersonate"
	"cloud.google.com/go/auth/httptransport"
	"github.com/deliri/primitive/v2026/exchange"
)

// Google's IAM ID-token endpoint requires an authenticated service-account
// request. The SDK owns both the self-signed access JWT and the IAM protocol.
const googleIAMCredentialScope = "https://www.googleapis.com/auth/cloud-platform"

func serviceAccountIAMToken(ctx context.Context, client *http.Client, audience Audience, document []byte, wire serviceAccountDocument) (Token, error) {
	base, err := credentials.NewCredentialsFromJSON(credentials.ServiceAccount, document, &credentials.DetectOptions{
		Client: client, Scopes: []string{googleIAMCredentialScope}, UseSelfSignedJWT: true,
	})
	if err != nil {
		return Token{}, contractError(err)
	}
	authenticated, err := httptransport.NewClient(&httptransport.Options{
		BaseRoundTripper: client.Transport, Credentials: base, UniverseDomain: wire.UniverseDomain, DisableTelemetry: true,
	})
	if err != nil {
		return Token{}, contractError(err)
	}
	owned, err := exchange.NewOfficialSDKHTTPClient(authenticated.Transport)
	if err != nil {
		return Token{}, contractError(err)
	}
	provider, err := impersonate.NewIDTokenCredentials(&impersonate.IDTokenOptions{
		Audience: audience.String(), TargetPrincipal: wire.ClientEmail, Credentials: base, Client: owned, UniverseDomain: wire.UniverseDomain,
	})
	if err != nil {
		return Token{}, contractError(err)
	}
	return tokenFromGoogleProvider(ctx, provider)
}
