package cloudflare

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func BenchmarkParseStreamSignature(b *testing.B) {
	mac := hmac.New(sha256.New, []byte("benchmark stream webhook secret"))
	mac.Write([]byte(`1700000000.{"uid":"video"}`))
	digest := mac.Sum(nil)
	header := "time=1700000000,sig1=" + hex.EncodeToString(digest)
	b.ReportAllocs()
	var got streamSignature
	var err error
	for b.Loop() {
		got, err = parseStreamSignature(header)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.timestamp != "1700000000" || !bytes.Equal(got.digest[:], digest) {
		b.Fatalf("signature = %q/%x, want 1700000000/%x", got.timestamp, got.digest, digest)
	}
}

func BenchmarkReadImageInfo(b *testing.B) {
	want := imageInfoFixture(b)
	wire := imageInfoBytes(b, want)
	source := bytes.NewReader(wire)
	b.SetBytes(int64(len(wire)))
	b.ReportAllocs()
	var got ImageInfo
	var err error
	for b.Loop() {
		source.Reset(wire)
		got, err = ReadImageInfo(source)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("image info = %+v, want %+v", got, want)
	}
}

func BenchmarkImageResizeRequestAddress(b *testing.B) {
	request := imageResizeFixture(b)
	want, err := request.Address()
	if err != nil || want.String() == "" {
		b.Fatalf("fixture address = %q, %v; want a projected address", want.String(), err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := request.Address()
		if err != nil {
			b.Fatal(err)
		}
		want = got
	}
	if want.String() == "" {
		b.Fatal("address vanished after timing")
	}
}
