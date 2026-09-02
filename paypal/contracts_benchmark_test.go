package paypal

import "testing"

func BenchmarkParseAccessToken(b *testing.B) {
	value := []byte("A21AA-test-token")
	b.ReportAllocs()
	for b.Loop() {
		token, err := ParseAccessToken(value)
		if err != nil {
			b.Fatal(err)
		}
		_ = token.Close()
	}
}
