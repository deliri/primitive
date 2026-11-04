package timematch

import (
	"bytes"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReviewRateLimitKeepsNativeDelayAcrossProviders(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{ProviderGoogle, ProviderMicrosoft, ProviderApple} {
		t.Run(provider.String(), func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
				w.Header().Set("Retry-After", "17")
				writeJSON(w, http.StatusTooManyRequests, `{}`)
			})
			connection := oauthConnection(t, provider)
			if provider == ProviderApple {
				connection = appleConnection(t)
			}
			got, err := client.ListCalendars(t.Context(), CalendarListRequest{Connection: connection, Policy: fixturePolicy(t)})
			var throttled ThrottledError
			want, durationErr := temporal.DurationFromSeconds(17)
			if durationErr != nil || !errors.Is(err, core.ErrTimeMatchResponse) || !errors.As(err, &throttled) || throttled.RetryAfter != want || len(got.Calendars) != 0 || len(fixture.record()) != 1 {
				t.Fatalf("ListCalendars() = (%d,%v), delay%v/calls%d, want zero/typed429/17seconds/one attempt", len(got.Calendars), err, throttled.RetryAfter, len(fixture.record()))
			}
		})
	}
}

func TestReviewInvitationDisclosurePrecedesEveryOutputByte(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		count    int
		disclose bool
		wantErr  error
	}{
		{name: "one recipient requires no disclosure", count: 1},
		{name: "multiple recipients require approval", count: 2, wantErr: core.ErrTimeMatchContract},
		{name: "approved multiple recipients are explicit", count: 2, disclose: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			invitation := invitationFixture(t)
			second, err := core.ParseOAuthEmail("second@example.test")
			if err != nil {
				t.Fatal(err)
			}
			invitation.Attendees = []core.OAuthEmail{invitation.Organizer, second}[:tc.count]
			invitation.DiscloseAttendees = tc.disclose
			var output bytes.Buffer
			err = WriteInvitation(&output, invitation)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || output.Len() != 0 {
					t.Fatalf("WriteInvitation() = (%dbytes,%v), want zero/%v", output.Len(), err, tc.wantErr)
				}
				return
			}
			if err != nil || strings.Count(output.String(), "ATTENDEE") != tc.count || !strings.Contains(output.String(), "UID:"+invitation.UID.String()+invitation.Organizer.String()[strings.LastIndex(invitation.Organizer.String(), "@"):]) {
				t.Fatalf("WriteInvitation() = (%dbytes,%v), want exact attendee count%d and qualified identity", output.Len(), err, tc.count)
			}
		})
	}
}

func TestAppleReviewSequenceArithmeticRefusesContradictoryProperty(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		values  []string
		changed bool
		want    uint32
		wantErr error
	}{
		{name: "absent sequence defaults to zero"},
		{name: "first significant revision increments", changed: true, want: 1},
		{name: "observed revision increments", values: []string{"7"}, changed: true, want: 8},
		{name: "exhausted unchanged revision stays stable", values: []string{strconv.Itoa(math.MaxInt32)}, want: math.MaxInt32},
		{name: "exhausted changed revision refuses", values: []string{strconv.Itoa(math.MaxInt32)}, changed: true, wantErr: core.ErrTimeMatchResponse},
		{name: "negative wire revision refuses", values: []string{"-1"}, wantErr: core.ErrTimeMatchResponse},
		{name: "conflicting duplicate cannot be reconciled by maximum", values: []string{"0", "7"}, changed: true, wantErr: core.ErrTimeMatchResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lines := []string{"DTSTART:20261104T100000Z"}
			for _, value := range tc.values {
				lines = append(lines, "SEQUENCE:"+value)
			}
			controlled := []string{lines[0]}
			if tc.changed {
				controlled[0] = "DTSTART:20261104T110000Z"
			}
			got, err := appleNextSequence(lines, controlled)
			if got != tc.want || (tc.wantErr == nil && err != nil) || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) {
				t.Fatalf("appleNextSequence() = (%d,%v), want (%d,%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
