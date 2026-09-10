package exchange_test

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	testOperationTimeoutMilliseconds = 60_000
	testAttemptTimeoutMilliseconds   = 30_000
	testDeadlockBackstop             = 60 * time.Second
)

// exchangeFixtureBackstop supplies only a deadlock backstop, never proof of
// elapsed work. Like time.After it remains usable during test teardown; the
// test owns cancellation of Temporal's standard context timer.
func exchangeFixtureBackstop(t testing.TB, limit time.Duration) <-chan struct{} {
	t.Helper()
	duration, err := temporal.NewDuration(limit)
	if err != nil {
		t.Fatalf("backstop duration = %v, want nil", err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{
		Parent: context.WithoutCancel(t.Context()), Duration: duration,
	})
	if err != nil {
		t.Fatalf("Temporal backstop construction = %v, want nil", err)
	}
	t.Cleanup(cancel)
	return ctx.Done()
}

// errTransportDocumentContract is the test-owned identity the example document
// returns from Validate. It lets ingress and response tests prove a rejection
// came from the owning type's contract rather than from the JSON grammar, which
// Core's shared parent identities cannot distinguish on their own.
var errTransportDocumentContract = errors.New("transport document message is empty")

type transportDocument struct {
	Message string `json:"message"`
}

func (d transportDocument) Validate() error {
	if d.Message == "" {
		return errors.Join(core.ErrPrimitiveContract, errTransportDocumentContract)
	}
	return nil
}

func (d transportDocument) MarshalJSON() ([]byte, error) {
	type wire transportDocument
	return json.Marshal(wire(d))
}

type projectedTransportDocument struct {
	Message string          `json:"message"`
	Method  exchange.Method `json:"-"`
}

func (d projectedTransportDocument) Validate() error {
	if d.Message == "" {
		return core.ErrPrimitiveContract
	}
	return d.Method.Validate()
}

func mustHTTPStatus(t testing.TB, value int) core.HTTPStatusCode {
	t.Helper()

	var got core.HTTPStatusCode
	if gotErr := got.AdmitInt(value); gotErr != nil {
		t.Fatalf("AdmitInt(%d) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func mustByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()

	got, gotErr := core.NewByteCount(value)
	if gotErr != nil {
		t.Fatalf("NewByteCount(%d) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func mustDurationMilliseconds(t testing.TB, value uint64) temporal.Duration {
	t.Helper()

	got, gotErr := temporal.DurationFromMilliseconds(value)
	if gotErr != nil {
		t.Fatalf("DurationFromMilliseconds(%d) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func mustEndpoint(t testing.TB, value string) core.HTTPEndpoint {
	t.Helper()

	got, gotErr := core.ParseHTTPEndpoint(value)
	if gotErr != nil {
		t.Fatalf("ParseHTTPEndpoint(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func mustExchangeClient(t testing.TB, client *http.Client) exchange.Client {
	t.Helper()

	got, gotErr := exchange.NewClient(client)
	if gotErr != nil {
		t.Fatalf("exchange.NewClient() setup error = %v, want nil", gotErr)
	}
	return got
}

func singleAttemptOperationPolicy(t testing.TB) exchange.OperationPolicy {
	t.Helper()

	return exchange.OperationPolicy{
		OperationTimeout: mustDurationMilliseconds(
			t,
			testOperationTimeoutMilliseconds,
		),
		AttemptTimeout: mustDurationMilliseconds(
			t,
			testAttemptTimeoutMilliseconds,
		),
		Retry: exchange.RetryPolicy{
			MaximumAttempts: 1,
		},
		Redirect: exchange.RedirectPolicy{
			Mode: exchange.RedirectReject,
		},
	}
}

func singleAttemptStreamPolicy(t testing.TB) exchange.StreamPolicy {
	t.Helper()

	return exchange.StreamPolicy{
		OperationTimeout: mustDurationMilliseconds(
			t,
			testOperationTimeoutMilliseconds,
		),
		AttemptTimeout: mustDurationMilliseconds(
			t,
			testAttemptTimeoutMilliseconds,
		),

		Redirect: exchange.RedirectPolicy{
			Mode: exchange.RedirectReject,
		},
	}
}
