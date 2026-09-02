package plunk

import "testing"

func BenchmarkParseCredential(b *testing.B) {
	value := []byte("sk_test_1234567890")
	b.ReportAllocs()
	for b.Loop() {
		credential, err := ParseCredential(value)
		if err != nil {
			b.Fatal(err)
		}
		_ = credential.Close()
	}
}
