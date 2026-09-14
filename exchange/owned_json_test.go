package exchange_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type ownedJSONBody struct {
	*bytes.Reader
	closeErr error
	closes   int
}

func (b *ownedJSONBody) Close() error { b.closes++; return b.closeErr }

func TestOwnedJSONReceiveLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		malformed            bool
		closeErr, releaseErr error
		wantReleases         int
	}{
		{name: "decoded custody transfers after successful close"},
		{name: "close refusal destroys decoded custody", closeErr: io.ErrClosedPipe, wantReleases: 1},
		{name: "cleanup refusal retains close and release errors", closeErr: io.ErrClosedPipe, releaseErr: io.ErrUnexpectedEOF, wantReleases: 1},
		{name: "empty input owns no decoded resource", malformed: true},
		{name: "empty input retains simultaneous close failure", malformed: true, closeErr: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{Offering: core.Offering{Token: "test-tool"}, AuthoritySeed: [32]byte{17}, DeviceSeed: [32]byte{23}})
			if err != nil {
				t.Fatalf("installation seed = %v, want nil", err)
			}
			defer clear(fixture.AuthorityPrivate)
			defer clear(fixture.DevicePrivate)
			token, err := controlwire.NewAccessToken([controlwire.AccessTokenBytes]byte{43})
			if err != nil {
				t.Fatalf("token seed = %v, want nil", err)
			}
			defer token.Destroy()
			nonce, err := controlwire.NewRequestNonce([32]byte{7})
			if err != nil {
				t.Fatalf("nonce = %v, want nil", err)
			}
			seed := controlplane.AccessRegistrationRequest{Token: token, Build: fixture.Build, DeviceKey: fixture.DevicePublic, Installation: fixture.Certificate.Body.Subject.DeviceID, RequestNonce: nonce, Revision: controlwire.Revision2026V1}
			canonical, err := seed.MarshalJSON()
			if err != nil {
				t.Fatalf("seed encoding = %v, want nil", err)
			}
			defer clear(canonical)
			wire := canonical
			if tc.malformed {
				wire = nil
			}
			body := &ownedJSONBody{Reader: bytes.NewReader(wire), closeErr: tc.closeErr}
			request := httptest.NewRequest(http.MethodPost, "/", body)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), nonce.String())
			releases := 0
			var discarded controlwire.AccessToken
			got, err := exchange.ReceiveOwnedJSON[controlplane.AccessRegistrationRequest, *controlplane.AccessRegistrationRequest](exchange.OwnedJSONReceiveCall[*controlplane.AccessRegistrationRequest]{
				Receive: exchange.JSONReceiveCall{Call: socketServerCall(t, request), Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}},
				Release: func(value *controlplane.AccessRegistrationRequest) error {
					releases++
					discarded = value.Token
					return errors.Join(value.Token.Destroy(), tc.releaseErr)
				},
			})
			if releases != tc.wantReleases || body.closes != 1 {
				t.Fatalf("release/close counts = %d/%d, want %d/1", releases, body.closes, tc.wantReleases)
			}
			if tc.closeErr == nil && !tc.malformed {
				if err != nil || got.Body == nil {
					t.Fatalf("accepted result = (%v,%v), want live owned body", got, err)
				}
				defer got.Body.Token.Destroy()
				observed, verifyErr := got.Body.Token.Verifier()
				want, wantErr := seed.Token.Verifier()
				if verifyErr != nil || wantErr != nil || observed != want || got.Body.Build != seed.Build {
					t.Fatalf("accepted token/build = %v/%v/%v, want exact seed", observed, verifyErr, wantErr)
				}
			} else {
				if !errors.Is(err, core.ErrExchangeContract) || got.Body != nil || got.IdempotencyKey != (exchange.IdempotencyKey{}) {
					t.Fatalf("rejected result = (%v,%v), want zero and typed exchange refusal", got, err)
				}
				if tc.closeErr != nil && !errors.Is(err, tc.closeErr) {
					t.Fatalf("close refusal = %v, want %v", err, tc.closeErr)
				}
				if tc.releaseErr != nil && !errors.Is(err, tc.releaseErr) {
					t.Fatalf("release refusal = %v, want %v", err, tc.releaseErr)
				}
				if releases > 0 && !errors.Is(discarded.Validate(), core.ErrControlWireToken) {
					t.Fatalf("discarded token = %v, want destroyed", discarded.Validate())
				}
			}
		})
	}
}
