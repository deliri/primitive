package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	fixtureMailbox           = "owner@example.com"
	fixtureToken             = "open-test-credential"
	fixtureThread  ThreadID  = "thread_a"
	fixtureMessage MessageID = "message_b"
)

type tokenFixture struct {
	Token   string        `json:"access_token"`
	Type    tokenTypeWire `json:"token_type"`
	Expires uint64        `json:"expires_in"`
	Scope   scopeWire     `json:"scope"`
}

func grantedTokenFixture() tokenFixture {
	return tokenFixture{Token: fixtureToken, Type: "Bearer", Expires: 3600, Scope: ReadOnlyScope}
}

type localProvider struct {
	next   http.RoundTripper
	target *url.URL
}

func (p localProvider) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || (r.URL.Host != "gmail.googleapis.com" && r.URL.Host != "oauth2.googleapis.com") {
		return nil, core.ErrGmailBinding
	}
	local := r.Clone(r.Context())
	local.URL.Scheme, local.URL.Host = p.target.Scheme, p.target.Host
	local.Header.Set("X-Fixture-Provider", r.URL.Host)
	return p.next.RoundTrip(local)
}

func fixtureServer(t testing.TB, handler http.HandlerFunc) Server {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	target, err := url.Parse(provider.URL)
	if err != nil {
		t.Fatal(err)
	}
	network, err := exchange.NewClient(&http.Client{Transport: localProvider{next: provider.Client().Transport, target: target}})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(network)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func fixturePolicy(t testing.TB) exchange.OperationPolicy {
	t.Helper()
	timeout, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatal(err)
	}
	return exchange.OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout,
		Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}

func fixtureRefresh(t testing.TB) RefreshRequest {
	t.Helper()
	email, mailErr := core.ParseOAuthEmail(fixtureMailbox)
	clientID, idErr := core.ParseOAuthClientID("fixture+opaque")
	secret, clientErr := core.NewOAuthClientSecret([]byte("fixture+secret=&"))
	refresh, refreshErr := NewRefreshToken([]byte("fixture+refresh=&"))
	if err := errors.Join(mailErr, idErr, clientErr, refreshErr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(secret.Close(), refresh.Close()); err != nil {
			t.Error(err)
		}
	})
	return RefreshRequest{Mailbox: email, ClientID: clientID, ClientSecret: secret, RefreshToken: refresh, Policy: fixturePolicy(t)}
}

func encodeFixture[T any](t testing.TB, value T) []byte {
	t.Helper()
	data, err := core.MarshalCanonicalJSONDocument(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func nativeFixtureWrite(t testing.TB, w http.ResponseWriter, status int, data []byte) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		t.Error(err)
	}
}

func fixtureThreadWire() threadWire {
	return threadWire{ID: fixtureThread, Messages: []messageHeaderWire{
		{ID: fixtureMessage, Thread: fixtureThread, Date: "1795708800000", Labels: []labelWire{labelUnread},
			Payload: metadataWire{Headers: []headerWire{{Name: "fRoM", Value: "Fixture <sender@example.com>"}, {Name: "Subject", Value: "=?UTF-8?Q?Site_=E2=9C=93?="}}}},
		{ID: "draft_id", Thread: fixtureThread, Labels: []labelWire{labelDraft}},
	}}
}

func fixtureMessageWire() messageWire {
	return messageWire{ID: fixtureMessage, Thread: fixtureThread, Payload: mimePartWire{Type: "multipart/mixed", Parts: []mimePartWire{
		{Type: "text/plain", Filename: "private.txt", Body: mimeBodyWire{Data: "invalid attachment bytes"}},
		{Type: "multipart/alternative", Parts: []mimePartWire{
			{Type: "text/html", Body: mimeBodyWire{Data: "invalid html bytes"}},
			{Type: "text/plain", Body: mimeBodyWire{Data: base64URLWire(base64.RawURLEncoding.EncodeToString([]byte("Hello ✓\n")))}},
		}},
	}}}
}

func TestGmailNativeReadThroughServer(t *testing.T) {
	t.Parallel()
	refresh := fixtureRefresh(t)
	token := encodeFixture(t, grantedTokenFixture())
	profile := encodeFixture(t, mailboxWire{Email: fixtureMailbox})
	page := encodeFixture(t, threadPageWire{Threads: []threadReferenceWire{{ID: fixtureThread}}, Next: "next+cursor"})
	thread := encodeFixture(t, fixtureThreadWire())
	message := encodeFixture(t, fixtureMessageWire())
	var calls atomic.Int32
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/token" {
			raw, err := io.ReadAll(r.Body)
			if err != nil || r.Method != http.MethodPost || r.Header.Get("X-Fixture-Provider") != "oauth2.googleapis.com" ||
				r.Header.Get("Authorization") != "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
				string(raw) != "grant_type=refresh_token&client_id=fixture%2Bopaque&client_secret=fixture%2Bsecret%3D%26&refresh_token=fixture%2Brefresh%3D%26" {
				t.Errorf("refresh request method, host, headers or encoded form did not match")
			}
			nativeFixtureWrite(t, w, http.StatusOK, token)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken || r.Method != http.MethodGet ||
			r.Header.Get("X-Fixture-Provider") != "gmail.googleapis.com" || strings.Contains(r.URL.String(), fixtureToken) {
			t.Errorf("Gmail request method, host or credential placement did not match")
		}
		query := r.URL.Query()
		switch r.URL.Path {
		case "/gmail/v1/users/me/profile":
			if query.Get("fields") != "emailAddress" {
				t.Error("profile field mask did not match")
			}
			nativeFixtureWrite(t, w, http.StatusOK, profile)
		case "/gmail/v1/users/me/threads":
			if !slices.Equal(query["labelIds"], []string{"INBOX", "UNREAD"}) || query.Get("pageToken") != "an+input&neq=4" ||
				query.Get("fields") != "threads(id),nextPageToken" || query.Get("maxResults") != "2" {
				t.Errorf("thread list query = %q, want the exact bounded unread lookup", r.URL.RawQuery)
			}
			nativeFixtureWrite(t, w, http.StatusOK, page)
		case "/gmail/v1/users/me/threads/thread_a":
			if query.Get("format") != "metadata" || !slices.Equal(query["metadataHeaders"], []string{"From", "Subject", "To", "Cc", "Bcc"}) ||
				query.Get("fields") != "id,messages(id,threadId,internalDate,labelIds,payload(headers(name,value)))" {
				t.Errorf("thread metadata query = %q", r.URL.RawQuery)
			}
			nativeFixtureWrite(t, w, http.StatusOK, thread)
		case "/gmail/v1/users/me/messages/message_b":
			if query.Get("format") != "full" || !strings.HasPrefix(query.Get("fields"), "id,threadId,labelIds,payload(") ||
				strings.Contains(query.Get("fields"), "attachmentId") {
				t.Errorf("message query = %q", r.URL.RawQuery)
			}
			nativeFixtureWrite(t, w, http.StatusOK, message)
		default:
			t.Errorf("unexpected Gmail route %v", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	access, err := server.Refresh().Execute(t.Context(), refresh)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	lifetime, _ := temporal.DurationFromHours(1)
	if access.Mailbox().String() != fixtureMailbox || access.Lifetime() != lifetime || strings.Contains(fmt.Sprintf("%#v", access), fixtureToken) {
		t.Fatalf("bound access = (%v, %v, %v)", access.Mailbox(), access.Lifetime(), access)
	}
	gotPage, err := server.ListThreads().Execute(t.Context(), ThreadListRequest{Access: access, Policy: refresh.Policy, Filter: ThreadFilterUnread, Limit: 2, After: "an+input&neq=4"})
	if err != nil || !slices.Equal(gotPage.Threads, []ThreadID{fixtureThread}) || gotPage.Next != "next+cursor" {
		t.Fatalf("list result = (%+v,%v)", gotPage, err)
	}
	gotThread, err := server.ReadThread().Execute(t.Context(), ThreadReadRequest{Access: access, Policy: refresh.Policy, Thread: fixtureThread})
	if err != nil || gotThread.ID != fixtureThread || len(gotThread.Messages) != 1 {
		t.Fatalf("thread = (%+v,%v), want one visible message", gotThread, err)
	}
	got := gotThread.Messages[0]
	if got.ID != fixtureMessage || got.Sender.String() != "sender@example.com" || got.Subject != "Site ✓" || !got.Unread ||
		got.At != temporal.InstantFromNanoseconds(1795708800000000000) {
		t.Fatalf("message header = %+v", got)
	}
	gotMessage, err := server.ReadMessage().Execute(t.Context(), MessageReadRequest{Access: access, Policy: refresh.Policy, Thread: fixtureThread, Message: fixtureMessage})
	want := Message{ID: fixtureMessage, Thread: fixtureThread, Kind: BodyPlainText, Text: "Hello ✓\n"}
	if err != nil || gotMessage != want || calls.Load() != 5 {
		t.Fatalf("message, calls, error = (%+v,%d,%v), want (%+v,5,nil)", gotMessage, calls.Load(), err, want)
	}
}

func fixtureAccess(t testing.TB) Access {
	t.Helper()
	email, err := core.ParseOAuthEmail(fixtureMailbox)
	if err != nil {
		t.Fatal(err)
	}
	a, err := admitAccess(encodeFixture(t, grantedTokenFixture()), email, GrantReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	return a
}

func TestGmailNativeProviderBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   func(testing.TB) []byte
		want   error
	}{
		{name: "exact message belongs to another thread", status: 200, body: func(t testing.TB) []byte {
			w := fixtureMessageWire()
			w.Thread = "foreign_thread"
			return encodeFixture(t, w)
		}, want: core.ErrGmailBinding},
		{name: "foreign message belongs to the right thread", status: 200, body: func(t testing.TB) []byte {
			w := fixtureMessageWire()
			w.ID = "foreign_message"
			return encodeFixture(t, w)
		}, want: core.ErrGmailBinding},
		{name: "draft is withheld even if its ID is known", status: 200, body: func(t testing.TB) []byte {
			w := fixtureMessageWire()
			w.Labels = []labelWire{labelDraft}
			return encodeFixture(t, w)
		}, want: core.ErrGmailBinding},
		{name: "null root is not a valid empty message", status: 200, body: func(testing.TB) []byte { return []byte("null") }, want: core.ErrJSONContract},
		{name: "duplicate top-level names fail closed", status: 200, body: func(testing.TB) []byte { return []byte(`{"id":"message_b","id":"other","threadId":"thread_a"}`) }, want: core.ErrJSONContract},
		{name: "malformed response does not disclose body", status: 200, body: func(testing.TB) []byte { return []byte(`{"id": "private-provider-text",`) }, want: core.ErrJSONContract},
		{name: "response byte cap rejects a foreign large member", status: 200, body: func(testing.TB) []byte {
			return []byte(`{"extra":"` + strings.Repeat("a", responseMaximumBytes) + `"}`)
		}, want: core.ErrExchangeBodyLimit},
		{name: "expired credential", status: 401, body: func(testing.TB) []byte { return []byte(`{"error":{"message":"private-provider-text"}}`) }, want: core.ErrGmailConsent},
		{name: "provider throttling remains typed", status: 429, body: func(testing.TB) []byte { return []byte(`{"error":{"message":"private-provider-text"}}`) }, want: core.ErrGmailResponse},
		{name: "redirect is never followed", status: 302, body: func(testing.TB) []byte { return nil }, want: core.ErrGmailResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := tc.body(t)
			var calls atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.status == 302 {
					w.Header().Set("Location", "https://oauth2.googleapis.com/token")
				}
				nativeFixtureWrite(t, w, tc.status, body)
			})
			got, err := server.ReadMessage().Execute(t.Context(), MessageReadRequest{Access: fixtureAccess(t), Policy: fixturePolicy(t), Thread: fixtureThread, Message: fixtureMessage})
			if got != (Message{}) || !errors.Is(err, tc.want) || calls.Load() != 1 || strings.Contains(fmt.Sprint(err), "private-provider-text") {
				t.Fatalf("message, error and calls = (%+v,%v,%d), want zero, %v and one call", got, err, calls.Load(), tc.want)
			}
			if tc.status == 429 {
				var status exchange.StatusError
				matched := errors.As(err, &status)
				code, _ := status.Status().Int()
				if !matched || code != 429 {
					t.Fatalf("throttling = %v, want native 429", err)
				}
			}
		})
	}
}

func TestGmailRefreshRefusesWrongScopeMailboxAndRevokedGrant(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		scope  scopeWire
		email  mailAddressWire
		status int
		calls  int32
		want   error
	}{
		{name: "wrong mailbox cannot borrow valid token", scope: ReadOnlyScope, email: "foreign@example.com", status: 200, calls: 2, want: core.ErrGmailBinding},
		{name: "broader scope is refused before profile", scope: "https://www.googleapis.com/auth/gmail.modify", email: fixtureMailbox, status: 200, calls: 1, want: core.ErrGmailConsent},
		{name: "absent scope is refused before profile", scope: "", email: fixtureMailbox, status: 200, calls: 1, want: core.ErrGmailConsent},
		{name: "revoked grant is a consent refusal", status: 400, calls: 1, want: core.ErrGmailConsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := fixtureRefresh(t)
			wire := grantedTokenFixture()
			wire.Scope = tc.scope
			token := encodeFixture(t, wire)
			if tc.status == 400 {
				token = encodeFixture(t, oauthErrorWire{Code: "invalid_grant"})
			}
			profile := encodeFixture(t, mailboxWire{Email: tc.email})
			var calls atomic.Int32
			server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/token" {
					nativeFixtureWrite(t, w, tc.status, token)
					return
				}
				nativeFixtureWrite(t, w, 200, profile)
			})
			got, err := server.Refresh().Execute(t.Context(), r)
			if got != (Access{}) || !errors.Is(err, tc.want) || calls.Load() != tc.calls {
				t.Fatalf("grant, error, calls = (%v,%v,%d), want zero, %v, %d", got, err, calls.Load(), tc.want, tc.calls)
			}
		})
	}
}

func TestGmailRequestRefusalNeverTouchesProvider(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	policy := fixturePolicy(t)
	access := fixtureAccess(t)
	cases := []struct {
		name   string
		id     ThreadID
		change func(*ThreadReadRequest)
	}{
		{name: "slash in path", id: "x/y"},
		{name: "encoded separator", id: "x%2Fy"},
		{name: "dot path", id: ".."},
		{name: "zero closed enum policy", id: fixtureThread, change: func(r *ThreadReadRequest) { r.Policy.Redirect.Mode = 0 }},
		{name: "invalid credential", id: fixtureThread, change: func(r *ThreadReadRequest) { r.Access = Access{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := ThreadReadRequest{Access: access, Policy: policy, Thread: tc.id}
			if tc.change != nil {
				tc.change(&req)
			}
			got, err := server.ReadThread().Execute(t.Context(), req)
			if got.ID != "" || got.Messages != nil || !errors.Is(err, core.ErrGmailContract) || calls.Load() != 0 {
				t.Fatalf("thread, error and calls = (%+v,%v,%d), want refusal without transport", got, err, calls.Load())
			}
		})
	}
}

func TestGmailCancelledDoesNotPublishProjection(t *testing.T) {
	t.Parallel()
	started, done := make(chan struct{}), make(chan struct{})
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		close(started)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req := ThreadReadRequest{Access: fixtureAccess(t), Policy: fixturePolicy(t), Thread: fixtureThread}
	result := make(chan error, 1)
	go func() {
		thread, err := server.ReadThread().Execute(ctx, req)
		if thread.ID != "" || thread.Messages != nil {
			t.Errorf("cancelled projection = %+v, want zero", thread)
		}
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled thread error = %v", err)
	}
	<-done
}
