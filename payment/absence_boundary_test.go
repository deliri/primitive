package payment

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"testing"
)

func TestPaymentServicePeriodBoundsLayerTriad(t *testing.T) {
	t.Parallel()
	at := temporal.InstantFromNanoseconds
	for _, tc := range []struct {
		name   string
		period ServicePeriod
		want   error
	}{
		{name: "positive smallest interval", period: ServicePeriod{Start: at(0), End: at(1)}},
		{name: "positive pre-epoch interval", period: ServicePeriod{Start: at(-2), End: at(-1)}},
		{name: "positive crosses epoch", period: ServicePeriod{Start: at(-1), End: at(1)}},
		{name: "negative absent start", period: ServicePeriod{End: at(1)}, want: core.ErrPaymentContract},
		{name: "negative absent end", period: ServicePeriod{Start: at(1)}, want: core.ErrPaymentContract},
		{name: "negative reversed interval", period: ServicePeriod{Start: at(1), End: at(0)}, want: core.ErrPaymentContract},
		{name: "neutral zero duration has no bounds", period: ServicePeriod{Start: at(0), End: at(0)}, want: core.ErrPaymentContract},
		{name: "neutral absent period has no bounds", want: core.ErrPaymentContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.period.Bounds()
			want := temporal.IntervalBounds{Start: tc.period.Start, End: tc.period.End}
			if tc.want != nil {
				want = temporal.IntervalBounds{}
			}
			if !errors.Is(err, tc.want) || got != want {
				t.Fatalf("bounds = (%v,%v), want (%v,%v)", got, err, want, tc.want)
			}
		})
	}
}

func TestPaymentJSONAbsentReceiverLayerTriad(t *testing.T) {
	t.Parallel()
	f := paymentFixturesForFuzz(t)
	paymentJSONAbsent[PaymentID](t, "identity", f.paymentID)
	paymentJSONAbsent[SigningDomain](t, "domain", f.signingDomain)
	paymentJSONAbsent[Payload](t, "payload", f.payload)
	paymentJSONAbsent[Document](t, "document", f.document)
	paymentJSONAbsent[QueryPayload](t, "query_payload", f.queryPayload)
	paymentJSONAbsent[QueryDocument](t, "query_document", f.queryDocument)
	paymentJSONAbsent[QueryCommitment](t, "commitment", f.queryCommitment)
	paymentJSONAbsent[Cursor](t, "cursor", f.cursor)
	paymentJSONAbsent[CatalogPayload](t, "catalog_payload", f.catalogPayload)
	paymentJSONAbsent[CatalogDocument](t, "catalog_document", f.catalogDocument)
}

func paymentJSONAbsent[T paymentJSONValue, P paymentJSONReceiver[T]](t *testing.T, name string, seed T) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		canonical, err := seed.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name               string
			nilReceiver, empty bool
			want               error
		}{
			{name: "positive present receiver decodes exact value"},
			{name: "negative absent receiver refuses valid value", nilReceiver: true, want: core.ErrJSONContract},
			{name: "neutral no value preserves unset receiver", empty: true, want: core.ErrJSONContract},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				var value T
				receiver := P(&value)
				if tc.nilReceiver {
					receiver = nil
				}
				input := canonical
				if tc.empty {
					input = nil
				}
				err := receiver.UnmarshalJSON(input)
				if !errors.Is(err, tc.want) {
					t.Fatalf("decode error = %v, want %v", err, tc.want)
				}
				encoded, encodeErr := value.MarshalJSON()
				if tc.want == nil {
					if encodeErr != nil || string(encoded) != string(canonical) {
						t.Fatalf("decode projection = (%q,%v), want %q", encoded, encodeErr, canonical)
					}
				} else if !errors.Is(encodeErr, core.ErrJSONContract) || encoded != nil {
					t.Fatalf("absent value emitted (%q,%v), want nil and JSON refusal", encoded, encodeErr)
				}
			})
		}
	})
}
