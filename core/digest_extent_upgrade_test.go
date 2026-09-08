package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math"
	"testing"
)

// Only the counter is advanced through this internal seam: allocating or
// streaming exabytes would not be a useful test. Go's hash state remains real,
// and refused writes must leave that state and the count unchanged.
func TestDigestWriterSignedExtentAdmission(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		count     uint64
		data      []byte
		wantCount uint64
		wantErr   error
	}{
		{name: "neutral/empty stream", wantCount: 0},
		{name: "positive/first binary byte", data: []byte{0xff}, wantCount: 1},
		{name: "boundary/below signed ceiling", count: math.MaxInt64 - 2, data: []byte{0}, wantCount: math.MaxInt64 - 1},
		{name: "boundary/exact signed ceiling", count: math.MaxInt64 - 1, data: []byte{0xff}, wantCount: math.MaxInt64},
		{name: "boundary/empty write at ceiling", count: math.MaxInt64, wantCount: math.MaxInt64},
		{name: "negative/one byte over signed ceiling", count: math.MaxInt64, data: []byte{1}, wantErr: ErrNumericOverflow},
		{name: "negative/chunk crosses signed ceiling", count: math.MaxInt64 - 1, data: []byte{0, 0xff}, wantErr: ErrNumericOverflow},
		{name: "negative/empty write cannot bless corrupt signed count", count: math.MaxInt64 + 1, wantErr: ErrNumericOverflow},
		{name: "negative/write cannot extend corrupt signed count", count: math.MaxInt64 + 1, data: []byte{1}, wantErr: ErrNumericOverflow},
		{name: "boundary/below unsigned wrap is already invalid", count: math.MaxUint64 - 1, data: []byte{1}, wantErr: ErrNumericOverflow},
		{name: "negative/wrap cannot fabricate empty stream", count: math.MaxUint64, data: []byte{1}, wantErr: ErrNumericOverflow},
		{name: "negative/wrap cannot fabricate small valid count", count: math.MaxUint64, data: []byte{1, 2}, wantErr: ErrNumericOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			writer := NewDigestWriter()
			writer.count = tc.count
			beforeHash := writer.digest.Sum(nil)
			n, err := writer.Write(tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("write=%d, %v; want error %v", n, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if n != 0 || writer.count != tc.count || !bytes.Equal(writer.digest.Sum(nil), beforeHash) {
					t.Fatalf("refused write=%d, count=%d, hash unchanged=%t; want no consumption", n, writer.count, bytes.Equal(writer.digest.Sum(nil), beforeHash))
				}
				again, againErr := writer.Write(nil)
				sum, length, sealErr := writer.Seal()
				if again != 0 || !errors.Is(againErr, tc.wantErr) || !errors.Is(sealErr, tc.wantErr) || sum != (SHA256Digest{}) || length != (ByteLength{}) {
					t.Fatalf("latched write=%d, %v; seal=%v, %v, %v; want persistent refusal and zero publication", again, againErr, sum, length, sealErr)
				}
			} else {
				sum, length, sealErr := writer.Seal()
				wantSum := NewSHA256Digest(sha256.Sum256(tc.data))
				if n != len(tc.data) || sealErr != nil || length.Uint64() != tc.wantCount || sum != wantSum {
					t.Fatalf("write=%d; seal=%v, %v, %v; want %d bytes and exact Go digest %v at count %d", n, sum, length, sealErr, len(tc.data), wantSum, tc.wantCount)
				}
			}
			if err := writer.Reset(); err != nil {
				t.Fatal(err)
			}
			sum, length, err := writer.Digest()
			if err != nil || sum != NewSHA256Digest(sha256.Sum256(nil)) || length != (ByteLength{}) {
				t.Fatalf("reset=%v, %v, %v; want Go empty digest and zero count", sum, length, err)
			}
		})
	}
}
