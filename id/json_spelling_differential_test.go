package id

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestMarshalJSONMatchesCanonicalStringEncoder holds the direct quoting of
// both identity spellings byte-identical to Core's canonical string encoder
// across the extreme bit patterns, and keeps the unset value refused.
func TestMarshalJSONMatchesCanonicalStringEncoder(t *testing.T) {
	t.Parallel()
	patterns := [][identityBytes]byte{
		{0: 0x01},
		{0: 0x01, 6: 0x70, 8: 0x80},
		{0: 0x7f, 1: 0xff, 2: 0xff, 3: 0xff, 4: 0xff, 5: 0xff, 6: 0x7f, 7: 0xff, 8: 0xbf, 9: 0xff, 10: 0xff, 11: 0xff, 12: 0xff, 13: 0xff, 14: 0xff, 15: 0xff},
		{0: 0x01, 6: 0x7a, 7: 0xbc, 8: 0x9d, 15: 0xef},
	}
	for _, pattern := range patterns {
		if ulid, err := NewULIDFromBytes(pattern); err == nil {
			want, wantErr := core.MarshalCanonicalJSONString(ulid.String())
			got, gotErr := ulid.MarshalJSON()
			if wantErr != nil || gotErr != nil || !bytes.Equal(got, want) {
				t.Fatalf("ULID(%x).MarshalJSON() = %q, %v; want %q, %v", pattern, got, gotErr, want, wantErr)
			}
		}
		spelling := fmt.Sprintf("%x-%x-%x-%x-%x", pattern[0:4], pattern[4:6], pattern[6:8], pattern[8:10], pattern[10:16])
		if uuid, err := ParseUUIDv7(spelling); err == nil {
			want, wantErr := core.MarshalCanonicalJSONString(uuid.String())
			got, gotErr := uuid.MarshalJSON()
			if wantErr != nil || gotErr != nil || !bytes.Equal(got, want) || uuid.String() != spelling {
				t.Fatalf("UUIDv7(%x).MarshalJSON() = %q, %v; want %q, %v", pattern, got, gotErr, want, wantErr)
			}
		}
	}
	for name, marshal := range map[string]func() ([]byte, error){"ULID": ULID{}.MarshalJSON, "UUIDv7": UUIDv7{}.MarshalJSON} {
		if got, err := marshal(); got != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("unset %s MarshalJSON() = %q, %v; want nil, %v", name, got, err, core.ErrJSONContract)
		}
	}
}
