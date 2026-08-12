package cloudidentity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// googleIdentityPath is the metadata path the audience request must reach.
const googleIdentityPath = "/computeMetadata/v1/instance/service-accounts/default/identity"

// providerAcquisition is one provider's complete entry point, so both providers
// run every layer of the triad rather than one standing in for the other.
type providerAcquisition struct {
	acquire  func(context.Context, Client, Audience) (Token, error)
	client   func(testing.TB, http.Handler) Client
	success  func(string) string
	name     string
	provider Provider
}

func providerAcquisitions() []providerAcquisition {
	return []providerAcquisition{
		{
			name:     "Google Cloud metadata",
			provider: ProviderGoogleCloud,
			success:  func(token string) string { return token },
			client:   googlePlaintextClient,
			acquire: func(
				ctx context.Context,
				client Client,
				audience Audience,
			) (Token, error) {
				return AcquireGoogleCloud(ctx, client, Request{
					Audience: audience,
					Policy:   defaultPolicyOrZero(),
				})
			},
		},
		{
			name:     "AWS regional STS",
			provider: ProviderAmazonWebServices,
			success:  amazonResponseXML,
			client:   amazonTLSClient,
			acquire: func(
				ctx context.Context,
				client Client,
				audience Audience,
			) (Token, error) {
				request, err := NewAmazonWebServicesRequest(
					AmazonWebServicesRequestInput{
						Request: Request{
							Audience: audience,
							Policy:   defaultPolicyOrZero(),
						},
						SignedURL: amazonSignedURL(
							audience,
							amazonTestHost,
						),
					},
				)
				if err != nil {
					return Token{}, err
				}
				return AcquireAmazonWebServices(ctx, client, request)
			},
		},
	}
}

// defaultPolicyOrZero keeps the triad's provider closures free of a testing
// handle. A zero policy would fail validation loudly, so a broken default
// cannot pass silently.
func defaultPolicyOrZero() Policy {
	policy, err := DefaultPolicy()
	if err != nil {
		return Policy{}
	}
	return policy
}

// countingHandler answers with one fixed status and body while recording how
// many requests actually reached the provider. The neutral layer depends on that
// count: a refusal that still contacted the authority already had an external
// effect.
type countingHandler struct {
	requests *atomic.Uint64
	body     string
	status   int
}

func (h countingHandler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	h.requests.Add(1)
	responseWithBody(writer, h.status, h.body)
}

// TestProviderAcquisitionEntryPointsLayerTriad proves all three layers for both
// entry points.
//
// positive: the exact provider request is emitted and one validated common token
// comes back.
// negative: a provider answer outside the contract fails loudly with the typed
// identity and yields no token.
// neutral: an ingress defect is refused without contacting the authority, so
// there is no token, no external effect, and no fabricated success.
func TestProviderAcquisitionEntryPointsLayerTriad(t *testing.T) {
	t.Parallel()

	for _, provider := range providerAcquisitions() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()

			t.Run("positive", func(t *testing.T) {
				t.Parallel()
				proveAcquisitionSucceeds(t, provider)
			})
			t.Run("negative", func(t *testing.T) {
				t.Parallel()
				proveAcquisitionFailsLoudly(t, provider)
			})
			t.Run("neutral", func(t *testing.T) {
				t.Parallel()
				proveIngressDefectNeverReachesProvider(t, provider)
			})
		})
	}
}

func proveAcquisitionSucceeds(t *testing.T, provider providerAcquisition) {
	t.Helper()

	audience := mustAudience(t, "https://api.example.com/release")
	var requests atomic.Uint64
	client := provider.client(t, http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			requests.Add(1)
			proveProviderRequestShape(t, request, audience)
			responseWithBody(
				writer,
				http.StatusOK,
				provider.success(testIdentityToken),
			)
		},
	))
	got, gotErr := provider.acquire(t.Context(), client, audience)
	if gotErr != nil {
		t.Fatalf("acquisition error = %v, want nil", gotErr)
	}
	if got.Provider() != provider.provider || got.Validate() != nil {
		t.Fatalf(
			"acquired token = (%v, %v), want (%v, nil)",
			got.Provider(),
			got.Validate(),
			provider.provider,
		)
	}
	gotBearer, gotBearerErr := got.BearerValue()
	wantBearer := bearerPrefix + testIdentityToken
	if gotBearerErr != nil || gotBearer != wantBearer {
		t.Fatalf(
			"Token.BearerValue() = (%q, %v), want (%q, nil)",
			gotBearer,
			gotBearerErr,
			wantBearer,
		)
	}
	if gotRequests := requests.Load(); gotRequests != 1 {
		t.Fatalf("provider request count = %d, want 1", gotRequests)
	}
}

// proveProviderRequestShape checks the exact outbound contract each authority
// publishes, including the audience and the explicitly selected Google format.
func proveProviderRequestShape(
	t *testing.T,
	request *http.Request,
	audience Audience,
) {
	t.Helper()

	if request.Method != exchange.MethodGet.String() {
		t.Errorf(
			"request method = %q, want %q",
			request.Method,
			exchange.MethodGet.String(),
		)
	}
	query := request.URL.Query()
	if request.URL.Path == googleIdentityPath {
		if got := request.Header.Get(googleMetadataHeaderName); got !=
			googleMetadataHeaderValue {
			t.Errorf(
				"metadata header = %q, want %q",
				got,
				googleMetadataHeaderValue,
			)
		}
		if len(query) != 2 ||
			query.Get(googleAudienceQueryName) != audience.String() ||
			query.Get(googleFormatQueryName) !=
				googleFormatStandardValue {
			t.Errorf(
				"Google query = %v, want audience %q and format %q",
				query,
				audience.String(),
				googleFormatStandardValue,
			)
		}
		return
	}
	if request.URL.Path != "/" {
		t.Errorf("request path = %q, want the STS root", request.URL.Path)
	}
	if query.Get(amazonActionQuery) != amazonActionValue ||
		query.Get(amazonAudienceQuery) != audience.String() {
		t.Errorf(
			"AWS action and audience = (%q, %q), want (%q, %q)",
			query.Get(amazonActionQuery),
			query.Get(amazonAudienceQuery),
			amazonActionValue,
			audience.String(),
		)
	}
}

func proveAcquisitionFailsLoudly(t *testing.T, provider providerAcquisition) {
	t.Helper()

	audience := mustAudience(t, "https://api.example.com/release")
	var requests atomic.Uint64
	client := provider.client(t, countingHandler{
		requests: &requests,
		status:   http.StatusForbidden,
		body:     "denied",
	})
	got, gotErr := provider.acquire(t.Context(), client, audience)
	if !errors.Is(gotErr, core.ErrCloudIdentityContract) ||
		!errors.Is(gotErr, core.ErrExchangeResponse) {
		t.Fatalf(
			"refused acquisition error = %v, want cloud identity and exchange response identities",
			gotErr,
		)
	}
	var statusErr exchange.StatusError
	if !errors.As(gotErr, &statusErr) {
		t.Fatalf(
			"refused acquisition error = %v, want a reachable exchange.StatusError",
			gotErr,
		)
	}
	if gotStatus, _ := statusErr.Status().Int(); gotStatus !=
		http.StatusForbidden {
		t.Fatalf(
			"exchange.StatusError.Status() = %d, want %d",
			gotStatus,
			http.StatusForbidden,
		)
	}
	if got != (Token{}) {
		t.Fatalf("refused acquisition token = %#v, want zero", got)
	}
	if gotRequests := requests.Load(); gotRequests != 1 {
		t.Fatalf(
			"refused acquisition request count = %d, want exactly one attempt",
			gotRequests,
		)
	}
}

func proveIngressDefectNeverReachesProvider(
	t *testing.T,
	provider providerAcquisition,
) {
	t.Helper()

	for _, tc := range []struct {
		name        string
		useAudience bool
		useClient   bool
	}{
		{
			name:        "unset client is refused",
			useAudience: true,
		},
		{
			name:      "unset audience is refused",
			useClient: true,
		},
		{
			name: "unset client and audience are both refused",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Uint64
			reachable := provider.client(t, countingHandler{
				requests: &requests,
				status:   http.StatusOK,
				body:     provider.success(testIdentityToken),
			})
			var client Client
			if tc.useClient {
				client = reachable
			}
			var audience Audience
			if tc.useAudience {
				audience = mustAudience(
					t,
					"https://api.example.com/release",
				)
			}
			got, gotErr := provider.acquire(
				t.Context(),
				client,
				audience,
			)
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"ingress refusal error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			if got != (Token{}) {
				t.Fatalf("ingress refusal token = %#v, want zero", got)
			}
			if gotRequests := requests.Load(); gotRequests != 0 {
				t.Fatalf(
					"provider request count after ingress refusal = %d, want 0",
					gotRequests,
				)
			}
		})
	}
}

// TestAcquisitionRefusesAnUnsetSignedCapability keeps the AWS capability's set
// flag honest: a zero value that skipped the constructor must never execute.
func TestAcquisitionRefusesAnUnsetSignedCapability(t *testing.T) {
	t.Parallel()

	var requests atomic.Uint64
	client := amazonTLSClient(t, countingHandler{
		requests: &requests,
		status:   http.StatusOK,
		body:     amazonResponseXML(testIdentityToken),
	})
	got, gotErr := AcquireAmazonWebServices(
		t.Context(),
		client,
		AmazonWebServicesRequest{},
	)
	if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
		t.Fatalf(
			"AcquireAmazonWebServices(zero capability) error = %v, want %v",
			gotErr,
			core.ErrCloudIdentityContract,
		)
	}
	if got != (Token{}) || requests.Load() != 0 {
		t.Fatalf(
			"AcquireAmazonWebServices(zero capability) = (%#v, %d requests), want (zero, 0)",
			got,
			requests.Load(),
		)
	}
}

// TestProviderResponseBoundsProjectTheTokenBound proves each provider's
// transport bound is derived from the one admissible token extent. A looser
// bound would let an authority push bytes Cloudidentity can never accept.
func TestProviderResponseBoundsProjectTheTokenBound(t *testing.T) {
	t.Parallel()

	google, gotErr := googleContracts()
	if gotErr != nil {
		t.Fatalf(
			"googleContracts() error = %v, want nil",
			gotErr,
		)
	}
	gotGoogleLimit, gotErr := google.responseLimit.Int64()
	if gotErr != nil || gotGoogleLimit != TokenMaximumBytes {
		t.Fatalf(
			"Google response limit = (%d, %v), want (%d, nil)",
			gotGoogleLimit,
			gotErr,
			TokenMaximumBytes,
		)
	}
	if AmazonResponseMaximumBytes !=
		TokenMaximumBytes+AmazonEnvelopeMaximumBytes {
		t.Fatalf(
			"AmazonResponseMaximumBytes = %d, want the token bound plus the envelope %d",
			AmazonResponseMaximumBytes,
			TokenMaximumBytes+AmazonEnvelopeMaximumBytes,
		)
	}
	if int64(AmazonResponseMaximumBytes) <= gotGoogleLimit {
		t.Fatalf(
			"AmazonResponseMaximumBytes = %d, want more than the bare token bound %d",
			AmazonResponseMaximumBytes,
			gotGoogleLimit,
		)
	}
}

func TestAcquisitionRejectsRedirectWithoutReachingTarget(t *testing.T) {
	t.Parallel()

	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			reached.Store(true)
		},
	))
	t.Cleanup(target.Close)
	redirect := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(
				writer,
				request,
				target.URL,
				http.StatusTemporaryRedirect,
			)
		},
	))
	t.Cleanup(redirect.Close)
	client := mustTestClient(t, redirect.URL, redirect.Client().Transport)
	_, gotErr := AcquireGoogleCloud(
		t.Context(),
		client,
		Request{
			Audience: mustAudience(t, "https://api.example.com"),
			Policy:   mustPolicy(t),
		},
	)
	if !errors.Is(gotErr, core.ErrCloudIdentityContract) ||
		!errors.Is(gotErr, core.ErrExchangeRedirect) {
		t.Fatalf(
			"AcquireGoogleCloud() error = %v, want cloud identity and redirect errors",
			gotErr,
		)
	}
	if reached.Load() {
		t.Fatal("redirect target reached = true, want false")
	}
}

func TestAcquisitionResponsePressureTable(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com")
	for _, tc := range []struct {
		wantErr    error
		name       string
		body       string
		status     int
		wantStatus bool
	}{
		{name: "opaque token response is accepted", body: "a.b.c", status: http.StatusOK},
		{name: "single byte token is accepted", body: "a", status: http.StatusOK},
		{name: "padded token is accepted", body: "abc==", status: http.StatusOK},
		{name: "one below the token bound is accepted", body: strings.Repeat("a", TokenMaximumBytes-1), status: http.StatusOK},
		{name: "exact token bound is accepted", body: strings.Repeat("a", TokenMaximumBytes), status: http.StatusOK},
		{name: "empty response is rejected", status: http.StatusOK, wantErr: core.ErrCloudIdentityContract},
		{name: "space-bearing response is rejected", body: "a b", status: http.StatusOK, wantErr: core.ErrCloudIdentityContract},
		{name: "newline-bearing response is rejected without trimming", body: testIdentityToken + "\n", status: http.StatusOK, wantErr: core.ErrCloudIdentityContract},
		{name: "leading padding is rejected", body: "=abc", status: http.StatusOK, wantErr: core.ErrCloudIdentityContract},
		{name: "JSON envelope is rejected because the metadata contract is bare", body: `{"token":"a.b.c"}`, status: http.StatusOK, wantErr: core.ErrCloudIdentityContract},
		{name: "one above the token bound is refused on the wire", body: strings.Repeat("a", TokenMaximumBytes+1), status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit},
		{name: "far above the token bound is refused on the wire", body: strings.Repeat("a", 4*TokenMaximumBytes), status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit},
		{name: "provider refusal preserves typed status", body: "denied", status: http.StatusForbidden, wantErr: core.ErrExchangeResponse, wantStatus: true},
		{name: "provider outage remains one response failure", body: "unavailable", status: http.StatusServiceUnavailable, wantErr: core.ErrExchangeResponse, wantStatus: true},
		{name: "unauthorized metadata access preserves typed status", body: "unauthorized", status: http.StatusUnauthorized, wantErr: core.ErrExchangeResponse, wantStatus: true},
		{name: "created is outside the acquisition contract", body: "a.b.c", status: http.StatusCreated, wantErr: core.ErrExchangeResponse, wantStatus: true},
		{name: "no content is outside the acquisition contract", status: http.StatusNoContent, wantErr: core.ErrExchangeResponse, wantStatus: true},
		{name: "server fault is outside the acquisition contract", body: "boom", status: http.StatusInternalServerError, wantErr: core.ErrExchangeResponse, wantStatus: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Uint64
			client := googlePlaintextClient(t, countingHandler{
				requests: &requests,
				status:   tc.status,
				body:     tc.body,
			})
			got, gotErr := AcquireGoogleCloud(
				t.Context(),
				client,
				Request{
					Audience: audience,
					Policy:   mustPolicy(t),
				},
			)
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf(
						"AcquireGoogleCloud() = (%v, %v), want validated token",
						got,
						gotErr,
					)
				}
			} else {
				if !errors.Is(gotErr, core.ErrCloudIdentityContract) ||
					!errors.Is(gotErr, tc.wantErr) {
					t.Fatalf(
						"AcquireGoogleCloud() error = %v, want %v and %v",
						gotErr,
						core.ErrCloudIdentityContract,
						tc.wantErr,
					)
				}
				if got != (Token{}) {
					t.Fatalf("AcquireGoogleCloud() token = %#v, want zero", got)
				}
			}
			if tc.wantStatus {
				var statusErr exchange.StatusError
				if !errors.As(gotErr, &statusErr) {
					t.Fatalf(
						"AcquireGoogleCloud() error = %v, want exchange.StatusError",
						gotErr,
					)
				}
				gotStatus, _ := statusErr.Status().Int()
				if gotStatus != tc.status {
					t.Fatalf(
						"exchange.StatusError.Status() = %d, want %d",
						gotStatus,
						tc.status,
					)
				}
			}
			if gotRequests := requests.Load(); gotRequests != 1 {
				t.Fatalf("provider request count = %d, want 1", gotRequests)
			}
		})
	}
}

func TestAmazonResponseXMLPressureTable(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com")
	envelope := amazonResponseXML(testIdentityToken)
	for _, tc := range []struct {
		name      string
		body      string
		wantToken string
		wantErr   error
	}{
		{name: "published namespaced envelope is accepted", body: envelope, wantToken: testIdentityToken},
		{name: "single byte token is accepted", body: amazonResponseXML("a"), wantToken: "a"},
		{name: "exact token bound is accepted", body: amazonResponseXML(strings.Repeat("a", TokenMaximumBytes)), wantToken: strings.Repeat("a", TokenMaximumBytes)},
		{name: "declared prefix namespace is accepted", body: `<sts:GetWebIdentityTokenResponse xmlns:sts="` + amazonResponseNamespace + `"><sts:GetWebIdentityTokenResult><sts:WebIdentityToken>a.b.c</sts:WebIdentityToken></sts:GetWebIdentityTokenResult></sts:GetWebIdentityTokenResponse>`, wantToken: "a.b.c"},
		{name: "comment preserves surrounding character data", body: amazonResponseXML("first.<!--ignored-->second"), wantToken: "first.second"},
		{name: "CDATA preserves exact token character data", body: amazonResponseXML("<![CDATA[a.b.c]]>"), wantToken: "a.b.c"},
		{name: "unnamespaced envelope is rejected", body: amazonResponseXMLInNamespace(testIdentityToken, ""), wantErr: core.ErrCloudIdentityContract},
		{name: "foreign namespace is rejected", body: amazonResponseXMLInNamespace(testIdentityToken, "https://evil.example.com/doc/2011-06-15/"), wantErr: core.ErrCloudIdentityContract},
		{name: "another STS API version namespace is rejected", body: amazonResponseXMLInNamespace(testIdentityToken, "https://sts.amazonaws.com/doc/2099-01-01/"), wantErr: core.ErrCloudIdentityContract},
		{name: "namespace missing its trailing separator is rejected", body: amazonResponseXMLInNamespace(testIdentityToken, strings.TrimSuffix(amazonResponseNamespace, "/")), wantErr: core.ErrCloudIdentityContract},
		{name: "plaintext HTTP namespace is rejected", body: amazonResponseXMLInNamespace(testIdentityToken, "http://sts.amazonaws.com/doc/2011-06-15/"), wantErr: core.ErrCloudIdentityContract},
		{name: "result switching to a foreign namespace is rejected", body: strings.Replace(envelope, "<GetWebIdentityTokenResult>", `<GetWebIdentityTokenResult xmlns="https://evil.example.com/">`, 1), wantErr: core.ErrCloudIdentityContract},
		{name: "token switching to a foreign namespace is rejected", body: strings.Replace(envelope, "<WebIdentityToken>", `<WebIdentityToken xmlns="https://evil.example.com/">`, 1), wantErr: core.ErrCloudIdentityContract},
		{name: "token clearing the response namespace is rejected", body: strings.Replace(envelope, "<WebIdentityToken>", `<WebIdentityToken xmlns="">`, 1), wantErr: core.ErrCloudIdentityContract},
		{name: "nested markup inside the token is rejected", body: amazonResponseXML("first.<Ignored/>second"), wantErr: core.ErrCloudIdentityContract},
		{name: "duplicated token element is rejected", body: strings.Replace(envelope, "<WebIdentityToken>"+testIdentityToken+"</WebIdentityToken>", "<WebIdentityToken>first.b.c</WebIdentityToken><WebIdentityToken>second.b.c</WebIdentityToken>", 1), wantErr: core.ErrCloudIdentityContract},
		{name: "duplicated result element is rejected", body: strings.Replace(envelope, "</GetWebIdentityTokenResponse>", "<GetWebIdentityTokenResult><WebIdentityToken>second.b.c</WebIdentityToken></GetWebIdentityTokenResult></GetWebIdentityTokenResponse>", 1), wantErr: core.ErrCloudIdentityContract},
		{name: "missing result is rejected", body: `<GetWebIdentityTokenResponse xmlns="` + amazonResponseNamespace + `"/>`, wantErr: core.ErrCloudIdentityContract},
		{name: "missing token is rejected", body: `<GetWebIdentityTokenResponse xmlns="` + amazonResponseNamespace + `"><GetWebIdentityTokenResult/></GetWebIdentityTokenResponse>`, wantErr: core.ErrCloudIdentityContract},
		{name: "empty token is rejected", body: amazonResponseXML(""), wantErr: core.ErrCloudIdentityContract},
		{name: "whitespace token is rejected", body: amazonResponseXML(" "), wantErr: core.ErrCloudIdentityContract},
		{name: "malformed XML is rejected", body: "<GetWebIdentityTokenResponse>", wantErr: core.ErrCloudIdentityContract},
		{name: "truncated result is rejected", body: "<GetWebIdentityTokenResponse><GetWebIdentityTokenResult>", wantErr: core.ErrCloudIdentityContract},
		{name: "wrong root is rejected", body: `<GetCallerIdentityResponse xmlns="` + amazonResponseNamespace + `"/>`, wantErr: core.ErrCloudIdentityContract},
		{name: "token outside result is rejected", body: `<GetWebIdentityTokenResponse xmlns="` + amazonResponseNamespace + `"><WebIdentityToken>` + testIdentityToken + `</WebIdentityToken></GetWebIdentityTokenResponse>`, wantErr: core.ErrCloudIdentityContract},
		{name: "STS error envelope is rejected", body: `<ErrorResponse xmlns="` + amazonResponseNamespace + `"><Error><Code>AccessDenied</Code></Error></ErrorResponse>`, wantErr: core.ErrCloudIdentityContract},
		{name: "over-bound token is rejected", body: amazonResponseXML(strings.Repeat("a", TokenMaximumBytes+1)), wantErr: core.ErrCloudIdentityContract},
		{name: "empty body is rejected", wantErr: core.ErrCloudIdentityContract},
		{name: "bare token without an envelope is rejected", body: testIdentityToken, wantErr: core.ErrCloudIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := amazonTLSClient(t, countingHandler{
				requests: new(atomic.Uint64),
				status:   http.StatusOK,
				body:     tc.body,
			})
			got, gotErr := AcquireAmazonWebServices(
				t.Context(),
				client,
				mustAmazonRequest(t, audience),
			)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf(
						"AcquireAmazonWebServices() error = %v, want %v",
						gotErr,
						core.ErrCloudIdentityContract,
					)
				}
				if got != (Token{}) {
					t.Fatalf(
						"AcquireAmazonWebServices() token = %#v, want zero",
						got,
					)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf(
					"AcquireAmazonWebServices() = (%v, %v), want validated token",
					got,
					gotErr,
				)
			}
			gotBearer, gotBearerErr := got.BearerValue()
			wantBearer := bearerPrefix + tc.wantToken
			if gotBearerErr != nil || gotBearer != wantBearer {
				t.Fatalf(
					"Token.BearerValue() = (%q, %v), want (%q, nil)",
					gotBearer,
					gotBearerErr,
					wantBearer,
				)
			}
		})
	}
}

func FuzzAmazonResponseToken(f *testing.F) {
	f.Add(amazonResponseXML(testIdentityToken))
	f.Add("")
	f.Add(amazonResponseXMLInNamespace(
		testIdentityToken,
		"https://evil.example.com/",
	))
	f.Add(amazonResponseXML("first.<Ignored/>second"))

	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > AmazonResponseMaximumBytes {
			return
		}
		got, gotErr := amazonResponseToken([]byte(body))
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"amazonResponseToken() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			if got != (Token{}) {
				t.Fatalf(
					"amazonResponseToken() rejected token = %#v, want zero",
					got,
				)
			}
			proveRedactedError(t, gotErr)
			return
		}
		if got.Provider() != ProviderAmazonWebServices ||
			got.Validate() != nil {
			t.Fatalf(
				"amazonResponseToken() accepted token = (%v, %v), want AWS and valid",
				got.Provider(),
				got.Validate(),
			)
		}
		bearer, bearerErr := got.BearerValue()
		value, found := strings.CutPrefix(bearer, bearerPrefix)
		if bearerErr != nil || !found {
			t.Fatalf(
				"Token.BearerValue() = (%q, %v), want a bearer value",
				bearer,
				bearerErr,
			)
		}
		roundTrip, roundTripErr := amazonResponseToken(
			[]byte(amazonResponseXML(value)),
		)
		if roundTripErr != nil {
			t.Fatalf(
				"canonical amazonResponseToken() error = %v, want nil",
				roundTripErr,
			)
		}
		roundTripBearer, roundTripBearerErr := roundTrip.BearerValue()
		if roundTripBearerErr != nil || roundTripBearer != bearer {
			t.Fatalf(
				"canonical Token.BearerValue() = (%q, %v), want (%q, nil)",
				roundTripBearer,
				roundTripBearerErr,
				bearer,
			)
		}
	})
}

func TestAcquisitionContextRefusalsStopBeforeNetwork(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		context func(testing.TB) context.Context
		name    string
	}{
		{
			name: "cancelled context is refused",
			context: func(tb testing.TB) context.Context {
				ctx, cancel := context.WithCancel(tb.Context())
				cancel()
				return ctx
			},
		},
		{
			name: "absent context is refused",
			context: func(testing.TB) context.Context {
				return nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Uint64
			client := googlePlaintextClient(t, countingHandler{
				requests: &requests,
				status:   http.StatusOK,
				body:     testIdentityToken,
			})
			got, gotErr := AcquireGoogleCloud(
				tc.context(t),
				client,
				Request{
					Audience: mustAudience(t, "https://api.example.com"),
					Policy:   mustPolicy(t),
				},
			)
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"AcquireGoogleCloud() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			if got != (Token{}) || requests.Load() != 0 {
				t.Fatalf(
					"AcquireGoogleCloud() = (%#v, %d requests), want (zero, 0)",
					got,
					requests.Load(),
				)
			}
		})
	}
}

func TestAcquisitionPreservesNativeConnectionFailure(t *testing.T) {
	t.Parallel()

	client := googlePlaintextClient(t, http.HandlerFunc(closeConnection(t)))
	_, gotErr := AcquireGoogleCloud(
		t.Context(),
		client,
		Request{
			Audience: mustAudience(t, "https://api.example.com"),
			Policy:   mustPolicy(t),
		},
	)
	if !errors.Is(gotErr, core.ErrCloudIdentityContract) ||
		!errors.Is(gotErr, core.ErrExchangeTransport) ||
		!errors.Is(gotErr, io.EOF) {
		t.Fatalf(
			"AcquireGoogleCloud() error = %v, want cloud identity, exchange transport, and EOF",
			gotErr,
		)
	}
}

// TestAmazonFailureModesAlwaysRedactTheCapability is the structural proof of the
// AWS redaction guarantee. Redaction cannot be a property of one wrapped call
// site, because a future step that forgets the wrapper discloses the signed URL.
// The table walks every reachable AWS failure mode and requires each to format
// to exactly the fixed text under every verb.
func TestAmazonFailureModesAlwaysRedactTheCapability(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com/release")
	const secret = "session-secret-must-never-appear"
	signature := strings.Repeat("b", 64)
	for _, tc := range []struct {
		handler func(*testing.T) http.Handler
		name    string
	}{
		{
			name: "transport failure redacts",
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(closeConnection(t))
			},
		},
		{
			name: "refused status redacts",
			handler: func(*testing.T) http.Handler {
				return fixedHandler(http.StatusForbidden, "denied")
			},
		},
		{
			name: "malformed XML redacts",
			handler: func(*testing.T) http.Handler {
				return fixedHandler(
					http.StatusOK,
					"<GetWebIdentityTokenResponse>",
				)
			},
		},
		{
			name: "foreign namespace redacts",
			handler: func(*testing.T) http.Handler {
				return fixedHandler(http.StatusOK, amazonResponseXMLInNamespace(
					testIdentityToken,
					"https://evil.example.com/",
				))
			},
		},
		{
			name: "unusable token redacts",
			handler: func(*testing.T) http.Handler {
				return fixedHandler(
					http.StatusOK,
					amazonResponseXML("not a token"),
				)
			},
		},
		{
			name: "oversized response redacts",
			handler: func(*testing.T) http.Handler {
				return fixedHandler(http.StatusOK, strings.Repeat(
					"a",
					AmazonResponseMaximumBytes+1,
				))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := amazonTLSClient(t, tc.handler(t))
			signedURL := setQuery(amazonSignatureQuery, signature)(
				setQuery(amazonSecurityTokenQuery, secret)(
					amazonSignedURL(audience, amazonTestHost),
				),
			)
			request, err := NewAmazonWebServicesRequest(
				AmazonWebServicesRequestInput{
					SignedURL: signedURL,
					Request: Request{
						Audience: audience,
						Policy:   mustPolicy(t),
					},
				},
			)
			if err != nil {
				t.Fatalf(
					"NewAmazonWebServicesRequest() setup error = %v, want nil",
					err,
				)
			}
			_, gotErr := AcquireAmazonWebServices(
				t.Context(),
				client,
				request,
			)
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf(
					"AcquireAmazonWebServices() error = %v, want %v",
					gotErr,
					core.ErrCloudIdentityContract,
				)
			}
			proveRedactedError(t, gotErr)
		})
	}
}

func fixedHandler(status int, body string) http.Handler {
	return countingHandler{
		requests: new(atomic.Uint64),
		status:   status,
		body:     body,
	}
}

func proveRedactedError(t *testing.T, err error) {
	t.Helper()

	for _, format := range []string{
		"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%c", "%f",
	} {
		if got := fmt.Sprintf(format, err); got != amazonRequestFailureText {
			t.Fatalf(
				"fmt.Sprintf(%q, error) = %q, want %q",
				format,
				got,
				amazonRequestFailureText,
			)
		}
	}
	if got := err.Error(); got != amazonRequestFailureText {
		t.Fatalf("error.Error() = %q, want %q", got, amazonRequestFailureText)
	}
}

// TestAmazonTransportFailureKeepsTheNativeErrorReachable proves redaction did
// not cost inspectability: the caller still reaches the standard library's own
// error through errors.As even though no verb prints it.
func TestAmazonTransportFailureKeepsTheNativeErrorReachable(t *testing.T) {
	t.Parallel()

	audience := mustAudience(t, "https://api.example.com/release")
	client := amazonTLSClient(t, http.HandlerFunc(closeConnection(t)))
	_, gotErr := AcquireAmazonWebServices(
		t.Context(),
		client,
		mustAmazonRequest(t, audience),
	)
	if !errors.Is(gotErr, core.ErrExchangeTransport) ||
		!errors.Is(gotErr, io.EOF) {
		t.Fatalf(
			"AcquireAmazonWebServices() error = %v, want exchange transport and EOF",
			gotErr,
		)
	}
	var native *url.Error
	if !errors.As(gotErr, &native) {
		t.Fatalf(
			"AcquireAmazonWebServices() error type = %T, want reachable *url.Error",
			gotErr,
		)
	}
}

// TestConcurrentAcquisitionsShareNoMutableState drives both entry points from
// many goroutines so the once-resolved protocol contracts and the token path are
// proved to hold no per-call mutable state. Under -race this is the package's
// concurrency proof.
func TestConcurrentAcquisitionsShareNoMutableState(t *testing.T) {
	t.Parallel()

	const workers = 16
	const rounds = 8
	audience := mustAudience(t, "https://api.example.com/release")
	for _, provider := range providerAcquisitions() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()

			client := provider.client(t, fixedHandler(
				http.StatusOK,
				provider.success(testIdentityToken),
			))
			var group sync.WaitGroup
			for range workers {
				group.Go(func() {
					for range rounds {
						token, err := provider.acquire(
							t.Context(),
							client,
							audience,
						)
						if err != nil {
							t.Errorf(
								"concurrent acquisition error = %v, want nil",
								err,
							)
							return
						}
						if token.Provider() != provider.provider {
							t.Errorf(
								"concurrent token provider = %v, want %v",
								token.Provider(),
								provider.provider,
							)
							return
						}
					}
				})
			}
			group.Wait()
		})
	}
}

func closeConnection(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()

	return func(writer http.ResponseWriter, _ *http.Request) {
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			t.Errorf("ResponseWriter implements http.Hijacker = false, want true")
			return
		}
		connection, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v, want nil", err)
			return
		}
		_ = connection.Close()
	}
}
