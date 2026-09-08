package awsidentity

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const awsTestExpiration = "2026-07-29T12:05:00Z"
const awsTestRequestID = "provider-receipt-id"

func awsProviderDocument(value string) amazonResponse {
	return amazonResponse{XMLName: awsProviderName[amazonResponse](),
		Results: []amazonResult{{XMLName: awsProviderName[amazonResult](),
			Tokens:      []amazonTokenElement{{XMLName: awsProviderName[amazonTokenElement](), Value: value}},
			Expirations: []amazonExpirationElement{{XMLName: awsProviderName[amazonExpirationElement](), Value: awsTestExpiration}},
		}},
		Metadata: []amazonResponseMetadata{{XMLName: awsProviderName[amazonResponseMetadata](), RequestIDs: []amazonRequestIDElement{{XMLName: awsProviderName[amazonRequestIDElement](), Value: awsTestRequestID}}}},
	}
}

// Read the real XMLName tag instead of duplicating provider element spellings.
func awsProviderName[T any]() xml.Name {
	field := reflect.TypeFor[T]().Field(0)
	return xml.Name{Space: amazonResponseNamespace, Local: field.Tag.Get("xml")}
}

// AWS supplies this document; the production structs are decode-only. Go's
// marshal tags override XMLName.Space, so use Encoder's explicit start elements
// to emit the actual typed namespace mutations. No handwritten valid XML.
func awsProviderBytes(tb testing.TB, document amazonResponse) []byte {
	tb.Helper()
	var buffer bytes.Buffer
	encoder := xml.NewEncoder(&buffer)
	write := func(token xml.Token) {
		if err := encoder.EncodeToken(token); err != nil {
			tb.Fatalf("XML fixture encoding error = %v, want nil", err)
		}
	}
	start := func(name xml.Name) { write(xml.StartElement{Name: name}) }
	end := func(name xml.Name) { write(xml.EndElement{Name: name}) }
	unexpected := func(elements []amazonUnexpectedElement) {
		for _, element := range elements {
			start(element.XMLName)
			end(element.XMLName)
		}
	}
	leaf := func(name xml.Name, value string, elements []amazonUnexpectedElement) {
		start(name)
		write(xml.CharData(value))
		unexpected(elements)
		end(name)
	}
	start(document.XMLName)
	for _, result := range document.Results {
		start(result.XMLName)
		for _, token := range result.Tokens {
			leaf(token.XMLName, token.Value, token.Unexpected)
		}
		for _, expiration := range result.Expirations {
			leaf(expiration.XMLName, expiration.Value, expiration.Unexpected)
		}
		unexpected(result.Unexpected)
		end(result.XMLName)
	}
	for _, metadata := range document.Metadata {
		start(metadata.XMLName)
		for _, id := range metadata.RequestIDs {
			leaf(id.XMLName, id.Value, id.Unexpected)
		}
		unexpected(metadata.Unexpected)
		end(metadata.XMLName)
	}
	unexpected(document.Unexpected)
	end(document.XMLName)
	if err := encoder.Close(); err != nil {
		tb.Fatalf("XML fixture Close error = %v, want nil", err)
	}
	return buffer.Bytes()
}

// This transport injects provider bytes at net/http.RoundTripper's documented
// seam. It proves public Acquire projection and body ownership; the separate
// TLS test owns wire framing. No live AWS or SigV4 authentication is claimed.
type awsResponseTransport struct {
	body     io.ReadCloser
	status   int
	length   int64
	calls    int
	observed *http.Request
	cause    error
}

func (r *awsResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.calls++
	r.observed = request.Clone(request.Context())
	if r.cause != nil {
		return nil, r.cause
	}
	return &http.Response{StatusCode: r.status, Body: r.body, ContentLength: r.length, Request: request, Header: make(http.Header)}, nil
}

type awsObservedBody struct {
	reader   io.Reader
	bytes    int
	closes   int
	readErr  error
	closeErr error
	cancel   context.CancelFunc
}

func (b *awsObservedBody) Read(p []byte) (int, error) {
	if b.cancel != nil {
		b.cancel()
		return 0, context.Canceled
	}
	n, err := b.reader.Read(p)
	b.bytes += n
	if b.readErr != nil {
		return n, b.readErr
	}
	return n, err
}
func (b *awsObservedBody) Close() error { b.closes++; return b.closeErr }

func awsClient(tb testing.TB, transport http.RoundTripper) Client {
	tb.Helper()
	substrate, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		tb.Fatalf("exchange.NewClient fixture error = %v, want nil", err)
	}
	client, err := NewClient(substrate)
	if err != nil {
		tb.Fatalf("NewClient fixture error = %v, want nil", err)
	}
	return client
}
func awsRequest(tb testing.TB) Request {
	tb.Helper()
	audience := mustAWSAudience(tb)
	request, err := NewRequest(RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, awsTestRegion), Audience: audience, Policy: mustAWSPolicy(tb)})
	if err != nil {
		tb.Fatalf("NewRequest fixture error = %v, want nil", err)
	}
	return request
}

type awsProviderCase struct {
	name      string
	change    func(*amazonResponse)
	wantToken string
	wantErr   error
}

func awsProviderCases() []awsProviderCase {
	unknown := amazonUnexpectedElement{XMLName: xml.Name{Space: amazonResponseNamespace, Local: "FutureElement"}}
	return []awsProviderCase{
		{name: "ordinary opaque token", wantToken: awsTestBearer},
		{name: "minimum token", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "a" }, wantToken: "a"},
		{name: "punctuation token", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "-._~+/" }, wantToken: "-._~+/"},
		{name: "padded token", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "a==" }, wantToken: "a=="},
		{name: "one below token ceiling", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = strings.Repeat("a", TokenMaximumBytes-1) }, wantToken: strings.Repeat("a", TokenMaximumBytes-1)},
		{name: "exact token ceiling", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = strings.Repeat("a", TokenMaximumBytes) }, wantToken: strings.Repeat("a", TokenMaximumBytes)},
		{name: "UTC numeric offset", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "2026-07-29T12:05:00+00:00" }, wantToken: awsTestBearer},
		{name: "nanosecond expiration", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "2026-07-29T12:05:00.123456789Z" }, wantToken: awsTestBearer},
		{name: "past expiration stays provider fact", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "1970-01-01T00:00:00Z" }, wantToken: awsTestBearer},
		{name: "future expiration is not product policy", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "2262-04-11T23:47:16Z" }, wantToken: awsTestBearer},
		{name: "one byte receipt ID", change: func(d *amazonResponse) { d.Metadata[0].RequestIDs[0].Value = "x" }, wantToken: awsTestBearer},
		{name: "escaped receipt ID", change: func(d *amazonResponse) { d.Metadata[0].RequestIDs[0].Value = "a<&b" }, wantToken: awsTestBearer},
		{name: "absent result", change: func(d *amazonResponse) { d.Results = nil }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate identical result", change: func(d *amazonResponse) { d.Results = append(d.Results, d.Results[0]) }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate conflicting result", change: func(d *amazonResponse) {
			other := awsProviderDocument("foreign")
			d.Results = append(d.Results, other.Results[0])
		}, wantErr: core.ErrAWSIdentityContract},
		{name: "absent metadata", change: func(d *amazonResponse) { d.Metadata = nil }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate metadata", change: func(d *amazonResponse) { d.Metadata = append(d.Metadata, d.Metadata[0]) }, wantErr: core.ErrAWSIdentityContract},
		{name: "absent token", change: func(d *amazonResponse) { d.Results[0].Tokens = nil }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate identical token", change: func(d *amazonResponse) { r := &d.Results[0]; r.Tokens = append(r.Tokens, r.Tokens[0]) }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate conflicting token", change: func(d *amazonResponse) {
			r := &d.Results[0]
			other := r.Tokens[0]
			other.Value = "foreign"
			r.Tokens = append(r.Tokens, other)
		}, wantErr: core.ErrAWSIdentityContract},
		{name: "absent expiration", change: func(d *amazonResponse) { d.Results[0].Expirations = nil }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate expiration", change: func(d *amazonResponse) { r := &d.Results[0]; r.Expirations = append(r.Expirations, r.Expirations[0]) }, wantErr: core.ErrAWSIdentityContract},
		{name: "absent receipt ID", change: func(d *amazonResponse) { d.Metadata[0].RequestIDs = nil }, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate receipt ID", change: func(d *amazonResponse) { m := &d.Metadata[0]; m.RequestIDs = append(m.RequestIDs, m.RequestIDs[0]) }, wantErr: core.ErrAWSIdentityContract},
		{name: "empty receipt ID", change: func(d *amazonResponse) { d.Metadata[0].RequestIDs[0].Value = "" }, wantErr: core.ErrAWSIdentityContract},
		{name: "empty token", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "" }, wantErr: core.ErrAWSIdentityContract},
		{name: "token one above ceiling", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = strings.Repeat("a", TokenMaximumBytes+1) }, wantErr: core.ErrAWSIdentityContract},
		{name: "token contains XML escaped non alphabet", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "a&b" }, wantErr: core.ErrAWSIdentityContract},
		{name: "token contains header injection", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "a\r\nb" }, wantErr: core.ErrAWSIdentityContract},
		{name: "token data after padding", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Value = "a=b" }, wantErr: core.ErrAWSIdentityContract},
		{name: "empty expiration", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "" }, wantErr: core.ErrAWSIdentityContract},
		{name: "zero instant", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "0001-01-01T00:00:00Z" }, wantErr: core.ErrAWSIdentityContract},
		{name: "year one is outside temporal extent", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "0001-01-01T00:00:00.000000001Z" }, wantErr: core.ErrAWSIdentityContract},
		{name: "positive non UTC offset", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "2026-07-29T12:05:00+00:01" }, wantErr: core.ErrAWSIdentityContract},
		{name: "negative non UTC offset", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "2026-07-29T12:05:00-00:01" }, wantErr: core.ErrAWSIdentityContract},
		{name: "impossible calendar date", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "2026-02-30T12:05:00Z" }, wantErr: core.ErrAWSIdentityContract},
		{name: "expiration wrong representation", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Value = "1785326700" }, wantErr: core.ErrAWSIdentityContract},
		{name: "root foreign namespace", change: func(d *amazonResponse) { d.XMLName.Space = "urn:foreign" }, wantErr: core.ErrAWSIdentityContract},
		{name: "result foreign namespace", change: func(d *amazonResponse) { d.Results[0].XMLName.Space = "urn:foreign" }, wantErr: core.ErrAWSIdentityContract},
		{name: "token foreign namespace", change: func(d *amazonResponse) { d.Results[0].Tokens[0].XMLName.Space = "urn:foreign" }, wantErr: core.ErrAWSIdentityContract},
		{name: "expiration foreign namespace", change: func(d *amazonResponse) { d.Results[0].Expirations[0].XMLName.Space = "urn:foreign" }, wantErr: core.ErrAWSIdentityContract},
		{name: "metadata foreign namespace", change: func(d *amazonResponse) { d.Metadata[0].XMLName.Space = "urn:foreign" }, wantErr: core.ErrAWSIdentityContract},
		{name: "receipt ID foreign namespace", change: func(d *amazonResponse) { d.Metadata[0].RequestIDs[0].XMLName.Space = "urn:foreign" }, wantErr: core.ErrAWSIdentityContract},
		{name: "unexpected root child", change: func(d *amazonResponse) { d.Unexpected = []amazonUnexpectedElement{unknown} }, wantErr: core.ErrAWSIdentityContract},
		{name: "unexpected result child", change: func(d *amazonResponse) { d.Results[0].Unexpected = []amazonUnexpectedElement{unknown} }, wantErr: core.ErrAWSIdentityContract},
		{name: "unexpected token child", change: func(d *amazonResponse) { d.Results[0].Tokens[0].Unexpected = []amazonUnexpectedElement{unknown} }, wantErr: core.ErrAWSIdentityContract},
		{name: "unexpected expiration child", change: func(d *amazonResponse) { d.Results[0].Expirations[0].Unexpected = []amazonUnexpectedElement{unknown} }, wantErr: core.ErrAWSIdentityContract},
		{name: "unexpected metadata child", change: func(d *amazonResponse) { d.Metadata[0].Unexpected = []amazonUnexpectedElement{unknown} }, wantErr: core.ErrAWSIdentityContract},
		{name: "unexpected receipt ID child", change: func(d *amazonResponse) { d.Metadata[0].RequestIDs[0].Unexpected = []amazonUnexpectedElement{unknown} }, wantErr: core.ErrAWSIdentityContract},
	}
}

func TestAWSAcquireProviderEnvelopeBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range awsProviderCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			document := awsProviderDocument(awsTestBearer)
			original := awsProviderBytes(t, document)
			if tc.change != nil {
				tc.change(&document)
			}
			data := awsProviderBytes(t, document)
			if tc.change != nil && bytes.Equal(data, original) {
				t.Fatalf("provider mutation=%x, want bytes different from %x", data, original)
			}
			body := &awsObservedBody{reader: bytes.NewReader(data)}
			transport := &awsResponseTransport{body: body, status: http.StatusOK, length: int64(len(data))}
			request := awsRequest(t)
			got, gotErr := Acquire(t.Context(), awsClient(t, transport), request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Acquire error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				var redacted requestError
				if got != (Token{}) || !errors.As(gotErr, &redacted) {
					t.Fatalf("Acquire rejection = (%v,%v), want zero redacting request error", got, gotErr)
				}
			} else {
				disclosed, err := got.BearerValue()
				if err != nil || got.Validate() != nil || disclosed != bearerPrefix+tc.wantToken {
					t.Fatalf("Acquire disclosure = (%q,%v), want exact token", disclosed, err)
				}
			}
			if transport.calls != 1 || body.closes != 1 || body.bytes != len(data) {
				t.Fatalf("effect (calls,closes,bytes) = (%d,%d,%d), want (1,1,%d)", transport.calls, body.closes, body.bytes, len(data))
			}
			observed := transport.observed
			if observed.Method != http.MethodGet || observed.URL.String() != request.endpoint.String() || (observed.Body != nil && observed.Body != http.NoBody) || observed.ContentLength != 0 {
				t.Fatalf("outbound request = %+v, want exact signed GET without body", observed)
			}
		})
	}
}

func TestAWSAcquireBodyOwnershipAndRefusalIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                     string
		readErr, closeErr, transportErr, wantErr error
		cancel                                   bool
		status                                   int
	}{
		{name: "complete receipt", status: http.StatusOK},
		{name: "partial bytes and read error cannot release token", readErr: io.ErrUnexpectedEOF, status: http.StatusOK, wantErr: io.ErrUnexpectedEOF},
		{name: "close failure cannot release token", closeErr: io.ErrClosedPipe, status: http.StatusOK, wantErr: io.ErrClosedPipe},
		{name: "transport cause survives redaction", transportErr: io.ErrUnexpectedEOF, status: http.StatusOK, wantErr: io.ErrUnexpectedEOF},
		{name: "cancel during read preserves context identity", cancel: true, status: http.StatusOK, wantErr: context.Canceled},
		{name: "unauthorized receipt body is not a token", status: http.StatusUnauthorized, wantErr: core.ErrExchangeResponse},
		{name: "retryable service failure is single attempt", status: http.StatusServiceUnavailable, wantErr: core.ErrExchangeResponse},
		{name: "empty success status is not expected OK", status: http.StatusNoContent, wantErr: core.ErrExchangeResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := awsProviderBytes(t, awsProviderDocument(awsTestBearer))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			body := &awsObservedBody{reader: bytes.NewReader(data), readErr: tc.readErr, closeErr: tc.closeErr}
			if tc.cancel {
				body.cancel = cancel
			}
			transport := &awsResponseTransport{body: body, status: tc.status, length: int64(len(data)), cause: tc.transportErr}
			got, gotErr := Acquire(ctx, awsClient(t, transport), awsRequest(t))
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Acquire error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.status != http.StatusOK && tc.transportErr == nil {
				var statusErr exchange.StatusError
				if !errors.As(gotErr, &statusErr) {
					t.Fatalf("Acquire status error = %v, want exchange.StatusError", gotErr)
				}
				status, statusValueErr := statusErr.Status().Int()
				if statusValueErr != nil || status != tc.status || statusErr.Expected() != core.HTTPStatusOK() {
					t.Fatalf("status facts = (%d,%v,%v), want (%d,OK,nil)", status, statusErr.Expected(), statusValueErr, tc.status)
				}
			}
			if tc.wantErr != nil {
				var redacted requestError
				if got != (Token{}) || !errors.Is(gotErr, core.ErrAWSIdentityContract) || !errors.As(gotErr, &redacted) {
					t.Fatalf("Acquire = (%v,%v), want zero and redacted AWS/cause identities", got, gotErr)
				}
				for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
					if text := fmt.Sprintf(format, gotErr); text != requestFailureText {
						t.Fatalf("error format = %q, want %q", text, requestFailureText)
					}
				}
			} else {
				value, err := got.BearerValue()
				if err != nil || value != bearerPrefix+awsTestBearer {
					t.Fatalf("Acquire = (%q,%v), want exact bearer", value, err)
				}
			}
			wantCloses := 1
			if tc.transportErr != nil {
				wantCloses = 0
			}
			if transport.calls != 1 || body.closes != wantCloses {
				t.Fatalf("calls/closes = %d/%d, want 1/%d", transport.calls, body.closes, wantCloses)
			}
		})
	}
}

func TestAWSAcquireEnforcesActualResponseExtent(t *testing.T) {
	t.Parallel()
	canonical := awsProviderBytes(t, awsProviderDocument(awsTestBearer))
	for _, tc := range []struct {
		name     string
		extent   int
		declared int64
		wantErr  error
		wantRead int
	}{
		{"one below ceiling with declaration", AmazonResponseMaximumBytes - 1, AmazonResponseMaximumBytes - 1, nil, AmazonResponseMaximumBytes - 1},
		{"exact ceiling with declaration", AmazonResponseMaximumBytes, AmazonResponseMaximumBytes, nil, AmazonResponseMaximumBytes},
		{"above ceiling refused from declaration", AmazonResponseMaximumBytes + 1, AmazonResponseMaximumBytes + 1, core.ErrExchangeBodyLimit, 0},
		{"one below ceiling with unknown length", AmazonResponseMaximumBytes - 1, -1, nil, AmazonResponseMaximumBytes - 1},
		{"exact ceiling with unknown length", AmazonResponseMaximumBytes, -1, nil, AmazonResponseMaximumBytes},
		{"above ceiling with unknown length", AmazonResponseMaximumBytes + 1, -1, core.ErrExchangeBodyLimit, AmazonResponseMaximumBytes + 1},
		{"understated length cannot widen ceiling", AmazonResponseMaximumBytes + 1, 1, core.ErrExchangeBodyLimit, AmazonResponseMaximumBytes + 1},
		{"large stream stops after limit probe", AmazonResponseMaximumBytes * 16, -1, core.ErrExchangeBodyLimit, AmazonResponseMaximumBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Padding is legal XML whitespace after the typed document. The repeated
			// reader provides the large extent without materializing that stream.
			padding := int64(tc.extent - len(canonical))
			body := &awsObservedBody{reader: io.MultiReader(bytes.NewReader(canonical), io.LimitReader(awsSpaceReader{}, padding))}
			transport := &awsResponseTransport{body: body, status: http.StatusOK, length: tc.declared}
			got, gotErr := Acquire(t.Context(), awsClient(t, transport), awsRequest(t))
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Acquire error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Token{}) || !errors.Is(gotErr, core.ErrAWSIdentityContract) {
					t.Fatalf("oversize result = (%v,%v), want zero AWS refusal", got, gotErr)
				}
			} else {
				value, err := got.BearerValue()
				if err != nil || value != bearerPrefix+awsTestBearer {
					t.Fatalf("bounded result = (%q,%v), want exact bearer", value, err)
				}
			}
			if body.bytes != tc.wantRead || body.closes != 1 || transport.calls != 1 {
				t.Fatalf("effect bytes/closes/calls = %d/%d/%d, want %d/1/1", body.bytes, body.closes, transport.calls, tc.wantRead)
			}
		})
	}
}

type awsSpaceReader struct{}

func (awsSpaceReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}
