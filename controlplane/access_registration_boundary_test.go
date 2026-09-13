package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// Each accepted row changes a fact retained in the identity or replay
// commitment. These are not whitespace permutations counted as new meaning.
func TestAccessRegistrationAcceptedFactsSurviveCanonicalBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(testing.TB, *controlplane.AccessRegistrationRequest)
	}{
		{"nominal registration retains all six facts", nil},
		{"offering selects the exact route and replay domain", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = testBuildForOffering(t, controlplaneOffering(t, 2))
		}},
		{"build commit is retained rather than inferred from version", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			commit, err := core.ParseBuildCommit("ffffffffffffffffffffffffffffffffffffffff")
			if err != nil {
				t.Fatalf("ParseBuildCommit() error = %v, want nil", err)
			}
			var buildErr error
			r.Build, buildErr = core.NewBuildIdentity(core.BuildIdentityRequest{Offering: r.Build.Offering(), Version: r.Build.Version(), Commit: commit, Platform: r.Build.Platform()})
			if buildErr != nil {
				t.Fatalf("NewBuildIdentity() error = %v, want nil", buildErr)
			}
		}},
		{"major version is retained in replay", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, core.NewReleaseVersion(2, 0, 0), r.Build.Platform())
		}},
		{"minor version is retained in replay", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, core.NewReleaseVersion(1, 1, 0), r.Build.Platform())
		}},
		{"patch version is retained in replay", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, core.NewReleaseVersion(1, 0, 1), r.Build.Platform())
		}},
		{"Windows platform is retained rather than host-detected", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, r.Build.Version(), core.Platform{OperatingSystem: core.OperatingSystemWindows, Architecture: core.CPUArchitectureAMD64})
		}},
		{"Linux arm64 architecture is retained rather than host-detected", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, r.Build.Version(), core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureARM64})
		}},
		{"nonce selects a distinct exact replay", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			var err error
			r.RequestNonce, err = controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{91})
			if err != nil {
				t.Fatalf("NewRequestNonce() error = %v, want nil", err)
			}
		}},
		{"independently bound device is retained in the proof", func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.DeviceKey, _ = testSigningKey(t, 77)
			var err error
			r.Installation, err = lease.DeviceIDForPublicKey(r.DeviceKey)
			if err != nil {
				t.Fatalf("DeviceIDForPublicKey() error = %v, want nil", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed, server := accessRegistrationFixture(t)
			baselineReplay, err := controlwire.CommitReplayIdentity(seed)
			if err != nil {
				t.Fatalf("CommitReplayIdentity(seed) error = %v, want nil", err)
			}
			want := seed
			if tc.change != nil {
				tc.change(t, &want)
				if want == seed {
					t.Fatal("mutation = unchanged, want changed retained fact")
				}
			}
			data, err := want.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON(typed seed) error = %v, want nil", err)
			}
			defer clear(data)
			var got controlplane.AccessRegistrationRequest
			if err := got.UnmarshalJSON(data); err != nil {
				t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
			}
			defer got.Token.Destroy()
			wantIdentity, err := want.Identity()
			if err != nil {
				t.Fatalf("Identity(seed) error = %v, want nil", err)
			}
			gotIdentity, err := got.Identity()
			if err != nil || gotIdentity != wantIdentity {
				t.Fatalf("decoded identity = (%+v, %v), want (%+v, nil)", gotIdentity, err, wantIdentity)
			}
			verifier, err := seed.Token.Verifier()
			if err != nil {
				t.Fatalf("Verifier() error = %v, want nil", err)
			}
			proof, err := server.VerifyAccessRegistration(controlplane.AccessRegistrationVerification{Request: got, ExpectedVerifier: verifier})
			if err != nil {
				t.Fatalf("VerifyAccessRegistration() error = %v, want nil", err)
			}
			identity, err := proof.Identity()
			if err != nil || identity != wantIdentity {
				t.Fatalf("proof identity = (%+v, %v), want (%+v, nil)", identity, err, wantIdentity)
			}
			replay, disposition, err := proof.Replay()
			if err != nil || disposition != controlwire.ReplayDispositionFresh || (tc.change != nil && replay.Equal(baselineReplay)) {
				t.Fatalf("replay = (%v, baselineEqual=%t, %v), want fresh, load-bearing mutation, nil", disposition, replay.Equal(baselineReplay), err)
			}
		})
	}
}

func accessRegistrationBuild(t testing.TB, original core.BuildIdentity, version core.ReleaseVersion, platform core.Platform) core.BuildIdentity {
	t.Helper()
	got, err := core.NewBuildIdentity(core.BuildIdentityRequest{Offering: original.Offering(), Version: version, Commit: original.Commit(), Platform: platform})
	if err != nil {
		t.Fatalf("NewBuildIdentity() error = %v, want nil", err)
	}
	return got
}

func TestAccessRegistrationHostileRepresentationBoundary(t *testing.T) {
	t.Parallel()
	seed, _ := accessRegistrationFixture(t)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	t.Cleanup(func() { clear(canonical) })
	// Every malformed representation starts with production-emitted bytes.
	// Raw JSON literals below are deliberately invalid/foreign input only.
	token, err := seed.Token.MarshalJSON()
	if err != nil {
		t.Fatalf("Token.MarshalJSON() error = %v, want nil", err)
	}
	t.Cleanup(func() { clear(token) })
	nonce, err := seed.RequestNonce.MarshalJSON()
	if err != nil {
		t.Fatalf("Nonce.MarshalJSON() error = %v, want nil", err)
	}
	installation, err := seed.Installation.MarshalJSON()
	if err != nil {
		t.Fatalf("Installation.MarshalJSON() error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		input   func() []byte
		wantErr error
	}{
		{"empty stream", func() []byte { return nil }, core.ErrJSONContract},
		{"null is not a registration", func() []byte { return []byte("null") }, core.ErrControlPlaneRegistration},
		{"array is not an object", func() []byte { return []byte("[]") }, core.ErrJSONContract},
		{"truncated outer object", func() []byte { return bytes.Clone(canonical[:len(canonical)-1]) }, core.ErrJSONContract},
		{"second document is trailing data", func() []byte { return append(bytes.Clone(canonical), canonical...) }, core.ErrJSONContract},
		{"unknown member is not ignored", func() []byte { return append([]byte(`{"foreign":1,`), canonical[1:]...) }, core.ErrJSONContract},
		{"duplicate nonce is not last-wins", func() []byte {
			return append(append(append([]byte(`{"request_nonce":`), nonce...), ','), canonical[1:]...)
		}, core.ErrJSONContract},
		{"case-folded member cannot alias nonce", func() []byte {
			return bytes.Replace(canonical, []byte(`"request_nonce"`), []byte(`"REQUEST_NONCE"`), 1)
		}, core.ErrJSONContract},
		{"token numeric type cannot coerce", func() []byte { return bytes.Replace(canonical, token, []byte("1"), 1) }, core.ErrControlWireToken},
		{"nested array cannot coerce to token", func() []byte { return bytes.Replace(canonical, token, []byte("[]"), 1) }, core.ErrControlWireToken},
		{"document one byte below ceiling", func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes-1-len(canonical))...)
		}, nil},
		{"document exactly at ceiling", func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes-len(canonical))...)
		}, nil},
		{"document one byte above ceiling", func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes+1-len(canonical))...)
		}, core.ErrJSONContract},
		{"document twice ceiling remains bounded rejection", func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, 2*controlplane.AccessRegistrationRequestJSONMaximumBytes-len(canonical))...)
		}, core.ErrJSONContract},
		{"token one byte below extent", func() []byte {
			short := append(bytes.Clone(token[:len(token)-2]), '"')
			return bytes.Replace(canonical, token, short, 1)
		}, core.ErrControlWireToken},
		{"token exactly at canonical extent", func() []byte { return bytes.Clone(canonical) }, nil},
		{"token one byte above extent", func() []byte {
			long := append(bytes.Clone(token[:len(token)-1]), '0', '"')
			return bytes.Replace(canonical, token, long, 1)
		}, core.ErrControlWireToken},
		{"token maximum-sized body cannot bypass nominal extent", func() []byte {
			return bytes.Replace(canonical, token, append(append([]byte{'"'}, bytes.Repeat([]byte{'a'}, 8192)...), '"'), 1)
		}, core.ErrControlWireToken},
		{"token prefix spelling cannot select another protocol", func() []byte {
			changed := bytes.Clone(token)
			changed[1] = 'P'
			return bytes.Replace(canonical, token, changed, 1)
		}, core.ErrControlWireToken},
		{"escaped token prefix is not canonical token encoding", func() []byte {
			changed := append([]byte(`"\u0070`), token[2:]...)
			return bytes.Replace(canonical, token, changed, 1)
		}, core.ErrControlWireToken},
		{"nonce one hex digit below extent", func() []byte {
			short := append(bytes.Clone(nonce[:len(nonce)-2]), '"')
			return bytes.Replace(canonical, nonce, short, 1)
		}, core.ErrControlPlaneRegistration},
		{"nonce one hex digit above extent", func() []byte {
			long := append(bytes.Clone(nonce[:len(nonce)-1]), '0', '"')
			return bytes.Replace(canonical, nonce, long, 1)
		}, core.ErrControlPlaneRegistration},
		{"zero nonce is not an absent replay slot", func() []byte {
			return bytes.Replace(canonical, nonce, append(append([]byte{'"'}, bytes.Repeat([]byte{'0'}, 64)...), '"'), 1)
		}, core.ErrControlPlaneRegistration},
		{"nonce null cannot create a request identity", func() []byte { return bytes.Replace(canonical, nonce, []byte("null"), 1) }, core.ErrControlPlaneRegistration},
		{"installation one hex digit below extent", func() []byte {
			short := append(bytes.Clone(installation[:len(installation)-2]), '"')
			return bytes.Replace(canonical, installation, short, 1)
		}, core.ErrControlPlaneRegistration},
		{"installation one hex digit above extent", func() []byte {
			long := append(bytes.Clone(installation[:len(installation)-1]), '0', '"')
			return bytes.Replace(canonical, installation, long, 1)
		}, core.ErrControlPlaneRegistration},
		{"individually valid foreign installation conflicts with key", func() []byte {
			changed := bytes.Clone(installation)
			if changed[1] == '1' {
				changed[1] = '2'
			} else {
				changed[1] = '1'
			}
			return bytes.Replace(canonical, installation, changed, 1)
		}, core.ErrControlPlaneInstallationBinding},
		{"zero installation cannot bind a device", func() []byte {
			return bytes.Replace(canonical, installation, append(append([]byte{'"'}, bytes.Repeat([]byte{'0'}, 32)...), '"'), 1)
		}, core.ErrControlPlaneRegistration},
		{"malformed UTF8 is refused before interpretation", func() []byte { return append([]byte{0xff}, canonical...) }, core.ErrJSONContract},
		{"depth beyond grammar ceiling is bounded refusal", func() []byte {
			return bytes.Replace(canonical, token, append(bytes.Repeat([]byte{'['}, 65), bytes.Repeat([]byte{']'}, 65)...), 1)
		}, core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := tc.input()
			defer clear(data)
			got := seed
			err := got.UnmarshalJSON(data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UnmarshalJSON() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if got != seed {
					t.Fatal("refused receiver = changed, want exact original")
				}
				var fresh controlplane.AccessRegistrationRequest
				if freshErr := fresh.UnmarshalJSON(data); !errors.Is(freshErr, tc.wantErr) || fresh != (controlplane.AccessRegistrationRequest{}) {
					t.Fatalf("fresh rejection = (%v, zero=%t), want typed refusal and zero", freshErr, fresh == (controlplane.AccessRegistrationRequest{}))
				}
				return
			}
			defer got.Token.Destroy()
			encoded, err := got.MarshalJSON()
			defer clear(encoded)
			if err != nil || !bytes.Equal(encoded, canonical) {
				t.Fatalf("accepted canonical equality = %t, error = %v, want true and nil", bytes.Equal(encoded, canonical), err)
			}
		})
	}
}
