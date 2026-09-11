package github

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

const treeLifecycleBackstop = 10 * time.Second

func TestTreeDownloadCancellationLayerTriad(t *testing.T) {
	t.Parallel()
	visitorErr := errors.New("visitor refused entry")
	cases := []struct {
		name      string
		malformed bool
		refuse    bool
		wantErr   error
	}{
		{name: "malformed prefix cancels stalled provider", malformed: true, wantErr: core.ErrGitHubResponse},
		{name: "visitor refusal cancels stalled provider", refuse: true, wantErr: visitorErr},
		{name: "empty completed tree leaves caller context usable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			payload := marshalGitHubFixture(t, treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryWire{}})
			if tc.malformed {
				payload = []byte("!")
			}
			if tc.refuse {
				payload = marshalGitHubFixture(t, treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryWire{treeWire("main.go", "blob", parsedCommit(t).String())}})
				end := bytes.Index(payload, []byte(`],"truncated"`))
				if end < 0 {
					t.Fatalf("fixture=%q, want entry prefix before truncated", payload)
				}
				payload = payload[:end]
			}
			providerDone := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(providerDone)
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(payload); err != nil {
					t.Errorf("provider Write()=%v, want nil", err)
					return
				}
				if tc.wantErr != nil {
					if err := http.NewResponseController(w).Flush(); err != nil {
						t.Errorf("provider Flush()=%v, want nil", err)
						return
					}
					<-r.Context().Done()
				}
			}))
			defer func() { cancel(); server.Close() }()
			client := clientFixture(t, server.URL)
			defer func() {
				if err := client.Close(); err != nil {
					t.Errorf("Client.Close()=%v, want nil", err)
				}
			}()
			visits := 0
			visitor := nilFuncVisitor(func(entry TreeEntry) error {
				visits++
				if err := entry.Validate(); err != nil {
					return err
				}
				return visitorErr
			})
			type result struct {
				observation TreeObservation
				err         error
			}
			done := make(chan result, 1)
			go func() {
				got, err := client.ReadTree(ctx, TreeRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Visitor: visitor})
				done <- result{got, err}
			}()
			var got result
			select {
			case got = <-done:
			case <-time.After(treeLifecycleBackstop):
				cancel()
				select {
				case <-done:
				case <-time.After(treeLifecycleBackstop):
					t.Fatalf("ReadTree worker exited = %t, want true after cancellation", false)
				}
				t.Fatalf("ReadTree returned after terminal refusal = %t, want true without caller cancellation", false)
			}
			if !errors.Is(got.err, tc.wantErr) {
				t.Fatalf("ReadTree()=%+v/%v, want %v", got.observation, got.err, tc.wantErr)
			}
			if tc.wantErr != nil && got.observation != (TreeObservation{}) {
				t.Fatalf("failed observation=%+v, want zero completion", got.observation)
			}
			wantVisits := 0
			if tc.refuse {
				wantVisits = 1
			}
			if visits != wantVisits {
				t.Fatalf("visitor calls=%d, want %d", visits, wantVisits)
			}
			if ctx.Err() != nil {
				t.Fatalf("caller context=%v, want still usable", ctx.Err())
			}
			select {
			case <-providerDone:
			case <-time.After(treeLifecycleBackstop):
				t.Fatalf("provider handler exited = %t, want true", false)
			}
		})
	}
}

func TestTreeContextRefusalBeforeNetwork(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{name: "absent context", wantErr: core.ErrGitHubContract},
		{name: "already cancelled context", ctx: cancelled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Errorf("provider request count = %d, want zero", 1)
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer server.Close()
			client := clientFixture(t, server.URL)
			defer func() {
				if err := client.Close(); err != nil {
					t.Errorf("Client.Close()=%v, want nil", err)
				}
			}()
			request := TreeRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Visitor: nilFuncVisitor(func(TreeEntry) error { t.Errorf("visitor calls = %d, want zero", 1); return nil })}
			got, err := client.ReadTree(tc.ctx, request)
			if !errors.Is(err, tc.wantErr) || got != (TreeObservation{}) {
				t.Fatalf("ReadTree()=%+v/%v, want zero and %v", got, err, tc.wantErr)
			}
		})
	}
}
