package controlwire_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type clientCapabilityCase struct {
	name            string
	authority       string
	withoutExchange bool
	wantErr         []error
	wantValid       bool
}

func TestControlSocketClientCapabilityPressuresFortyOriginBoundaries(t *testing.T) {
	t.Parallel()

	cases := []clientCapabilityCase{
		{name: "valid minimum HTTP origin", authority: "http://a", wantValid: true},
		{name: "valid minimum HTTPS origin", authority: "https://a", wantValid: true},
		{name: "valid IPv4 origin", authority: "http://127.0.0.1", wantValid: true},
		{name: "valid IPv6 origin", authority: "http://[::1]", wantValid: true},
		{name: "valid minimum port origin", authority: "http://example.test:1", wantValid: true},
		{name: "valid maximum port origin", authority: "https://example.test:65535", wantValid: true},
		{name: "valid explicit HTTP default port", authority: "http://example.test:80", wantValid: true},
		{name: "valid explicit HTTPS default port", authority: "https://example.test:443", wantValid: true},
		{name: "valid normalized scheme origin", authority: "HTTPS://example.test", wantValid: true},
		{name: "valid nested DNS origin", authority: "https://control.example.test", wantValid: true},

		{name: "reject zero Exchange capability", authority: "https://example.test", withoutExchange: true, wantErr: []error{core.ErrControlWireContract, core.ErrExchangeContract}},
		{name: "reject one-segment path", authority: "https://example.test/base", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject control prefix path", authority: "https://example.test/v2026/control", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject escaped path", authority: "https://example.test/a%2Fb", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject root query", authority: "https://example.test/?page=1", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject path and query", authority: "https://example.test/base?page=1", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject empty query marker", authority: "https://example.test/?", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject encoded query value", authority: "https://example.test/?q=a%20b", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject repeated query values", authority: "https://example.test/?q=1&q=2", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "reject mounted registration route", authority: "https://example.test/v2026/control/tool/registrations", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},

		{name: "boundary HTTP minimum origin with root slash is accepted", authority: "http://a/", wantValid: true},
		{name: "boundary HTTPS minimum origin with root slash is accepted", authority: "https://a/", wantValid: true},
		{name: "boundary IPv4 origin with root slash is accepted", authority: "http://127.0.0.1/", wantValid: true},
		{name: "boundary IPv6 origin with root slash is accepted", authority: "https://[::1]/", wantValid: true},
		{name: "boundary minimum port with root slash is accepted", authority: "http://example.test:1/", wantValid: true},
		{name: "boundary maximum port with root slash is accepted", authority: "https://example.test:65535/", wantValid: true},
		{name: "boundary HTTP default port with root slash is accepted", authority: "http://example.test:80/", wantValid: true},
		{name: "boundary HTTPS default port with root slash is accepted", authority: "https://example.test:443/", wantValid: true},
		{name: "boundary normalized scheme with root slash is accepted", authority: "HTTPS://example.test/", wantValid: true},
		{name: "boundary nested DNS origin with root slash is accepted", authority: "https://control.example.test/", wantValid: true},
		{name: "boundary one path byte after root is rejected", authority: "https://example.test/a", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary two path bytes after root are rejected", authority: "https://example.test/ab", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary trailing slash after path is rejected", authority: "https://example.test/a/", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary doubled slash path is rejected", authority: "https://example.test//", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary dot path is rejected", authority: "https://example.test/.", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary escaped slash path is rejected", authority: "https://example.test/%2F", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary one query byte is rejected", authority: "https://example.test/?a", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary empty query value is rejected", authority: "https://example.test/?a=", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary query separator is rejected", authority: "https://example.test/?a=1&b=2", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "boundary path plus empty query is rejected", authority: "https://example.test/a?", wantErr: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
	}

	if got, want := len(cases), 40; got != want {
		t.Fatalf("client capability pressure cases = %d, want %d", got, want)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			authority, gotParseErr := core.ParseHTTPEndpoint(tc.authority)
			if gotParseErr != nil {
				t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want nil typed setup", tc.authority, gotParseErr)
			}
			exchangeClient := standardControlExchangeClient(t)
			if tc.withoutExchange {
				exchangeClient = exchange.Client{}
			}
			configuration := controlwire.ClientConfiguration{
				Exchange: exchangeClient, Authority: authority,
			}
			got, gotErr := controlwire.NewClient(configuration)
			if tc.wantValid {
				if gotErr != nil || got.Validate() != nil || configuration.Validate() != nil {
					t.Fatalf("controlwire.NewClient() = (%+v, %v), want validated client and nil", got, gotErr)
				}
				return
			}
			if got != (controlwire.Client{}) {
				t.Fatalf("controlwire.NewClient(rejected) = %+v, want zero client", got)
			}
			for _, wantErr := range tc.wantErr {
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("controlwire.NewClient() error = %v, want errors.Is %v", gotErr, wantErr)
				}
				if !errors.Is(configuration.Validate(), wantErr) {
					t.Fatalf("ClientConfiguration.Validate() error = %v, want errors.Is %v", configuration.Validate(), wantErr)
				}
			}
			if gotValidateErr := got.Validate(); !errors.Is(gotValidateErr, core.ErrControlWireContract) {
				t.Fatalf("zero Client.Validate() error = %v, want %v", gotValidateErr, core.ErrControlWireContract)
			}
		})
	}
}

func TestControlSocketClientCapabilityLayerTriad(t *testing.T) {
	t.Parallel()

	exchangeClient := standardControlExchangeClient(t)

	t.Run("positive exact origin produces one validated client capability", func(t *testing.T) {
		t.Parallel()

		authority := controlAuthorityEndpoint(t, "https://control.example.test")
		got, gotErr := controlwire.NewClient(controlwire.ClientConfiguration{
			Exchange: exchangeClient, Authority: authority,
		})
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("controlwire.NewClient(complete) = (%+v, %v), want validated client and nil", got, gotErr)
		}
	})

	t.Run("negative routed authority cannot become an origin capability", func(t *testing.T) {
		t.Parallel()

		authority := controlAuthorityEndpoint(t, "https://control.example.test/v2026/control/tool/check-ins")
		got, gotErr := controlwire.NewClient(controlwire.ClientConfiguration{
			Exchange: exchangeClient, Authority: authority,
		})
		if got != (controlwire.Client{}) || !errors.Is(gotErr, core.ErrControlWireRoute) ||
			!errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("controlwire.NewClient(routed authority) = (%+v, %v), want zero, route and exchange identities", got, gotErr)
		}
	})

	t.Run("neutral zero configuration creates no client authority", func(t *testing.T) {
		t.Parallel()

		got, gotErr := controlwire.NewClient(controlwire.ClientConfiguration{})
		if got != (controlwire.Client{}) || !errors.Is(gotErr, core.ErrControlWireContract) ||
			!errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("controlwire.NewClient(zero) = (%+v, %v), want zero, control-wire and exchange identities", got, gotErr)
		}
	})
}

func TestControlSocketServerCapabilityLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive published support produces one validated server capability", func(t *testing.T) {
		t.Parallel()

		support, supportErr := controlwire.PublishedProtocolSupport()
		if supportErr != nil {
			t.Fatalf("controlwire.PublishedProtocolSupport() error = %v, want nil", supportErr)
		}
		got, gotErr := controlwire.NewServer(controlwire.ServerConfiguration{Support: support})
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("controlwire.NewServer(published) = (%+v, %v), want validated server and nil", got, gotErr)
		}
	})

	t.Run("negative zero support cannot become an authority capability", func(t *testing.T) {
		t.Parallel()

		got, gotErr := controlwire.NewServer(controlwire.ServerConfiguration{})
		if got != (controlwire.Server{}) || !errors.Is(gotErr, core.ErrControlWireContract) {
			t.Fatalf("controlwire.NewServer(zero support) = (%+v, %v), want zero and %v", got, gotErr, core.ErrControlWireContract)
		}
	})

	t.Run("neutral zero server carries no protocol authority", func(t *testing.T) {
		t.Parallel()

		var got controlwire.Server
		if gotValidateErr := got.Validate(); !errors.Is(gotValidateErr, core.ErrControlWireContract) {
			t.Fatalf("zero Server.Validate() error = %v, want %v", gotValidateErr, core.ErrControlWireContract)
		}
	})
}

func TestControlSocketSideCapabilitiesExcludeOppositePeerAuthority(t *testing.T) {
	t.Parallel()

	contextType := reflect.TypeFor[context.Context]()
	requestType := reflect.TypeFor[controlplane.RegistrationRequest]()
	responseType := reflect.TypeFor[controlplane.ResponseProjection[controlplane.RegistrationDocument]]()
	cases := []struct {
		name                     string
		structure                reflect.Type
		wantFieldTypes           []reflect.Type
		wantUnexportedFieldCount int
	}{
		{
			name:      "client configuration owns only Exchange and authority origin",
			structure: reflect.TypeFor[controlwire.ClientConfiguration](),
			wantFieldTypes: []reflect.Type{
				reflect.TypeFor[exchange.Client](), reflect.TypeFor[core.HTTPEndpoint](),
			},
		},
		{
			name:                     "opaque client closes exactly one private client configuration",
			structure:                reflect.TypeFor[controlwire.Client](),
			wantFieldTypes:           []reflect.Type{reflect.TypeFor[controlwire.ClientConfiguration]()},
			wantUnexportedFieldCount: 1,
		},
		{
			name:           "server configuration owns only published protocol support",
			structure:      reflect.TypeFor[controlwire.ServerConfiguration](),
			wantFieldTypes: []reflect.Type{reflect.TypeFor[controlwire.ProtocolSupport]()},
		},
		{
			name:                     "opaque server closes exactly one private server configuration",
			structure:                reflect.TypeFor[controlwire.Server](),
			wantFieldTypes:           []reflect.Type{reflect.TypeFor[controlwire.ServerConfiguration]()},
			wantUnexportedFieldCount: 1,
		},
		{
			name:           "client send call carries context body and client capability only",
			structure:      reflect.TypeFor[controlwire.ClientJSONCall[controlplane.RegistrationRequest]](),
			wantFieldTypes: []reflect.Type{contextType, requestType, reflect.TypeFor[controlwire.Client]()},
		},
		{
			name:      "server receive call carries request route and server capability only",
			structure: reflect.TypeFor[controlwire.AuthorityJSONReceiveCall](),
			wantFieldTypes: []reflect.Type{
				reflect.TypeFor[*http.Request](), reflect.TypeFor[controlwire.RouteContract](),
				reflect.TypeFor[controlwire.Server](),
			},
		},
		{
			name:      "server write call carries writer body and server capability only",
			structure: reflect.TypeFor[controlwire.ControlJSONWriteCall[controlplane.ResponseProjection[controlplane.RegistrationDocument]]](),
			wantFieldTypes: []reflect.Type{
				reflect.TypeFor[http.ResponseWriter](), responseType, reflect.TypeFor[controlwire.Server](),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotFieldTypes := make([]reflect.Type, 0, tc.structure.NumField())
			gotUnexportedFieldCount := 0
			for field := range tc.structure.Fields() {
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

func standardControlExchangeClient(t testing.TB) exchange.Client {
	t.Helper()

	client, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatalf("exchange.NewStandardClient() error = %v, want nil", err)
	}
	return client
}

func controlAuthorityEndpoint(t testing.TB, value string) core.HTTPEndpoint {
	t.Helper()

	endpoint, err := core.ParseHTTPEndpoint(value)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want nil", value, err)
	}
	return endpoint
}
