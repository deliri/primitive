package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const lifetimeBackstop = 10 * time.Second

// lifetimeTransport observes the actual response body returned by Go's native
// TLS transport. It does not synthesize a response or replace network behavior.
type lifetimeTransport struct {
	native          *http.Transport
	reads           atomic.Int64
	bytes           atomic.Int64
	closes          atomic.Int64
	held            *atomic.Bool
	readsDuringHold atomic.Int64
}

func (s *lifetimeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := s.native.RoundTrip(request)
	if response != nil && response.Body != nil {
		response.Body = &lifetimeResponseBody{native: response.Body, owner: s}
	}
	return response, err
}

type lifetimeResponseBody struct {
	native io.ReadCloser
	owner  *lifetimeTransport
}

func (s *lifetimeResponseBody) Read(buffer []byte) (int, error) {
	s.owner.reads.Add(1)
	if s.owner.held != nil && s.owner.held.Load() {
		s.owner.readsDuringHold.Add(1)
	}
	count, err := s.native.Read(buffer)
	s.owner.bytes.Add(int64(count))
	return count, err
}
func (s *lifetimeResponseBody) Close() error { s.owner.closes.Add(1); return s.native.Close() }

type lifetimePatternSource struct {
	left int64
	read int64
}

func (s *lifetimePatternSource) Read(buffer []byte) (int, error) {
	if s.left == 0 {
		return 0, io.EOF
	}
	if int64(len(buffer)) > s.left {
		buffer = buffer[:s.left]
	}
	for index := range buffer {
		buffer[index] = byte((s.read + int64(index)) % 251)
	}
	s.left -= int64(len(buffer))
	s.read += int64(len(buffer))
	return len(buffer), nil
}

type lifetimeGateSink struct {
	ctx            context.Context
	adapter        *HTTPServer
	transport      *lifetimeTransport
	entered        chan struct{}
	release        chan struct{}
	held           atomic.Bool
	written        int64
	maximum        int
	firstReadBytes int64
	firstReadCalls int64
	lockWasFree    bool
}

func (s *lifetimeGateSink) Write(buffer []byte) (int, error) {
	if s.written == 0 {
		s.firstReadBytes = s.transport.bytes.Load()
		s.firstReadCalls = s.transport.reads.Load()
		// A native TryLock observes whether RoundTrip truly retains credential
		// custody across native transport and destination calls. It catches removal
		// of the read lock without depending on when the closing goroutine runs.
		s.lockWasFree = s.adapter.mu.TryLock()
		if s.lockWasFree {
			s.adapter.mu.Unlock()
		}
		s.held.Store(true)
		close(s.entered)
		select {
		case <-s.release:
		case <-s.ctx.Done():
			s.held.Store(false)
			return 0, s.ctx.Err()
		}
		s.held.Store(false)
	}
	for index, value := range buffer {
		if value != byte((s.written+int64(index))%251) {
			return 0, core.ErrAIBinding
		}
	}
	s.written += int64(len(buffer))
	if len(buffer) > s.maximum {
		s.maximum = len(buffer)
	}
	return len(buffer), nil
}

type lifetimeAcknowledgingSink struct {
	written      int64
	extent       int64
	acknowledged chan struct{}
}

func (s *lifetimeAcknowledgingSink) Write(buffer []byte) (int, error) {
	for index, value := range buffer {
		if value != byte((s.written+int64(index))%251) {
			return 0, core.ErrAIBinding
		}
	}
	s.written += int64(len(buffer))
	if s.written == s.extent {
		close(s.acknowledged)
	}
	return len(buffer), nil
}

type lifetimeCallResult struct {
	response Response
	err      error
}

func nativeLifetimeAdapter(t *testing.T, fixture *httptest.Server, provider Provider, held *atomic.Bool) (*HTTPServer, *lifetimeTransport) {
	t.Helper()
	transport := fixture.Client().Transport.(*http.Transport).Clone()
	host, _, err := net.SplitHostPort(fixture.Listener.Addr().String())
	if err != nil {
		t.Fatalf("TLS fixture authority = %v, want native host", err)
	}
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = host
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fixture.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	observer := &lifetimeTransport{native: transport, held: held}
	native, err := exchange.NewClient(&http.Client{Transport: observer})
	if err != nil {
		t.Fatalf("native exchange construction = %v, want nil", err)
	}
	credential, err := ParseCredential(provider, []byte("sk-owned-lifetime-test"))
	if err != nil {
		t.Fatalf("credential fixture = %v, want nil", err)
	}
	defer func() {
		if err := credential.Close(); err != nil {
			t.Errorf("borrowed credential cleanup = %v, want nil", err)
		}
	}()
	adapter, err := NewHTTPServer(native, credential)
	if err != nil {
		t.Fatalf("adapter fixture = %v, want nil", err)
	}
	return adapter, observer
}

func nativeLifetimeRequest(t *testing.T, provider Provider, sink io.Writer) Request {
	t.Helper()
	route := Route{Provider: provider, Operation: Responses}
	if provider == Anthropic {
		route.Operation = Messages
	}
	endpoint, err := route.Endpoint()
	if err != nil {
		t.Fatalf("native route fixture = %v, want nil", err)
	}
	return Request{Route: route, Stream: exchange.StreamRoundTripRequest{Target: endpoint, Source: bytes.NewReader(nil), Destination: sink, Buffer: make([]byte, 32768), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, RequestContentType: core.HTTPMediaTypeJSON(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}}
}

func awaitLifetimeExit(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(lifetimeBackstop):
		t.Fatalf("%s did not exit, want owned completion", name)
	}
}

func TestHTTPServerNativeBackpressureRetainsCredentialCustodyUntilClose(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{OpenAI, Anthropic} {
		for _, extent := range []int64{32767, 32768, 32769, 8*32768 + 7, 4*1024*1024 + 1} {
			t.Run(fmt.Sprintf("provider-%d-response-extent-%d", provider, extent), func(t *testing.T) {
				t.Parallel()
				handlerDone := make(chan struct{})
				handlerResult := make(chan error, 1)
				fixture := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					defer close(handlerDone)
					_, readErr := io.Copy(io.Discard, request.Body)
					wantHeader, wantKey := "Authorization", "Bearer sk-owned-lifetime-test"
					if provider == Anthropic {
						wantHeader, wantKey = core.AIAnthropicKeyHeader, "sk-owned-lifetime-test"
					}
					if request.Header.Get(wantHeader) != wantKey || request.Method != http.MethodPost {
						handlerResult <- core.ErrAIBinding
						return
					}
					writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
					writer.WriteHeader(http.StatusOK)
					_, writeErr := io.CopyBuffer(writer, &lifetimePatternSource{left: extent}, make([]byte, 32768))
					handlerResult <- errors.Join(readErr, writeErr)
				}))
				defer fixture.Close()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				sink := &lifetimeGateSink{ctx: ctx, entered: make(chan struct{}), release: make(chan struct{})}
				adapter, transport := nativeLifetimeAdapter(t, fixture, provider, &sink.held)
				sink.adapter, sink.transport = adapter, transport
				ownedKey := adapter.credential.value
				request := nativeLifetimeRequest(t, provider, sink)
				callDone := make(chan struct{})
				result := make(chan lifetimeCallResult, 1)
				go func() {
					defer close(callDone)
					response, err := adapter.RoundTrip(ctx, request, exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}})
					result <- lifetimeCallResult{response: response, err: err}
				}()
				closeDone := make(chan struct{})
				closeResult := make(chan error, 1)
				closeStarted := false
				defer func() {
					cancel()
					awaitLifetimeExit(t, callDone, "native SDK stream")
					if closeStarted {
						awaitLifetimeExit(t, closeDone, "credential close")
					}
					if err := adapter.Close(); err != nil {
						t.Errorf("adapter cleanup = %v, want nil", err)
					}
				}()
				awaitLifetimeExit(t, sink.entered, "first borrowed sink write")
				closeStarted = true
				closerEntered := make(chan struct{})
				go func() { defer close(closeDone); close(closerEntered); closeResult <- adapter.Close() }()
				awaitLifetimeExit(t, closerEntered, "owned closer invocation")
				close(sink.release)
				awaitLifetimeExit(t, callDone, "native SDK stream")
				awaitLifetimeExit(t, closeDone, "credential close")
				awaitLifetimeExit(t, handlerDone, "native TLS handler")
				got := <-result
				if got.err != nil || <-closeResult != nil || <-handlerResult != nil || sink.written != extent || got.response.Stream.Metadata.Bytes.Uint64() != uint64(extent) || got.response.Stream.Metadata.Attempts != 1 || transport.closes.Load() != 1 {
					t.Fatalf("native stream = (%v,%d bytes,metadata %+v,closes %d), want exact %d bytes/single attempt/single body close", got.err, sink.written, got.response.Stream.Metadata, transport.closes.Load(), extent)
				}
				if sink.lockWasFree || sink.firstReadBytes <= 0 || sink.firstReadBytes > int64(len(request.Stream.Buffer)) || sink.firstReadCalls != 1 || sink.maximum > len(request.Stream.Buffer) || transport.readsDuringHold.Load() != 0 {
					t.Fatalf("stream custody = lock free:%t first-read:%d/%d largest-write:%d reads-during-backpressure:%d, want native read lock and bounded synchronous handoff", sink.lockWasFree, sink.firstReadBytes, sink.firstReadCalls, sink.maximum, transport.readsDuringHold.Load())
				}
				for _, value := range ownedKey {
					if value != 0 {
						t.Fatal("closed credential backing slice retained material, want every owned byte cleared")
					}
				}
				if err := adapter.Validate(); !errors.Is(err, core.ErrAIAuthentication) || errors.Is(err, core.ErrAIContract) {
					t.Fatalf("closed adapter = %v, want authentication identity only", err)
				}
				// Closure revokes future authority before any transport or sink effect.
				reads := transport.reads.Load()
				written := sink.written
				response, err := adapter.RoundTrip(ctx, request, exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}})
				if !errors.Is(err, core.ErrAIAuthentication) || response.Stream.Metadata.Attempts != 0 || transport.reads.Load() != reads || sink.written != written {
					t.Fatalf("post-close call = (%v,%+v), want authentication refusal and no native effects", err, response.Stream.Metadata)
				}
				if err := adapter.client.Validate(); err != nil {
					t.Fatalf("borrowed native transport after adapter close = %v, want still valid", err)
				}
			})
		}
	}
}

func TestHTTPServerNativeMidstreamCancelPreservesProgressAndJoinsOwners(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{OpenAI, Anthropic} {
		for _, extent := range []int64{1, 32767, 32768, 32769, 8*32768 + 7} {
			t.Run(fmt.Sprintf("provider-%d-acknowledged-prefix-%d", provider, extent), func(t *testing.T) {
				t.Parallel()
				handlerDone := make(chan struct{})
				handlerCancelled := make(chan struct{})
				fixture := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					defer close(handlerDone)
					if _, err := io.Copy(io.Discard, request.Body); err != nil {
						return
					}
					writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
					writer.WriteHeader(http.StatusOK)
					if _, err := io.CopyBuffer(writer, &lifetimePatternSource{left: extent}, make([]byte, 32768)); err != nil {
						return
					}
					writer.(http.Flusher).Flush()
					<-request.Context().Done()
					close(handlerCancelled)
				}))
				defer fixture.Close()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				adapter, transport := nativeLifetimeAdapter(t, fixture, provider, nil)
				sink := &lifetimeAcknowledgingSink{extent: extent, acknowledged: make(chan struct{})}
				request := nativeLifetimeRequest(t, provider, sink)
				done := make(chan struct{})
				result := make(chan lifetimeCallResult, 1)
				go func() {
					defer close(done)
					response, err := adapter.RoundTrip(ctx, request, exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}})
					result <- lifetimeCallResult{response: response, err: err}
				}()
				defer func() {
					cancel()
					awaitLifetimeExit(t, done, "cancelled SDK call")
					if err := adapter.Close(); err != nil {
						t.Errorf("adapter close after cancel = %v, want nil", err)
					}
				}()
				awaitLifetimeExit(t, sink.acknowledged, "native prefix acknowledgement")
				cancel()
				awaitLifetimeExit(t, done, "cancelled SDK call")
				awaitLifetimeExit(t, handlerDone, "cancelled native TLS handler")
				awaitLifetimeExit(t, handlerCancelled, "native request cancellation")
				got := <-result
				if !errors.Is(got.err, context.Canceled) || sink.written != extent || got.response.Stream.Metadata.Bytes.Uint64() != uint64(extent) || got.response.Stream.Metadata.Attempts != 1 || transport.closes.Load() != 1 {
					t.Fatalf("native cancel = (%v,%d bytes,%+v,closes %d), want cancellation/exact acknowledged %d bytes/single attempt/closed response", got.err, sink.written, got.response.Stream.Metadata, transport.closes.Load(), extent)
				}
				for _, identity := range []core.ErrorIdentity{core.ErrAIContract, core.ErrAIAuthentication, core.ErrAIBinding, core.ErrAIRateLimited, core.ErrAIUnavailable, core.ErrAIRequestRejected} {
					if errors.Is(got.err, identity) {
						t.Fatalf("native cancellation error = %v matches %v, want native cause without invented provider identity", got.err, identity)
					}
				}
				if err := adapter.Validate(); err != nil {
					t.Fatalf("cancelled stream credential = %v, want authority retained until explicit close", err)
				}
			})
		}
	}
}
