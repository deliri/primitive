package timematch

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppointmentLifetimeRefusesUnboundRefresh(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		lifetime Lifetime
		access   Access
		wantErr  error
	}{
		{name: "appointment refresh retains managed event capability", lifetime: LifetimeAppointment, access: AccessManageEvents},
		{name: "business refresh preserves its existing read-only capability", lifetime: LifetimeBusiness, access: AccessAvailability},
		{name: "business refresh preserves its existing managed capability", lifetime: LifetimeBusiness, access: AccessManageEvents},
		{name: "one-shot visitor cannot gain refresh authority", lifetime: LifetimeVisitor, access: AccessManageEvents, wantErr: core.ErrTimeMatchContract},
		{name: "unset lifetime cannot infer business access", access: AccessManageEvents, wantErr: core.ErrTimeMatchContract},
		{name: "future lifetime cannot infer business access", lifetime: LifetimeAppointment + 1, access: AccessManageEvents, wantErr: core.ErrTimeMatchContract},
		{name: "maximum lifetime cannot infer business access", lifetime: Lifetime(255), access: AccessManageEvents, wantErr: core.ErrTimeMatchContract},
		{name: "appointment cannot become retained availability-only access", lifetime: LifetimeAppointment, access: AccessAvailability, wantErr: core.ErrTimeMatchContract},
		{name: "appointment rejects absent event authority", lifetime: LifetimeAppointment, wantErr: core.ErrTimeMatchContract},
		{name: "appointment rejects future event authority", lifetime: LifetimeAppointment, access: AccessManageEvents + 1, wantErr: core.ErrTimeMatchContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := RefreshRequest{Application: application(t, ProviderGoogle, MicrosoftTenantInvalid), RefreshToken: secret(t, "retained-fixture"), ObservedAt: at(t, "2026-11-02T10:00:00Z"), Policy: fixturePolicy(t), Access: tc.access, Lifetime: tc.lifetime}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("refresh lifetime/access validation error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
