package awsidentity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const awsTestHost = "sts.us-east-2.amazonaws.com"

type awsRewriteTransport struct {
	transport http.RoundTripper
	base      url.URL
}

func (r awsRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Scheme, copy.URL.Host, copy.Host = r.base.Scheme, r.base.Host, ""
	return r.transport.RoundTrip(copy)
}

func mustAWSPolicy(tb testing.TB) Policy {
	tb.Helper()
	got, err := DefaultPolicy()
	if err != nil {
		tb.Fatalf("DefaultPolicy() error = %v, want nil", err)
	}
	return got
}

func mustAWSAudience(tb testing.TB) Audience {
	tb.Helper()
	got, err := ParseAudience("https://api.example.com/release")
	if err != nil {
		tb.Fatalf("ParseAudience() error = %v, want nil", err)
	}
	return got
}

func awsSignedURL(audience Audience, host, region string) string {
	query := url.Values{
		amazonActionQuery: {amazonActionValue}, amazonVersionQuery: {amazonVersionValue},
		amazonAudienceQuery: {audience.String()}, amazonSigningAlgorithmQuery: {amazonSigningAlgorithmValue},
		amazonDurationQuery: {amazonDurationValue}, amazonSigAlgorithmQuery: {amazonSigAlgorithmValue},
		amazonCredentialQuery: {"AKIATEST/20260729/" + region + "/sts/aws4_request"},
		amazonDateQuery:       {"20260729T120000Z"}, amazonExpiresQuery: {"60"},
		amazonSignedHeadersQuery: {amazonSignedHeadersValue}, amazonSignatureQuery: {strings.Repeat("a", 64)},
	}
	return (&url.URL{Scheme: core.SchemeHTTPS, Host: host, Path: "/", RawQuery: query.Encode()}).String()
}

func mutateAWSURL(t *testing.T, raw, kind, value string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v, want nil", raw, err)
	}
	switch kind {
	case "scheme":
		parsed.Scheme = value
	case "host":
		parsed.Host = value
	case "path":
		parsed.Path = value
	case "delete":
		query := parsed.Query()
		query.Del(value)
		parsed.RawQuery = query.Encode()
	case "append":
		query := parsed.Query()
		query.Add(value, "duplicate")
		parsed.RawQuery = query.Encode()
	default:
		query := parsed.Query()
		query.Set(kind, value)
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func TestAWSRequestInputHostileBoundaryTable(t *testing.T) {
	t.Parallel()
	audience := mustAWSAudience(t)
	base := awsSignedURL(audience, awsTestHost, "us-east-2")
	type testCase struct {
		wantErr error
		name    string
		kind    string
		value   string
	}
	cases := []testCase{
		{name: "commercial regional endpoint is admitted"},
		{name: "commercial dual stack endpoint is admitted", kind: "host", value: "sts.us-east-2.api.aws"},
		{name: "commercial FIPS endpoint is admitted", kind: "host", value: "sts-fips.us-east-2.amazonaws.com"},
		{name: "commercial FIPS dual stack endpoint is admitted", kind: "host", value: "sts-fips.us-east-2.api.aws"},
		{name: "one second expiry is admitted", kind: amazonExpiresQuery, value: "1"},
		{name: "exact maximum expiry is admitted", kind: amazonExpiresQuery, value: "300"},
		{name: "session credential token is admitted", kind: amazonSecurityTokenQuery, value: "session-token"},
		{name: "lowercase signature is admitted", kind: amazonSignatureQuery, value: strings.Repeat("0", 64)},
		{name: "maximum hexadecimal signature is admitted", kind: amazonSignatureQuery, value: strings.Repeat("f", 64)},
		{name: "root path omission is admitted", kind: "path", value: ""},
		{name: "global endpoint is refused", kind: "host", value: "sts.amazonaws.com", wantErr: core.ErrAWSIdentityContract},
		{name: "plaintext endpoint is refused", kind: "scheme", value: "http", wantErr: core.ErrAWSIdentityContract},
		{name: "non root path is refused", kind: "path", value: "/identity", wantErr: core.ErrAWSIdentityContract},
		{name: "custom port is refused", kind: "host", value: "sts.us-east-2.amazonaws.com:8443", wantErr: core.ErrAWSIdentityContract},
		{name: "foreign host is refused", kind: "host", value: "identity.example.com", wantErr: core.ErrAWSIdentityContract},
		{name: "empty region is refused", kind: "host", value: "sts..amazonaws.com", wantErr: core.ErrAWSIdentityContract},
		{name: "dotted region is refused", kind: "host", value: "sts.us.east-2.amazonaws.com", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong action is refused", kind: amazonActionQuery, value: "GetCallerIdentity", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong version is refused", kind: amazonVersionQuery, value: "2026-01-01", wantErr: core.ErrAWSIdentityContract},
		{name: "contradictory audience is refused", kind: amazonAudienceQuery, value: "other", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong signing algorithm is refused", kind: amazonSigningAlgorithmQuery, value: "ES384", wantErr: core.ErrAWSIdentityContract},
		{name: "wrong duration is refused", kind: amazonDurationQuery, value: "3600", wantErr: core.ErrAWSIdentityContract},
		{name: "zero expiry is refused", kind: amazonExpiresQuery, value: "0", wantErr: core.ErrAWSIdentityContract},
		{name: "noncanonical expiry is refused", kind: amazonExpiresQuery, value: "060", wantErr: core.ErrAWSIdentityContract},
		{name: "expiry above maximum is refused", kind: amazonExpiresQuery, value: "301", wantErr: core.ErrAWSIdentityContract},
		{name: "short signature is refused", kind: amazonSignatureQuery, value: strings.Repeat("a", 63), wantErr: core.ErrAWSIdentityContract},
		{name: "long signature is refused", kind: amazonSignatureQuery, value: strings.Repeat("a", 65), wantErr: core.ErrAWSIdentityContract},
		{name: "uppercase noncanonical signature is refused", kind: amazonSignatureQuery, value: strings.Repeat("A", 64), wantErr: core.ErrAWSIdentityContract},
		{name: "nonhex signature is refused", kind: amazonSignatureQuery, value: strings.Repeat("z", 64), wantErr: core.ErrAWSIdentityContract},
		{name: "unknown query field is refused", kind: "FutureParameter", value: "value", wantErr: core.ErrAWSIdentityContract},
		{name: "missing action is refused", kind: "delete", value: amazonActionQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "missing version is refused", kind: "delete", value: amazonVersionQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "missing audience is refused", kind: "delete", value: amazonAudienceQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "missing credential is refused", kind: "delete", value: amazonCredentialQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "missing signed date is refused", kind: "delete", value: amazonDateQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "missing signature is refused", kind: "delete", value: amazonSignatureQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate action is refused", kind: "append", value: amazonActionQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate audience is refused", kind: "append", value: amazonAudienceQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate expiry is refused", kind: "append", value: amazonExpiresQuery, wantErr: core.ErrAWSIdentityContract},
		{name: "duplicate signature is refused", kind: "append", value: amazonSignatureQuery, wantErr: core.ErrAWSIdentityContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := base
			if tc.kind != "" {
				raw = mutateAWSURL(t, raw, tc.kind, tc.value)
			}
			input := RequestInput{SignedURL: raw, Audience: audience, Policy: mustAWSPolicy(t)}
			got, gotErr := NewRequest(input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (Request{}) {
					t.Fatalf("NewRequest() = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || input.Validate() != nil || got.Validate() != nil {
				t.Fatalf("NewRequest() = (%v, %v), want validated request", got, gotErr)
			}
		})
	}
}

func TestLayerTriadAWSAcquireRealTLS(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		body      string
		status    int
		wantCalls uint64
		cancel    bool
	}{
		{name: "provider receipt produces redacted bearer", body: awsResponseXML("a.b.c"), status: http.StatusOK, wantCalls: 1},
		{name: "malformed provider receipt is refused", body: "<truncated", status: http.StatusOK, wantErr: core.ErrAWSIdentityContract, wantCalls: 1},
		{name: "cancelled intent performs no effect", body: awsResponseXML("a.b.c"), status: http.StatusOK, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			base, _ := url.Parse(server.URL)
			httpClient := &http.Client{Transport: awsRewriteTransport{base: *base, transport: server.Client().Transport}}
			exchangeClient, err := exchange.NewClient(httpClient)
			if err != nil {
				t.Fatalf("exchange.NewClient() error = %v, want nil", err)
			}
			client, err := NewClient(exchangeClient)
			if err != nil {
				t.Fatalf("NewClient() error = %v, want nil", err)
			}
			audience := mustAWSAudience(t)
			request, err := NewRequest(RequestInput{SignedURL: awsSignedURL(audience, awsTestHost, "us-east-2"), Audience: audience, Policy: mustAWSPolicy(t)})
			if err != nil {
				t.Fatalf("NewRequest() error = %v, want nil", err)
			}
			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, gotErr := Acquire(ctx, client, request)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (Token{}) {
					t.Fatalf("Acquire() = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
			} else {
				bearer, bearerErr := got.BearerValue()
				if gotErr != nil || bearerErr != nil || bearer != "Bearer a.b.c" || fmt.Sprint(got) != core.RedactedValueText {
					t.Fatalf("Acquire() bearer = (%q, %v, %v), want redacted valid bearer", bearer, gotErr, bearerErr)
				}
			}
			if calls.Load() != tc.wantCalls {
				t.Fatalf("provider calls = %d, want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

func awsResponseXML(value string) string {
	return `<GetWebIdentityTokenResponse xmlns="` + amazonResponseNamespace + `"><GetWebIdentityTokenResult><WebIdentityToken>` + value + `</WebIdentityToken><Expiration>2026-07-29T12:05:00Z</Expiration></GetWebIdentityTokenResult><ResponseMetadata><RequestId>request-id</RequestId></ResponseMetadata></GetWebIdentityTokenResponse>`
}

type awsExternalDoorInventory struct {
	Acquire       func(context.Context, Client, Request) (Token, error)
	NewRequest    func(RequestInput) (Request, error)
	ParseAudience func(string) (Audience, error)
}

var awsExternalDoors = awsExternalDoorInventory{Acquire: Acquire, NewRequest: NewRequest, ParseAudience: ParseAudience}

func TestAWSExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	got, err := scanAWSExternalDoors(".")
	if err != nil {
		t.Fatalf("scanAWSExternalDoors() error = %v, want nil", err)
	}
	typeOf := reflect.TypeOf(awsExternalDoors)
	want := make([]string, 0, typeOf.NumField())
	for field := range typeOf.Fields() {
		want = append(want, field.Name)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("AWS external doors = %q, want %q", got, want)
	}
}

func scanAWSExternalDoors(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var doors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(set, filepath.Join(root, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok || declaration.Recv != nil {
				return true
			}
			name := declaration.Name.Name
			if name == "Acquire" || name == "NewRequest" || name == "ParseAudience" {
				doors = append(doors, name)
			}
			return false
		})
	}
	slices.Sort(doors)
	return doors, nil
}

func FuzzAWSProviderResponseSemanticClosure(f *testing.F) {
	for _, seed := range [][]byte{[]byte(awsResponseXML("a.b.c")), nil, {}, []byte("<truncated"), []byte(awsResponseXML("")), bytes.Repeat([]byte{'a'}, AmazonResponseMaximumBytes+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := amazonResponseToken(data)
		if err != nil {
			if !errors.Is(err, core.ErrAWSIdentityContract) || got != (Token{}) {
				t.Fatalf("amazonResponseToken(rejected) = (%v, %v), want zero and %v", got, err, core.ErrAWSIdentityContract)
			}
			return
		}
		bearer, bearerErr := got.BearerValue()
		if got.Validate() != nil || bearerErr != nil || !strings.HasPrefix(bearer, "Bearer ") {
			t.Fatalf("amazonResponseToken(accepted) = (%v, %v), want validated bearer", got, bearerErr)
		}
	})
}
