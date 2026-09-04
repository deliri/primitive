package distributionauth

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
)

func distributionAuthServer(t testing.TB, trusted attest.TrustedKeys) controlplane.Authority {
	t.Helper()
	server, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewAuthority(distribution authority) error = %v, want nil", err)
	}
	return server
}

func distributionAuthClient(t testing.TB, trusted attest.TrustedKeys) controlplane.Client {
	t.Helper()
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewClient(distribution authority) error = %v, want nil", err)
	}
	return client
}
