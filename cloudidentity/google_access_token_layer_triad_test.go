package cloudidentity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type accessTokenCaseClass uint8

const (
	accessTokenCaseUnknown accessTokenCaseClass = iota
	accessTokenCaseAccepted
	accessTokenCaseRejected
	accessTokenCaseBoundary
)

type googleAccessTokenCase struct {
	context   func(testing.TB) context.Context
	request   func(testing.TB) GoogleCloudAccessTokenRequest
	wantErr   error
	name      string
	body      string
	wantToken string
	status    int
	class     accessTokenCaseClass
	wantCalls uint64
}

func TestLayerTriadGoogleCloudAccessTokenRealHTTPHostileTable(t *testing.T) {
	t.Parallel()

	cases := googleAccessTokenHostileCases(t)
	gotAccepted, gotRejected, gotBoundary := countAccessTokenCases(cases)
	if gotAccepted != 10 || gotRejected != 10 || gotBoundary != 20 {
		t.Fatalf("hostile case classes = (%d accepted, %d rejected, %d boundary), want (10, 10, 20)", gotAccepted, gotRejected, gotBoundary)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Uint64
			client := googlePlaintextClient(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				if request.Method != http.MethodGet || request.URL.Path != googleMetadataAccessTokenPath || request.URL.RawQuery != "" {
					t.Errorf("metadata request = %s %s?%s, want GET %s with no query", request.Method, request.URL.Path, request.URL.RawQuery, googleMetadataAccessTokenPath)
				}
				if got := request.Header.Get(googleMetadataHeaderName); got != googleMetadataHeaderValue {
					t.Errorf("metadata header = %q, want %q", got, googleMetadataHeaderValue)
				}
				responseWithBody(writer, tc.status, tc.body)
			}))
			got, gotErr := AcquireGoogleCloudAccessToken(tc.context(t), client, tc.request(t))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("AcquireGoogleCloudAccessToken() = (%v, %v), want validated access token", got, gotErr)
				}
				gotBearer, bearerErr := got.BearerValue()
				wantBearer := bearerPrefix + tc.wantToken
				if bearerErr != nil || gotBearer != wantBearer {
					t.Fatalf("AccessToken.BearerValue() = (%q, %v), want (%q, nil)", gotBearer, bearerErr, wantBearer)
				}
				if got.Provider() != ProviderGoogleCloud || got.Lifetime().IsZero() {
					t.Fatalf("AccessToken facts = (%v, %v), want Google Cloud and positive lifetime", got.Provider(), got.Lifetime())
				}
				if formatted := fmt.Sprintf("%v", got); formatted != core.RedactedValueText {
					t.Fatalf("formatted AccessToken = %q, want %q", formatted, core.RedactedValueText)
				}
			} else {
				if !errors.Is(gotErr, core.ErrCloudIdentityContract) || !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("AcquireGoogleCloudAccessToken() error = %v, want %v and %v", gotErr, core.ErrCloudIdentityContract, tc.wantErr)
				}
				if got != (AccessToken{}) {
					t.Fatalf("AcquireGoogleCloudAccessToken() token = %#v, want zero", got)
				}
				if errors.Is(tc.wantErr, core.ErrExchangeResponse) {
					var statusErr exchange.StatusError
					if !errors.As(gotErr, &statusErr) {
						t.Fatalf("AcquireGoogleCloudAccessToken() error = %v, want exchange.StatusError", gotErr)
					}
				}
			}
			if gotCalls := calls.Load(); gotCalls != tc.wantCalls {
				t.Fatalf("metadata request count = %d, want %d", gotCalls, tc.wantCalls)
			}
		})
	}
}

func googleAccessTokenHostileCases(tb testing.TB) []googleAccessTokenCase {
	tb.Helper()
	valid := func(token string, seconds uint64) string {
		body, err := json.Marshal(googleAccessTokenResponse{AccessToken: token, ExpiresIn: googleAccessTokenLifetimeSeconds(seconds), TokenType: googleAccessTokenTypeBearer})
		if err != nil {
			tb.Fatalf("json.Marshal(googleAccessTokenResponse) error = %v, want nil", err)
		}
		return string(body)
	}
	accepted := []googleAccessTokenCase{
		accessTokenCase("single byte token", accessTokenCaseAccepted, valid("a", 1), "a"),
		accessTokenCase("ordinary token", accessTokenCaseAccepted, valid("abc.DEF-123_~+/==", 3600), "abc.DEF-123_~+/=="),
		accessTokenCase("one second lifetime", accessTokenCaseAccepted, valid("second", 1), "second"),
		accessTokenCase("one minute lifetime", accessTokenCaseAccepted, valid("minute", 60), "minute"),
		accessTokenCase("one hour lifetime", accessTokenCaseAccepted, valid("hour", 3600), "hour"),
		accessTokenCase("maximum temporal seconds", accessTokenCaseAccepted, valid("maximum", GoogleCloudAccessTokenLifetimeMaximumSeconds), "maximum"),
		accessTokenCase("plus and slash token", accessTokenCaseAccepted, valid("a+b/c", 300), "a+b/c"),
		accessTokenCase("tilde token", accessTokenCaseAccepted, valid("a~b", 300), "a~b"),
		accessTokenCase("underscore token", accessTokenCaseAccepted, valid("a_b", 300), "a_b"),
		accessTokenCase("padding token", accessTokenCaseAccepted, valid("abc==", 300), "abc=="),
	}
	rejected := []googleAccessTokenCase{
		accessTokenRefusal("empty document", accessTokenCaseRejected, "", core.ErrJSONContract),
		accessTokenRefusal("empty token", accessTokenCaseRejected, valid("", 300), core.ErrCloudIdentityContract),
		accessTokenRefusal("space bearing token", accessTokenCaseRejected, valid("a b", 300), core.ErrCloudIdentityContract),
		accessTokenRefusal("leading padding token", accessTokenCaseRejected, valid("=abc", 300), core.ErrCloudIdentityContract),
		accessTokenRefusal("zero lifetime", accessTokenCaseRejected, valid("abc", 0), core.ErrCloudIdentityContract),
		accessTokenRefusal("wrong token type", accessTokenCaseRejected, strings.Replace(valid("abc", 300), googleAccessTokenTypeBearer, "MAC", 1), core.ErrCloudIdentityContract),
		accessTokenRefusal("unknown member", accessTokenCaseRejected, `{"access_token":"abc","expires_in":300,"token_type":"Bearer","scope":"all"}`, core.ErrJSONContract),
		accessTokenRefusal("duplicate token member", accessTokenCaseRejected, `{"access_token":"abc","access_token":"def","expires_in":300,"token_type":"Bearer"}`, core.ErrJSONContract),
		accessTokenRefusal("truncated document", accessTokenCaseRejected, `{"access_token":"abc"`, core.ErrJSONContract),
		accessTokenRefusal("trailing document", accessTokenCaseRejected, valid("abc", 300)+` {}`, core.ErrJSONContract),
	}
	maximumToken := strings.Repeat("a", TokenMaximumBytes)
	maximumDocument := valid(maximumToken, GoogleCloudAccessTokenLifetimeMaximumSeconds)
	if gotSyntaxBytes := len(maximumDocument) - len(maximumToken); gotSyntaxBytes != googleAccessTokenResponseSyntaxMaximumBytes {
		tb.Fatalf("maximum typed response framing = %d bytes, want %d", gotSyntaxBytes, googleAccessTokenResponseSyntaxMaximumBytes)
	}
	boundary := []googleAccessTokenCase{
		accessTokenCase("token exact minimum", accessTokenCaseBoundary, valid("a", 300), "a"),
		accessTokenCase("token one above minimum", accessTokenCaseBoundary, valid("ab", 300), "ab"),
		accessTokenCase("token one below maximum", accessTokenCaseBoundary, valid(strings.Repeat("a", TokenMaximumBytes-1), 300), strings.Repeat("a", TokenMaximumBytes-1)),
		accessTokenCase("token exact maximum", accessTokenCaseBoundary, valid(maximumToken, 300), maximumToken),
		accessTokenRefusal("token one above maximum", accessTokenCaseBoundary, valid(strings.Repeat("a", TokenMaximumBytes+1), 300), core.ErrCloudIdentityContract),
		accessTokenCase("lifetime exact minimum", accessTokenCaseBoundary, valid("abc", 1), "abc"),
		accessTokenCase("lifetime one above minimum", accessTokenCaseBoundary, valid("abc", 2), "abc"),
		accessTokenCase("lifetime one below maximum", accessTokenCaseBoundary, valid("abc", GoogleCloudAccessTokenLifetimeMaximumSeconds-1), "abc"),
		accessTokenCase("lifetime exact maximum", accessTokenCaseBoundary, valid("abc", GoogleCloudAccessTokenLifetimeMaximumSeconds), "abc"),
		accessTokenRefusal("lifetime one above maximum", accessTokenCaseBoundary, valid("abc", GoogleCloudAccessTokenLifetimeMaximumSeconds+1), core.ErrCloudIdentityContract),
		accessTokenCase("response one below bound", accessTokenCaseBoundary, padJSONDocument(maximumDocument, GoogleCloudAccessTokenResponseMaximumBytes-1), maximumToken),
		accessTokenCase("response exact bound", accessTokenCaseBoundary, padJSONDocument(maximumDocument, GoogleCloudAccessTokenResponseMaximumBytes), maximumToken),
		accessTokenRefusal("response one above bound", accessTokenCaseBoundary, padJSONDocument(maximumDocument, GoogleCloudAccessTokenResponseMaximumBytes+1), core.ErrExchangeBodyLimit),
		accessTokenStatus("provider unauthorized", http.StatusUnauthorized),
		accessTokenStatus("provider forbidden", http.StatusForbidden),
		accessTokenStatus("provider throttled", http.StatusTooManyRequests),
		accessTokenStatus("provider unavailable", http.StatusServiceUnavailable),
		accessTokenNoEffect("cancelled context is silent", func(tb testing.TB) context.Context {
			ctx, cancel := context.WithCancel(tb.Context())
			cancel()
			return ctx
		}, valid("abc", 300), context.Canceled),
		accessTokenNoEffect("nil context is silent", func(testing.TB) context.Context { return nil }, valid("abc", 300), core.ErrNilContext),
		accessTokenInvalidRequest("zero policy is silent", valid("abc", 300)),
	}
	return append(append(accepted, rejected...), boundary...)
}

func accessTokenCase(name string, class accessTokenCaseClass, body, token string) googleAccessTokenCase {
	return googleAccessTokenCase{name: name, class: class, body: body, wantToken: token, status: http.StatusOK, wantCalls: 1, context: func(tb testing.TB) context.Context { return tb.Context() }, request: validGoogleAccessTokenRequest}
}

func accessTokenRefusal(name string, class accessTokenCaseClass, body string, wantErr error) googleAccessTokenCase {
	tc := accessTokenCase(name, class, body, "")
	tc.wantErr = wantErr
	return tc
}

func accessTokenStatus(name string, status int) googleAccessTokenCase {
	tc := accessTokenRefusal(name, accessTokenCaseBoundary, "provider refusal", core.ErrExchangeResponse)
	tc.status = status
	return tc
}

func accessTokenNoEffect(name string, context func(testing.TB) context.Context, body string, wantErr error) googleAccessTokenCase {
	tc := accessTokenRefusal(name, accessTokenCaseBoundary, body, wantErr)
	tc.context = context
	tc.wantCalls = 0
	return tc
}

func accessTokenInvalidRequest(name, body string) googleAccessTokenCase {
	tc := accessTokenRefusal(name, accessTokenCaseBoundary, body, core.ErrCloudIdentityContract)
	tc.request = func(testing.TB) GoogleCloudAccessTokenRequest { return GoogleCloudAccessTokenRequest{} }
	tc.wantCalls = 0
	return tc
}

func validGoogleAccessTokenRequest(tb testing.TB) GoogleCloudAccessTokenRequest {
	tb.Helper()
	return GoogleCloudAccessTokenRequest{Policy: mustPolicy(tb)}
}

func countAccessTokenCases(cases []googleAccessTokenCase) (int, int, int) {
	var accepted, rejected, boundary int
	for _, tc := range cases {
		switch tc.class {
		case accessTokenCaseAccepted:
			accepted++
		case accessTokenCaseRejected:
			rejected++
		case accessTokenCaseBoundary:
			boundary++
		}
	}
	return accepted, rejected, boundary
}

func padJSONDocument(document string, extent int) string {
	if len(document) >= extent {
		return document
	}
	return document + strings.Repeat(" ", extent-len(document))
}
