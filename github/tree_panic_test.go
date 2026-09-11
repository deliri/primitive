package github

import (
	"bytes"
	"context"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"net/http"
	"testing"
	"time"
)

type treePanicTransport struct {
	payload []byte
	closed  chan struct{}
}

func (p treePanicTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {core.HTTPMediaTypeJSON().String()}}, Body: &treePanicBody{Reader: bytes.NewReader(p.payload), closed: p.closed}, Request: request}, nil
}

type treePanicBody struct {
	*bytes.Reader
	closed chan struct{}
}

func (b *treePanicBody) Close() error { close(b.closed); return nil }

// This injected transport pins a response read larger than its first entry.
// Real HTTP cancellation is separately exercised by the lifecycle triad.
func TestTreeVisitorPanicJoinsOwnedDownload(t *testing.T) {
	t.Parallel()
	payload := marshalGitHubFixture(t, treeResponseFixture{SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree", Tree: []treeEntryFixture{treeWire("main.go", "blob", parsedCommit(t).String())}})
	payload = append(payload, bytes.Repeat([]byte(" "), 16<<10)...)
	closed := make(chan struct{})
	transport, err := exchange.NewClient(&http.Client{Transport: treePanicTransport{payload: payload, closed: closed}})
	if err != nil {
		t.Fatalf("NewClient()=%v, want nil", err)
	}
	client := clientFixture(t, "http://127.0.0.1")
	client.state.client = transport
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close()=%v, want nil", err)
		}
	}()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	panicFact := &struct{ name string }{"visitor"}
	request := TreeRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Visitor: nilFuncVisitor(func(*TreeEntryStream) error { panic(panicFact) })}
	done := make(chan bool, 1)
	go func() {
		defer func() { done <- recover() == panicFact }()
		got, err := client.ReadTree(ctx, request)
		t.Errorf("ReadTree returned=%+v/%v, want original visitor panic", got, err)
	}()
	select {
	case got := <-done:
		if !got {
			t.Fatal("propagated visitor panic=false, want true")
		}
	case <-time.After(treeLifecycleBackstop):
		cancel()
		t.Fatal("ReadTree panic exit=false, want true")
	}
	select {
	case <-closed:
	case <-time.After(treeLifecycleBackstop):
		t.Fatal("owned download body closed=false, want true before panic propagates")
	}
}
