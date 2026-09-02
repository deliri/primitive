package twilio

import "testing"

func BenchmarkParseAccountSID(b *testing.B) {
	value := "AC0123456789abcdefABCDEF0123456789"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ParseAccountSID(value); err != nil {
			b.Fatal(err)
		}
	}
}
