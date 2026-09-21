package upgradereport

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkSignedRequestVerification(b *testing.B) {
	request, _, keys, authority := reportFixture(b)
	if err := request.Verify(authority, keys); err != nil {
		b.Fatalf("genuine signed observation = %v, want authenticated", err)
	}
	forged := request
	forged.Payload.Outcome = Cancelled
	if forged == request {
		b.Fatalf("outcome mutation = %v, want a changed signed observation", forged)
	}
	if err := forged.Verify(authority, keys); !errors.Is(err, core.ErrReportAuthentication) {
		b.Fatalf("changed signed observation = %v, want authentication refusal", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := request.Verify(authority, keys); err != nil {
			b.Fatalf("verify fixed signed observation = %v, want nil", err)
		}
	}
}
