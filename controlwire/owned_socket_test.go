package controlwire_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

func ownedRegistrationSeed(t testing.TB) controlplane.AccessRegistrationRequest {
	t.Helper()
	fixture := productionSocketFixture(t)
	token, err := controlwire.NewAccessToken([controlwire.AccessTokenBytes]byte{39})
	if err != nil {
		t.Fatalf("access token seed = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := token.Destroy(); err != nil {
			t.Errorf("seed destruction = %v, want nil", err)
		}
	})
	original := fixture.request
	seed := controlplane.AccessRegistrationRequest{Token: token, Build: original.Build, DeviceKey: original.DeviceKey, Installation: original.Installation, RequestNonce: original.RequestNonce, Revision: original.Revision}
	if err := seed.Validate(); err != nil {
		t.Fatalf("registration seed = %v, want nil", err)
	}
	return seed
}

func TestOwnedRoutedReceiverCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                       string
		foreignKey, foreignRoute, empty, omitRelease, closeFailure bool
		releaseErr, wantErr                                        error
		wantReleases                                               int
	}{
		{name: "accepted token transfers live custody to caller"},
		{name: "foreign nonce destroys decoded token", foreignKey: true, wantErr: core.ErrControlWireNonce, wantReleases: 1},
		{name: "foreign route destroys decoded token", foreignRoute: true, wantErr: core.ErrControlWireRoute, wantReleases: 1},
		{name: "cleanup error remains beside nonce refusal", foreignKey: true, releaseErr: io.ErrClosedPipe, wantErr: core.ErrControlWireNonce, wantReleases: 1},
		{name: "body close refusal destroys decoded token", closeFailure: true, wantErr: io.ErrClosedPipe, wantReleases: 1},
		{name: "empty body allocates no token", empty: true, wantErr: core.ErrExchangeContract},
		{name: "missing ownership refuses before decoding", omitRelease: true, wantErr: core.ErrControlWireRoute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed := ownedRegistrationSeed(t)
			canonical, err := seed.MarshalJSON()
			if err != nil {
				t.Fatalf("seed encoding = %v, want nil", err)
			}
			defer clear(canonical)
			route, err := seed.ControlRoute()
			if err != nil {
				t.Fatalf("route = %v, want nil", err)
			}
			if tc.foreignRoute {
				route, err = controlwire.NewRouteContract(core.Offering{Token: "foreign-tool"}, controlwire.RouteFamilyRegistrations)
				if err != nil {
					t.Fatalf("foreign route = %v, want nil", err)
				}
			}
			path, err := route.Path()
			if err != nil {
				t.Fatalf("path = %v, want nil", err)
			}
			key, err := seed.ControlNonce().IdempotencyKey()
			if err != nil {
				t.Fatalf("replay key = %v, want nil", err)
			}
			body := canonical
			if tc.empty {
				body = nil
			}
			reader := bytes.NewReader(body)
			request := httptest.NewRequest(http.MethodPost, path, reader)
			if tc.closeFailure {
				request.Body = &ownedCloseFailure{Reader: reader}
			}
			request.Header.Set(core.HTTPHeaderContentType().String(), "application/json")
			replay := key.String()
			if tc.foreignKey {
				replay = "foreign-operation"
			}
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), replay)
			support, err := controlwire.PublishedProtocolSupport()
			if err != nil {
				t.Fatalf("support = %v, want nil", err)
			}
			releases := 0
			var discarded controlwire.AccessToken
			call := controlwire.OwnedAuthorityJSONReceiveCall[*controlplane.AccessRegistrationRequest]{
				Receive: controlwire.AuthorityJSONReceiveCall{Call: fixtureSocketServerCall(t, request), Route: route, Authority: socketServer(t, support)},
				Release: func(decoded *controlplane.AccessRegistrationRequest) error {
					releases++
					discarded = decoded.Token
					return errors.Join(decoded.Token.Destroy(), tc.releaseErr)
				},
			}
			if tc.omitRelease {
				call.Release = nil
			}
			got, gotErr := controlwire.ReceiveOwnedRoutedJSON[controlplane.AccessRegistrationRequest, *controlplane.AccessRegistrationRequest](call)
			if !errors.Is(gotErr, tc.wantErr) || releases != tc.wantReleases {
				t.Fatalf("owned receive error/releases = %v/%d, want %v/%d", gotErr, releases, tc.wantErr, tc.wantReleases)
			}
			if tc.releaseErr != nil && !errors.Is(gotErr, tc.releaseErr) {
				t.Fatalf("cleanup error = %v, want %v", gotErr, tc.releaseErr)
			}
			if tc.omitRelease && reader.Len() != len(body) {
				t.Fatalf("unowned input remaining = %d, want %d", reader.Len(), len(body))
			}
			if gotErr != nil {
				if got.Body != nil || got.Replay.Validate() == nil || got.Assessment.Validate() == nil {
					t.Fatalf("refused receive = %+v, want zero body, replay and assessment", got)
				}
				if releases != 0 && !errors.Is(discarded.Validate(), core.ErrControlWireToken) {
					t.Fatalf("discarded token validation = %v, want destroyed token", discarded.Validate())
				}
			} else {
				if got.Body == nil {
					t.Fatal("accepted body = nil, want owned token")
				}
				defer got.Body.Token.Destroy()
				verifier, err := got.Body.Token.Verifier()
				wantVerifier, wantErr := seed.Token.Verifier()
				if err != nil || wantErr != nil || verifier != wantVerifier || got.Body.Build != seed.Build || got.Body.RequestNonce != seed.RequestNonce || got.Body.DeviceKey != seed.DeviceKey || got.Body.Installation != seed.Installation {
					t.Fatalf("accepted token/facts = %v/%v/%v, want exact source facts", verifier, err, wantErr)
				}
			}
			if err := seed.Token.Validate(); err != nil {
				t.Fatalf("source token after receive = %v, want independently owned live seed", err)
			}
		})
	}
}

type ownedCloseFailure struct{ *bytes.Reader }

func (*ownedCloseFailure) Close() error { return io.ErrClosedPipe }
