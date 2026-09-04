package retrievalauth

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
)

func retrievalAuthServer(t testing.TB, trusted attest.TrustedKeys) controlplane.Authority {
	t.Helper()
	server, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewAuthority(retrieval authority) error = %v, want nil", err)
	}
	return server
}
