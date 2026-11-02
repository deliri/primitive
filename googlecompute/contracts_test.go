package googlecompute

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleComputeInstanceStates(t *testing.T) {
	t.Parallel()
	for index, text := range []string{"PENDING", "PROVISIONING", "STAGING", "RUNNING", "STOPPING", "SUSPENDING", "SUSPENDED", "REPAIRING", "TERMINATED", "", "FUTURE"} {
		t.Run("state_"+text, func(t *testing.T) {
			t.Parallel()
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(nativeInstance(t, text)); err != nil {
					return
				}
			}))
			got, err := client.ReadInstance(t.Context(), InstanceReadRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
			if index < 9 {
				want := InstanceState(index + 1)
				if err != nil || got.State != want || got.Validate() != nil {
					t.Fatalf("ReadInstance(%q) = (%+v,%v), want state %v", text, got, err, want)
				}
			} else if !errors.Is(err, core.ErrGoogleComputeResponse) || got != (InstanceObservation{}) {
				t.Fatalf("ReadInstance(%q) = (%+v,%v), want zero,Response", text, got, err)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
		})
	}
}

func TestGoogleComputePathSegments(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, text string
		want       bool
	}{
		{name: "one byte resource", text: "a", want: true},
		{name: "decimal project number", text: "123456789", want: true},
		{name: "hyphenated project", text: "fixture-project", want: true},
		{name: "zonal coordinate", text: "northamerica-northeast2-a", want: true},
		{name: "provider operation punctuation", text: "operation-123_ABC", want: true},
		{name: "large segment has no invented quota", text: strings.Repeat("a", 16385), want: true},
		{name: "empty segment"},
		{name: "path traversal", text: ".."},
		{name: "separator", text: "a/b"},
		{name: "encoded separator", text: "a%2fb"},
		{name: "query injection", text: "a?x=1"},
		{name: "fragment injection", text: "a#x"},
		{name: "leading whitespace", text: " a"},
		{name: "trailing newline", text: "a\n"},
		{name: "non ASCII", text: "é"},
		{name: "NUL", text: "a\x00"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			resource, rErr := NewResourceName(row.text)
			operation, oErr := NewOperationName(row.text)
			if row.want {
				if rErr != nil || oErr != nil || resource.String() != row.text || operation.String() != row.text {
					t.Fatalf("segment admission = (%v,%v), want exact acceptance", rErr, oErr)
				}
			} else if !errors.Is(rErr, core.ErrGoogleComputeContract) || !errors.Is(oErr, core.ErrGoogleComputeContract) || resource != (ResourceName{}) || operation != (OperationName{}) {
				t.Fatalf("segment admission = (%v,%v), want zero,Contract", rErr, oErr)
			}
		})
	}
}

func TestGoogleComputeMalformedHTTP(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, media string
		status            int
	}{
		{name: "null instance", body: "null", media: "application/json"},
		{name: "empty body", media: "application/json"},
		{name: "truncated JSON", body: "{", media: "application/json"},
		{name: "wrong field type", body: `{"status":42}`, media: "application/json"},
		{name: "duplicate member", body: `{"status":"RUNNING","status":"TERMINATED"}`, media: "application/json"},
		{name: "multiple documents", body: `{} {}`, media: "application/json"},
		{name: "wrong media type", body: `{}`, media: "text/html"},
		{name: "redirect refused", body: `{}`, media: "application/json", status: http.StatusTemporaryRedirect},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", row.media)
				if row.status != 0 {
					w.Header().Set("Location", "https://compute.googleapis.com/other")
					w.WriteHeader(row.status)
				}
				if _, err := w.Write([]byte(row.body)); err != nil {
					return
				}
			}))
			got, err := client.ReadInstance(t.Context(), InstanceReadRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
			if !errors.Is(err, core.ErrGoogleComputeResponse) || got != (InstanceObservation{}) || calls.Load() != 1 {
				t.Fatalf("ReadInstance = (%+v,%v,%d calls), want zero,Response,1 call", got, err, calls.Load())
			}
		})
	}
}
