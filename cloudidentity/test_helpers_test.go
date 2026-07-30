package cloudidentity

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

const (
	testIdentityToken = "eyJhbGciOiJSUzI1NiJ9.eyJhdWQiOiJ0ZXN0In0.signature"
	// amazonTestHost is the one regional STS authority the AWS tests sign for.
	amazonTestHost = "sts.us-east-2.amazonaws.com"
)

type rewriteTransport struct {
	transport http.RoundTripper
	base      url.URL
}

func (r rewriteTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Scheme = r.base.Scheme
	copy.URL.Host = r.base.Host
	copy.Host = ""
	return r.transport.RoundTrip(copy)
}

func mustTestClient(
	tb testing.TB,
	target string,
	transport http.RoundTripper,
) Client {
	tb.Helper()
	parsed, err := url.Parse(target)
	if err != nil {
		tb.Fatalf("url.Parse(%q) setup error = %v, want nil", target, err)
	}
	httpClient := &http.Client{
		Transport: rewriteTransport{
			base:      *parsed,
			transport: transport,
		},
	}
	exchangeClient, err := exchange.NewClient(httpClient)
	if err != nil {
		tb.Fatalf("exchange.NewClient() setup error = %v, want nil", err)
	}
	client, err := NewClient(exchangeClient)
	if err != nil {
		tb.Fatalf("cloudidentity.NewClient() setup error = %v, want nil", err)
	}
	return client
}

func mustPolicy(tb testing.TB) Policy {
	tb.Helper()
	policy, err := DefaultPolicy()
	if err != nil {
		tb.Fatalf("DefaultPolicy() setup error = %v, want nil", err)
	}
	return policy
}

func mustAudience(tb testing.TB, value string) Audience {
	tb.Helper()
	audience, err := ParseAudience(value)
	if err != nil {
		tb.Fatalf("ParseAudience(%q) setup error = %v, want nil", value, err)
	}
	return audience
}

func amazonSignedURL(audience Audience, host string) string {
	query := url.Values{
		amazonActionQuery:           []string{amazonActionValue},
		amazonVersionQuery:          []string{amazonVersionValue},
		amazonAudienceQuery:         []string{audience.String()},
		amazonSigningAlgorithmQuery: []string{amazonSigningAlgorithmValue},
		amazonDurationQuery:         []string{amazonDurationValue},
		amazonSigAlgorithmQuery:     []string{amazonSigAlgorithmValue},
		amazonCredentialQuery: []string{
			"AKIATEST/20260729/us-east-2/sts/aws4_request",
		},
		amazonDateQuery:          []string{"20260729T120000Z"},
		amazonExpiresQuery:       []string{"60"},
		amazonSignedHeadersQuery: []string{amazonSignedHeadersValue},
		amazonSignatureQuery: []string{
			strings.Repeat("a", 64),
		},
	}
	if strings.Contains(host, "cn-north-1") {
		query.Set(
			amazonCredentialQuery,
			"AKIATEST/20260729/cn-north-1/sts/aws4_request",
		)
	}
	if strings.Contains(host, "us-gov-west-1") {
		query.Set(
			amazonCredentialQuery,
			"AKIATEST/20260729/us-gov-west-1/sts/aws4_request",
		)
	}
	return (&url.URL{
		Scheme:   httpsScheme,
		Host:     host,
		Path:     "/",
		RawQuery: query.Encode(),
	}).String()
}

// amazonResponseXML is the published AWS STS envelope, including the namespace
// AWS declares for the API version the request pins. A document without it is
// not a response this package may accept.
func amazonResponseXML(token string) string {
	return amazonResponseXMLInNamespace(token, amazonResponseNamespace)
}

func amazonResponseXMLInNamespace(token, namespace string) string {
	root := "<GetWebIdentityTokenResponse"
	if namespace != "" {
		root += ` xmlns="` + namespace + `"`
	}
	return root + ">" +
		"<GetWebIdentityTokenResult>" +
		"<WebIdentityToken>" + token + "</WebIdentityToken>" +
		"<Expiration>2026-07-29T12:05:00Z</Expiration>" +
		"</GetWebIdentityTokenResult>" +
		"<ResponseMetadata><RequestId>request-id</RequestId></ResponseMetadata>" +
		"</GetWebIdentityTokenResponse>"
}

// amazonTLSClient reaches a local server while preserving the HTTPS scheme the
// signed capability declares, so the AWS path is proved over a real TLS
// handshake rather than through a silent downgrade to plaintext.
func amazonTLSClient(tb testing.TB, handler http.Handler) Client {
	tb.Helper()
	server := httptest.NewTLSServer(handler)
	tb.Cleanup(server.Close)
	return mustTestClient(tb, server.URL, server.Client().Transport)
}

// googlePlaintextClient matches the metadata service's published plaintext
// endpoint, which is reachable only inside a Google Cloud instance.
func googlePlaintextClient(tb testing.TB, handler http.Handler) Client {
	tb.Helper()
	server := httptest.NewServer(handler)
	tb.Cleanup(server.Close)
	return mustTestClient(tb, server.URL, server.Client().Transport)
}

func mustAmazonRequest(tb testing.TB, audience Audience) AmazonWebServicesRequest {
	tb.Helper()
	request, err := NewAmazonWebServicesRequest(
		AmazonWebServicesRequestInput{
			SignedURL: amazonSignedURL(audience, amazonTestHost),
			Request: Request{
				Audience: audience,
				Policy:   mustPolicy(tb),
			},
		},
	)
	if err != nil {
		tb.Fatalf(
			"NewAmazonWebServicesRequest() setup error = %v, want nil",
			err,
		)
	}
	return request
}

func responseWithBody(
	writer http.ResponseWriter,
	status int,
	body string,
) {
	writer.WriteHeader(status)
	_, _ = io.WriteString(writer, body)
}
