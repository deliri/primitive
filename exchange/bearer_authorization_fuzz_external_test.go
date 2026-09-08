package exchange_test

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzBearerAuthorizationTokenSemanticClosure(f *testing.F) {
	grammar := regexp.MustCompile(`^[A-Za-z0-9._~+/-]+=*$`)
	for _, token := range [][]byte{[]byte("A"), []byte("AZaz09-._~+/=="), bytes.Repeat([]byte{'Z'}, exchange.BearerAuthorizationTokenMaximumBytes)} {
		seed := exchange.BearerAuthorization{Token: token}
		if err := seed.Validate(); err != nil {
			f.Fatalf("typed token seed = %v, want nil", err)
		}
		// Raw token bytes use HeaderValue's exact wire projection. The bearer
		// constructor under test is called only inside the fuzz callback, so
		// a refuse-all constructor cannot hide as seed setup failure.
		value := mustHeaderValue(f, string(seed.Token))
		wire, err := value.Value()
		if err != nil {
			f.Fatalf("token wire seed = %v, want nil", err)
		}
		f.Add([]byte(wire), []byte(wire))
	}
	f.Add([]byte{}, []byte("A"))
	f.Add([]byte("="), []byte("="))
	f.Add([]byte("A=A"), []byte("A=="))
	f.Add([]byte("A"), []byte("Z"))
	f.Add([]byte("A"), []byte("AA"))
	f.Add([]byte{'A', 0xff}, []byte("A"))
	f.Add([]byte("A"), []byte("="))
	f.Add(bytes.Repeat([]byte{'Z'}, exchange.BearerAuthorizationTokenMaximumBytes+1), []byte("A"))
	f.Fuzz(func(t *testing.T, token, peer []byte) {
		if len(token) > exchange.BearerAuthorizationTokenMaximumBytes+1 || len(peer) > exchange.BearerAuthorizationTokenMaximumBytes+1 {
			return
		}
		wantToken := len(token) <= exchange.BearerAuthorizationTokenMaximumBytes && grammar.Match(token)
		wantPeer := len(peer) <= exchange.BearerAuthorizationTokenMaximumBytes && grammar.Match(peer)
		var wantErr, wantMatchErr error
		if !wantToken {
			wantErr = core.ErrExchangeContract
		}
		if !wantToken || !wantPeer {
			wantMatchErr = core.ErrExchangeContract
		}
		owned := bytes.Clone(token)
		authorization := exchange.BearerAuthorization{Token: owned}
		if gotErr := authorization.Validate(); !errors.Is(gotErr, wantErr) {
			t.Fatalf("token admission = %v, want %v", gotErr, wantErr)
		}
		gotMatch, matchErr := exchange.BearerAuthorizationMatches(authorization, exchange.BearerAuthorization{Token: peer})
		wantMatch := wantToken && wantPeer && bytes.Equal(token, peer)
		if gotMatch != wantMatch || !errors.Is(matchErr, wantMatchErr) {
			t.Fatalf("token match = (%t, %v), want (%t, %v)", gotMatch, matchErr, wantMatch, wantMatchErr)
		}
		if text := fmt.Sprintf("%#v", authorization); text != core.RedactedValueText {
			t.Fatalf("token diagnostics = %q, want %q", text, core.RedactedValueText)
		}
		header, gotErr := exchange.NewBearerAuthorizationHeader(authorization)
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("token header admission = %v, want %v", gotErr, wantErr)
		}
		if !bytes.Equal(owned, token) {
			t.Fatalf("token bytes preserved=%t, want true", bytes.Equal(owned, token))
		}
		if !wantToken {
			if header.Name != (core.HTTPHeaderName{}) || header.Values != nil {
				t.Fatalf("refused header = %v, want zero", header)
			}
			return
		}
		if err := header.Validate(); err != nil {
			t.Fatalf("header validation = %v, want nil", err)
		}
		if header.Name.String() != exchange.StandardHeaderAuthorization.String() || len(header.Values) != 1 {
			t.Fatalf("header name/count = %v/%d, want exact Authorization/1", header.Name, len(header.Values))
		}
		clear(owned)
		wire, err := header.Values[0].Value()
		wantWire := exchange.BearerAuthorizationScheme + " " + string(token)
		if err != nil || wire != wantWire {
			t.Fatalf("owned header wire = (%q, %v), want (%q, nil)", wire, err, wantWire)
		}
	})
}

func FuzzReceiveBearerAuthorizationSemanticClosure(f *testing.F) {
	grammar := regexp.MustCompile(`(?i:^` + regexp.QuoteMeta(exchange.BearerAuthorizationScheme) + ` )([A-Za-z0-9._~+/-]+=*)$`)
	for _, token := range [][]byte{[]byte("A"), []byte("AZaz09-._~+/=="), bytes.Repeat([]byte{'Z'}, exchange.BearerAuthorizationTokenMaximumBytes)} {
		seed := exchange.BearerAuthorization{Token: token}
		if err := seed.Validate(); err != nil {
			f.Fatalf("typed receive seed = %v, want nil", err)
		}
		header, err := exchange.NewBearerAuthorizationHeader(seed)
		if err != nil {
			f.Fatalf("header seed = %v, want nil", err)
		}
		wire, err := header.Values[0].Value()
		if err != nil {
			f.Fatalf("header wire seed = %v, want nil", err)
		}
		f.Add(wire, uint8(1))
		f.Add(strings.ToLower(exchange.BearerAuthorizationScheme)+wire[len(exchange.BearerAuthorizationScheme):], uint8(1))
	}
	for _, wire := range []string{"", exchange.BearerAuthorizationScheme + " ", exchange.BearerAuthorizationScheme + " =", exchange.BearerAuthorizationScheme + " A=A", exchange.BearerAuthorizationScheme + "\tA", exchange.BearerAuthorizationScheme + " A\r\n"} {
		f.Add(wire, uint8(1))
	}
	f.Add(exchange.BearerAuthorizationScheme+" A", uint8(0))
	f.Add(exchange.BearerAuthorizationScheme+" A", uint8(2))
	f.Add(exchange.BearerAuthorizationScheme+" "+strings.Repeat("A", exchange.BearerAuthorizationTokenMaximumBytes+1), uint8(1))
	f.Fuzz(func(t *testing.T, wire string, count uint8) {
		if len(wire) > exchange.BearerAuthorizationHeaderMaximumBytes+1 || count > 3 {
			return
		}
		matches := grammar.FindStringSubmatch(wire)
		wantAdmitted := count == 1 && len(wire) <= exchange.BearerAuthorizationHeaderMaximumBytes && len(matches) == 2
		var wantErr error
		if !wantAdmitted {
			wantErr = core.ErrExchangeRequest
		}
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		name := exchange.StandardHeaderAuthorization.String()
		for range count {
			request.Header.Add(name, wire)
		}
		recorder := httptest.NewRecorder()
		recorder.Code = 0
		call, err := exchange.NewSocketServerCall(recorder, request)
		if err != nil {
			t.Fatalf("socket fixture = %v, want nil", err)
		}
		got, gotErr := exchange.ReceiveBearerAuthorization(call)
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("receive admission = %v, want %v", gotErr, wantErr)
		}
		if recorder.Code != 0 || recorder.Body.Len() != 0 || len(recorder.Header()) != 0 || recorder.Flushed {
			t.Fatalf("response code/body/header/flush=%d/%d/%d/%t, want 0/0/0/false", recorder.Code, recorder.Body.Len(), len(recorder.Header()), recorder.Flushed)
		}
		if !wantAdmitted {
			if got.Token != nil || !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("refused credential = (%x, %v), want exact zero and request contract identity", got.Token, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil || string(got.Token) != matches[1] {
			t.Fatalf("received token = (%x, %v), want exact input capture", got.Token, err)
		}
		projected, err := exchange.NewBearerAuthorizationHeader(got)
		if err != nil {
			t.Fatalf("canonical header = %v, want nil", err)
		}
		canonical, err := projected.Values[0].Value()
		wantCanonical := exchange.BearerAuthorizationScheme + " " + matches[1]
		if err != nil || canonical != wantCanonical {
			t.Fatalf("canonical wire = (%q, %v), want (%q, nil)", canonical, err, wantCanonical)
		}
		clear(got.Token)
		if retained := request.Header.Get(name); retained != wire {
			t.Fatalf("request field after receiver clear = %q, want %q", retained, wire)
		}
		request.Header.Set(name, canonical)
		roundTrip, roundTripErr := exchange.ReceiveBearerAuthorization(call)
		if roundTripErr != nil || string(roundTrip.Token) != matches[1] {
			t.Fatalf("canonical receive = (%x, %v), want exact input token", roundTrip.Token, roundTripErr)
		}
		clear(roundTrip.Token)
	})
}
