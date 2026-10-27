package plunk

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type sendNilContext struct{ context.Context }

func TestSendContextNativeTerminalAndTypedNil(t *testing.T) {
	t.Parallel()
	var typedNil *sendNilContext
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		cause error
	}{
		{"untyped nil", nil, core.ErrNilContext},
		{"native typed nil panic contained", typedNil, core.ErrContextObservation},
		{"cancelled context preserves native cause", cancelled, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeSendResponse(tc.ctx, sendFixture(t, SentEmail{EmailID: "message-fixture", Status: SendStatusQueued}), core.ExtensibleJSONLimits())
			if !errors.Is(err, core.ErrPlunkContract) || !errors.Is(err, tc.cause) || !sameSendResponse(got, SendResponse{}) {
				t.Fatalf("unsafe context did not yield zero with exact cause: %v", err)
			}
		})
	}
}

func TestSendTimestampAbsenceEpochAndNegativeAreDistinct(t *testing.T) {
	t.Parallel()
	if _, err := NewSendTimestamp(temporal.Instant{}); !errors.Is(err, core.ErrPlunkContract) {
		t.Fatal("unset instant admitted")
	}
	for _, text := range []string{"1970-01-01T00:00:00Z", "1969-12-31T23:59:59Z", "2026-09-14T12:00:00.123Z"} {
		instant, err := temporal.ParseRFC3339(text)
		if err != nil {
			t.Fatal(err)
		}
		value, err := NewSendTimestamp(instant)
		if err != nil || value.IsZero() || value.Instant() != instant {
			t.Fatalf("observed native instant became absence: %v", err)
		}
		wire, err := value.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var decoded SendTimestamp
		if err := decoded.UnmarshalJSON(wire); err != nil || decoded != value {
			t.Fatalf("instant changed on wire: %v", err)
		}
		if err := decoded.UnmarshalJSON([]byte("null")); err != nil || !decoded.IsZero() {
			t.Fatal("null did not clear observed timestamp")
		}
		before := value
		if err := value.UnmarshalJSON([]byte(`"tomorrow"`)); !errors.Is(err, core.ErrPlunkContract) || value != before {
			t.Fatal("refused timestamp changed destination")
		}
	}
	var absent *SendTimestamp
	if err := absent.UnmarshalJSON([]byte("null")); !errors.Is(err, core.ErrPlunkContract) {
		t.Fatal("nil receiver admitted")
	}
	var status *SendStatus
	if err := status.UnmarshalJSON([]byte(`"queued"`)); !errors.Is(err, core.ErrPlunkContract) {
		t.Fatal("nil status receiver admitted")
	}
}
