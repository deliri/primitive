package tailnet

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"net/netip"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

type refusingIdentity struct {
	calls int
	cause error
}

func (s *refusingIdentity) Identity(context.Context, googleidentity.Audience) (googleidentity.Token, error) {
	s.calls++
	return googleidentity.Token{}, s.cause
}

func TestClientEnrollmentLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive construction owns transport without enrollment", func(t *testing.T) {
		t.Parallel()
		identity := &refusingIdentity{cause: tailnetconfig.ErrContract}
		client, err := NewClient(fixtureConfiguration(t), identity)
		if err != nil {
			t.Fatalf("NewClient() = %v, want nil", err)
		}
		if _, err := client.Exchange(); err != nil {
			t.Fatalf("Exchange() = %v, want nil", err)
		}
		if identity.calls != 0 {
			t.Fatalf("enrollments = %d, want 0", identity.calls)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("Close() = %v, want nil", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("Close(replay) = %v, want nil", err)
		}
		if _, err := client.dial(t.Context(), "tcp", client.configuration.Destination.String()); !errors.Is(err, ErrClosed) {
			t.Fatalf("dial(closed) = %v, want %v", err, ErrClosed)
		}
	})
	t.Run("negative refusal retains cause and permits explicit next attempt", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("identity unavailable")
		identity := &refusingIdentity{cause: cause}
		client, err := NewClient(fixtureConfiguration(t), identity)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}
		}()
		for attempt := 1; attempt <= 2; attempt++ {
			conn, err := client.dial(t.Context(), "tcp", client.configuration.Destination.String())
			if conn != nil || !errors.Is(err, ErrEnrollment) || !errors.Is(err, cause) || identity.calls != attempt {
				t.Fatalf("dial(attempt %d) = (%v,%v,%d calls), want nil, typed refusal, %d", attempt, conn, err, identity.calls, attempt)
			}
			if client.server != nil {
				t.Fatal("refused identity retained server, want none")
			}
		}
	})
	t.Run("neutral cancellation and foreign destinations have no enrollment", func(t *testing.T) {
		t.Parallel()
		identity := &refusingIdentity{cause: tailnetconfig.ErrContract}
		client, err := NewClient(fixtureConfiguration(t), identity)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}
		}()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := client.dial(ctx, "tcp", client.configuration.Destination.String()); !errors.Is(err, context.Canceled) {
			t.Fatalf("dial(cancelled) = %v, want cancelled", err)
		}
		if _, err := client.dial(t.Context(), "tcp", "100.84.35.45:8088"); !errors.Is(err, ErrDestination) {
			t.Fatalf("dial(other host) = %v, want %v", err, ErrDestination)
		}
		if _, err := client.dial(t.Context(), "udp", client.configuration.Destination.String()); !errors.Is(err, ErrDestination) {
			t.Fatalf("dial(other protocol) = %v, want %v", err, ErrDestination)
		}
		if identity.calls != 0 {
			t.Fatalf("enrollments = %d, want 0", identity.calls)
		}
	})
}

func fixtureConfiguration(t testing.TB) tailnetconfig.Configuration {
	t.Helper()
	path, err := core.ParseAbsolutePath("/tmp/primitive-tailnet-test-unused")
	if err != nil {
		t.Fatal(err)
	}
	audience, err := googleidentity.ParseAudience("api.tailscale.com/fixture-client")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(30)
	if err != nil {
		t.Fatal(err)
	}
	return tailnetconfig.Configuration{ClientID: "fixture-client", Hostname: "blink-api", Tag: "tag:blink-api", Audience: audience, StateDirectory: path, Destination: netip.MustParseAddrPort("100.84.35.44:8088"), StartupTimeout: timeout}
}
