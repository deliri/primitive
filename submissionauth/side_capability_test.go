package submissionauth

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
)

func submissionAuthServer(t testing.TB, trusted attest.TrustedKeys) controlplane.Authority {
	t.Helper()
	server, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewAuthority(submission authority) error = %v, want nil", err)
	}
	return server
}

func submissionAuthClient(t testing.TB, trusted attest.TrustedKeys) controlplane.Client {
	t.Helper()
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewClient(submission authority) error = %v, want nil", err)
	}
	return client
}
