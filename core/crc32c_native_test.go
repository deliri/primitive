package core

import (
	"bytes"
	"hash/crc32"
	"testing"
)

func TestCRC32COfNativeCastagnoliVectors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data []byte
		want uint32
	}{
		{name: "absent bytes have a set zero checksum", want: 0},
		{name: "empty borrowed bytes have a set zero checksum", data: []byte{}, want: 0},
		{name: "check sequence selects Castagnoli rather than IEEE", data: []byte("123456789"), want: 0xe3069283},
		{name: "zero block", data: make([]byte, 32), want: 0x8a9136aa},
		{name: "all bits set block", data: bytes.Repeat([]byte{0xff}, 32), want: 0x62a8ab43},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := bytes.Clone(tc.data)
			got := CRC32COf(tc.data)
			value, err := got.Uint32()
			if err != nil || got.Validate() != nil || value != tc.want || !bytes.Equal(tc.data, before) {
				t.Fatalf("CRC32COf(%x) = (%x, %v), source=%x, want set %x and unchanged source", before, value, err, tc.data, tc.want)
			}
		})
	}
}

// The byte window is not a source quota: native work crosses every boundary.
func TestCRC32COfBorrowedWindowConservation(t *testing.T) {
	t.Parallel()
	for _, size := range []int{32767, 32768, 32769, 4*32768 + 17} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i*131 + 17)
		}
		digest := crc32.New(crc32.MakeTable(crc32.Castagnoli))
		for start := 0; start < len(data); start += 257 {
			end := min(start+257, len(data))
			if _, err := digest.Write(data[start:end]); err != nil {
				t.Fatal(err)
			}
		}
		got, err := CRC32COf(data).Uint32()
		if err != nil || got != digest.Sum32() {
			t.Fatalf("CRC32COf(%d bytes) = (%x, %v), want incremental %x", size, got, err, digest.Sum32())
		}
	}
}
