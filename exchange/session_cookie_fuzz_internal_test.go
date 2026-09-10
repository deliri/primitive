package exchange

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type sessionCookieObservation struct {
	Name, Value string
	Quoted      bool
}

func sessionCookieObservations(cookies []*http.Cookie) []sessionCookieObservation {
	facts := make([]sessionCookieObservation, len(cookies))
	for index, cookie := range cookies {
		facts[index] = sessionCookieObservation{Name: cookie.Name, Value: cookie.Value, Quoted: cookie.Quoted}
	}
	return facts
}

func FuzzSessionClientGoCookieCustody(f *testing.F) {
	cookie := http.Cookie{Name: sessionCookieName, Value: sessionCookieValue, Path: sessionCookiePath}
	if err := cookie.Valid(); err != nil {
		f.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	call, err := NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		f.Fatal(err)
	}
	if err := SetCookie(call, cookie); err != nil {
		f.Fatal(err)
	}
	// The owning Go writer supplies both field identity and canonical value.
	// No duplicated cookie wire name or invented serialization is needed.
	var field, canonical string
	for name, values := range recorder.Header() {
		if field != "" || len(values) != 1 {
			f.Fatalf("cookie seed field/count=%q/%d, want one field with one value", name, len(values))
		}
		field, canonical = name, values[0]
	}
	if field == "" || canonical == "" {
		f.Fatalf("cookie seed field/value=%q/%q, want nonempty", field, canonical)
	}
	for host := range uint8(5) {
		f.Add(canonical, "", host, sessionCookiePath, false)
	}
	for _, path := range []string{sessionCookiePath, sessionCookiePath + "/child", sessionCookiePath + "-foreign", "/"} {
		f.Add(canonical, "", uint8(0), path, true)
	}
	for _, replacement := range []http.Cookie{
		{Name: sessionCookieName, Value: sessionOtherCookieValue, Path: sessionCookiePath},
		{Name: sessionCookieName, Path: sessionCookiePath, MaxAge: -1},
		{Name: sessionCookieName, Value: sessionCookieValue, Path: sessionCookiePath, Secure: true},
		{Name: sessionCookieName, Value: sessionCookieValue, Path: sessionCookiePath, Domain: sessionFixtureSuffix},
		{Name: sessionCookieName, Value: sessionCookieValue, Path: sessionCookiePath, Domain: sessionFixtureParent},
		{Name: sessionCookieName, Value: sessionCookieValue, Path: sessionCookiePath, MaxAge: 1},
	} {
		if err := replacement.Valid(); err != nil {
			f.Fatal(err)
		}
		f.Add(canonical, replacement.String(), uint8(0), sessionCookiePath, false)
	}
	f.Add("malformed\r\nfield", "", uint8(0), sessionCookiePath, false)
	f.Add("", "", uint8(0), sessionCookiePath, false)
	f.Fuzz(func(t *testing.T, first, second string, host uint8, path string, secure bool) {
		if len(first) > 8192 || len(second) > 8192 || len(path) > 1024 || host >= 5 {
			return
		}
		// Go's test clock freezes expiration across both independent jars. No
		// real-time sleep, global clock mutation, or cookie runtime is introduced.
		synctest.Test(t, func(t *testing.T) {
			suffixes := sessionFixtureSuffixes{sessionFixtureSuffix}
			client, err := NewSessionClient(SessionClientRequest{PublicSuffixList: suffixes})
			if err != nil || client.Validate() != nil {
				t.Fatalf("session constructor = (%v,%v), want admitted Go client", client, err)
			}
			independent, err := NewSessionClient(SessionClientRequest{PublicSuffixList: suffixes})
			if err != nil {
				t.Fatal(err)
			}
			oracle, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: suffixes})
			if err != nil {
				t.Fatal(err)
			}
			source := &url.URL{Scheme: "https", Host: sessionFixtureHost, Path: sessionCookiePath + sessionEstablishPath}
			hosts := []string{sessionFixtureHost, sessionFixtureChild, sessionFixtureSibling, sessionFixtureParent, sessionFixtureForeign}
			scheme := "http"
			if secure {
				scheme = "https"
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			target := &url.URL{Scheme: scheme, Host: hosts[host], Path: path}
			responseHeader := make(http.Header)
			if first != "" {
				responseHeader.Add(field, first)
			}
			if second != "" {
				responseHeader.Add(field, second)
			}
			provider := &http.Response{Header: responseHeader}
			oracle.SetCookies(source, provider.Cookies())
			// The jar returns stored facts. Go's request writer may quote a value
			// containing spaces/commas, so compare the same wire observation on
			// both sides rather than confusing storage metadata with wire syntax.
			oracleRequest := &http.Request{Header: make(http.Header)}
			for _, cookie := range oracle.Cookies(target) {
				oracleRequest.AddCookie(cookie)
			}
			want := sessionCookieObservations(oracleRequest.Cookies())
			var observed [][]sessionCookieObservation
			transport := opaqueRoundTripper(func(request *http.Request) (*http.Response, error) {
				observed = append(observed, sessionCookieObservations(request.Cookies()))
				header := make(http.Header)
				if len(observed) == 1 {
					header = responseHeader.Clone()
				}
				return &http.Response{StatusCode: http.StatusOK, Header: header, Body: http.NoBody, ContentLength: 0, Request: request}, nil
			})
			// Only the network seam changes. The constructor's client and jar are
			// retained and exercised through the public Exchange operation.
			client.http.Transport, independent.http.Transport = transport, transport
			timeout, err := temporal.DurationFromMilliseconds(30000)
			if err != nil {
				t.Fatal(err)
			}
			steps := []struct {
				client Client
				target *url.URL
				want   []sessionCookieObservation
			}{
				{client: client, target: source},
				{client: client, target: target, want: want},
				{client: independent, target: target},
			}
			for index, step := range steps {
				endpoint, err := core.ParseHTTPEndpoint(step.target.String())
				if err != nil {
					t.Fatalf("Go-generated fixture endpoint refused: %v", err)
				}
				got, err := SendNoBodyBounded(NoBodyBoundedCall{Context: t.Context(), Client: step.client, Request: NoBodyBoundedRequest{Target: endpoint, Semantics: RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()}, Policy: NoBodyBoundedPolicy{Operation: OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Retry: RetryPolicy{MaximumAttempts: 1}, Redirect: RedirectPolicy{Mode: RedirectReject}}}})
				if err != nil || got.Validate() != nil || got.Metadata.Status != core.HTTPStatusOK() || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != 0 || len(got.Body) != 0 {
					t.Fatalf("session step %d response=(%+v,%v), want empty one-attempt HTTP success", index, got, err)
				}
				if len(observed) != index+1 || !slices.Equal(observed[index], step.want) {
					t.Fatalf("session step %d cookies=%+v, want exact independent Go jar facts %+v", index, observed, step.want)
				}
			}
			if client.http == independent.http || client.http.Jar == independent.http.Jar {
				t.Fatalf("client/jar shared=%t/%t, want false/false", client.http == independent.http, client.http.Jar == independent.http.Jar)
			}
		})
	})
}
