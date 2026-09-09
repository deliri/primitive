package tailnet

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

func TestClientAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		identity IdentitySource
		wantErr  error
	}{
		{name: "explicit refusing source is admitted without acquisition", identity: &refusingIdentity{}, wantErr: nil},
		{name: "absent source cannot create a capability", wantErr: core.ErrTailnetContract},
		{name: "typed nil source cannot become a deferred panic", identity: (*refusingIdentity)(nil), wantErr: core.ErrTailnetContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewClient(fixtureConfiguration(t), tc.identity)
			if client != nil {
				t.Cleanup(func() {
					if err := client.Close(); err != nil {
						t.Errorf("Close() = %v, want nil", err)
					}
				})
			}
			if !errors.Is(err, tc.wantErr) || (client == nil) != (tc.wantErr != nil) {
				t.Fatalf("NewClient() = (%p,%v), want presence=%t and %v", client, err, tc.wantErr == nil, tc.wantErr)
			}
		})
	}
}

func TestExchangeCapabilityLifetime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(*testing.T) *Client
		wantErr error
	}{
		{name: "nil receiver refuses capability", setup: func(*testing.T) *Client { return nil }, wantErr: core.ErrTailnetContract},
		{name: "zero receiver refuses capability with owning identity", setup: func(*testing.T) *Client { return new(Client) }, wantErr: core.ErrTailnetContract},
		{name: "closed receiver cannot issue a fresh usable capability", setup: func(t *testing.T) *Client {
			client, err := NewClient(fixtureConfiguration(t), &refusingIdentity{})
			if err != nil {
				t.Fatalf("NewClient() = %v, want nil", err)
			}
			if err := client.Close(); err != nil {
				t.Fatalf("Close() = %v, want nil", err)
			}
			return client
		}, wantErr: core.ErrTailnetClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.setup(t).Exchange()
			if !errors.Is(err, tc.wantErr) || got != (exchange.Client{}) {
				t.Fatalf("Exchange() = (%v,%v), want zero and %v", got, err, tc.wantErr)
			}
		})
	}
}
