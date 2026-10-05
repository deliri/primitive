package cloudflare

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func imageDetailsFixture() imageDetailsWire {
	return imageDetailsWire{ID: "session/photo.jpg", Creator: "owner", Filename: "photo.jpg", Uploaded: "2026-10-05T12:00:00Z", Variants: []string{"https://imagedelivery.net/account/session/photo.jpg/public"}}
}

func TestImagesDetailsAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		change    func(*imageDetailsWire)
		wantReady bool
		wantErr   error
	}{
		{name: "uploaded image admits exact identity and delivery URL", wantReady: true},
		{name: "draft exists but cannot count as uploaded", change: func(w *imageDetailsWire) { w.Draft = true }},
		{name: "absent variants do not invent delivery", change: func(w *imageDetailsWire) { w.Variants = nil }},
		{name: "crossed image identity is refused", change: func(w *imageDetailsWire) { w.ID = "different" }, wantErr: core.ErrCloudflareBinding},
		{name: "absent upload time is refused", change: func(w *imageDetailsWire) { w.Uploaded = "" }, wantErr: core.ErrCloudflareResponse},
		{name: "invalid upload time is refused", change: func(w *imageDetailsWire) { w.Uploaded = "tomorrow" }, wantErr: core.ErrCloudflareResponse},
		{name: "HTTP variant is refused", change: func(w *imageDetailsWire) { w.Variants[0] = "http://imagedelivery.net/account/session/photo.jpg/public" }, wantErr: core.ErrCloudflareBinding},
		{name: "foreign delivery host is refused", change: func(w *imageDetailsWire) { w.Variants[0] = "https://attacker.example/account/session/photo.jpg/public" }, wantErr: core.ErrCloudflareBinding},
		{name: "crossed image variant is refused", change: func(w *imageDetailsWire) {
			w.Variants[0] = "https://imagedelivery.net/account/session/other.jpg/public"
		}, wantErr: core.ErrCloudflareBinding},
		{name: "embedded credentials are refused", change: func(w *imageDetailsWire) {
			w.Variants[0] = "https://person@imagedelivery.net/account/session/photo.jpg/public"
		}, wantErr: core.ErrCloudflareResponse},
		{name: "fragment does not become delivery authority", change: func(w *imageDetailsWire) { w.Variants[0] += "#other" }, wantErr: core.ErrCloudflareResponse},
		{name: "unasked query does not become delivery authority", change: func(w *imageDetailsWire) { w.Variants[0] += "?token=unexpected" }, wantErr: core.ErrCloudflareBinding},
		{name: "missing account path is refused", change: func(w *imageDetailsWire) { w.Variants[0] = "https://imagedelivery.net/" }, wantErr: core.ErrCloudflareBinding},
		{name: "missing variant is refused", change: func(w *imageDetailsWire) { w.Variants[0] = "https://imagedelivery.net/account/session/photo.jpg/" }, wantErr: core.ErrCloudflareBinding},
		{name: "nested variant path is refused", change: func(w *imageDetailsWire) { w.Variants[0] += "/suffix" }, wantErr: core.ErrCloudflareBinding},
		{name: "creator maximum accepted", change: func(w *imageDetailsWire) {
			w.Creator = strings.Repeat("a", core.CloudflareImageCreatorMaximumCharacters)
		}, wantReady: true},
		{name: "creator maximum plus one refused", change: func(w *imageDetailsWire) {
			w.Creator = strings.Repeat("a", core.CloudflareImageCreatorMaximumCharacters+1)
		}, wantErr: core.ErrCloudflareResponse},
		{name: "missing optional creator remains missing", change: func(w *imageDetailsWire) { w.Creator = "" }, wantReady: true},
		{name: "filename maximum accepted", change: func(w *imageDetailsWire) {
			w.Filename = strings.Repeat("a", core.CloudflareMultipartFilenameMaximumBytes)
		}, wantReady: true},
		{name: "filename maximum plus one refused", change: func(w *imageDetailsWire) {
			w.Filename = strings.Repeat("a", core.CloudflareMultipartFilenameMaximumBytes+1)
		}, wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := imageDetailsFixture()
			if tc.change != nil {
				tc.change(&wire)
			}
			yes := true
			body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDetailsWire]{Success: &yes, Result: wire})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/images/v1/session/photo.jpg") {
					t.Errorf("request=(%s,%s), want exact image GET", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			server, err := NewImagesServer(client, testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			})
			id, err := ParseImageID("session/photo.jpg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := server.Details(t.Context(), id, testPolicy())
			if !errors.Is(err, tc.wantErr) || calls != 1 || got.Ready() != tc.wantReady {
				t.Fatalf("Details=(%v,ready=%t,%d calls), want %v,ready=%t,one call", err, got.Ready(), calls, tc.wantErr, tc.wantReady)
			}
			if err == nil && (got.ID != id || got.Draft != wire.Draft || got.Creator != wire.Creator || got.Filename != wire.Filename || len(got.Variants) != len(wire.Variants)) {
				t.Fatal("admitted details differ from provider facts")
			}
			if err != nil && (got.ID != (ImageID{}) || got.Ready() || len(got.Variants) != 0) {
				t.Fatal("refused details leaked ready metadata")
			}
		})
	}
}

func TestImagesDeleteRequiresTypedAcceptanceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		success *bool
		issues  []APIIssue
		wantErr error
	}{
		{name: "explicit acceptance closes deletion", success: new(true)},
		{name: "missing acceptance cannot close deletion", wantErr: core.ErrCloudflareResponse},
		{name: "provider refusal remains refusal", success: new(false), issues: []APIIssue{{Code: 1000, Message: "denied"}}, wantErr: core.ErrCloudflareResponse},
		{name: "contradictory success cannot hide errors", success: new(true), issues: []APIIssue{{Code: 1000, Message: "denied"}}, wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDeleteWire]{Success: tc.success, Errors: tc.issues, Result: imageDeleteWire{body: []byte(`{}`)}})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodDelete {
					t.Errorf("method=%s,want DELETE", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			server, err := NewImagesServer(client, testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			})
			id, err := ParseImageID("session/photo.jpg")
			if err != nil {
				t.Fatal(err)
			}
			if err := server.Delete(t.Context(), id, testPolicy()); !errors.Is(err, tc.wantErr) || calls != 1 {
				t.Fatalf("Delete=(%v,%d calls), want %v and one", err, calls, tc.wantErr)
			}
		})
	}
}
