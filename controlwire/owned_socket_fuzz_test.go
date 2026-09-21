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
	"github.com/deliri/primitive/v2026/exchange"
)

// Both public ownership doors receive the mutated representation. The oracle
// independently decodes nominal facts, checks canonical closure and accounts for
// live versus destroyed token custody, including body-close failure.
func FuzzOwnedJSONAndRoutedReceiversSemanticClosure(f *testing.F) {
	seed := ownedRegistrationSeed(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("seed encoding = %v, want nil", err)
	}
	defer clear(canonical)
	route, err := seed.ControlRoute()
	if err != nil {
		f.Fatalf("route = %v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		f.Fatalf("path = %v, want nil", err)
	}
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		f.Fatalf("support = %v, want nil", err)
	}
	authority := socketServer(f, support)
	f.Add(canonical, seed.ControlNonce().String(), false)
	f.Add(canonical, "foreign-operation", false)
	f.Add(canonical, seed.ControlNonce().String(), true)
	f.Add([]byte{}, seed.ControlNonce().String(), false)
	f.Add(canonical[:len(canonical)-1], seed.ControlNonce().String(), false)
	f.Fuzz(func(t *testing.T, data []byte, key string, closeFailure bool) {
		var nominal controlplane.AccessRegistrationRequest
		decodeErr := nominal.UnmarshalJSON(data)
		if decodeErr == nil {
			defer func() {
				if err := nominal.Token.Destroy(); err != nil {
					t.Errorf("nominal.Token.Destroy() cleanup error = %v, want nil", err)
				}
			}()
		}
		for _, routed := range [...]bool{false, true} {
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
			if closeFailure {
				request.Body = &ownedCloseFailure{Reader: bytes.NewReader(data)}
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), key)
			releases := 0
			var discarded controlwire.AccessToken
			release := func(body *controlplane.AccessRegistrationRequest) error {
				releases++
				discarded = body.Token
				return body.Token.Destroy()
			}
			call := fixtureSocketServerCall(t, request)
			var got *controlplane.AccessRegistrationRequest
			var receiveErr error
			if routed {
				result, err := controlwire.ReceiveOwnedRoutedJSON[controlplane.AccessRegistrationRequest, *controlplane.AccessRegistrationRequest](controlwire.OwnedAuthorityJSONReceiveCall[*controlplane.AccessRegistrationRequest]{Receive: controlwire.AuthorityJSONReceiveCall{Call: call, Route: route, Authority: authority}, Release: release})
				got, receiveErr = result.Body, err
				if err != nil && (result.Replay.Validate() == nil || result.Assessment.Validate() == nil) {
					t.Fatalf("refused routed facts = %+v, want no replay or assessment", result)
				}
			} else {
				result, err := exchange.ReceiveOwnedJSON[controlplane.AccessRegistrationRequest, *controlplane.AccessRegistrationRequest](exchange.OwnedJSONReceiveCall[*controlplane.AccessRegistrationRequest]{Receive: exchange.JSONReceiveCall{Call: call, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}}, Release: release})
				got, receiveErr = result.Body, err
				if err != nil && result.IdempotencyKey != (exchange.IdempotencyKey{}) {
					t.Fatalf("refused exchange key = %v, want zero", result.IdempotencyKey)
				}
			}
			if got != nil {
				defer func() {
					if err := got.Token.Destroy(); err != nil {
						t.Errorf("got.Token.Destroy() cleanup error = %v, want nil", err)
					}
				}()
			}
			if receiveErr != nil {
				if got != nil || (!errors.Is(receiveErr, core.ErrExchangeContract) && !errors.Is(receiveErr, core.ErrControlWireContract)) {
					t.Fatalf("refused body/error = %v/%v, want zero and typed boundary refusal", got, receiveErr)
				}
				if releases > 1 || (releases == 1 && !errors.Is(discarded.Validate(), core.ErrControlWireToken)) {
					t.Fatalf("refused releases/token = %d/%v, want at most one destroyed token", releases, discarded.Validate())
				}
				if closeFailure && decodeErr == nil && key == nominal.ControlNonce().String() && !errors.Is(receiveErr, io.ErrClosedPipe) {
					t.Fatalf("close failure identity = %v, want ErrClosedPipe", receiveErr)
				}
				// Canonical bound input must either transfer custody or release it once.
				if decodeErr == nil && key == nominal.ControlNonce().String() && nominal.Build.Offering() == route.Offering() {
					if !closeFailure || releases != 1 {
						t.Fatalf("valid bound refusal/error/releases = %v/%d, want close failure and one release", receiveErr, releases)
					}
				}
				continue
			}
			if closeFailure || decodeErr != nil || got == nil || releases != 0 || got.Validate() != nil {
				t.Fatalf("admitted body/decode/releases = %v/%v/%d, want live validated custody", got, decodeErr, releases)
			}
			if routed && (got.ControlNonce().String() != key || got.Build.Offering() != route.Offering()) {
				t.Fatalf("routed admission nonce/offering = %v/%v, want exact header and route", got.ControlNonce(), got.Build.Offering())
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("accepted encoding = %v, want nil", err)
			}
			defer clear(encoded)
			want, err := nominal.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, want) {
				t.Fatalf("accepted canonical equality/error = %t/%v, want true,nil", bytes.Equal(encoded, want), err)
			}
			clear(want)
			var second controlplane.AccessRegistrationRequest
			if err := second.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("accepted roundtrip = %v, want nil", err)
			}
			defer func() {
				if err := second.Token.Destroy(); err != nil {
					t.Errorf("second.Token.Destroy() cleanup error = %v, want nil", err)
				}
			}()
			again, err := second.MarshalJSON()
			if err != nil || !bytes.Equal(again, encoded) {
				t.Fatalf("canonical second pass equality/error = %t/%v, want true,nil", bytes.Equal(again, encoded), err)
			}
			clear(again)
		}
	})
}
