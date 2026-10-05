package cloudflare

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Arithmetic only: no files, allocations proportional to bytes, or transfers.
func TestR2MultipartLayoutConservesExactExtent(t *testing.T) {
	t.Parallel()
	minimum := core.CloudflareR2MultipartMinimumPartBytes
	maximum := core.CloudflareR2MultipartMaximumObjectBytes
	for _, tc := range []struct {
		name             string
		total, preferred uint64
		wantErr          bool
	}{
		{name: "one byte final part", total: 1, preferred: minimum},
		{name: "short final part", total: minimum - 1, preferred: minimum},
		{name: "exact one part", total: minimum, preferred: minimum},
		{name: "one byte second part", total: minimum + 1, preferred: minimum},
		{name: "exact equal parts", total: minimum * 2, preferred: minimum},
		{name: "part count at limit", total: minimum * 10000, preferred: minimum},
		{name: "part count needs wider extent", total: minimum*10000 + 1, preferred: minimum},
		{name: "below provider object ceiling", total: maximum - 1, preferred: minimum},
		{name: "exact provider object ceiling", total: maximum, preferred: minimum},
		{name: "largest requested part extent", total: maximum, preferred: core.CloudflareR2MultipartMaximumPartBytes},
		{name: "empty object refused", preferred: minimum, wantErr: true},
		{name: "object beyond provider ceiling refused", total: maximum + 1, preferred: minimum, wantErr: true},
		{name: "largest admitted byte length refused before arithmetic", total: 1<<63 - 1, preferred: minimum, wantErr: true},
		{name: "zero part refused", total: 1, wantErr: true},
		{name: "part below minimum refused", total: 1, preferred: minimum - 1, wantErr: true},
		{name: "part beyond provider ceiling refused", total: 1, preferred: core.CloudflareR2MultipartMaximumPartBytes + 1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total, err := core.NewByteLength(tc.total)
			if err != nil {
				t.Fatal(err)
			}
			preferred, err := core.NewByteLength(tc.preferred)
			if err != nil {
				t.Fatal(err)
			}
			got, err := PlanR2Multipart(total, preferred)
			if tc.wantErr {
				if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2MultipartLayout{}) {
					t.Fatalf("plan=%+v/%v, want zero/binding refusal", got, err)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.PartBytes.Uint64() < tc.preferred {
				t.Fatalf("plan=%+v/%v, want valid layout respecting preferred extent", got, err)
			}
			var end uint64
			for part := uint16(1); part <= got.Parts; part++ {
				offset, length, err := got.PartExtent(part)
				if err != nil || offset != end || length.Uint64() == 0 || (part < got.Parts && length != got.PartBytes) {
					t.Fatalf("part=%d offset=%d length=%d err=%v, want contiguous uniform non-final parts", part, offset, length.Uint64(), err)
				}
				end += length.Uint64()
			}
			if end != tc.total {
				t.Fatalf("covered=%d, want %d", end, tc.total)
			}
			for _, outside := range []uint16{0, got.Parts + 1} {
				offset, length, err := got.PartExtent(outside)
				if !errors.Is(err, core.ErrCloudflareBinding) || offset != 0 || length != (core.ByteLength{}) {
					t.Fatalf("outside part=%d gives %d/%v/%v, want zero/refusal", outside, offset, length, err)
				}
			}
			changed := got
			changed.Parts++
			if !errors.Is(changed.Validate(), core.ErrCloudflareBinding) {
				t.Fatal("changed count accepted, want refusal")
			}
		})
	}
}
