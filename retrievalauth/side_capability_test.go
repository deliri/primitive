package retrievalauth

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
)

func retrievalAuthServer(t testing.TB, trusted attest.TrustedKeys) controlplane.Server {
	t.Helper()
	server, err := controlplane.NewServer(controlplane.ServerConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewServer(retrieval authority) error = %v, want nil", err)
	}
	return server
}
