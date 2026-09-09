package tailnet

import "testing"

// BenchmarkClientLifecycle measures effect-free construction and owned closure.
// It does not measure Tailscale enrollment, network throughput, or live latency.
func BenchmarkClientLifecycle(b *testing.B) {
	configuration := fixtureConfiguration(b)
	identity := &refusingIdentity{}
	if err := configuration.Validate(); err != nil {
		b.Fatalf("configuration.Validate() = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		client, err := NewClient(configuration, identity)
		if err != nil {
			b.Fatalf("NewClient() = %v, want nil", err)
		}
		if _, err := client.Exchange(); err != nil {
			b.Fatalf("Exchange() = %v, want nil", err)
		}
		if err := client.Close(); err != nil {
			b.Fatalf("Close() = %v, want nil", err)
		}
	}
	if identity.calls != 0 {
		b.Fatalf("identity calls = %d, want zero effects", identity.calls)
	}
}
