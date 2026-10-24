package nm

import "testing"

// BenchmarkCustom suppresses ns/op and reports only a custom metric, as
// testing.B.ReportMetric permits; its native row carries no ns/op.
func BenchmarkCustom(b *testing.B) {
	for b.Loop() {
	}
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(5, "widgets/op")
}

// BenchmarkCountOnly suppresses ns/op and reports nothing else: the native
// driver then prints only the name and its iteration count.
func BenchmarkCountOnly(b *testing.B) {
	for b.Loop() {
	}
	b.ReportMetric(0, "ns/op")
}
