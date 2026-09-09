package objectstore

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestScalarJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	// One canonical digest token is shared only as a syntax fixture. Each door
	// owns its typed preexisting value and must preserve it on every refusal.
	canonical := append([]byte{'"'}, bytes.Repeat([]byte{'a'}, 2*BLAKE3DigestBytes)...)
	canonical = append(canonical, '"')
	for _, tc := range []struct {
		name string
		data func() []byte
		want error
	}{
		{name: "minimum canonical token", data: func() []byte { return bytes.Clone(canonical) }},
		{name: "one below shared document ceiling", data: func() []byte {
			return append(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes-len(canonical)-1), canonical...)
		}},
		{name: "exact shared document ceiling", data: func() []byte {
			return append(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes-len(canonical)), canonical...)
		}},
		{name: "one byte beyond shared document ceiling", data: func() []byte {
			return append(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes-len(canonical)+1), canonical...)
		}, want: core.ErrObjectStoreSize},
		{name: "absent token cannot clear prior identity", data: func() []byte { return nil }, want: core.ErrObjectStoreContract},
		{name: "trailing token cannot replace prior identity", data: func() []byte { return append(bytes.Clone(canonical), canonical...) }, want: core.ErrObjectStoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := tc.data()
			for _, door := range []struct {
				name string
				run  func([]byte) (bool, error)
			}{
				{name: "BLAKE3", run: func(data []byte) (bool, error) {
					before := NewBLAKE3Digest([BLAKE3DigestBytes]byte{1})
					got := before
					err := got.UnmarshalJSON(data)
					if err != nil {
						return got == before, err
					}
					var want [BLAKE3DigestBytes]byte
					for i := range want {
						want[i] = 0xaa
					}
					return got == NewBLAKE3Digest(want), got.Validate()
				}},
				{name: "upload commitment", run: func(data []byte) (bool, error) {
					before, err := newUploadCapabilityCommitment(core.NewSHA256Digest([core.SHA256DigestBytes]byte{1}))
					if err != nil {
						t.Fatal(err)
					}
					got := before
					err = got.UnmarshalJSON(data)
					if err != nil {
						return got == before, err
					}
					var want [core.SHA256DigestBytes]byte
					for i := range want {
						want[i] = 0xaa
					}
					return got.digest == core.NewSHA256Digest(want), got.Validate()
				}},
				{name: "download commitment", run: func(data []byte) (bool, error) {
					before, err := newDownloadCapabilityCommitment(core.NewSHA256Digest([core.SHA256DigestBytes]byte{1}))
					if err != nil {
						t.Fatal(err)
					}
					got := before
					err = got.UnmarshalJSON(data)
					if err != nil {
						return got == before, err
					}
					var want [core.SHA256DigestBytes]byte
					for i := range want {
						want[i] = 0xaa
					}
					return got.digest == core.NewSHA256Digest(want), got.Validate()
				}},
			} {
				match, err := door.run(data)
				if !match || !errors.Is(err, tc.want) || (tc.want != nil && !errors.Is(err, core.ErrObjectStoreContract)) {
					t.Errorf("%s exact identity/preservation = %t, error=%v; want true and %v", door.name, match, err, tc.want)
				}
			}
		})
	}
}
