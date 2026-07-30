package cloudidentity

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkAcquireGoogleCloudLoopback(b *testing.B) {
	b.ReportAllocs()

	for _, tc := range []struct {
		name  string
		bytes int
	}{
		{name: "one_kibibyte", bytes: 1 << 10},
		{name: "sixteen_kibibytes", bytes: TokenMaximumBytes},
	} {
		b.Run(tc.name, func(b *testing.B) {
			token := strings.Repeat("a", tc.bytes)
			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, _ *http.Request) {
					responseWithBody(writer, http.StatusOK, token)
				},
			))
			b.Cleanup(server.Close)
			client := mustTestClient(
				b,
				server.URL,
				server.Client().Transport,
			)
			request := Request{
				Audience: mustAudience(
					b,
					"https://api.example.com",
				),
				Policy: mustPolicy(b),
			}
			b.SetBytes(int64(tc.bytes))
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				token, err := AcquireGoogleCloud(
					b.Context(),
					client,
					request,
				)
				if err != nil {
					b.Fatalf(
						"AcquireGoogleCloud() error = %v, want nil",
						err,
					)
				}
				if token.Provider() != ProviderGoogleCloud {
					b.Fatalf(
						"Token.Provider() = %v, want %v",
						token.Provider(),
						ProviderGoogleCloud,
					)
				}
			}
		})
	}
}

func BenchmarkAcquireAmazonWebServicesLoopback(b *testing.B) {
	b.ReportAllocs()

	for _, tc := range []struct {
		name  string
		bytes int
	}{
		{name: "one_kibibyte", bytes: 1 << 10},
		{name: "sixteen_kibibytes", bytes: TokenMaximumBytes},
	} {
		b.Run(tc.name, func(b *testing.B) {
			token := strings.Repeat("a", tc.bytes)
			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, _ *http.Request) {
					responseWithBody(
						writer,
						http.StatusOK,
						amazonResponseXML(token),
					)
				},
			))
			b.Cleanup(server.Close)
			client := mustTestClient(
				b,
				server.URL,
				server.Client().Transport,
			)
			audience := mustAudience(b, "https://api.example.com")
			request, err := NewAmazonWebServicesRequest(
				AmazonWebServicesRequestInput{
					SignedURL: amazonSignedURL(
						audience,
						"sts.us-east-2.amazonaws.com",
					),
					Request: Request{
						Audience: audience,
						Policy:   mustPolicy(b),
					},
				},
			)
			if err != nil {
				b.Fatalf(
					"NewAmazonWebServicesRequest() setup error = %v, want nil",
					err,
				)
			}
			b.SetBytes(int64(tc.bytes))
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				token, err := AcquireAmazonWebServices(
					b.Context(),
					client,
					request,
				)
				if err != nil {
					b.Fatalf(
						"AcquireAmazonWebServices() error = %v, want nil",
						err,
					)
				}
				if token.Provider() != ProviderAmazonWebServices {
					b.Fatalf(
						"Token.Provider() = %v, want %v",
						token.Provider(),
						ProviderAmazonWebServices,
					)
				}
			}
		})
	}
}
