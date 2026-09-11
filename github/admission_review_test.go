package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type nilPointerVisitor struct{}

func (*nilPointerVisitor) VisitGitHubTreeEntry(*TreeEntryStream) error { panic("nil visitor invoked") }

type nilMapVisitor map[string]int

func (nilMapVisitor) VisitGitHubTreeEntry(*TreeEntryStream) error { return nil }

type nilSliceVisitor []int

func (nilSliceVisitor) VisitGitHubTreeEntry(*TreeEntryStream) error { return nil }

type nilChanVisitor chan int

func (nilChanVisitor) VisitGitHubTreeEntry(*TreeEntryStream) error { return nil }

type nilFuncVisitor func(*TreeEntryStream) error

func (v nilFuncVisitor) VisitGitHubTreeEntry(e *TreeEntryStream) error { return v(e) }

func TestTreeVisitorAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		visitor TreeVisitor
		wantErr error
	}{
		{name: "absent visitor", wantErr: core.ErrGitHubContract},
		{name: "nil pointer receiver", visitor: (*nilPointerVisitor)(nil), wantErr: core.ErrGitHubContract},
		{name: "nil map receiver", visitor: nilMapVisitor(nil), wantErr: core.ErrGitHubContract},
		{name: "nil slice receiver", visitor: nilSliceVisitor(nil), wantErr: core.ErrGitHubContract},
		{name: "nil channel receiver", visitor: nilChanVisitor(nil), wantErr: core.ErrGitHubContract},
		{name: "nil function receiver", visitor: nilFuncVisitor(nil), wantErr: core.ErrGitHubContract},
		{name: "present empty map receiver", visitor: nilMapVisitor{}},
		{name: "present empty slice receiver", visitor: nilSliceVisitor{}},
		{name: "present channel receiver", visitor: make(nilChanVisitor)},
		{name: "present function receiver", visitor: nilFuncVisitor(func(*TreeEntryStream) error { return nil })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				writeJSON(t, w, treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{}}, http.StatusOK)
			}))
			defer server.Close()
			request := TreeRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Visitor: tc.visitor}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("TreeRequest.Validate()=%v, want %v", err, tc.wantErr)
			}
			client := clientFixture(t, server.URL)
			defer func() {
				if err := client.Close(); err != nil {
					t.Errorf("Client.Close()=%v, want nil", err)
				}
			}()
			got, err := client.ReadTree(t.Context(), request)
			wantCalls := int64(1)
			if tc.wantErr != nil {
				wantCalls = 0
			}
			if !errors.Is(err, tc.wantErr) || calls.Load() != wantCalls {
				t.Fatalf("ReadTree()=%+v/%v requests=%d, want %v and %d requests", got, err, calls.Load(), tc.wantErr, wantCalls)
			}
			if tc.wantErr != nil && got != (TreeObservation{}) {
				t.Fatalf("refused observation=%+v, want zero", got)
			}
			if tc.wantErr == nil && (got.Entries != 0 || got.Repository != request.Repository || got.Commit != request.Commit) {
				t.Fatalf("empty tree=%+v, want requested coordinates and zero entries", got)
			}
		})
	}
}

func TestAppClientCredentialAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	transport, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatal(err)
	}
	agent, err := ParseUserAgent("primitive-test")
	if err != nil {
		t.Fatal(err)
	}
	app, appErr := NewAppID(1)
	installation, installationErr := NewInstallationID(2)
	credential, credentialErr := NewAppCredential(app, installation, []byte(fixedRSAPrivateKey))
	if err := errors.Join(appErr, installationErr, credentialErr); err != nil {
		t.Fatalf("credential fixture=%v, want nil", err)
	}
	alias := credential
	owned, err := NewAppClient(transport, agent, credential)
	if err != nil {
		t.Fatalf("NewAppClient(valid)=%v, want nil", err)
	}
	defer func() {
		if err := owned.Close(); err != nil {
			t.Errorf("Client.Close()=%v, want nil", err)
		}
	}()
	if err := credential.Close(); err != nil {
		t.Fatalf("AppCredential.Close()=%v, want nil", err)
	}
	if err := owned.Validate(); err != nil {
		t.Fatalf("owned client after caller close=%v, want nil", err)
	}
	for _, tc := range []struct {
		name       string
		credential AppCredential
	}{
		{name: "absent credential cannot downgrade to public"},
		{name: "destroyed aliased key cannot construct client", credential: alias},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewAppClient(transport, agent, tc.credential)
			if !errors.Is(err, core.ErrGitHubAuthentication) || got != (Client{}) {
				t.Fatalf("NewAppClient()=%v/%v, want zero and %v", got, err, core.ErrGitHubAuthentication)
			}
		})
	}
	public, err := NewClient(transport, agent)
	if err != nil || public.Validate() != nil {
		t.Fatalf("NewClient(public)=%v/%v, want valid public client", public, err)
	}
	if err := public.Close(); err != nil {
		t.Fatalf("public Close()=%v, want nil", err)
	}
}
