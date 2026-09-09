package tailnetconfig_test

import "testing"

func BenchmarkConfigurationValidate(b *testing.B) {
	configuration := fixtureConfiguration(b)
	if err := configuration.Validate(); err != nil {
		b.Fatalf("fixture.Validate() = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := configuration.Validate(); err != nil {
			b.Fatalf("Validate() = %v, want nil", err)
		}
	}
}
