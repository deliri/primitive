package awsidentity

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func BenchmarkAWSAudienceMaximum(b *testing.B) {
	input := strings.Repeat("a", AudienceMaximumBytes)
	var got Audience
	var err error
	b.ReportAllocs()
	for b.Loop() {
		got, err = ParseAudience(input)
		if err != nil {
			b.Fatalf("ParseAudience error = %v, want nil", err)
		}
	}
	if got.String() != input {
		b.Fatalf("audience = %q, want exact source", got.String())
	}
}

func BenchmarkAWSNewRequest(b *testing.B) {
	audience := mustAWSAudience(b)
	input := RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(b)}
	var got Request
	var err error
	b.ReportAllocs()
	for b.Loop() {
		got, err = NewRequest(input)
		if err != nil {
			b.Fatalf("NewRequest error = %v, want nil", err)
		}
	}
	if got.endpoint.String() != input.SignedURL || got.audience != audience || got.policy != input.Policy {
		b.Fatalf("request = %v, want exact source facts", got)
	}
}

func BenchmarkAWSProviderXML(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		bytes int
	}{{"minimum", 1}, {"maximum", TokenMaximumBytes}} {
		b.Run(tc.name, func(b *testing.B) {
			value := strings.Repeat("a", tc.bytes)
			data := awsProviderBytes(b, awsProviderDocument(value))
			var got Token
			var err error
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				got, err = amazonResponseToken(data)
				if err != nil {
					b.Fatalf("provider XML error = %v, want nil", err)
				}
			}
			disclosed, err := got.BearerValue()
			if err != nil || disclosed != bearerPrefix+value {
				b.Fatalf("provider token = (%q,%v), want exact source token", disclosed, err)
			}
		})
	}
}

func BenchmarkAWSBearerDisclosure(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		bytes int
	}{{"minimum", 1}, {"maximum", TokenMaximumBytes}} {
		b.Run(tc.name, func(b *testing.B) {
			input := strings.Repeat("a", tc.bytes)
			token, err := newToken(input)
			if err != nil {
				b.Fatalf("token fixture error = %v, want nil", err)
			}
			var got string
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for b.Loop() {
				got, err = token.BearerValue()
				if err != nil {
					b.Fatalf("BearerValue error = %v, want nil", err)
				}
			}
			if got != bearerPrefix+input {
				b.Fatalf("disclosure = %q, want exact source token", got)
			}
		})
	}
}

// Measures Acquire's owned validation/Exchange/projection path using a fixed
// RoundTripper response. It excludes network and TLS latency. The body reader
// and transport are reset outside measured allocation construction; per-call
// net/http request objects remain part of the measured operation. The response
// is a prebuilt fixture, so no observation-clone allocation is charged to AWS.
func BenchmarkAWSAcquireTransportSeam(b *testing.B) {
	value := strings.Repeat("a", TokenMaximumBytes)
	data := awsProviderBytes(b, awsProviderDocument(value))
	reader := bytes.NewReader(data)
	body := &awsObservedBody{reader: reader}
	transport := &awsBenchmarkTransport{response: http.Response{Body: body, StatusCode: http.StatusOK, ContentLength: int64(len(data)), Header: make(http.Header)}}
	client := awsClient(b, transport)
	request := awsRequest(b)
	var got Token
	var err error
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		reader.Reset(data)
		got, err = Acquire(b.Context(), client, request)
		if err != nil {
			b.Fatalf("Acquire error = %v, want nil", err)
		}
	}
	disclosed, err := got.BearerValue()
	if err != nil || disclosed != bearerPrefix+value || transport.calls != b.N || body.closes != b.N {
		b.Fatalf("Acquire facts = (error %v,calls %d,closes %d), want exact token and %d calls/closes", err, transport.calls, body.closes, b.N)
	}
}

// Prebuilt response avoids charging test observation/cloning allocations to AWS.
type awsBenchmarkTransport struct {
	response http.Response
	calls    int
}

func (r *awsBenchmarkTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.calls++
	r.response.Request = request
	return &r.response, nil
}
