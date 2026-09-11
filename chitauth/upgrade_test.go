package chitauth

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestCredentialNominationAndRouteLayerTriad(t *testing.T) {
	t.Parallel()
	base := newQueryFixture(t, standardQueryFixtureRequest(t))
	otherSpec := standardQueryFixtureRequest(t)
	otherSpec.deviceByte++
	other := newQueryFixture(t, otherSpec)
	foreign := base.document
	foreign.Request = other.document.Request
	if foreign.Request.Attestation.Signer == foreign.Certificate.Body.DeviceKey {
		t.Fatal("foreign signer = nominated key, want a real one-fact identity change")
	}
	absent := base.document
	absent.Certificate = controlplane.InstallationCertificateDocument{}
	for _, tc := range []struct {
		name     string
		document RequestDocument
		wantErr  error
	}{
		{name: "nominated signer admits exact route", document: base.document},
		{name: "foreign device cannot assemble or route", document: foreign, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "absent certificate cannot manufacture route", document: absent, wantErr: core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Assemble(RequestAssembly(tc.document))
			route, routeErr := tc.document.ControlRoute()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (RequestDocument{}) {
					t.Fatalf("Assemble = %v/%v, want zero/%v", got, err, tc.wantErr)
				}
				if !errors.Is(routeErr, tc.wantErr) || route != (controlwire.RouteContract{}) {
					t.Fatalf("ControlRoute = %v/%v, want zero/%v", route, routeErr, tc.wantErr)
				}
				var wire []byte
				if tc.document.Certificate == (controlplane.InstallationCertificateDocument{}) {
					wire, err = core.MarshalCanonicalJSONDocument(struct {
						Request chit.QueryDocument `json:"request"`
					}{tc.document.Request})
				} else {
					wire, err = core.MarshalCanonicalJSONDocument(requestDocumentWire(tc.document))
				}
				if err != nil {
					t.Fatal(err)
				}
				receiver := base.document
				err = receiver.UnmarshalJSON(wire)
				if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, tc.wantErr) || receiver != base.document {
					t.Fatalf("JSON refusal = %v/%v, want preserved/%v", receiver, err, tc.wantErr)
				}
				return
			}
			wantRoute, errWant := controlwire.NewRouteContract(base.payload.Build.Offering(), controlwire.RouteFamilyChits)
			if err != nil || got != base.document || routeErr != nil || errWant != nil || route != wantRoute {
				t.Fatalf("credential/route = %v/%v/%v, want exact document and route", got, err, routeErr)
			}
		})
	}
}

func TestCredentialWhitespaceExtentPreservesAuthentication(t *testing.T) {
	t.Parallel()
	fixture := newQueryFixture(t, standardQueryFixtureRequest(t))
	wire, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	data := append(bytes.Repeat([]byte(" \n\t\r"), 262144), wire...)
	var got RequestDocument
	if err := got.UnmarshalJSON(data); err != nil || got != fixture.document {
		t.Fatalf("large whitespace decode = %v/%v, want exact credential", got, err)
	}
	proof, err := Verify(Verification{Document: got, Server: fixture.server})
	payload, payloadErr := proof.Payload()
	if err != nil || payloadErr != nil || payload != fixture.payload {
		t.Fatalf("large whitespace authentication = %v/%v, want exact signed payload", err, payloadErr)
	}
}
