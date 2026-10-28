package stripe

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzDecodeCheckoutSessionSemanticClosure(f *testing.F) {
	fixture := checkoutFixture()
	f.Add(checkoutFixtureBytes(f, fixture))
	fixture.AmountTotal, fixture.Currency, fixture.Status = nil, nil, nil
	f.Add(checkoutFixtureBytes(f, fixture))
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`{"amount_total":0,"amount_total":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(data), 4096))
		if err != nil {
			if !errors.Is(err, core.ErrStripeContract) || !checkoutSessionZero(got) {
				t.Fatalf("DecodeCheckoutSession(rejected) = (%+v, %v), want exact zero and Stripe identity", got, err)
			}
			return
		}
		if len(data) > 4096 || got.Validate() != nil {
			t.Fatalf("DecodeCheckoutSession(accepted) = (%+v, %d bytes), want validated bounded facts", got, len(data))
		}
		// Independent whole-value standard-library representations observe
		// presence and typed scalar values; production uses the token cursor.
		var members map[string]stdjson.RawMessage
		var wire checkoutSessionFixture
		if err := stdjson.Unmarshal(data, &members); err != nil {
			t.Fatalf("stdlib member decode error = %v, want nil for admitted input", err)
		}
		if err := stdjson.Unmarshal(data, &wire); err != nil {
			t.Fatalf("stdlib typed decode error = %v, want nil for admitted input", err)
		}
		for _, name := range []string{"id", "object", "livemode", "status", "payment_status", "mode", "amount_total", "currency", "customer", "subscription", "metadata"} {
			if _, present := members[name]; !present {
				t.Fatalf("admitted member %s absent, want required presence", name)
			}
		}
		live := bytes.TrimSpace(members["livemode"])
		if !bytes.Equal(live, []byte("true")) && !bytes.Equal(live, []byte("false")) {
			t.Fatalf("admitted livemode = %q, want JSON boolean literal", live)
		}
		wantStatus := "null"
		if wire.Status != nil {
			wantStatus = string(*wire.Status)
		}
		if got.ID != wire.ID || got.Object != wire.Object || got.Live != wire.Live || got.Status.String() != wantStatus || got.PaymentStatus.String() != string(wire.PaymentStatus) || got.Mode.String() != string(wire.Mode) || got.Metadata != wire.Metadata || !checkoutPointerEqual(got.AmountTotal, wire.AmountTotal) || !checkoutPointerEqual(got.Currency, wire.Currency) || !checkoutPointerEqual(got.Customer, wire.Customer) || !checkoutPointerEqual(got.Subscription, wire.Subscription) {
			t.Fatalf("admitted facts = %+v, want exact independent wire %+v", got, wire)
		}
		canonical := checkoutFixtureBytes(t, wire)
		round, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(canonical), uint64(len(canonical))))
		if err != nil || round.ID != got.ID || round.Object != got.Object || round.Live != got.Live || round.Status != got.Status || round.PaymentStatus != got.PaymentStatus || round.Mode != got.Mode || round.Metadata != got.Metadata || !checkoutPointerEqual(round.AmountTotal, got.AmountTotal) || !checkoutPointerEqual(round.Currency, got.Currency) || !checkoutPointerEqual(round.Customer, got.Customer) || !checkoutPointerEqual(round.Subscription, got.Subscription) {
			t.Fatalf("canonical decode = (%+v, %v), want same admitted facts %+v", round, err, got)
		}
		second := checkoutFixtureBytes(t, wire)
		if !bytes.Equal(canonical, second) {
			t.Fatalf("second canonical bytes = %q, want %q", second, canonical)
		}
	})
}
