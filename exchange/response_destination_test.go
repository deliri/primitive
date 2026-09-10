package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type deliveryFault uint8

const (
	deliveryExact deliveryFault = iota
	deliveryReadFailure
	deliveryCloseFailure
	deliveryCancelAtClose
	deliverySinkRefusal
)

type deliveryBody struct {
	source *bytes.Reader
	fault  deliveryFault
	cancel context.CancelFunc
	closes int
}

func (b *deliveryBody) Read(p []byte) (int, error) {
	n, err := b.source.Read(p)
	if err == io.EOF && b.fault == deliveryReadFailure {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
func (b *deliveryBody) Close() error {
	b.closes++
	if b.fault == deliveryCancelAtClose {
		b.cancel()
	}
	if b.fault == deliveryCloseFailure {
		return io.ErrClosedPipe
	}
	return nil
}

type deliverySink struct {
	body    []byte
	maximum int
}

func (s *deliverySink) Write(p []byte) (int, error) {
	n := min(len(p), s.maximum-len(s.body))
	s.body = append(s.body, p[:n]...)
	if n < len(p) {
		return n, core.ErrExchangeBodyLimit
	}
	return n, nil
}

func TestResponseDestinationLayerTriad(t *testing.T) {
	t.Parallel()
	// Each size crosses a distinct copy-window boundary; each fault changes
	// completion or refusal independently, on success and provider-error replies.
	for _, size := range []int{1, exchange.TransferBufferBytes - 1, exchange.TransferBufferBytes, exchange.TransferBufferBytes + 1, 3*exchange.TransferBufferBytes + 7} {
		for _, fault := range []deliveryFault{deliveryExact, deliveryReadFailure, deliveryCloseFailure, deliveryCancelAtClose, deliverySinkRefusal} {
			for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
				for _, absent := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/%d/%d/absent=%t", size, fault, status, absent), func(t *testing.T) {
						t.Parallel()
						checkResponseDelivery(t, bytes.Repeat([]byte{0, 0xff, 1}, (size+2)/3)[:size], fault, status, absent)
					})
				}
			}
		}
	}
}

func checkResponseDelivery(t *testing.T, payload []byte, fault deliveryFault, status int, absent bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	body := &deliveryBody{source: bytes.NewReader(payload), fault: fault, cancel: cancel}
	calls := 0
	client := mustExchangeClient(t, &http.Client{Transport: bindingTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: status, ContentLength: -1, Body: body, Request: request}, nil
	})})
	sink := &deliverySink{maximum: len(payload)}
	if fault == deliverySinkRefusal && len(payload) > 0 {
		sink.maximum--
	}
	opened := uint64(0)
	factory := func(attemptContext context.Context, attempt uint64) (io.Writer, error) {
		if attemptContext == nil || attemptContext.Err() != nil {
			t.Fatal("invalid attempt context")
		}
		opened = attempt
		return sink, nil
	}
	target := mustEndpoint(t, "https://provider.example.test/delivery")
	semantics := exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}
	policy := singleAttemptOperationPolicy(t)
	var got exchange.ResponseDelivery
	var err error
	if absent {
		got, err = exchange.SendNoBodyTo(exchange.NoBodyBoundedCall{Context: ctx, Client: client,
			Request: exchange.NoBodyBoundedRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK()},
			Policy:  exchange.NoBodyBoundedPolicy{Operation: policy}}, factory)
	} else {
		got, err = exchange.SendTo(exchange.BoundedCall{Context: ctx, Client: client,
			Request: exchange.BoundedRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK(), RequestContentType: core.HTTPMediaTypeOctetStream(), Body: []byte{1}},
			Policy:  exchange.BoundedPolicy{Operation: policy}}, factory)
	}
	refused := fault == deliverySinkRefusal && len(payload) > 0
	complete := fault != deliveryReadFailure && !refused
	if calls != 1 || body.closes != 1 || opened != 1 || got.Metadata.Attempts != 1 || got.Complete != complete {
		t.Fatalf("calls/closes/opened=%d/%d/%d result=%+v; complete want %t, err=%v", calls, body.closes, opened, got, complete, err)
	}
	if got.Validate() != nil || got.Metadata.Status != mustHTTPStatus(t, status) || got.Metadata.Bytes.Uint64() != uint64(len(sink.body)) || !bytes.Equal(sink.body, payload[:sink.maximum]) {
		t.Fatalf("delivery metadata or bytes diverged: %+v, retained=%d, err=%v", got, len(sink.body), err)
	}
	identities := []struct {
		err  error
		want bool
	}{
		{io.ErrUnexpectedEOF, fault == deliveryReadFailure}, {io.ErrClosedPipe, fault == deliveryCloseFailure},
		{core.ErrExchangeBodyLimit, refused}, {context.Canceled, fault == deliveryCancelAtClose},
	}
	for _, identity := range identities {
		if errors.Is(err, identity.err) != identity.want {
			t.Fatalf("error=%v; %v present want %t", err, identity.err, identity.want)
		}
	}
	var statusError exchange.StatusError
	wantStatus := status != http.StatusOK && (fault == deliveryExact || fault == deliverySinkRefusal && !refused)
	if errors.As(err, &statusError) != wantStatus {
		t.Fatalf("status error=%v; want %t", err, wantStatus)
	}
	wantError := fault != deliveryExact && !(fault == deliverySinkRefusal && len(payload) == 0) || status != http.StatusOK
	if (err != nil) != wantError {
		t.Fatalf("error=%v; refusal want %t", err, wantError)
	}
}

func FuzzResponseDestinationPreservesDelivery(f *testing.F) {
	for _, size := range []int{0, 1, exchange.TransferBufferBytes - 1, exchange.TransferBufferBytes, exchange.TransferBufferBytes + 1} {
		for fault := deliveryExact; fault <= deliverySinkRefusal; fault++ {
			f.Add(bytes.Repeat([]byte{0xff}, size), uint8(fault), false, true)
			f.Add(bytes.Repeat([]byte{0}, size), uint8(fault), true, false)
		}
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawFault uint8, failureStatus, absent bool) {
		if len(payload) > 4*exchange.TransferBufferBytes || rawFault > uint8(deliverySinkRefusal) {
			return
		}
		status := http.StatusOK
		if failureStatus {
			status = http.StatusBadRequest
		}
		checkResponseDelivery(t, payload, deliveryFault(rawFault), status, absent)
	})
}

func TestResponseDestinationRetryCustody(t *testing.T) {
	t.Parallel()
	var sinks []*deliverySink
	calls, closes := 0, 0
	client := mustExchangeClient(t, &http.Client{Transport: bindingTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		status := http.StatusServiceUnavailable
		payload := []byte("first failure")
		if calls == 2 {
			status = http.StatusOK
			payload = []byte("second success")
		}
		return &http.Response{StatusCode: status, ContentLength: -1, Body: &retryDeliveryBody{Reader: bytes.NewReader(payload), closes: &closes}, Request: request}, nil
	})})
	delivery, err := exchange.SendNoBodyTo(exchange.NoBodyBoundedCall{Context: t.Context(), Client: client,
		Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, "https://provider.example.test/retry"), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySafe}, ExpectedStatus: core.HTTPStatusOK()},
		Policy:  exchange.NoBodyBoundedPolicy{Operation: retryOperationPolicy(t, 2)}},
		func(ctx context.Context, attempt uint64) (io.Writer, error) {
			if attempt != uint64(len(sinks)+1) {
				t.Fatalf("attempt=%d sinks=%d", attempt, len(sinks))
			}
			sink := &deliverySink{maximum: 64}
			sinks = append(sinks, sink)
			return sink, nil
		})
	if err != nil || !delivery.Complete || delivery.Metadata.Attempts != 2 || calls != 2 || closes != 2 || len(sinks) != 2 {
		t.Fatalf("delivery=%+v calls/closes/sinks=%d/%d/%d err=%v", delivery, calls, closes, len(sinks), err)
	}
	if !bytes.Equal(sinks[0].body, []byte("first failure")) || !bytes.Equal(sinks[1].body, []byte("second success")) {
		t.Fatal("retry contaminated response retention")
	}
}

type retryDeliveryBody struct {
	*bytes.Reader
	closes *int
}

func (b *retryDeliveryBody) Close() error { *b.closes++; return nil }

func TestResponseDestinationRefusesInvalidSinkBeforeHTTP(t *testing.T) {
	t.Parallel()
	var typedNil *deliverySink
	cases := []struct {
		name     string
		factory  exchange.ResponseDestination
		identity error
	}{
		{"absent", nil, core.ErrExchangeContract},
		{"nil writer", func(context.Context, uint64) (io.Writer, error) { return nil, nil }, core.ErrExchangeContract},
		{"typed nil", func(context.Context, uint64) (io.Writer, error) { return typedNil, nil }, core.ErrExchangeContract},
		{"factory failure", func(context.Context, uint64) (io.Writer, error) { return nil, io.ErrClosedPipe }, io.ErrClosedPipe},
		{"factory panic", func(context.Context, uint64) (io.Writer, error) { panic("fixture") }, core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := mustExchangeClient(t, &http.Client{Transport: bindingTransport(func(*http.Request) (*http.Response, error) {
				t.Fatal("refused sink reached HTTP")
				return nil, io.ErrClosedPipe
			})})
			got, err := exchange.SendNoBodyTo(exchange.NoBodyBoundedCall{Context: t.Context(), Client: client, Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, "https://provider.example.test/refused"), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()}, Policy: exchange.NoBodyBoundedPolicy{Operation: singleAttemptOperationPolicy(t)}}, tc.factory)
			if !errors.Is(err, tc.identity) || got.Complete || got.Metadata.Attempts != 0 {
				t.Fatalf("delivery=%+v err=%v want %v", got, err, tc.identity)
			}
		})
	}
}
