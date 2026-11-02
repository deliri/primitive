package instagram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const testToken = "EAAtesttokenvalue0123456789abcdefghijklmnop"

// metaFake answers in place of graph.facebook.com and records what it saw.
type metaFake struct {
	status int
	body   string
	calls  atomic.Int32
	seen   atomic.Pointer[http.Request]
}

func (m *metaFake) RoundTrip(request *http.Request) (*http.Response, error) {
	m.calls.Add(1)
	m.seen.Store(request)
	return &http.Response{StatusCode: m.status, Header: http.Header{"Content-Type": {"application/json; charset=UTF-8"}}, Body: io.NopCloser(strings.NewReader(m.body)), Request: request}, nil
}

func fakeClient(t *testing.T, fake *metaFake) Client {
	t.Helper()
	network, err := exchange.NewClient(&http.Client{Transport: fake})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := ParseCredential([]byte(testToken))
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(network, credential)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestListMediaLayerTriad(t *testing.T) {
	t.Parallel()
	request := func(t *testing.T) MediaListRequest {
		return MediaListRequest{User: testUser(t), Policy: testPolicy(t)}
	}
	t.Run("positive: one GET to the exact URL, the token only as a bearer header", func(t *testing.T) {
		t.Parallel()
		fake := &metaFake{status: http.StatusOK, body: `{"data":[{"id":"18123456789012345","media_type":"VIDEO","thumbnail_url":"` + reelImage + `","permalink":"` + reelLink + `","caption":"Hello","timestamp":"2026-10-27T15:04:05+0000"}],"paging":{"cursors":{"before":"a","after":"x"},"next":"https://graph.facebook.com/v21.0/17841432921003907/media?after=x"}}`}
		page, err := fakeClient(t, fake).ListMedia(context.Background(), request(t))
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Media) != 1 || page.Media[0].Caption != "Hello" || !page.Media[0].HasImage {
			t.Fatalf("page %+v", page)
		}
		seen := fake.seen.Load()
		want, _ := request(t).Endpoint()
		wantURL := want.HTTPURL()
		if seen.Method != http.MethodGet || seen.URL.String() != wantURL.String() {
			t.Fatalf("sent %s %s", seen.Method, seen.URL)
		}
		if seen.Header.Get("Authorization") != "Bearer "+testToken || strings.Contains(seen.URL.String(), testToken) {
			t.Fatal("the token must travel only in the bearer header")
		}
	})
	t.Run("negative: a provider error is a typed response failure with no page", func(t *testing.T) {
		t.Parallel()
		fake := &metaFake{status: http.StatusBadRequest, body: `{"error":{"message":"Unsupported get request","type":"GraphMethodException","code":100}}`}
		page, err := fakeClient(t, fake).ListMedia(context.Background(), request(t))
		if !errors.Is(err, core.ErrInstagramResponse) || !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
			t.Fatalf("got %v %+v", err, page)
		}
	})
	t.Run("neutral: an invalid request never reaches the network", func(t *testing.T) {
		t.Parallel()
		fake := &metaFake{status: http.StatusOK, body: `{"data":[]}`}
		bad := request(t)
		bad.Policy = exchange.OperationPolicy{}
		if _, err := fakeClient(t, fake).ListMedia(context.Background(), bad); !errors.Is(err, core.ErrInstagramContract) {
			t.Fatalf("got %v", err)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := fakeClient(t, fake).ListMedia(cancelled, request(t)); err == nil {
			t.Fatal("a cancelled context was admitted")
		}
		if fake.calls.Load() != 0 {
			t.Fatalf("refused requests reached Meta %d times", fake.calls.Load())
		}
	})
}

// Meta documents code 190 as "Access token has expired" and does not document
// the HTTP status of errors, so only a documented body with code 190 is
// authentication. An unreadable or undocumented body never is.
func TestListMediaFailureIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"expired session, code 190 subcode 463", http.StatusBadRequest, `{"error":{"message":"Error validating access token: Session has expired","type":"OAuthException","code":190,"error_subcode":463,"fbtrace_id":"A1"}}`, core.ErrInstagramAuthentication},
		{"invalid token, code 190 without subcode", http.StatusBadRequest, `{"error":{"message":"Invalid OAuth access token","type":"OAuthException","code":190}}`, core.ErrInstagramAuthentication},
		{"code 190 under a 403 is still authentication", http.StatusForbidden, `{"error":{"code":190,"error_subcode":467}}`, core.ErrInstagramAuthentication},
		{"every documented member present", http.StatusBadRequest, `{"error":{"message":"m","type":"OAuthException","code":190,"error_subcode":460,"error_user_title":"t","error_user_msg":"u","fbtrace_id":"f"}}`, core.ErrInstagramAuthentication},
		{"401 alone is not authentication", http.StatusUnauthorized, ``, core.ErrInstagramResponse},
		{"bad parameter, code 100", http.StatusBadRequest, `{"error":{"message":"Unsupported get request","type":"GraphMethodException","code":100}}`, core.ErrInstagramResponse},
		{"throttled, code 4", http.StatusBadRequest, `{"error":{"message":"Application request limit reached","code":4}}`, core.ErrInstagramResponse},
		{"code 190 as a string", http.StatusBadRequest, `{"error":{"code":"190"}}`, core.ErrInstagramResponse},
		{"code 190 beside an undocumented member", http.StatusBadRequest, `{"error":{"code":190,"is_transient":false}}`, core.ErrInstagramResponse},
		{"code 191, one past the token code", http.StatusBadRequest, `{"error":{"code":191}}`, core.ErrInstagramResponse},
		{"code 189, one before the token code", http.StatusBadRequest, `{"error":{"code":189}}`, core.ErrInstagramResponse},
		{"an HTML gateway page", http.StatusBadGateway, `<html>bad gateway</html>`, core.ErrInstagramResponse},
		{"an outage with no body", http.StatusInternalServerError, ``, core.ErrInstagramResponse},
		{"a 200 with an unreadable body", http.StatusOK, `{"data":`, core.ErrInstagramResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &metaFake{status: tc.status, body: tc.body}
			page, err := fakeClient(t, fake).ListMedia(context.Background(), MediaListRequest{User: testUser(t), Policy: testPolicy(t)})
			if !errors.Is(err, tc.want) || !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
				t.Fatalf("got %v with %d media, want %v", err, len(page.Media), tc.want)
			}
			other := core.ErrInstagramResponse
			if tc.want == core.ErrInstagramResponse {
				other = core.ErrInstagramAuthentication
			}
			if errors.Is(err, other) {
				t.Fatalf("the failure carries both identities: %v", err)
			}
			if fake.calls.Load() != 1 {
				t.Fatalf("a single-attempt policy made %d calls", fake.calls.Load())
			}
		})
	}
}
