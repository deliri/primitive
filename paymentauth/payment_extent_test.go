package paymentauth

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestPaymentQueryExtentLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newPaymentQueryFixture(t, standardPaymentQueryFixtureRequest(t))
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	const window = 2 << 20
	padded := paymentQueryJSONAtLength(t, canonical, window)
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{name: "positive valid request crosses many windows", data: padded},
		{name: "negative second object beyond many windows", data: append(bytes.Clone(padded), []byte("{}")...), want: core.ErrJSONContract},
		{name: "negative truncated request after many windows", data: padded[:len(padded)-1], want: core.ErrJSONContract},
		{name: "neutral whitespace contains no request", data: bytes.Repeat([]byte(" "), window), want: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fixture.document
			err := got.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.want) || got != fixture.document {
				t.Fatalf("decode = (%v,%v), want exact receiver and %v", got, err, tc.want)
			}
			if tc.want == nil {
				verified, err := Verify(Verification{Server: fixture.server, Document: got})
				if err != nil {
					t.Fatal(err)
				}
				payload, err := verified.Payload()
				if err != nil || payload != fixture.payload {
					t.Fatalf("authenticated payload differs: %v", err)
				}
			}
		})
	}
}
