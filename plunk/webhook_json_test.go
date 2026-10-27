package plunk

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type webhookApplicationInput struct {
	Name string `json:"name"`
	Kind uint8  `json:"kind"`
}

func (p webhookApplicationInput) Validate() error {
	if p.Name == "" {
		return core.ErrInvalidInput
	}
	return nil
}

func webhookApplicationWire(t testing.TB) []byte {
	t.Helper()
	value := webhookApplicationInput{Name: "application-fixture", Kind: 7}
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

type webhookBodyLifetime struct {
	source   io.Reader
	readErr  error
	closeErr error
	reads    int
	closes   int
}

func (r *webhookBodyLifetime) Read(destination []byte) (int, error) {
	r.reads++
	n, err := r.source.Read(destination)
	if r.readErr != nil {
		return n, r.readErr
	}
	return n, err
}
func (r *webhookBodyLifetime) Close() error { r.closes++; return r.closeErr }

func webhookJSONCall(t testing.TB, body io.ReadCloser, length int, bearer string) exchange.SocketServerCall {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "https://example.test/workflow", nil)
	request.Body = body
	request.ContentLength = int64(length)
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatal(err)
	}
	return call
}

func TestWebhookJSONAuthenticationStructureAndLifetimeLayerTriad(t *testing.T) {
	t.Parallel()
	canonical := webhookApplicationWire(t)
	for _, tc := range []struct {
		name       string
		body       []byte
		bearer     string
		wantCause  error
		readErr    error
		closeErr   error
		wantReads  bool
		wantCloses int
		wantBody   webhookApplicationInput
	}{
		{name: "authenticated typed custom body", body: canonical, bearer: "plunk-shared-secret", wantReads: true, wantCloses: 1, wantBody: webhookApplicationInput{Name: "application-fixture", Kind: 7}},
		{name: "foreign bearer cannot read or close caller body", body: canonical, bearer: "foreign-secret", wantCause: core.ErrPlunkVerification},
		{name: "missing bearer cannot read or close caller body", body: canonical, wantCause: core.ErrPlunkAuthentication},
		{name: "unknown application member stays strict", body: bytes.Replace(canonical, []byte(`"kind":7`), []byte(`"private-member":"private-value"`), 1), bearer: "plunk-shared-secret", wantCause: json.ErrUnknownName, wantReads: true, wantCloses: 1},
		{name: "duplicate application member refused", body: bytes.Replace(canonical, []byte(`"kind":7`), []byte(`"kind":7,"kind":7`), 1), bearer: "plunk-shared-secret", wantCause: core.ErrJSONContract, wantReads: true, wantCloses: 1},
		{name: "truncated document cannot leak prefix fields", body: canonical[:len(canonical)-1], bearer: "plunk-shared-secret", wantCause: core.ErrJSONContract, wantReads: true, wantCloses: 1},
		{name: "second document cannot become one event", body: append(bytes.Clone(canonical), []byte("{}")...), bearer: "plunk-shared-secret", wantCause: core.ErrJSONContract, wantReads: true, wantCloses: 1},
		{name: "read failure closes once and withdraws all facts", body: canonical, bearer: "plunk-shared-secret", readErr: syscall.EIO, wantCause: syscall.EIO, wantReads: true, wantCloses: 1},
		{name: "close failure withdraws decoded facts", body: canonical, bearer: "plunk-shared-secret", closeErr: syscall.ENOSPC, wantCause: syscall.ENOSPC, wantReads: true, wantCloses: 1},
		{name: "custom null is structural data with no business acceptance", body: []byte("null"), bearer: "plunk-shared-secret", wantReads: true, wantCloses: 1},
		{name: "empty object is structural data with no business acceptance", body: []byte("{}"), bearer: "plunk-shared-secret", wantReads: true, wantCloses: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver, secret := plunkWebhookTestReceiver(t)
			if err := secret.Close(); err != nil {
				t.Fatal(err)
			} // receiver owns its credential copy
			defer func() {
				if err := receiver.Close(); err != nil {
					t.Errorf("receiver close=%v,want nil", err)
				}
			}()
			body := &webhookBodyLifetime{source: bytes.NewReader(tc.body), readErr: tc.readErr, closeErr: tc.closeErr}
			call := webhookJSONCall(t, body, len(tc.body), tc.bearer)
			at := temporal.InstantFromNanoseconds(1)
			got, err := ReceiveWebhookJSON[webhookApplicationInput](receiver, WebhookJSONRequest{Call: call, ObservedAt: at, Limits: core.ExtensibleJSONLimits()})
			if !errors.Is(err, tc.wantCause) {
				t.Fatalf("webhook error=%v,want %v", err, tc.wantCause)
			}
			if tc.wantCause != nil {
				if got != (WebhookDocument[webhookApplicationInput]{}) || !errors.Is(err, core.ErrPlunkContract) {
					t.Fatalf("refusal=%+v/%v,want zero and SDK identity", got, err)
				}
				if strings.Contains(err.Error(), "private-member") || strings.Contains(err.Error(), "private-value") {
					t.Fatal("application body leaked through diagnostic")
				}
			} else if got.Body != tc.wantBody || got.Observation.ObservedAt != at || got.Observation.Bytes.Uint64() != uint64(len(tc.body)) {
				t.Fatalf("typed body/observation=%+v,want exact caller facts", got)
			}
			if body.closes != tc.wantCloses || (body.reads > 0) != tc.wantReads || receiver.Validate() != nil {
				t.Fatalf("lifetime reads=%d closes=%d receiver=%v,want read=%t close=%d and borrowed receiver unchanged", body.reads, body.closes, receiver.Validate(), tc.wantReads, tc.wantCloses)
			}
			if tc.wantCloses == 0 {
				if err := body.Close(); err != nil {
					t.Fatal(err)
				}
			} // caller still owns pre-auth body
		})
	}
}

func TestWebhookJSONCallerExtentAndCancellation(t *testing.T) {
	t.Parallel()
	wire := webhookApplicationWire(t)
	for _, delta := range []int{-1, 0, 1} {
		receiver, secret := plunkWebhookTestReceiver(t)
		body := &webhookBodyLifetime{source: bytes.NewReader(wire)}
		limits := core.ExtensibleJSONLimits()
		var err error
		limits.DocumentMaximumBytes, err = core.NewByteCount(uint64(len(wire) + delta))
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReceiveWebhookJSON[webhookApplicationInput](receiver, WebhookJSONRequest{Call: webhookJSONCall(t, body, len(wire), "plunk-shared-secret"), ObservedAt: temporal.InstantFromNanoseconds(1), Limits: limits})
		if delta < 0 {
			if !errors.Is(err, core.ErrExchangeBodyLimit) || got != (WebhookDocument[webhookApplicationInput]{}) {
				t.Fatalf("byte limit refusal=%+v/%v,want zero/bodylimit", got, err)
			}
		} else if err != nil || got.Body.Name != "application-fixture" {
			t.Fatalf("byte boundary=%+v/%v,want exact body", got, err)
		}
		if body.closes != 1 || receiver.Validate() != nil {
			t.Fatal("bounded operation changed borrowed receiver or failed close")
		}
		if err := errors.Join(receiver.Close(), secret.Close()); err != nil {
			t.Fatal(err)
		}
	}
	receiver, secret := plunkWebhookTestReceiver(t)
	defer func() { _ = errors.Join(receiver.Close(), secret.Close()) }()
	body := &webhookBodyLifetime{source: bytes.NewReader(wire)}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	call, err := webhookJSONCall(t, body, len(wire), "plunk-shared-secret").WithContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReceiveWebhookJSON[webhookApplicationInput](receiver, WebhookJSONRequest{Call: call, ObservedAt: temporal.InstantFromNanoseconds(1), Limits: core.ExtensibleJSONLimits()})
	if !errors.Is(err, context.Canceled) || got != (WebhookDocument[webhookApplicationInput]{}) || body.reads != 0 || body.closes != 0 || receiver.Validate() != nil {
		t.Fatalf("pre-cancelled caller=%+v/%v reads%d closes%d,want zero/unread borrowed lifetime", got, err, body.reads, body.closes)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
}

func FuzzWebhookJSONTypedCustomBody(f *testing.F) {
	f.Add(webhookApplicationWire(f))
	for _, raw := range []string{`null`, `{}`, `{"private-member":"value"}`, `{"name":"x"} {}`, `[`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		receiver, secret := plunkWebhookTestReceiver(t)
		defer func() { _ = errors.Join(receiver.Close(), secret.Close()) }()
		body := &webhookBodyLifetime{source: bytes.NewReader(raw)}
		at := temporal.InstantFromNanoseconds(1)
		got, err := ReceiveWebhookJSON[webhookApplicationInput](receiver, WebhookJSONRequest{Call: webhookJSONCall(t, body, len(raw), "plunk-shared-secret"), ObservedAt: at, Limits: core.ExtensibleJSONLimits()})
		if receiver.Validate() != nil || body.closes != 1 {
			t.Fatal("receive changed borrowed capability or failed owned body close")
		}
		if err != nil {
			if !errors.Is(err, core.ErrPlunkContract) || got != (WebhookDocument[webhookApplicationInput]{}) {
				t.Fatal("webhook refusal lost zero/SDK identity")
			}
			return
		}
		var independent webhookApplicationInput
		if err := json.Unmarshal(raw, &independent, json.RejectUnknownMembers(true)); err != nil || independent != got.Body || got.Observation.Bytes.Uint64() != uint64(len(raw)) || got.Observation.ObservedAt != at {
			t.Fatal("authenticated typed facts disagree with native JSON/source extent")
		}
		canonical, err := json.Marshal(got.Body)
		if err != nil {
			t.Fatal(err)
		}
		var round webhookApplicationInput
		if err := json.Unmarshal(canonical, &round); err != nil || round != got.Body {
			t.Fatal("custom body canonical roundtrip changed facts")
		}
		second, err := json.Marshal(round)
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatal("custom body second canonical write changed")
		}
	})
}
