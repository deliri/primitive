package timematch

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type matrixClass string

const (
	classValid    matrixClass = "valid"
	classInvalid  matrixClass = "invalid"
	classBoundary matrixClass = "boundary"
)

func nanos(t testing.TB, text string, delta int64) temporal.Instant {
	t.Helper()
	base, err := at(t, text).Nanoseconds()
	if err != nil {
		t.Fatalf("Nanoseconds() error = %v, want nil", err)
	}
	return temporal.InstantFromNanoseconds(base + delta)
}

// TestAvailabilityValidateHostileTable pressures the availability classifier
// that every provider projection and the browser projection must pass.
// Window is [09:00, 17:00) UTC; each row changes one load-bearing fact.
func TestAvailabilityValidateHostileTable(t *testing.T) {
	t.Parallel()
	const start, end = "2026-11-02T09:00:00Z", "2026-11-02T17:00:00Z"
	w := func(t *testing.T) temporal.IntervalBounds { return window(t, start, end) }
	span := func(t *testing.T, from, to string, fromDelta, toDelta int64) temporal.IntervalBounds {
		return temporal.IntervalBounds{Start: nanos(t, from, fromDelta), End: nanos(t, to, toDelta)}
	}
	known := func(id string, busy ...temporal.IntervalBounds) CalendarAvailability {
		return CalendarAvailability{ID: CalendarID{value: id}, State: AvailabilityKnown, Busy: busy}
	}
	cases := []struct {
		name    string
		class   matrixClass
		build   func(t *testing.T) Availability
		wantErr bool
	}{
		{name: "one known calendar with no busy time", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a")}}
		}},
		{name: "one unknown calendar without facts", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}, State: AvailabilityUnknown}}}
		}},
		{name: "busy strictly inside the window", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z"))}}
		}},
		{name: "busy equal to the whole window", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", w(t))}}
		}},
		{name: "mixed known and unknown calendars", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a"), {ID: CalendarID{value: "b"}, State: AvailabilityUnknown}, known("c", window(t, "2026-11-02T12:00:00Z", "2026-11-02T13:00:00Z"))}}
		}},
		{name: "two sorted disjoint busy spans", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z"), window(t, "2026-11-02T13:00:00Z", "2026-11-02T14:00:00Z"))}}
		}},
		{name: "same busy span on two calendars is independent", class: classValid, build: func(t *testing.T) Availability {
			busy := window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z")
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", busy), known("b", busy)}}
		}},
		{name: "apple collection url is an admitted calendar id", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("https://p42-caldav.icloud.com/1/calendars/home/")}}
		}},
		{name: "dst transition day in utc has no local gap", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: window(t, "2026-11-01T04:00:00Z", "2026-11-01T08:00:00Z"), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-01T05:30:00Z", "2026-11-01T06:30:00Z"))}}
		}},
		{name: "busy across midnight utc inside a two-day window", class: classValid, build: func(t *testing.T) Availability {
			return Availability{Window: window(t, "2026-11-02T00:00:00Z", "2026-11-04T00:00:00Z"), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T23:00:00Z", "2026-11-03T01:00:00Z"))}}
		}},

		{name: "no calendars", class: classInvalid, build: func(t *testing.T) Availability { return Availability{Window: w(t)} }, wantErr: true},
		{name: "duplicate calendar id", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a"), known("a")}}
		}, wantErr: true},
		{name: "unknown calendar carrying busy facts", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}, State: AvailabilityUnknown, Busy: []temporal.IntervalBounds{window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z")}}}}
		}, wantErr: true},
		{name: "reversed busy span", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T11:00:00Z", "2026-11-02T10:00:00Z"))}}
		}, wantErr: true},
		{name: "empty busy span", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T10:00:00Z", "2026-11-02T10:00:00Z"))}}
		}, wantErr: true},
		{name: "unsorted busy spans", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T13:00:00Z", "2026-11-02T14:00:00Z"), window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z"))}}
		}, wantErr: true},
		{name: "empty calendar id", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("")}}
		}, wantErr: true},
		{name: "calendar id with whitespace", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a b")}}
		}, wantErr: true},
		{name: "unset busy bounds", class: classInvalid, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", temporal.IntervalBounds{})}}
		}, wantErr: true},
		{name: "unset window", class: classInvalid, build: func(t *testing.T) Availability { return Availability{Calendars: []CalendarAvailability{known("a")}} }, wantErr: true},

		{name: "busy start one nanosecond before window", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, start, "2026-11-02T10:00:00Z", -1, 0))}}
		}, wantErr: true},
		{name: "busy start exactly at window start", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, start, "2026-11-02T10:00:00Z", 0, 0))}}
		}},
		{name: "busy start one nanosecond after window start", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, start, "2026-11-02T10:00:00Z", 1, 0))}}
		}},
		{name: "busy start at the earliest representable instant", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(-1 << 63), End: at(t, "2026-11-02T10:00:00Z")})}}
		}, wantErr: true},
		{name: "busy end one nanosecond before window end", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, "2026-11-02T16:00:00Z", end, 0, -1))}}
		}},
		{name: "busy end exactly at window end", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, "2026-11-02T16:00:00Z", end, 0, 0))}}
		}},
		{name: "busy end one nanosecond after window end", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, "2026-11-02T16:00:00Z", end, 0, 1))}}
		}, wantErr: true},
		{name: "busy end at the latest representable instant", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", temporal.IntervalBounds{Start: at(t, "2026-11-02T16:00:00Z"), End: temporal.InstantFromNanoseconds(1<<63 - 1)})}}
		}, wantErr: true},
		{name: "successor overlapping by one nanosecond", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z"), span(t, "2026-11-02T11:00:00Z", "2026-11-02T12:00:00Z", -1, 0))}}
		}, wantErr: true},
		{name: "successor touching without merge", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z"), window(t, "2026-11-02T11:00:00Z", "2026-11-02T12:00:00Z"))}}
		}, wantErr: true},
		{name: "successor after a one nanosecond gap", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", window(t, "2026-11-02T10:00:00Z", "2026-11-02T11:00:00Z"), span(t, "2026-11-02T11:00:00Z", "2026-11-02T12:00:00Z", 1, 0))}}
		}},
		{name: "successor at the far end of the window", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{known("a", span(t, start, start, 0, 1), span(t, end, end, -1, 0))}}
		}},
		{name: "window length zero", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: window(t, start, start), Calendars: []CalendarAvailability{known("a")}}
		}, wantErr: true},
		{name: "window length one nanosecond", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: span(t, start, start, 0, 1), Calendars: []CalendarAvailability{known("a")}}
		}},
		{name: "window length minus one nanosecond", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: span(t, start, start, 0, -1), Calendars: []CalendarAvailability{known("a")}}
		}, wantErr: true},
		{name: "window spanning centuries in representable range", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: window(t, "1700-01-01T00:00:00Z", "2200-01-01T00:00:00Z"), Calendars: []CalendarAvailability{known("a")}}
		}},
		{name: "lowest valid state known", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}, State: AvailabilityKnown}}}
		}},
		{name: "highest valid state unknown", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}, State: AvailabilityUnknown}}}
		}},
		{name: "state immediately above the domain", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}, State: AvailabilityUnknown + 1}}}
		}, wantErr: true},
		{name: "zero state", class: classBoundary, build: func(t *testing.T) Availability {
			return Availability{Window: w(t), Calendars: []CalendarAvailability{{ID: CalendarID{value: "a"}}}}
		}, wantErr: true},
	}
	counts := map[matrixClass]int{}
	for _, tc := range cases {
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.build(t).Validate()
			if tc.wantErr != (err != nil) || err != nil && !errors.Is(err, core.ErrTimeMatchContract) && !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("Availability.Validate() error = %v, want error %t with TimeMatch or Temporal identity", err, tc.wantErr)
			}
		})
	}
	if counts[classValid] != 10 || counts[classInvalid] != 10 || counts[classBoundary] != 20 {
		t.Fatalf("hostile table classes = %v, want 10 valid, 10 invalid, 20 boundary", counts)
	}
}

func TestScalarDomainsExhaustBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		check func() error
		want  bool
	}{
		{name: "event id at minimum length five", check: func() error { _, err := ParseEventID("abcde"); return err }, want: true},
		{name: "event id one below minimum", check: func() error { _, err := ParseEventID("abcd"); return err }},
		{name: "event id at maximum length 1024", check: func() error { _, err := ParseEventID(strings.Repeat("v", 1024)); return err }, want: true},
		{name: "event id one above maximum", check: func() error { _, err := ParseEventID(strings.Repeat("v", 1025)); return err }},
		{name: "event id letter w is outside base32hex", check: func() error { _, err := ParseEventID("abcdw"); return err }},
		{name: "event id uppercase is outside base32hex", check: func() error { _, err := ParseEventID("ABCDE"); return err }},
		{name: "event id hyphen is outside base32hex", check: func() error { _, err := ParseEventID("abc-de"); return err }},
		{name: "secret lowest visible byte", check: func() error { _, err := ParseSecret([]byte{0x21}); return err }, want: true},
		{name: "secret highest visible byte", check: func() error { _, err := ParseSecret([]byte{0x7e}); return err }, want: true},
		{name: "secret space", check: func() error { _, err := ParseSecret([]byte("a b")); return err }},
		{name: "secret delete byte", check: func() error { _, err := ParseSecret([]byte{0x7f}); return err }},
		{name: "secret empty", check: func() error { _, err := ParseSecret(nil); return err }},
		{name: "secret newline injection", check: func() error { _, err := ParseSecret([]byte("a\r\nX-Evil: 1")); return err }},
		{name: "apple account with colon breaks basic user-id", check: func() error { _, err := ParseAppleAccount("a:b@icloud.com"); return err }},
		{name: "apple account email", check: func() error { _, err := ParseAppleAccount("a@icloud.com"); return err }, want: true},
		{name: "summary with unicode", check: func() error { _, err := ParseEventSummary("Rendez-vous café ☕"); return err }, want: true},
		{name: "summary with newline", check: func() error { _, err := ParseEventSummary("a\nb"); return err }},
		{name: "summary with invalid utf8", check: func() error { _, err := ParseEventSummary("a\xffb"); return err }},
		{name: "summary empty", check: func() error { _, err := ParseEventSummary(""); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.check()
			if tc.want != (err == nil) || err != nil && !errors.Is(err, core.ErrTimeMatchContract) {
				t.Fatalf("check() error = %v, want accepted %t or ErrTimeMatchContract", err, tc.want)
			}
		})
	}
}

func TestRequestShapesRefuseUnsafeCombinations(t *testing.T) {
	t.Parallel()
	retrying := func(t testing.TB) exchange.OperationPolicy {
		p := fixturePolicy(t)
		p.Retry.MaximumAttempts = 2
		return p
	}
	following := func(t testing.TB) exchange.OperationPolicy {
		p := fixturePolicy(t)
		p.Redirect = exchange.RedirectPolicy{Mode: exchange.RedirectFollowSameOrigin, MaximumHops: 1}
		return p
	}
	cases := []struct {
		name    string
		request func(t *testing.T) core.Validatable
		want    bool
	}{
		{name: "oauth connection with token", request: func(t *testing.T) core.Validatable { return oauthConnection(t, ProviderGoogle) }, want: true},
		{name: "apple connection with account and password", request: func(t *testing.T) core.Validatable { return appleConnection(t) }, want: true},
		{name: "oauth connection carrying apple password", request: func(t *testing.T) core.Validatable {
			c := oauthConnection(t, ProviderMicrosoft)
			c.ApplePassword = secret(t, "x")
			return c
		}},
		{name: "apple connection carrying oauth token", request: func(t *testing.T) core.Validatable {
			c := appleConnection(t)
			c.AccessToken = secret(t, "x")
			return c
		}},
		{name: "apple connection without password", request: func(t *testing.T) core.Validatable { c := appleConnection(t); c.ApplePassword = Secret{}; return c }},
		{name: "connection for future provider", request: func(t *testing.T) core.Validatable {
			return Connection{Provider: ProviderApple + 1, AccessToken: secret(t, "x")}
		}},
		{name: "availability with duplicate calendars", request: func(t *testing.T) core.Validatable {
			return AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T10:00:00Z"), Calendars: []CalendarID{calendarID(t, "a"), calendarID(t, "a")}, Policy: fixturePolicy(t)}
		}},
		{name: "availability with retrying policy", request: func(t *testing.T) core.Validatable {
			return AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T10:00:00Z"), Calendars: []CalendarID{calendarID(t, "a")}, Policy: retrying(t)}
		}},
		{name: "availability following redirects with a credential", request: func(t *testing.T) core.Validatable {
			return AvailabilityRequest{Connection: oauthConnection(t, ProviderGoogle), Window: window(t, "2026-11-02T09:00:00Z", "2026-11-02T10:00:00Z"), Calendars: []CalendarID{calendarID(t, "a")}, Policy: following(t)}
		}},
		{name: "delete across providers", request: func(t *testing.T) core.Validatable {
			return EventDelete{Connection: oauthConnection(t, ProviderGoogle), Ref: EventRef{Provider: ProviderMicrosoft, Calendar: calendarID(t, "a"), Resource: "x"}, Policy: fixturePolicy(t)}
		}},
		{name: "grant visitor with refresh", request: func(t *testing.T) core.Validatable {
			return Grant{Provider: ProviderGoogle, Lifetime: LifetimeVisitor, Access: AccessAvailability, AccessToken: secret(t, "a"), RefreshToken: secret(t, "r"), ExpiresAt: at(t, "2026-11-02T10:00:00Z")}
		}},
		{name: "grant for apple", request: func(t *testing.T) core.Validatable {
			return Grant{Provider: ProviderApple, Lifetime: LifetimeBusiness, Access: AccessAvailability, AccessToken: secret(t, "a"), ExpiresAt: at(t, "2026-11-02T10:00:00Z")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.request(t).Validate()
			if tc.want != (err == nil) {
				t.Fatalf("Validate() error = %v, want accepted %t", err, tc.want)
			}
		})
	}
}

func TestSecretCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive close zeroes every shared copy", func(t *testing.T) {
		t.Parallel()
		original := secret(t, "token-value")
		backing := original.value
		copied := original
		if err := original.Close(); err != nil || string(backing) != strings.Repeat("\x00", len("token-value")) || original.IsSet() {
			t.Fatalf("Close() = %v backing %q, want zeroed shared bytes", err, backing)
		}
		if string(copied.value) == "token-value" {
			t.Fatal("copied Secret still holds the token after Close, want shared destruction")
		}
	})
	t.Run("negative formatting and json never reveal the value", func(t *testing.T) {
		t.Parallel()
		value := secret(t, "token-value")
		grant := Grant{AccessToken: value, RefreshToken: secret(t, "refresh-value")}
		account, _ := ParseAppleAccount("visitor@icloud.com")
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
			text := fmt.Sprintf(verb, value) + fmt.Sprintf(verb, grant) + fmt.Sprintf(verb, account)
			if strings.Contains(text, "token-value") || strings.Contains(text, "refresh-value") || strings.Contains(text, "visitor@icloud.com") {
				t.Fatalf("fmt %s = %q, want redacted", verb, text)
			}
		}
		if encoded, err := json.Marshal(struct{ S Secret }{value}); err == nil || strings.Contains(string(encoded), "token-value") {
			t.Fatalf("json.Marshal(Secret) = (%q, %v), want refusal", encoded, err)
		}
		if encoded, err := json.Marshal(BrowserAppleConnection{Account: account, Password: value}); err == nil || len(encoded) != 0 {
			t.Fatalf("json.Marshal(BrowserAppleConnection) = (%q, %v), want refusal", encoded, err)
		}
	})
	t.Run("neutral closing nil or zero values creates no state", func(t *testing.T) {
		t.Parallel()
		var nilSecret *Secret
		var zero Secret
		var grant Grant
		if err := nilSecret.Close(); !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("(*Secret)(nil).Close() = %v, want %v", err, core.ErrTimeMatchContract)
		}
		if err := errors.Join(zero.Close(), grant.Close()); err != nil || zero.IsSet() || grant.AccessToken.IsSet() {
			t.Fatalf("zero Close() = %v, want nil and still unset", err)
		}
	})
}
