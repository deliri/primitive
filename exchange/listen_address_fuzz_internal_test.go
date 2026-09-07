package exchange

import (
	"errors"
	"math"
	"net"
	"net/netip"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzParseListenAddressSemanticClosure(f *testing.F) {
	// Trusted typed seeds are validated and projected through production
	// String. The public parser runs only inside the callback, so refusing
	// every parse cannot hide as a seed-construction failure.
	seeds := []ListenAddress{
		{value: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 8080)},
		{value: netip.AddrPortFrom(netip.IPv6Loopback(), math.MaxUint16)},
	}
	for _, seed := range seeds {
		if err := seed.Validate(); err != nil {
			f.Fatalf("typed listen seed validation = %v, want nil", err)
		}
		f.Add(seed.String())
	}
	valid := seeds[0]
	f.Add("")
	f.Add(netip.AddrPortFrom(valid.value.Addr(), 0).String())
	f.Add("0.0.0.0:0")
	f.Add("localhost:8080")
	f.Add("[::ffff:0.0.0.0]:8080")
	f.Add("[::%lo0]:8080")
	f.Add("[::ffff:0.0.0.0%lo0]:8080")
	f.Add("127.0.0.1:65536")
	f.Add(" " + valid.String())
	f.Add(valid.String() + "/path")

	f.Fuzz(func(t *testing.T, text string) {
		const oracleMaximumBytes = 4096
		if len(text) > oracleMaximumBytes {
			return
		}
		// Go netip owns the literal grammar; net.IP independently supplies
		// the listener's wildcard semantics, including mapped and zoned IPs.
		standard, standardErr := netip.ParseAddrPort(text)
		wantAdmitted := standardErr == nil && !net.IP(standard.Addr().AsSlice()).IsUnspecified()
		var wantErr error
		if !wantAdmitted {
			wantErr = core.ErrExchangeContract
		}
		got, gotErr := ParseListenAddress(text)
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("ParseListenAddress(%q) = %v, want %v from Go grammar and wildcard interpretation", text, gotErr, wantErr)
		}
		if !wantAdmitted {
			if got != (ListenAddress{}) {
				t.Fatalf("ParseListenAddress(%q) = %v, want zero after refusal", text, got)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("ParseListenAddress(%q).Validate() error = %v, want nil", text, err)
		}
		canonical := got.String()
		if canonical != standard.String() {
			t.Fatalf("canonical address = %q, want exact Go address %q", canonical, standard.String())
		}
		roundTrip, roundTripErr := ParseListenAddress(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("listen address canonical closure = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
	})
}
