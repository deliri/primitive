package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	delegationSigner               = "gmail-reader@fixture-project.iam.gserviceaccount.com"
	delegationRuntimeToken         = "runtime-fixture"
	delegationInitialNS      int64 = 1795708800000000000
	fixtureDelegationTimeout       = 8 * time.Second
)

type scenario struct {
	sign          func(context.Context, delegatedFixtureClaims) (delegatedFixtureClaims, int)
	token         func(int) (int, []byte)
	profileEmail  string
	profileStatus int
	readStatus    func(int) int
}

type delegatedFixtureClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Scope    string `json:"scope"`
	Issued   int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

type delegatedFixtureCounts struct {
	metadata, sign, token, profile, reads int
}

type delegationProvider struct {
	metadata, sign, token, profile, reads atomic.Int32
	clock                                 atomic.Int64
}

func (p *delegationProvider) counts() delegatedFixtureCounts {
	return delegatedFixtureCounts{int(p.metadata.Load()), int(p.sign.Load()), int(p.token.Load()), int(p.profile.Load()), int(p.reads.Load())}
}

func delegatedTestServer(t testing.TB, behavior scenario) (Server, *delegationProvider, DelegatedRequest) {
	t.Helper()
	var p delegationProvider
	p.clock.Store(delegationInitialNS)
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.metadata.Add(1)
		if r.Host != "metadata.google.internal" || r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/token" ||
			r.Method != http.MethodGet || r.Header.Get("Metadata-Flavor") != "Google" || r.Header.Get("Authorization") != "" {
			t.Errorf("metadata target = %s %s%s with flavor %q, want attached-account metadata GET", r.Method, r.Host, r.URL.Path, r.Header.Get("Metadata-Flavor"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Metadata-Flavor", "Google")
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, struct {
			Token   string `json:"access_token"`
			Type    string `json:"token_type"`
			Expires uint64 `json:"expires_in"`
		}{Token: delegationRuntimeToken, Type: "Bearer", Expires: 3600}))
	}))
	t.Cleanup(meta.Close)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "iamcredentials.googleapis.com":
			p.handleSign(t, behavior, w, r)
		case "oauth2.googleapis.com":
			p.handleToken(t, behavior, w, r)
		case "gmail.googleapis.com":
			p.handleGmail(t, behavior, w, r)
		default:
			t.Errorf("fixture received unexpected authority %q", r.Host)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(provider.Close)

	transport := provider.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		var target string
		switch address {
		case "metadata.google.internal:80":
			target = meta.Listener.Addr().String()
		case "iamcredentials.googleapis.com:443", "oauth2.googleapis.com:443", "gmail.googleapis.com:443":
			target = provider.Listener.Addr().String()
		default:
			return nil, core.ErrGmailBinding
		}
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	t.Cleanup(transport.CloseIdleConnections)
	netClient, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(netClient)
	if err != nil {
		t.Fatal(err)
	}
	signer, signErr := googleidentity.ParseServiceAccountEmail(delegationSigner)
	subject, subjectErr := googleidentity.ParseWorkspaceSubject(fixtureMailbox)
	if err := errors.Join(signErr, subjectErr); err != nil {
		t.Fatal(err)
	}
	return server, &p, DelegatedRequest{Signer: signer, Subject: subject, Policy: fixturePolicy(t)}
}

func (p *delegationProvider) handleSign(t testing.TB, behavior scenario, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	p.sign.Add(1)
	if r.URL.Path != "/v1/projects/-/serviceAccounts/"+delegationSigner+":signJwt" || r.Method != http.MethodPost ||
		r.Header.Get("Authorization") != "Bearer "+delegationRuntimeToken || r.Header.Get("Content-Type") != "application/json" {
		t.Errorf("IAM request = %s %s, runtime bearer matches %t, want signer POST with attached identity", r.Method, r.URL.Path, r.Header.Get("Authorization") == "Bearer "+delegationRuntimeToken)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var input struct {
		Payload string `json:"payload"`
	}
	raw, err := io.ReadAll(r.Body)
	var claims delegatedFixtureClaims
	if err != nil || json.Unmarshal(raw, &input) != nil || json.Unmarshal([]byte(input.Payload), &claims) != nil {
		t.Errorf("IAM payload length = %d, read error %v, want one JSON claims string", len(input.Payload), err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	at := p.clock.Load() / 1000000000
	if claims.Issuer != delegationSigner || claims.Subject != fixtureMailbox ||
		claims.Scope != "https://www.googleapis.com/auth/gmail.readonly" ||
		claims.Audience != "https://oauth2.googleapis.com/token" || claims.Issued != at || claims.Expires != at+300 {
		t.Errorf("unexpected nonsecret IAM claims: %+v", claims)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	status := http.StatusOK
	if behavior.sign != nil {
		claims, status = behavior.sign(r.Context(), claims)
	}
	if status != http.StatusOK {
		nativeFixtureWrite(t, w, status, []byte(`{"error":{"status":"PERMISSION_DENIED"}}`))
		return
	}
	header := encodeFixture(t, struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}{Algorithm: "RS256", KeyID: "fixture-key"})
	compact := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(encodeFixture(t, claims)) + ".Zml4dHVyZS1zaWduYXR1cmU"
	nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, struct {
		KeyID string `json:"keyId"`
		JWT   string `json:"signedJwt"`
	}{KeyID: "fixture-key", JWT: compact}))
}

func (p *delegationProvider) handleToken(t testing.TB, behavior scenario, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	count := int(p.token.Add(1))
	if r.Method != http.MethodPost || r.URL.Path != "/token" || r.Header.Get("Authorization") != "" ||
		r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.ParseForm() != nil ||
		r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" ||
		len(r.PostForm) != 2 || !strings.HasSuffix(r.PostForm.Get("assertion"), ".Zml4dHVyZS1zaWduYXR1cmU") {
		t.Errorf("OAuth request = %s %s, form fields %d, want POST with only grant_type and assertion", r.Method, r.URL.Path, len(r.PostForm))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if behavior.token != nil {
		status, body := behavior.token(count)
		nativeFixtureWrite(t, w, status, body)
		return
	}
	token := grantedTokenFixture()
	token.Token = fixtureToken + "-" + strconv.Itoa(count)
	nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, token))
}

func (p *delegationProvider) handleGmail(t testing.TB, behavior scenario, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodGet || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "+fixtureToken+"-") {
		t.Errorf("Gmail request = %s %s, OAuth bearer present %t, want authenticated GET", r.Method, r.URL.Path, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "+fixtureToken+"-"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if r.URL.Path == "/gmail/v1/users/me/profile" {
		p.profile.Add(1)
		if r.URL.Query().Get("fields") != "emailAddress" {
			t.Errorf("profile field mask = %q, want emailAddress", r.URL.Query().Get("fields"))
		}
		if behavior.profileStatus != 0 {
			nativeFixtureWrite(t, w, behavior.profileStatus, []byte(`{"error":{"status":"UNAUTHENTICATED"}}`))
			return
		}
		email := behavior.profileEmail
		if email == "" {
			email = fixtureMailbox
		}
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, mailboxWire{Email: mailAddressWire(email)}))
		return
	}
	count := int(p.reads.Add(1))
	if behavior.readStatus != nil {
		if status := behavior.readStatus(count); status != http.StatusOK {
			nativeFixtureWrite(t, w, status, []byte(`{"error":{"status":"UNAUTHENTICATED"}}`))
			return
		}
	}
	switch r.URL.Path {
	case "/gmail/v1/users/me/threads":
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, threadPageWire{Threads: []threadReferenceWire{{ID: fixtureThread}}}))
	case "/gmail/v1/users/me/threads/thread_a":
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, fixtureThreadWire()))
	case "/gmail/v1/users/me/messages/message_b":
		nativeFixtureWrite(t, w, http.StatusOK, encodeFixture(t, fixtureMessageWire()))
	default:
		t.Errorf("unexpected delegated API path %q", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	}
}

func delegatedFixtureAccess(t testing.TB, server Server, p *delegationProvider, request DelegatedRequest) Access {
	t.Helper()
	access, err := server.Delegate().Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	access.delegated.observe = func() (temporal.Instant, error) {
		return temporal.InstantFromNanoseconds(p.clock.Load()), nil
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	return access
}

func delegatedList(ctx context.Context, s Server, access Access, p exchange.OperationPolicy) (ThreadPage, error) {
	return s.ListThreads().Execute(ctx, ThreadListRequest{Access: access, Policy: p, Filter: ThreadFilterInbox, Limit: 1})
}

func TestDelegatedGmailUsesOneVerifiedBearerForAllReads(t *testing.T) {
	t.Parallel()
	server, provider, request := delegatedTestServer(t, scenario{})
	access := delegatedFixtureAccess(t, server, provider, request)
	if got := provider.counts(); got != (delegatedFixtureCounts{}) || access.Mailbox().String() != fixtureMailbox {
		t.Fatalf("new delegated source = counts %v, mailbox %s", got, access.Mailbox())
	}
	page, pageErr := delegatedList(t.Context(), server, access, request.Policy)
	thread, threadErr := server.ReadThread().Execute(t.Context(), ThreadReadRequest{Access: access, Policy: request.Policy, Thread: fixtureThread})
	message, messageErr := server.ReadMessage().Execute(t.Context(), MessageReadRequest{Access: access, Policy: request.Policy, Thread: fixtureThread, Message: fixtureMessage})
	if err := errors.Join(pageErr, threadErr, messageErr); err != nil {
		t.Fatal(err)
	}
	if len(page.Threads) != 1 || page.Threads[0] != fixtureThread || thread.ID != fixtureThread ||
		message.ID != fixtureMessage || message.Text != "Hello ✓\n" {
		t.Fatalf("typed records = %v / %v / %v, want one bound thread and inline text", page, thread, message)
	}
	if got, want := provider.counts(), (delegatedFixtureCounts{1, 1, 1, 1, 3}); got != want {
		t.Fatalf("provider calls = %v, want %v", got, want)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := delegatedList(t.Context(), server, access, request.Policy); !errors.Is(err, core.ErrGmailConsent) {
		t.Fatalf("closed access error = %v, want Gmail consent", err)
	}
}

func TestDelegatedGmailBindsProviderAndBoundsFailures(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name        string
		behavior    scenario
		wantErr     error
		want        delegatedFixtureCounts
		wantCleared bool
	}{
		{name: "signer response carries another issuer", behavior: scenario{sign: func(_ context.Context, in delegatedFixtureClaims) (delegatedFixtureClaims, int) {
			in.Issuer = "other@fixture-project.iam.gserviceaccount.com"
			return in, 200
		}}, wantErr: core.ErrGoogleDelegationBinding, want: delegatedFixtureCounts{1, 1, 0, 0, 0}},
		{name: "signer response carries another subject", behavior: scenario{sign: func(_ context.Context, in delegatedFixtureClaims) (delegatedFixtureClaims, int) {
			in.Subject = "other@example.com"
			return in, 200
		}}, wantErr: core.ErrGoogleDelegationBinding, want: delegatedFixtureCounts{1, 1, 0, 0, 0}},
		{name: "signer response tried to widen scope", behavior: scenario{sign: func(_ context.Context, in delegatedFixtureClaims) (delegatedFixtureClaims, int) {
			in.Scope += " https://mail.google.com/"
			return in, 200
		}}, wantErr: core.ErrGoogleDelegationBinding, want: delegatedFixtureCounts{1, 1, 0, 0, 0}},
		{name: "IAM signer permission absent", behavior: scenario{sign: func(_ context.Context, in delegatedFixtureClaims) (delegatedFixtureClaims, int) {
			return in, http.StatusForbidden
		}}, wantErr: core.ErrGoogleDelegationDenied, want: delegatedFixtureCounts{1, 1, 0, 0, 0}},
		{name: "workspace delegation forbidden", behavior: scenario{token: func(int) (int, []byte) {
			return http.StatusForbidden, []byte(`{"error":"access_denied"}`)
		}}, wantErr: core.ErrGoogleDelegationDenied, want: delegatedFixtureCounts{1, 1, 1, 0, 0}},
		{name: "oauth unauthorized client", behavior: scenario{token: func(int) (int, []byte) {
			return http.StatusBadRequest, []byte(`{"error":"unauthorized_client","error_description":"provider fixture"}`)
		}}, wantErr: core.ErrGoogleDelegationDenied, want: delegatedFixtureCounts{1, 1, 1, 0, 0}},
		{name: "token response exceeds owned byte bound", behavior: scenario{token: func(int) (int, []byte) {
			return 200, []byte(strings.Repeat(" ", 16*1024+1))
		}}, wantErr: core.ErrExchangeBodyLimit, want: delegatedFixtureCounts{1, 1, 1, 0, 0}},
		{name: "profile points to a different mailbox", behavior: scenario{profileEmail: "other@example.com"},
			wantErr: core.ErrGmailBinding, want: delegatedFixtureCounts{1, 1, 1, 1, 0}},
		{name: "unauthorized read renews once", behavior: scenario{readStatus: func(n int) int {
			if n == 1 {
				return http.StatusUnauthorized
			}
			return http.StatusOK
		}}, want: delegatedFixtureCounts{2, 2, 2, 2, 2}},
		{name: "second unauthorized read stops", behavior: scenario{readStatus: func(int) int {
			return http.StatusUnauthorized
		}}, wantErr: core.ErrGmailConsent, want: delegatedFixtureCounts{2, 2, 2, 2, 2}, wantCleared: true},
		{name: "transient token response retries then succeeds", behavior: scenario{token: func(n int) (int, []byte) {
			if n == 1 {
				return http.StatusServiceUnavailable, []byte(`{"error":"temporarily_unavailable"}`)
			}
			return 200, []byte(`{"access_token":"open-test-credential-2","token_type":"Bearer","expires_in":3600}`)
		}}, want: delegatedFixtureCounts{2, 2, 2, 1, 1}},
		{name: "persistent transient error stops at two attempts", behavior: scenario{token: func(int) (int, []byte) {
			return http.StatusServiceUnavailable, []byte(`{"error":"temporarily_unavailable"}`)
		}}, wantErr: core.ErrExchangeUnavailable, want: delegatedFixtureCounts{2, 2, 2, 0, 0}},
		{name: "quota refusal adds no retry traffic", behavior: scenario{token: func(int) (int, []byte) {
			return http.StatusTooManyRequests, []byte(`{"error":"rate_limit"}`)
		}}, wantErr: core.ErrExchangeRateLimited, want: delegatedFixtureCounts{1, 1, 1, 0, 0}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			server, provider, request := delegatedTestServer(t, row.behavior)
			access := delegatedFixtureAccess(t, server, provider, request)
			got, err := delegatedList(t.Context(), server, access, request.Policy)
			if !errors.Is(err, row.wantErr) || provider.counts() != row.want {
				t.Fatalf("list calls/error = %v/%v, want %v/%v", provider.counts(), err, row.want, row.wantErr)
			}
			if row.wantCleared && !access.Lifetime().IsZero() {
				t.Fatalf("two unauthorized replies left the rejected bearer cached for %v, want zero", access.Lifetime())
			}
			if row.wantErr != nil && (len(got.Threads) != 0 || got.Next != "") ||
				row.wantErr == nil && (len(got.Threads) != 1 || got.Threads[0] != fixtureThread) {
				t.Fatalf("Gmail projection = %v with %v, want bound success or zero refusal", got, err)
			}
		})
	}
}

func TestDelegatedGmailSharesRenewalAtExactEarlyExpiry(t *testing.T) {
	t.Parallel()
	var renewals atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	server, p, request := delegatedTestServer(t, scenario{sign: func(ctx context.Context, in delegatedFixtureClaims) (delegatedFixtureClaims, int) {
		if renewals.Add(1) == 2 {
			close(entered)
			select {
			case <-ctx.Done():
				return in, http.StatusServiceUnavailable
			case <-release:
			}
		}
		return in, 200
	}})
	access := delegatedFixtureAccess(t, server, p, request)
	if _, err := delegatedList(t.Context(), server, access, request.Policy); err != nil {
		t.Fatal(err)
	}
	const earlyRenew = 3540 * 1000000000
	p.clock.Store(delegationInitialNS + earlyRenew - 1)
	if _, err := delegatedList(t.Context(), server, access, request.Policy); err != nil || p.token.Load() != 1 {
		t.Fatalf("one ns before early renewal = %d tokens/%v, want 1/nil", p.token.Load(), err)
	}
	p.clock.Store(delegationInitialNS + earlyRenew)
	ctx, cancel := context.WithTimeout(t.Context(), fixtureDelegationTimeout)
	var joined sync.WaitGroup
	defer func() {
		cancel()
		joined.Wait()
	}()
	const callers = 8
	done := make(chan error, callers)
	launch := func() {
		joined.Go(func() {
			_, err := delegatedList(ctx, server, access, request.Policy)
			done <- err
		})
	}
	launch()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatalf("renewal did not reach IAM, sign calls = %d, want two including the first mint", p.sign.Load())
	}
	for range callers - 1 {
		launch()
	}
	close(release)
	for range callers {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatalf("caller did not complete, reads = %d, want %d", p.reads.Load(), callers+2)
		}
	}
	if got, want := p.counts(), (delegatedFixtureCounts{2, 2, 2, 2, callers + 2}); got != want {
		t.Fatalf("shared provider calls = %v, want %v", got, want)
	}
	p.clock.Store(delegationInitialNS - 1)
	if _, err := delegatedList(t.Context(), server, access, request.Policy); err != nil || p.token.Load() != 3 {
		t.Fatalf("clock rollback = %d tokens/%v, want a fresh acquisition", p.token.Load(), err)
	}
}

type renewalWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *renewalWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestDelegatedGmailCancellationReleasesSharedRenewal(t *testing.T) {
	t.Parallel()
	var signs atomic.Int32
	entered, returned := make(chan struct{}), make(chan struct{})
	server, p, request := delegatedTestServer(t, scenario{sign: func(ctx context.Context, in delegatedFixtureClaims) (delegatedFixtureClaims, int) {
		if signs.Add(1) == 1 {
			close(entered)
			<-ctx.Done()
			close(returned)
			return in, http.StatusServiceUnavailable
		}
		return in, 200
	}})
	access := delegatedFixtureAccess(t, server, p, request)
	backstop, stop := context.WithTimeout(t.Context(), fixtureDelegationTimeout)
	defer stop()
	ctx, cancel := context.WithCancelCause(backstop)
	cause := errors.New("synthetic requester left")
	var joined sync.WaitGroup
	defer func() {
		cancel(cause)
		joined.Wait()
	}()
	done := make(chan error, 1)
	joined.Go(func() {
		_, err := delegatedList(ctx, server, access, request.Policy)
		done <- err
	})
	select {
	case <-entered:
	case <-backstop.Done():
		t.Fatalf("IAM call was not entered, calls = %v, want one signer call in progress", p.counts())
	}
	access.delegated.mu.Lock()
	incumbent := access.delegated.pending
	access.delegated.mu.Unlock()
	waiterContext, cancelWaiter := context.WithCancelCause(backstop)
	defer cancelWaiter(cause)
	waiter := &renewalWaitContext{Context: waiterContext, waiting: make(chan struct{})}
	waiterDone := make(chan error, 1)
	joined.Go(func() {
		lease, err := access.delegated.borrow(waiter)
		if err == nil {
			err = lease.secret.Close()
		}
		waiterDone <- err
	})
	select {
	case <-waiter.waiting:
	case <-backstop.Done():
		t.Fatalf("second caller did not join the incumbent renewal, sign calls = %d, want one", p.sign.Load())
	}
	access.delegated.mu.Lock()
	pending := access.delegated.pending
	access.delegated.mu.Unlock()
	if incumbent == nil || pending != incumbent {
		t.Fatalf("second caller replaced the active renewal, joined incumbent = %t, want true", incumbent != nil && pending == incumbent)
	}
	cancelWaiter(cause)
	select {
	case err := <-waiterDone:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || p.sign.Load() != 1 {
			t.Fatalf("cancelled waiter error and sign calls = %v/%d, want own cause and one incumbent", err, p.sign.Load())
		}
	case <-backstop.Done():
		t.Fatalf("second caller did not join after cancellation, sign calls = %d, want one", p.sign.Load())
	}

	cancel(cause)
	select {
	case <-returned:
	case <-backstop.Done():
		t.Fatalf("IAM did not see cancellation, calls = %v, want the first signer to return", p.counts())
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) {
			t.Fatalf("cancelled read error = %v, want native and caller causes", err)
		}
	case <-backstop.Done():
		t.Fatalf("cancelled read did not join, calls = %v, want the requester to return its cancellation cause", p.counts())
	}
	if _, err := delegatedList(backstop, server, access, request.Policy); err != nil {
		t.Fatal(err)
	}
	if got, want := p.counts(), (delegatedFixtureCounts{2, 2, 1, 1, 1}); got != want {
		t.Fatalf("cancelled/recovered calls = %v, want %v", got, want)
	}
}

func TestDelegatedGmailClockOverflowRefusesBeforeProvider(t *testing.T) {
	t.Parallel()
	s, p, r := delegatedTestServer(t, scenario{})
	access := delegatedFixtureAccess(t, s, p, r)
	p.clock.Store(math.MaxInt64)
	_, err := delegatedList(t.Context(), s, access, r.Policy)
	if !errors.Is(err, core.ErrGoogleIdentityContract) || p.counts() != (delegatedFixtureCounts{}) {
		t.Fatalf("overflow attempt = %v/%v, want zero provider calls and typed refusal", p.counts(), err)
	}
	if got := fmt.Sprintf("%#v", access); got != core.RedactedValueText {
		t.Fatalf("format = %q, want redaction", got)
	}
}

func TestDelegatedGmailRetainsReplacementOnLateUnauthorized(t *testing.T) {
	t.Parallel()
	s, p, request := delegatedTestServer(t, scenario{readStatus: func(n int) int {
		if n == 2 {
			return http.StatusUnauthorized
		}
		return http.StatusOK
	}})
	access := delegatedFixtureAccess(t, s, p, request)
	for range 2 {
		if _, err := delegatedList(t.Context(), s, access, request.Policy); err != nil {
			t.Fatal(err)
		}
	}
	if err := access.delegated.invalidate(1); err != nil {
		t.Fatal(err)
	}
	if _, err := delegatedList(t.Context(), s, access, request.Policy); err != nil {
		t.Fatal(err)
	}
	if got, want := p.counts(), (delegatedFixtureCounts{2, 2, 2, 2, 4}); got != want {
		t.Fatalf("late generation-1 failure caused calls %v, want %v", got, want)
	}
}

func TestDelegatedGmailRefusesBearerExpiredDuringProfile(t *testing.T) {
	t.Parallel()
	s, p, request := delegatedTestServer(t, scenario{})
	access := delegatedFixtureAccess(t, s, p, request)
	access.delegated.observe = func() (temporal.Instant, error) {
		if p.profile.Load() != 0 {
			return temporal.InstantFromNanoseconds(delegationInitialNS + 3600*1000000000), nil
		}
		return temporal.InstantFromNanoseconds(delegationInitialNS), nil
	}
	_, err := delegatedList(t.Context(), s, access, request.Policy)
	if got, want := p.counts(), (delegatedFixtureCounts{1, 1, 1, 1, 0}); got != want || !errors.Is(err, core.ErrGmailConsent) {
		t.Fatalf("expiry during provider admission = %v/%v, want %v and no Gmail data read", got, err, want)
	}
}
