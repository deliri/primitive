package instagram

import (
	"errors"
	"testing"

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
	return exchange.OperationPolicy{
		OperationTimeout: total, AttemptTimeout: attempt,
		// One attempt: a single-attempt policy carries no retry delays.
		Retry:    exchange.RetryPolicy{MaximumAttempts: 1},
		Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject},
	}
}

func testUser(t testing.TB) UserID {
	t.Helper()
	user, err := ParseUserID("17841432921003907")
	if err != nil {
		t.Fatal(err)
	}
	return user
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
	// Meta always sends a data array; an empty account is [], never absent.
	return mediaListWire{Data: append([]mediaWire{}, items...)}.page()
}

func TestMediaProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive: a reel shows its cover, a photo shows itself, in Meta's order", func(t *testing.T) {
		t.Parallel()
		photo := validWire()
		photo.ID, photo.MediaType, photo.MediaURL, photo.ThumbnailURL, photo.Permalink = "17999", "IMAGE", photoImage, "", "https://www.instagram.com/p/17999/"
		carousel := validWire()
		carousel.ID, carousel.MediaType, carousel.MediaURL, carousel.ThumbnailURL, carousel.Permalink = "17998", "CAROUSEL_ALBUM", photoImage, "", "https://www.instagram.com/p/17998/"
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
