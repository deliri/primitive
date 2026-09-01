package runnercontrol_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

func FuzzPeerCredentialKindExternalJSONSemanticClosure(f *testing.F) {
	for _, kind := range []runnercontrol.PeerCredentialKind{
		runnercontrol.PeerCredentialMutualTLS,
		runnercontrol.PeerCredentialGoogleCloud,
	} {
		encoded, err := kind.MarshalJSON()
		if err != nil {
			f.Fatalf("PeerCredentialKind.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))

	f.Fuzz(func(t *testing.T, data []byte) {
		proveEnumJSONClosure(
			t,
			"PeerCredentialKind",
			runnercontrol.PeerCredentialGoogleCloud,
			data,
			(*runnercontrol.PeerCredentialKind).UnmarshalJSON,
		)
	})
}
