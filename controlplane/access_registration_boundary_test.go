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
		change func(testing.TB, *controlplane.AccessRegistrationRequest)
		name   string
	}{
		{name: "nominal registration retains all six facts", change: nil},
		{name: "offering selects the exact route and replay domain", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = testBuildForOffering(t, controlplaneOffering(t, 2))
		}},
		{name: "build commit is retained rather than inferred from version", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
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
		{name: "major version is retained in replay", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, core.NewReleaseVersion(2, 0, 0), r.Build.Platform())
		}},
		{name: "minor version is retained in replay", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, core.NewReleaseVersion(1, 1, 0), r.Build.Platform())
		}},
		{name: "patch version is retained in replay", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, core.NewReleaseVersion(1, 0, 1), r.Build.Platform())
		}},
		{name: "Windows platform is retained rather than host-detected", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, r.Build.Version(), core.Platform{OperatingSystem: core.OperatingSystemWindows, Architecture: core.CPUArchitectureAMD64})
		}},
		{name: "Linux arm64 architecture is retained rather than host-detected", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			r.Build = accessRegistrationBuild(t, r.Build, r.Build.Version(), core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureARM64})
		}},
		{name: "nonce selects a distinct exact replay", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
			var err error
			r.RequestNonce, err = controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{91})
			if err != nil {
				t.Fatalf("NewRequestNonce() error = %v, want nil", err)
			}
		}},
		{name: "independently bound device is retained in the proof", change: func(t testing.TB, r *controlplane.AccessRegistrationRequest) {
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
			defer func() {
				if err := got.Token.Destroy(); err != nil {
					t.Errorf("got.Token.Destroy() cleanup error = %v, want nil", err)
				}
			}()
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
		wantErr error
		input   func() []byte
		name    string
	}{
		{name: "empty stream", input: func() []byte { return nil }, wantErr: core.ErrJSONContract},
		{name: "null is not a registration", input: func() []byte { return []byte("null") }, wantErr: core.ErrControlPlaneRegistration},
		{name: "array is not an object", input: func() []byte { return []byte("[]") }, wantErr: core.ErrJSONContract},
		{name: "truncated outer object", input: func() []byte { return bytes.Clone(canonical[:len(canonical)-1]) }, wantErr: core.ErrJSONContract},
		{name: "second document is trailing data", input: func() []byte { return append(bytes.Clone(canonical), canonical...) }, wantErr: core.ErrJSONContract},
		{name: "unknown member is not ignored", input: func() []byte { return append([]byte(`{"foreign":1,`), canonical[1:]...) }, wantErr: core.ErrJSONContract},
		{name: "duplicate nonce is not last-wins", input: func() []byte {
			return append(append(append([]byte(`{"request_nonce":`), nonce...), ','), canonical[1:]...)
		}, wantErr: core.ErrJSONContract},
		{name: "case-folded member cannot alias nonce", input: func() []byte {
			return bytes.Replace(canonical, []byte(`"request_nonce"`), []byte(`"REQUEST_NONCE"`), 1)
		}, wantErr: core.ErrJSONContract},
		{name: "token numeric type cannot coerce", input: func() []byte { return bytes.Replace(canonical, token, []byte("1"), 1) }, wantErr: core.ErrControlWireToken},
		{name: "nested array cannot coerce to token", input: func() []byte { return bytes.Replace(canonical, token, []byte("[]"), 1) }, wantErr: core.ErrControlWireToken},
		{name: "document one byte below ceiling", input: func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes-1-len(canonical))...)
		}, wantErr: nil},
		{name: "document exactly at ceiling", input: func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes-len(canonical))...)
		}, wantErr: nil},
		{name: "document one byte above ceiling", input: func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, controlplane.AccessRegistrationRequestJSONMaximumBytes+1-len(canonical))...)
		}, wantErr: core.ErrJSONContract},
		{name: "document twice ceiling remains bounded rejection", input: func() []byte {
			return append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, 2*controlplane.AccessRegistrationRequestJSONMaximumBytes-len(canonical))...)
		}, wantErr: core.ErrJSONContract},
		{name: "token one byte below extent", input: func() []byte {
			short := append(bytes.Clone(token[:len(token)-2]), '"')
			return bytes.Replace(canonical, token, short, 1)
		}, wantErr: core.ErrControlWireToken},
		{name: "token exactly at canonical extent", input: func() []byte { return bytes.Clone(canonical) }, wantErr: nil},
		{name: "token one byte above extent", input: func() []byte {
			long := append(bytes.Clone(token[:len(token)-1]), '0', '"')
			return bytes.Replace(canonical, token, long, 1)
		}, wantErr: core.ErrControlWireToken},
		{name: "token maximum-sized body cannot bypass nominal extent", input: func() []byte {
			return bytes.Replace(canonical, token, append(append([]byte{'"'}, bytes.Repeat([]byte{'a'}, 8192)...), '"'), 1)
		}, wantErr: core.ErrControlWireToken},
		{name: "token prefix spelling cannot select another protocol", input: func() []byte {
			changed := bytes.Clone(token)
			changed[1] = 'P'
			return bytes.Replace(canonical, token, changed, 1)
		}, wantErr: core.ErrControlWireToken},
		{name: "escaped token prefix is not canonical token encoding", input: func() []byte {
			changed := append([]byte(`"\u0070`), token[2:]...)
			return bytes.Replace(canonical, token, changed, 1)
		}, wantErr: core.ErrControlWireToken},
		{name: "nonce one hex digit below extent", input: func() []byte {
			short := append(bytes.Clone(nonce[:len(nonce)-2]), '"')
			return bytes.Replace(canonical, nonce, short, 1)
		}, wantErr: core.ErrControlPlaneRegistration},
		{name: "nonce one hex digit above extent", input: func() []byte {
			long := append(bytes.Clone(nonce[:len(nonce)-1]), '0', '"')
			return bytes.Replace(canonical, nonce, long, 1)
		}, wantErr: core.ErrControlPlaneRegistration},
		{name: "zero nonce is not an absent replay slot", input: func() []byte {
			return bytes.Replace(canonical, nonce, append(append([]byte{'"'}, bytes.Repeat([]byte{'0'}, 64)...), '"'), 1)
		}, wantErr: core.ErrControlPlaneRegistration},
		{name: "nonce null cannot create a request identity", input: func() []byte { return bytes.Replace(canonical, nonce, []byte("null"), 1) }, wantErr: core.ErrControlPlaneRegistration},
		{name: "installation one hex digit below extent", input: func() []byte {
			short := append(bytes.Clone(installation[:len(installation)-2]), '"')
			return bytes.Replace(canonical, installation, short, 1)
		}, wantErr: core.ErrControlPlaneRegistration},
		{name: "installation one hex digit above extent", input: func() []byte {
			long := append(bytes.Clone(installation[:len(installation)-1]), '0', '"')
			return bytes.Replace(canonical, installation, long, 1)
		}, wantErr: core.ErrControlPlaneRegistration},
		{name: "individually valid foreign installation conflicts with key", input: func() []byte {
			changed := bytes.Clone(installation)
			if changed[1] == '1' {
				changed[1] = '2'
			} else {
				changed[1] = '1'
			}
			return bytes.Replace(canonical, installation, changed, 1)
		}, wantErr: core.ErrControlPlaneInstallationBinding},
		{name: "zero installation cannot bind a device", input: func() []byte {
			return bytes.Replace(canonical, installation, append(append([]byte{'"'}, bytes.Repeat([]byte{'0'}, 32)...), '"'), 1)
		}, wantErr: core.ErrControlPlaneRegistration},
		{name: "malformed UTF8 is refused before interpretation", input: func() []byte { return append([]byte{0xff}, canonical...) }, wantErr: core.ErrJSONContract},
		{name: "depth beyond grammar ceiling is bounded refusal", input: func() []byte {
			return bytes.Replace(canonical, token, append(bytes.Repeat([]byte{'['}, 65), bytes.Repeat([]byte{']'}, 65)...), 1)
		}, wantErr: core.ErrJSONContract},
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
			defer func() {
				if err := got.Token.Destroy(); err != nil {
					t.Errorf("got.Token.Destroy() cleanup error = %v, want nil", err)
				}
			}()
			encoded, err := got.MarshalJSON()
			defer clear(encoded)
			if err != nil || !bytes.Equal(encoded, canonical) {
				t.Fatalf("accepted canonical equality = %t, error = %v, want true and nil", bytes.Equal(encoded, canonical), err)
			}
		})
	}
}
