package instagram

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func testPolicy(t testing.TB) exchange.OperationPolicy {
	t.Helper()
	total, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	return exchange.OperationPolicy{OperationTimeout: total, AttemptTimeout: attempt}
}

func testUser(t testing.TB) UserID {
	t.Helper()
	user, err := ParseUserID("17841432921003907")
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestParseUserIDAdmitsOnlyGraphDigits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{"17841432921003907", true},
		{"1", true},
		{strings.Repeat("9", 32), true},
		{strings.Repeat("9", 33), false},
		{"", false},
		{"1784143292100390a", false},
		{"-1", false},
		{" 1", false},
		{"1/media", false},
		{"１", false},
	} {
		_, err := ParseUserID(tc.raw)
		if tc.want != (err == nil) {
			t.Fatalf("ParseUserID(%q) = %v, want valid=%v", tc.raw, err, tc.want)
		}
		if err != nil && !errors.Is(err, core.ErrInstagramContract) {
			t.Fatalf("ParseUserID(%q) cause %v", tc.raw, err)
		}
	}
	if (UserID{}).Validate() == nil {
		t.Fatal("zero UserID validated")
	}
}

func TestMediaListEndpointIsExactAndBounded(t *testing.T) {
	t.Parallel()
	user, policy := testUser(t), testPolicy(t)
	for _, tc := range []struct {
		limit uint8
		want  string
	}{
		{0, ""},
		{1, "https://graph.facebook.com/v21.0/17841432921003907/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp&limit=1"},
		{12, "https://graph.facebook.com/v21.0/17841432921003907/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp&limit=12"},
		{49, "https://graph.facebook.com/v21.0/17841432921003907/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp&limit=49"},
		{50, "https://graph.facebook.com/v21.0/17841432921003907/media?fields=id,media_type,media_url,thumbnail_url,permalink,caption,timestamp&limit=50"},
		{51, ""},
		{255, ""},
	} {
		endpoint, err := MediaListRequest{User: user, Limit: tc.limit, Policy: policy}.Endpoint()
		if tc.want == "" {
			if !errors.Is(err, core.ErrInstagramContract) {
				t.Fatalf("limit %d admitted: %v", tc.limit, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("limit %d: %v", tc.limit, err)
		}
		address := endpoint.HTTPURL()
		if got := address.String(); got != tc.want {
			t.Fatalf("limit %d endpoint %q, want %q", tc.limit, got, tc.want)
		}
		if strings.Contains(address.RawQuery, "access_token") {
			t.Fatal("the token must never be part of the URL")
		}
	}
	if _, err := (MediaListRequest{Limit: 12, Policy: policy}).Endpoint(); !errors.Is(err, core.ErrInstagramContract) {
		t.Fatalf("missing user admitted: %v", err)
	}
	if _, err := (MediaListRequest{User: user, Limit: 12}).Endpoint(); !errors.Is(err, core.ErrInstagramContract) {
		t.Fatalf("missing policy admitted: %v", err)
	}
}

const (
	reelImage  = "https://scontent-yyz1-1.cdninstagram.com/v/t51.71878-15/frame.jpg?oe=6A0B1C2D"
	reelLink   = "https://www.instagram.com/reel/DQabc123/"
	photoImage = "https://scontent.xx.fbcdn.net/v/t51.2885-15/photo.jpg"
)

func validWire() mediaWire {
	return mediaWire{ID: "18123456789012345", MediaType: "VIDEO", ThumbnailURL: reelImage, MediaURL: "https://scontent-yyz1-1.cdninstagram.com/o1/v/video.mp4", Permalink: reelLink, Caption: "Their pizza takes 90 seconds.\nTheir website takes almost nine.", Timestamp: "2026-10-27T15:04:05+0000"}
}

func project(t testing.TB, items ...mediaWire) (MediaPage, error) {
	t.Helper()
	return mediaListWire{Data: items}.page(core.InstagramMediaPageMaximum)
}

func TestMediaProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive: a reel shows its cover, a photo shows itself, in Meta's order", func(t *testing.T) {
		t.Parallel()
		photo := validWire()
		photo.ID, photo.MediaType, photo.MediaURL, photo.ThumbnailURL = "17999", "IMAGE", photoImage, ""
		carousel := validWire()
		carousel.ID, carousel.MediaType, carousel.MediaURL, carousel.ThumbnailURL = "17998", "CAROUSEL_ALBUM", photoImage, ""
		page, err := project(t, validWire(), photo, carousel)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Media) != 3 || page.Media[0].Kind != MediaKindVideo || page.Media[1].Kind != MediaKindImage || page.Media[2].Kind != MediaKindCarousel {
			t.Fatalf("kinds or order changed: %+v", page.Media)
		}
		if image := page.Media[0].Image.HTTPURL(); !page.Media[0].HasImage || image.String() != reelImage {
			t.Fatalf("a reel must show its cover frame, got %q", image.String())
		}
		if image := page.Media[1].Image.HTTPURL(); image.String() != photoImage {
			t.Fatalf("a photo must show itself, got %q", image.String())
		}
		published, err := page.Media[0].Published.RFC3339()
		if err != nil || published != "2026-10-27T15:04:05Z" {
			t.Fatalf("timestamp %q, %v", published, err)
		}
		if page.Media[0].Caption != validWire().Caption {
			t.Fatal("caption text changed")
		}
	})
	t.Run("negative: one bad post refuses the whole page", func(t *testing.T) {
		t.Parallel()
		bad := validWire()
		bad.Permalink = "https://instagram.com.evil.example/reel/x/"
		page, err := project(t, validWire(), bad)
		if !errors.Is(err, core.ErrInstagramBinding) || !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
			t.Fatalf("a foreign permalink was admitted or partially kept: %v %+v", err, page)
		}
	})
	t.Run("neutral: an empty account is an empty page, and a withheld cover is reported, not invented", func(t *testing.T) {
		t.Parallel()
		page, err := project(t)
		if err != nil || len(page.Media) != 0 {
			t.Fatalf("empty data: %v %+v", err, page)
		}
		withheld := validWire()
		withheld.ThumbnailURL = ""
		page, err = project(t, withheld)
		if err != nil || page.Media[0].HasImage || page.Media[0].Image != (core.HTTPEndpoint{}) {
			t.Fatalf("a withheld cover must stay absent: %v %+v", err, page.Media)
		}
	})
}

func TestMediaProjectionRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*mediaWire)
		want   error
	}{
		{"empty id", func(w *mediaWire) { w.ID = "" }, core.ErrInstagramResponse},
		{"non-numeric id", func(w *mediaWire) { w.ID = "1/../2" }, core.ErrInstagramResponse},
		{"oversized id", func(w *mediaWire) { w.ID = strings.Repeat("1", 65) }, core.ErrInstagramResponse},
		{"unknown media type", func(w *mediaWire) { w.MediaType = "STORY" }, core.ErrInstagramResponse},
		{"lowercase media type", func(w *mediaWire) { w.MediaType = "video" }, core.ErrInstagramResponse},
		{"empty media type", func(w *mediaWire) { w.MediaType = "" }, core.ErrInstagramResponse},
		{"RFC 3339 colon offset", func(w *mediaWire) { w.Timestamp = "2026-10-27T15:04:05+00:00" }, core.ErrInstagramResponse},
		{"missing timestamp", func(w *mediaWire) { w.Timestamp = "" }, core.ErrInstagramResponse},
		{"missing permalink", func(w *mediaWire) { w.Permalink = "" }, core.ErrInstagramResponse},
		{"plain-HTTP permalink", func(w *mediaWire) { w.Permalink = "http://www.instagram.com/reel/x/" }, core.ErrInstagramBinding},
		{"lookalike image host", func(w *mediaWire) { w.ThumbnailURL = "https://cdninstagram.com.evil.example/x.jpg" }, core.ErrInstagramBinding},
		{"bare image host suffix", func(w *mediaWire) { w.ThumbnailURL = "https://evilcdninstagram.com/x.jpg" }, core.ErrInstagramBinding},
		{"plain-HTTP image", func(w *mediaWire) { w.ThumbnailURL = "http://scontent.cdninstagram.com/x.jpg" }, core.ErrInstagramBinding},
		{"credentials in image URL", func(w *mediaWire) { w.ThumbnailURL = "https://u:p@scontent.cdninstagram.com/x.jpg" }, core.ErrInstagramResponse},
		{"invalid UTF-8 caption", func(w *mediaWire) { w.Caption = "ok\xff" }, core.ErrInstagramResponse},
		{"caption one byte over", func(w *mediaWire) { w.Caption = strings.Repeat("a", core.InstagramCaptionMaximumBytes+1) }, core.ErrInstagramResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := validWire()
			tc.mutate(&wire)
			if wire == validWire() {
				t.Fatal("vacuous mutation")
			}
			page, err := project(t, wire)
			if !errors.Is(err, tc.want) || len(page.Media) != 0 {
				t.Fatalf("got %v %+v, want %v", err, page, tc.want)
			}
		})
	}
}

func TestMediaPageBounds(t *testing.T) {
	t.Parallel()
	exact := validWire()
	exact.Caption = strings.Repeat("a", core.InstagramCaptionMaximumBytes)
	if _, err := project(t, exact); err != nil {
		t.Fatalf("caption at the bound refused: %v", err)
	}
	if _, err := project(t, validWire(), validWire()); !errors.Is(err, core.ErrInstagramResponse) {
		t.Fatalf("duplicate media identity admitted: %v", err)
	}
	if _, err := (mediaListWire{}).page(12); !errors.Is(err, core.ErrInstagramResponse) {
		t.Fatalf("missing data array admitted: %v", err)
	}
	items := func(n int) []mediaWire {
		out := make([]mediaWire, n)
		for i := range out {
			out[i] = validWire()
			out[i].ID = strings.Repeat("1", i+1)
		}
		return out
	}
	for _, tc := range []struct {
		count, limit int
		want         bool
	}{
		{12, 12, true},
		{13, 12, false},
		{50, 50, true},
		{51, 50, false},
	} {
		_, err := mediaListWire{Data: items(tc.count)}.page(uint8(tc.limit))
		if tc.want != (err == nil) {
			t.Fatalf("%d items under limit %d: %v", tc.count, tc.limit, err)
		}
	}
}

// Meta's response is external ingress. Whatever bytes arrive, an admitted page
// holds only HTTPS links on Meta's own hosts, unique numeric IDs, valid UTF-8
// captions inside the bound, and an image exactly when HasImage says so.
func FuzzMediaListWireProjection(f *testing.F) {
	canonical, err := json.Marshal(mediaListWire{Data: []mediaWire{validWire()}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte(`{"data":[],"paging":{"cursors":{"before":"a","after":"b"}}}`))
	for _, hostile := range []string{"", "null", "{}", `{"data":null}`, `{"data":[{}]}`, `{"data":[{"id":"1","media_type":"VIDEO","permalink":"javascript:alert(1)","timestamp":"2026-10-27T15:04:05+0000"}]}`, `{"data":"x"}`, `[`, `{"data":[],"data":[]}`} {
		f.Add([]byte(hostile))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		wire, err := core.DecodeStrictJSONBytes[mediaListWire](raw, core.ExtensibleJSONLimits())
		if err != nil {
			return
		}
		page, err := wire.page(core.InstagramMediaPageMaximum)
		if err != nil {
			if !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
				t.Fatalf("refusal kept facts or lost its family: %v %+v", err, page)
			}
			return
		}
		seen := map[string]bool{}
		for _, media := range page.Media {
			if seen[media.ID] || media.ID == "" || strings.Trim(media.ID, "0123456789_") != "" {
				t.Fatalf("bad identity admitted: %q", media.ID)
			}
			seen[media.ID] = true
			if !utf8.ValidString(media.Caption) || len(media.Caption) > core.InstagramCaptionMaximumBytes {
				t.Fatal("bad caption admitted")
			}
			link := media.Permalink.HTTPURL()
			if link.Scheme != "https" || !instagramPageHost(link.Hostname()) {
				t.Fatalf("foreign permalink admitted: %q", link.String())
			}
			image := media.Image.HTTPURL()
			if media.HasImage != (media.Image != core.HTTPEndpoint{}) || media.HasImage && (image.Scheme != "https" || !instagramImageHost(image.Hostname())) {
				t.Fatalf("image fact inconsistent: %v %q", media.HasImage, image.String())
			}
			if media.Kind.Validate() != nil || !media.Published.IsSet() {
				t.Fatal("unknown kind or unset time admitted")
			}
		}
	})
}
