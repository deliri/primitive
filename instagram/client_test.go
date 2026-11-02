package instagram

import (
	"context"
	"errors"
	"fmt"
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
		return MediaListRequest{User: testUser(t), Limit: 12, Policy: testPolicy(t)}
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
		fake := &metaFake{status: http.StatusBadRequest, body: `{"error":{"message":"Invalid OAuth access token","code":190}}`}
		page, err := fakeClient(t, fake).ListMedia(context.Background(), request(t))
		if !errors.Is(err, core.ErrInstagramResponse) || !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
			t.Fatalf("got %v %+v", err, page)
		}
	})
	t.Run("neutral: an invalid request never reaches the network", func(t *testing.T) {
		t.Parallel()
		fake := &metaFake{status: http.StatusOK, body: `{"data":[]}`}
		bad := request(t)
		bad.Limit = 0
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

func TestCredentialCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		raw  string
		want bool
	}{
		{"real-length token", testToken, true},
		{"one byte under the minimum", strings.Repeat("a", core.InstagramCredentialMinimumBytes-1), false},
		{"exactly the minimum", strings.Repeat("a", core.InstagramCredentialMinimumBytes), true},
		{"exactly the custody maximum", strings.Repeat("a", core.InstagramCredentialCustodyMaximumBytes), true},
		{"one byte over the maximum", strings.Repeat("a", core.InstagramCredentialCustodyMaximumBytes+1), false},
		{"empty", "", false},
		{"embedded space", testToken[:20] + " " + testToken[21:], false},
		{"newline", testToken + "\n", false},
		{"non-ASCII", testToken + "é", false},
	} {
		credential, err := ParseCredential([]byte(tc.raw))
		if tc.want != (err == nil) {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err != nil && (!errors.Is(err, core.ErrInstagramContract) || credential.Validate() == nil) {
			t.Fatalf("%s: refusal kept material or lost its cause: %v", tc.name, err)
		}
	}
	credential, err := ParseCredential([]byte(testToken))
	if err != nil {
		t.Fatal(err)
	}
	if text := fmt.Sprintf("%v %+v %s", credential, credential, credential); strings.Contains(text, testToken[:12]) {
		t.Fatal("the credential formatted its value")
	}
	if err := credential.Close(); err != nil || credential.Validate() == nil {
		t.Fatalf("Close left a usable credential: %v", err)
	}
	var missing *Credential
	if !errors.Is(missing.Close(), core.ErrInstagramContract) {
		t.Fatal("closing a nil credential must refuse")
	}
}
