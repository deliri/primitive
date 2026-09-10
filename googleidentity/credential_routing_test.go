package googleidentity

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/auth/credentials"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const googleTestUniverse = "sovereign.invalid"

type googleCredentialRouteTransport struct {
	endpoint string
	method   string
	calls    int
}

func (r *googleCredentialRouteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.calls++
	r.endpoint = request.URL.String()
	r.method = request.Method
	return nil, io.ErrClosedPipe
}

// The real SDK builds each request. The transport refuses it before network IO,
// proving credential validation and routing without contacting Google.
func TestGoogleCredentialDocumentRoutingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		change       func(*serviceAccountDocument)
		changeJSON   func([]byte) []byte
		wantEndpoint string
		wantErr      error
	}{
		{name: "absent_token_uri_uses_sdk_documented_endpoint", wantEndpoint: googleServiceAccountTokenURL, wantErr: io.ErrClosedPipe},
		{name: "explicit_default_universe_keeps_oauth_protocol", change: func(d *serviceAccountDocument) { d.UniverseDomain = googleServiceAccountUniverse }, wantEndpoint: googleServiceAccountTokenURL, wantErr: io.ErrClosedPipe},
		{name: "explicit_endpoint_is_not_silently_replaced", change: func(d *serviceAccountDocument) { d.TokenURI = "https://credential.invalid/token" }, wantEndpoint: "https://credential.invalid/token", wantErr: io.ErrClosedPipe},
		{name: "nondefault_universe_keeps_sdk_iam_and_exchange_client", change: func(d *serviceAccountDocument) { d.UniverseDomain = googleTestUniverse }, wantEndpoint: "https://iamcredentials." + googleTestUniverse + "/v1/projects/-/serviceAccounts/" + serviceAccountFixtureEmail + ":generateIdToken", wantErr: io.ErrClosedPipe},
		{name: "documented_metadata_fields_do_not_change_intent", change: func(d *serviceAccountDocument) {
			d.ProjectID = "fixture-project"
			d.ClientID = "123"
			d.AuthURI = "https://accounts.google.com/o/oauth2/auth"
			d.AuthProviderX509CertURL = "https://www.googleapis.com/oauth2/v1/certs"
			d.ClientX509CertURL = "https://www.googleapis.com/robot/v1/metadata/x509/fixture"
			d.QuotaProjectID = "quota"
		}, wantEndpoint: googleServiceAccountTokenURL, wantErr: io.ErrClosedPipe},
		{name: "missing_email_cannot_issue", change: func(d *serviceAccountDocument) { d.ClientEmail = "" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "missing_key_cannot_issue", change: func(d *serviceAccountDocument) { d.PrivateKey = "" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "malformed_key_cannot_reach_provider", change: func(d *serviceAccountDocument) { d.PrivateKey = "not a private key" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "foreign_credential_cannot_discover_ambient_identity", change: func(d *serviceAccountDocument) { d.Type = credentials.AuthorizedUser }, wantErr: core.ErrGoogleIdentityContract},
		{name: "relative_token_endpoint_cannot_be_repaired", change: func(d *serviceAccountDocument) { d.TokenURI = "/token" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "unknown_credential_field_is_not_a_hidden_contract", changeJSON: func(b []byte) []byte { return append(b[:len(b)-1], []byte(",\"unowned\":true}")...) }, wantErr: core.ErrJSONContract},
		{name: "duplicate_email_cannot_change_signing_identity", changeJSON: func(b []byte) []byte { return append(b[:len(b)-1], []byte(",\"client_email\":\"other@invalid\"}")...) }, wantErr: core.ErrJSONContract},
		{name: "trailing_document_cannot_replace_credential", changeJSON: func(b []byte) []byte { return append(b, []byte("{}")...) }, wantErr: core.ErrJSONContract},
		{name: "large_credential_whitespace_is_admitted", changeJSON: func(b []byte) []byte { return append([]byte(strings.Repeat(" ", 128<<10)), b...) }, wantEndpoint: googleServiceAccountTokenURL, wantErr: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key, err := verifierTestKeys.ReadFile("testdata/verifier_rsa.pem")
			if err != nil {
				t.Fatal(err)
			}
			wire := serviceAccountDocument{Type: credentials.ServiceAccount, ClientEmail: serviceAccountFixtureEmail, PrivateKey: string(key), PrivateKeyID: serviceAccountFixtureKeyID}
			if tc.change != nil {
				tc.change(&wire)
			}
			data, err := core.MarshalCanonicalJSONDocument(wire)
			if err != nil {
				t.Fatal(err)
			}
			if tc.changeJSON != nil {
				data = tc.changeJSON(data)
			}
			source := serviceAccountSourceWithBytes(t, t.TempDir(), data)
			route := &googleCredentialRouteTransport{}
			source.client, err = exchange.NewClient(&http.Client{Transport: route})
			if err != nil {
				t.Fatal(err)
			}
			got, err := source.Acquire(t.Context(), serviceAccountFixtureRequest(t))
			wantCalls := 0
			wantMethod := ""
			if tc.wantEndpoint != "" {
				wantCalls = 1
				wantMethod = http.MethodPost
			}
			if got != (Token{}) || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrGoogleIdentityContract) || route.endpoint != tc.wantEndpoint || route.calls != wantCalls || route.method != wantMethod {
				t.Fatalf("token=%v error=%v endpoint=%q method=%q calls=%d, want zero, %v, %q, %q, %d", got, err, route.endpoint, route.method, route.calls, tc.wantErr, tc.wantEndpoint, wantMethod, wantCalls)
			}
		})
	}
}
