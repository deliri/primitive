package controlplane_test

import (
	"errors"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

type controlplaneCapabilityStateCase struct {
	run       func() (bool, error)
	name      string
	wantErr   []error
	wantValid bool
}

func TestControlplaneSideCapabilitiesExhaustEveryReachableConfigurationState(t *testing.T) {
	t.Parallel()

	public, _ := testSigningKey(t, 71)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{public},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	clientConfiguration := controlplane.ClientConfiguration{TrustedAuthorityKeys: trusted}
	serverConfiguration := controlplane.ServerConfiguration{TrustedAuthorityKeys: trusted}
	cases := []controlplaneCapabilityStateCase{
		{
			name: "valid client configuration retains one closed authority set",
			run: func() (bool, error) {
				return true, clientConfiguration.Validate()
			},
			wantValid: true,
		},
		{
			name: "valid client constructor returns one usable capability",
			run: func() (bool, error) {
				got, gotErr := controlplane.NewClient(clientConfiguration)
				return got != (controlplane.Client{}), errors.Join(gotErr, got.Validate())
			},
			wantValid: true,
		},
		{
			name: "zero client configuration is rejected before construction",
			run: func() (bool, error) {
				return false, (controlplane.ClientConfiguration{}).Validate()
			},
			wantErr: []error{core.ErrControlPlaneContract, core.ErrAttestContract},
		},
		{
			name: "zero client configuration cannot mint a partial capability",
			run: func() (bool, error) {
				got, gotErr := controlplane.NewClient(controlplane.ClientConfiguration{})
				return got != (controlplane.Client{}), gotErr
			},
			wantErr: []error{core.ErrControlPlaneContract, core.ErrAttestContract},
		},
		{
			name: "zero client capability carries no authority",
			run: func() (bool, error) {
				return false, (controlplane.Client{}).Validate()
			},
			wantErr: []error{core.ErrControlPlaneContract, core.ErrAttestContract},
		},
		{
			name: "valid server configuration retains one closed authority set",
			run: func() (bool, error) {
				return true, serverConfiguration.Validate()
			},
			wantValid: true,
		},
		{
			name: "valid server constructor returns one usable capability",
			run: func() (bool, error) {
				got, gotErr := controlplane.NewServer(serverConfiguration)
				return got != (controlplane.Server{}), errors.Join(gotErr, got.Validate())
			},
			wantValid: true,
		},
		{
			name: "zero server configuration is rejected before construction",
			run: func() (bool, error) {
				return false, (controlplane.ServerConfiguration{}).Validate()
			},
			wantErr: []error{core.ErrControlPlaneContract, core.ErrAttestContract},
		},
		{
			name: "zero server configuration cannot mint a partial capability",
			run: func() (bool, error) {
				got, gotErr := controlplane.NewServer(controlplane.ServerConfiguration{})
				return got != (controlplane.Server{}), gotErr
			},
			wantErr: []error{core.ErrControlPlaneContract, core.ErrAttestContract},
		},
		{
			name: "zero server capability carries no authority",
			run: func() (bool, error) {
				return false, (controlplane.Server{}).Validate()
			},
			wantErr: []error{core.ErrControlPlaneContract, core.ErrAttestContract},
		},
	}
	if got, want := len(cases), 10; got != want {
		t.Fatalf("reachable control-plane capability states = %d, want exact %d", got, want)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotValid, gotErr := tc.run()
			if gotValid != tc.wantValid {
				t.Fatalf("capability presence = %t, want %t", gotValid, tc.wantValid)
			}
			if tc.wantValid {
				if gotErr != nil {
					t.Fatalf("capability state error = %v, want nil", gotErr)
				}
				return
			}
			for _, wantErr := range tc.wantErr {
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("capability state error = %v, want errors.Is %v", gotErr, wantErr)
				}
			}
		})
	}
}

func TestControlplaneSideCapabilitiesPermitOnlyTheirPeerOperations(t *testing.T) {
	t.Parallel()

	clientMethods := compilerOwnedControlplaneMethodNames(
		reflect.ValueOf(controlplane.Client.IssueCheckIn),
		reflect.ValueOf(controlplane.Client.Validate),
		reflect.ValueOf(controlplane.Client.VerifyCheckInResponse),
		reflect.ValueOf(controlplane.Client.VerifyInstallationCertificate),
		reflect.ValueOf(controlplane.Client.VerifyRegistration),
	)
	serverMethods := compilerOwnedControlplaneMethodNames(
		reflect.ValueOf(controlplane.Server.CommitCheckIn),
		reflect.ValueOf(controlplane.Server.IssueCheckInResponse),
		reflect.ValueOf(controlplane.Server.IssueCommittedCheckInResponse),
		reflect.ValueOf(controlplane.Server.IssueInstallationCertificate),
		reflect.ValueOf(controlplane.Server.IssueRegisteredInstallation),
		reflect.ValueOf(controlplane.Server.IssueRegistration),
		reflect.ValueOf(controlplane.Server.PrepareCheckInResponse),
		reflect.ValueOf(controlplane.Server.Validate),
		reflect.ValueOf(controlplane.Server.VerifyCheckIn),
		reflect.ValueOf(controlplane.Server.VerifyInstallationCertificate),
		reflect.ValueOf(controlplane.Server.VerifyRegistrationAuthority),
	)
	cases := []struct {
		name        string
		capability  reflect.Type
		wantMethods []string
	}{
		{name: "installed client cannot issue authority decisions", capability: reflect.TypeFor[controlplane.Client](), wantMethods: clientMethods},
		{name: "authority server cannot initiate installed-client operations", capability: reflect.TypeFor[controlplane.Server](), wantMethods: serverMethods},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotMethods := exportedControlplaneMethodNames(tc.capability)
			if !slices.Equal(gotMethods, tc.wantMethods) {
				t.Fatalf("%s exported methods = %v, want compiler-owned %v", tc.capability, gotMethods, tc.wantMethods)
			}
		})
	}
}

func TestControlplaneSideCapabilitiesBindRealAuthoritySignaturesWithoutCrossSideTrust(t *testing.T) {
	t.Parallel()

	authorityPublic, authoritySigner := testSigningKey(t, 72)
	authorityTrust, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{authorityPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(authority) error = %v, want nil", err)
	}
	client := testControlplaneClient(t, authorityTrust)
	server := testControlplaneServer(t, authorityTrust)
	body := issueTestRegistration(t).document.Payload.Certificate.Body
	document, err := server.IssueInstallationCertificate(body, authoritySigner)
	if err != nil {
		t.Fatalf("Server.IssueInstallationCertificate() error = %v, want nil", err)
	}
	lowerProof, err := attest.Verify(attest.VerifyRequest[controlplane.SigningDomain]{
		Body: document.Body, Envelope: document.Attestation, TrustedKeys: authorityTrust,
	})
	if err != nil || lowerProof.Validate() != nil {
		t.Fatalf("attest.Verify(real issued certificate) = (%v, %v), want validated proof and nil", lowerProof, err)
	}

	foreignPublic, _ := testSigningKey(t, 73)
	foreignTrust, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{foreignPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(foreign) error = %v, want nil", err)
	}
	foreignClient := testControlplaneClient(t, foreignTrust)
	cases := []struct {
		verify    func() (controlplane.VerifiedInstallationCertificate, error)
		name      string
		wantErr   []error
		wantValid bool
	}{
		{name: "configured client authenticates the authority document", verify: func() (controlplane.VerifiedInstallationCertificate, error) {
			return client.VerifyInstallationCertificate(document)
		}, wantValid: true},
		{name: "configured server authenticates a presented authority document", verify: func() (controlplane.VerifiedInstallationCertificate, error) {
			return server.VerifyInstallationCertificate(document)
		}, wantValid: true},
		{name: "foreign client rejects an otherwise valid authority document", verify: func() (controlplane.VerifiedInstallationCertificate, error) {
			return foreignClient.VerifyInstallationCertificate(document)
		}, wantErr: []error{core.ErrControlPlaneRegistration, core.ErrAttestVerification}},
		{name: "zero client cannot authenticate any document", verify: func() (controlplane.VerifiedInstallationCertificate, error) {
			return (controlplane.Client{}).VerifyInstallationCertificate(document)
		}, wantErr: []error{core.ErrControlPlaneRegistration, core.ErrControlPlaneContract}},
		{name: "zero server cannot authenticate any document", verify: func() (controlplane.VerifiedInstallationCertificate, error) {
			return (controlplane.Server{}).VerifyInstallationCertificate(document)
		}, wantErr: []error{core.ErrControlPlaneRegistration, core.ErrControlPlaneContract}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := tc.verify()
			if tc.wantValid {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("VerifyInstallationCertificate() = (%v, %v), want validated proof and nil", got, gotErr)
				}
				gotBody, gotBodyErr := got.Body()
				if gotBodyErr != nil || gotBody != body {
					t.Fatalf("VerifiedInstallationCertificate.Body() = (%v, %v), want (%v, nil)", gotBody, gotBodyErr, body)
				}
				return
			}
			for _, wantErr := range tc.wantErr {
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("VerifyInstallationCertificate() error = %v, want errors.Is %v", gotErr, wantErr)
				}
			}
			if got.Validate() == nil {
				t.Fatalf("rejected VerifyInstallationCertificate() proof = %v, want invalid zero proof", got)
			}
		})
	}
}

func TestControlplaneSideCapabilityStorageExcludesPeerAndTransportState(t *testing.T) {
	t.Parallel()

	trustedKeysType := reflect.TypeFor[attest.TrustedKeys]()
	cases := []struct {
		name                     string
		structure                reflect.Type
		wantFieldTypes           []reflect.Type
		wantUnexportedFieldCount int
	}{
		{name: "client configuration owns only authority trust", structure: reflect.TypeFor[controlplane.ClientConfiguration](), wantFieldTypes: []reflect.Type{trustedKeysType}},
		{name: "opaque client closes exactly one private client configuration", structure: reflect.TypeFor[controlplane.Client](), wantFieldTypes: []reflect.Type{reflect.TypeFor[controlplane.ClientConfiguration]()}, wantUnexportedFieldCount: 1},
		{name: "server configuration owns only authority trust", structure: reflect.TypeFor[controlplane.ServerConfiguration](), wantFieldTypes: []reflect.Type{trustedKeysType}},
		{name: "opaque server closes exactly one private server configuration", structure: reflect.TypeFor[controlplane.Server](), wantFieldTypes: []reflect.Type{reflect.TypeFor[controlplane.ServerConfiguration]()}, wantUnexportedFieldCount: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotFieldTypes := make([]reflect.Type, 0, tc.structure.NumField())
			gotUnexportedFieldCount := 0
			for index := range tc.structure.NumField() {
				field := tc.structure.Field(index)
				gotFieldTypes = append(gotFieldTypes, field.Type)
				if field.PkgPath != "" {
					gotUnexportedFieldCount++
				}
			}
			if !slices.Equal(gotFieldTypes, tc.wantFieldTypes) {
				t.Fatalf("%s field types = %v, want %v", tc.structure, gotFieldTypes, tc.wantFieldTypes)
			}
			if gotUnexportedFieldCount != tc.wantUnexportedFieldCount {
				t.Fatalf("%s unexported field count = %d, want %d", tc.structure, gotUnexportedFieldCount, tc.wantUnexportedFieldCount)
			}
		})
	}
}

func compilerOwnedControlplaneMethodNames(methods ...reflect.Value) []string {
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		function := runtime.FuncForPC(method.Pointer())
		name := function.Name()
		names = append(names, name[strings.LastIndexByte(name, '.')+1:])
	}
	slices.Sort(names)
	return names
}

func exportedControlplaneMethodNames(capability reflect.Type) []string {
	names := make([]string, 0, capability.NumMethod())
	for index := range capability.NumMethod() {
		names = append(names, capability.Method(index).Name)
	}
	slices.Sort(names)
	return names
}
