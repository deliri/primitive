package exchange

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// BenchmarkListenAddressAdmission measures literal parsing, policy admission,
// validation, and exact canonical projection. It performs no DNS lookup or
// listen effect. Rejected inputs must return a zero capability; an older
// revision that admits a wildcard fails rather than reporting a useful speed.
func BenchmarkListenAddressAdmission(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name     string
		input    string
		wantText string
		wantErr  error
	}{
		{name: "concrete_ipv4", input: "127.0.0.1:65535", wantText: "127.0.0.1:65535"},
		{name: "concrete_mapped_ipv4", input: "[::ffff:127.0.0.1]:8080", wantText: "[::ffff:127.0.0.1]:8080"},
		{name: "mapped_wildcard", input: "[::ffff:0.0.0.0]:8080", wantErr: core.ErrExchangeContract},
		{name: "zoned_wildcard", input: "[::%lo0]:8080", wantErr: core.ErrExchangeContract},
		{name: "zoned_mapped_wildcard", input: "[::ffff:0.0.0.0%lo0]:8080", wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			var got ListenAddress
			var gotErr error
			b.ReportAllocs()
			for b.Loop() {
				got, gotErr = ParseListenAddress(tc.input)
				if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.wantText {
					b.Fatalf("admission of %q = (%q, %v), want (%q, %v)", tc.input, got.String(), gotErr, tc.wantText, tc.wantErr)
				}
				if !errors.Is(got.Validate(), tc.wantErr) {
					b.Fatalf("admitted capability validation = %v, want %v", got.Validate(), tc.wantErr)
				}
				if tc.wantErr != nil && got != (ListenAddress{}) {
					b.Fatalf("refused capability = %v, want zero", got)
				}
			}
			if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.wantText {
				b.Fatalf("retained result = (%q, %v), want (%q, %v)", got.String(), gotErr, tc.wantText, tc.wantErr)
			}
		})
	}
}
