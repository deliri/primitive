package gcsobjects

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGCSUploadDeadlineNeverWidensCallerAuthority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		date    string
		expiry  string
		ceiling int64
		want    int64
		extra   string
		wantErr bool
	}{
		{name: "exact one second deadline", date: "20300101T000000Z", expiry: "1", ceiling: 1, want: 1},
		{name: "fractional ceiling remains narrower", date: "20300101T000000Z", expiry: "1", ceiling: 2, want: 1},
		{name: "expiry at authorization deadline", date: "20300101T000000Z", expiry: "300", ceiling: 300, want: 300},
		{name: "one second below maximum", date: "20300101T000000Z", expiry: "604799", ceiling: 604800, want: 604799},
		{name: "seven day maximum", date: "20300101T000000Z", expiry: "604800", ceiling: 604800, want: 604800},
		{name: "zero second expiry", date: "20300101T000000Z", expiry: "0", ceiling: 300, wantErr: true},
		{name: "one second above maximum", date: "20300101T000000Z", expiry: "604801", ceiling: 604801, wantErr: true},
		{name: "one second beyond authorization", date: "20300101T000000Z", expiry: "301", ceiling: 300, wantErr: true},
		{name: "reversed authorization", date: "20300101T000000Z", expiry: "1", ceiling: -1, wantErr: true},
		{name: "negative expiry", date: "20300101T000000Z", expiry: "-1", ceiling: 300, wantErr: true},
		{name: "overflow expiry", date: "20300101T000000Z", expiry: "18446744073709551616", ceiling: 300, wantErr: true},
		{name: "fractional expiry", date: "20300101T000000Z", expiry: "1.5", ceiling: 300, wantErr: true},
		{name: "truncated date", date: "20300101", expiry: "300", ceiling: 300, wantErr: true},
		{name: "invalid calendar date", date: "20300230T000000Z", expiry: "300", ceiling: 300, wantErr: true},
		{name: "date beyond nanosecond representation", date: "99990101T000000Z", expiry: "300", ceiling: 300, wantErr: true},
		{name: "date before nanosecond representation", date: "00010101T000000Z", expiry: "300", ceiling: 300, wantErr: true},
		{name: "duplicate date", date: "20300101T000000Z", expiry: "300", ceiling: 300, extra: "&X-Goog-Date=20300101T000000Z", wantErr: true},
		{name: "duplicate expiry", date: "20300101T000000Z", expiry: "300", ceiling: 300, extra: "&X-Goog-Expires=300", wantErr: true},
		{name: "missing date", expiry: "300", ceiling: 300, wantErr: true},
		{name: "missing expiry", date: "20300101T000000Z", ceiling: 300, wantErr: true},
		{name: "malformed query escape", date: "20300101T000000Z", expiry: "300", ceiling: 300, extra: "&broken=%", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			const epoch = int64(1_893_456_000_000_000_000)
			target := "https://storage.googleapis.com/example-bucket/object?X-Goog-Date=" + url.QueryEscape(tc.date) + "&X-Goog-Expires=" + url.QueryEscape(tc.expiry) + tc.extra
			got, err := gcsUploadSignedExpiry(target, temporal.InstantFromNanoseconds(epoch+tc.ceiling*1_000_000_000))
			if tc.wantErr {
				if !errors.Is(err, core.ErrObjectStoreContract) || got != (temporal.Instant{}) {
					t.Fatalf("deadline = (%v,%v), want zero and contract refusal", got, err)
				}
				return
			}
			if err != nil || got != temporal.InstantFromNanoseconds(epoch+tc.want*1_000_000_000) {
				t.Fatalf("deadline = (%v,%v), want exact %d seconds and nil", got, err, tc.want)
			}
		})
	}
}

func TestGCSUploadSDKReportsItsActualSignatureDeadline(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		issuer, calls := gcsCapabilityIssuer(t, gcsCapabilityProviderOutcomeSigned)
		request := gcsCapabilityClockRequest(t)
		fractional, err := temporal.DurationFromNanoseconds(999_999_999)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpiresAt, err = request.ExpiresAt.Add(fractional)
		if err != nil {
			t.Fatal(err)
		}
		capability, err := IssueGCSUploadCapability(context.Background(), issuer, request)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := objectstore.NewUploadHTTPProjection(capability, request.Integrity, request.ContentType)
		if err != nil {
			t.Fatal(err)
		}
		transport, err := projection.Transport()
		if err != nil {
			t.Fatal(err)
		}
		want, err := request.ExpiresAt.Truncate(temporal.PrecisionSecond)
		if err != nil {
			t.Fatal(err)
		}
		if transport.ExpiresAt != want || calls.Load() != 1 {
			t.Fatalf("SDK deadline/calls = %v/%d, want exact signed deadline %v/1", transport.ExpiresAt, calls.Load(), want)
		}
	})
}

// This fuzz target projects controlled SDK output, not caller-supplied authority.
// The independent time.Time oracle checks the effective signature expiry; every
// refused output must retain a zero capability and the contract identity.
func FuzzGCSUploadSignedDeadline(f *testing.F) {
	issuer, _ := gcsCapabilityIssuer(f, gcsCapabilityProviderOutcomeSigned)
	request := gcsCapabilityClockRequest(f)
	issued, err := IssueGCSUploadCapability(context.Background(), issuer, request)
	if err != nil {
		f.Fatal(err)
	}
	projection, err := objectstore.NewUploadHTTPProjection(issued, request.Integrity, request.ContentType)
	if err != nil {
		f.Fatal(err)
	}
	transport, err := projection.Transport()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(transport.Target.String())
	f.Add("")
	f.Add("https://storage.googleapis.com/b/o?X-Goog-Date=%")
	ceiling := temporal.InstantFromNanoseconds(2_208_988_800_000_000_000)
	f.Fuzz(func(t *testing.T, raw string) {
		got, err := projectGCSUploadCapability(raw, ceiling)
		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreContract) || !got.IsZero() {
				t.Fatalf("refused projection = (%v,%v), want zero and contract identity", got, err)
			}
			return
		}
		browser, err := objectstore.NewUploadHTTPProjection(got, request.Integrity, request.ContentType)
		// Signed-header binding remains a separate gate; a structurally valid URL
		// without the checksum/create-only fields must never become transport.
		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatal(err)
			}
			return
		}
		output, err := browser.Transport()
		if err != nil {
			t.Fatal(err)
		}
		endpoint := output.Target.HTTPURL()
		query := endpoint.Query()
		stamp, err := time.Parse("20060102T150405Z", query.Get("X-Goog-Date"))
		if err != nil {
			t.Fatal(err)
		}
		duration, err := time.ParseDuration(query.Get("X-Goog-Expires") + "s")
		if err != nil {
			t.Fatal(err)
		}
		want, err := temporal.NewInstant(stamp.Add(duration))
		if err != nil {
			t.Fatal(err)
		}
		remaining, err := ceiling.Since(output.ExpiresAt)
		if err != nil || remaining.Nanoseconds() < 0 || duration <= 0 || duration > 7*24*time.Hour || output.ExpiresAt != want || got.Validate() != nil {
			t.Fatalf("admitted deadline = (%v,%v), want exact bounded signature expiry %v", output.ExpiresAt, err, want)
		}
	})
}
