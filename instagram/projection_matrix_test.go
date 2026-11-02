package instagram

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The producer is Exchange's strict JSON decode of Meta's bytes; the
// classifier projects each record and decides which still a page may show.
// Every row runs the exact production path from raw bytes to MediaPage.

type matrixClass uint8

const (
	classContradiction matrixClass = iota + 1
	classRefusal
	classNeutral
	classBoundary
)

// errDecoderRefusal marks rows the strict decoder itself must refuse.
var errDecoderRefusal = errors.New("decoder refusal")

const (
	coverURL = "https://scontent-yyz1-1.cdninstagram.com/v/cover.jpg"
	videoURL = "https://scontent-yyz1-1.cdninstagram.com/o1/v/reel.mp4"
	photoURL = "https://scontent.xx.fbcdn.net/v/photo.jpg"
)

func reel(id string) mediaWire {
	return mediaWire{ID: id, MediaType: "VIDEO", MediaURL: videoURL, ThumbnailURL: coverURL, Permalink: "https://www.instagram.com/reel/" + id + "/", Caption: "A reel", Timestamp: "2026-10-27T15:04:05+0000"}
}

func photo(id string) mediaWire {
	return mediaWire{ID: id, MediaType: "IMAGE", MediaURL: photoURL, Permalink: "https://www.instagram.com/p/" + id + "/", Caption: "A photo", Timestamp: "2026-10-27T15:04:05+0000"}
}

func reels(n int) []mediaWire {
	out := make([]mediaWire, n)
	for i := range out {
		out[i] = reel(strings.Repeat("1", i+1))
	}
	return out
}

func encodeData(t *testing.T, items []mediaWire) []byte {
	t.Helper()
	raw, err := json.Marshal(mediaListWire{Data: append([]mediaWire{}, items...)})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type matrixRow struct {
	name   string
	class  matrixClass
	raw    func(*testing.T) []byte
	err    error
	images []string // per admitted record: the exact still, or "" for none
}

func items(list ...mediaWire) func(*testing.T) []byte {
	return func(t *testing.T) []byte { return encodeData(t, list) }
}

func with(base mediaWire, mutate func(*mediaWire)) mediaWire {
	mutate(&base)
	return base
}

func rawJSON(text string) func(*testing.T) []byte {
	return func(*testing.T) []byte { return []byte(text) }
}

func matrixRows() []matrixRow {
	r1, r2, r3 := reel("101"), reel("102"), reel("103")
	relinked := func(w mediaWire, link string) mediaWire { w.Permalink = link; return w }
	return []matrixRow{
		// Contradiction: admissible facts that cannot all be true of one post,
		// or a still offered in a field Meta does not use for that kind.
		{name: "one id twice, adjacent", class: classContradiction, raw: items(r1, relinked(r1, "https://www.instagram.com/reel/x/")), err: core.ErrInstagramResponse},
		{name: "one id twice, two places apart", class: classContradiction, raw: items(r1, r2, relinked(r1, "https://www.instagram.com/reel/y/")), err: core.ErrInstagramResponse},
		{name: "two ids, one post link, adjacent", class: classContradiction, raw: items(r1, relinked(r2, r1.Permalink)), err: core.ErrInstagramResponse},
		{name: "two ids, one post link, two places apart", class: classContradiction, raw: items(r1, r2, relinked(r3, r1.Permalink)), err: core.ErrInstagramResponse},
		{name: "a photo offering only a video thumbnail", class: classContradiction, raw: items(with(photo("201"), func(w *mediaWire) { w.MediaURL, w.ThumbnailURL = "", coverURL })), images: []string{""}},
		{name: "a carousel offering only a video thumbnail", class: classContradiction, raw: items(with(photo("202"), func(w *mediaWire) { w.MediaType, w.MediaURL, w.ThumbnailURL = "CAROUSEL_ALBUM", "", coverURL })), images: []string{""}},
		{name: "a reel offering only its video file", class: classContradiction, raw: items(with(reel("203"), func(w *mediaWire) { w.ThumbnailURL = "" })), images: []string{""}},
		{name: "a reel with file and thumbnail shows the thumbnail", class: classContradiction, raw: items(reel("204")), images: []string{coverURL}},
		{name: "a photo with media and thumbnail shows the media", class: classContradiction, raw: items(with(photo("205"), func(w *mediaWire) { w.ThumbnailURL = coverURL })), images: []string{photoURL}},
		{name: "a carousel with media and thumbnail shows the media", class: classContradiction, raw: items(with(photo("206"), func(w *mediaWire) { w.MediaType, w.ThumbnailURL = "CAROUSEL_ALBUM", coverURL })), images: []string{photoURL}},

		// Typed refusal: material the producer cannot admit stays refused.
		{name: "an unknown media type", class: classRefusal, raw: items(with(r1, func(w *mediaWire) { w.MediaType = "STORY" })), err: core.ErrInstagramResponse},
		{name: "an empty id", class: classRefusal, raw: items(with(r1, func(w *mediaWire) { w.ID = "" })), err: core.ErrInstagramResponse},
		{name: "an RFC 3339 colon offset", class: classRefusal, raw: items(with(r1, func(w *mediaWire) { w.Timestamp = "2026-10-27T15:04:05+00:00" })), err: core.ErrInstagramResponse},
		{name: "a missing post link", class: classRefusal, raw: items(with(r1, func(w *mediaWire) { w.Permalink = "" })), err: core.ErrInstagramResponse},
		{name: "invalid UTF-8 in a caption", class: classRefusal, raw: rawJSON("{\"data\":[{\"id\":\"1\",\"media_type\":\"VIDEO\",\"permalink\":\"https://www.instagram.com/reel/1/\",\"caption\":\"ok\xff\",\"timestamp\":\"2026-10-27T15:04:05+0000\"}]}"), err: errDecoderRefusal},
		{name: "data as a string", class: classRefusal, raw: rawJSON(`{"data":"posts"}`), err: errDecoderRefusal},
		{name: "an undocumented top-level member", class: classRefusal, raw: rawJSON(`{"data":[],"summary":{"total":46}}`), err: errDecoderRefusal},
		{name: "truncated JSON", class: classRefusal, raw: rawJSON(`{"data":[{"id":"1"`), err: errDecoderRefusal},
		{name: "credentials inside a post link", class: classRefusal, raw: items(with(r1, func(w *mediaWire) { w.Permalink = "https://u:p@www.instagram.com/reel/1/" })), err: core.ErrInstagramResponse},
		{name: "data null", class: classRefusal, raw: rawJSON(`{"data":null}`), err: core.ErrInstagramResponse},

		// Neutral: absence and no-ops never become facts.
		{name: "an account with no posts", class: classNeutral, raw: rawJSON(`{"data":[]}`), images: []string{}},
		{name: "no caption member", class: classNeutral, raw: rawJSON(`{"data":[{"id":"1","media_type":"IMAGE","media_url":"` + photoURL + `","permalink":"https://www.instagram.com/p/1/","timestamp":"2026-10-27T15:04:05+0000"}]}`), images: []string{photoURL}},
		{name: "an empty caption", class: classNeutral, raw: items(with(photo("301"), func(w *mediaWire) { w.Caption = "" })), images: []string{photoURL}},
		{name: "no paging block", class: classNeutral, raw: items(photo("302")), images: []string{photoURL}},
		{name: "next and previous links are accepted and ignored", class: classNeutral, raw: rawJSON(`{"data":[],"paging":{"cursors":{"before":"a","after":"b"},"next":"https://graph.facebook.com/next","previous":"https://graph.facebook.com/prev"}}`), images: []string{}},
		{name: "an empty cursors block", class: classNeutral, raw: rawJSON(`{"data":[],"paging":{"cursors":{}}}`), images: []string{}},
		{name: "a photo whose media Meta withholds", class: classNeutral, raw: items(with(photo("303"), func(w *mediaWire) { w.MediaURL = "" })), images: []string{""}},
		{name: "a reel with neither file nor thumbnail", class: classNeutral, raw: items(with(reel("304"), func(w *mediaWire) { w.MediaURL, w.ThumbnailURL = "", "" })), images: []string{""}},
		{name: "a carousel with neither media nor thumbnail", class: classNeutral, raw: items(with(photo("305"), func(w *mediaWire) { w.MediaType, w.MediaURL = "CAROUSEL_ALBUM", "" })), images: []string{""}},
		{name: "a whitespace-only caption is kept exactly", class: classNeutral, raw: items(with(photo("306"), func(w *mediaWire) { w.Caption = " \n " })), images: []string{photoURL}},

		// Boundary: Meta's documented 2200-character caption maximum.
		{name: "2199 characters", class: classBoundary, raw: items(with(photo("401"), func(w *mediaWire) { w.Caption = strings.Repeat("a", core.InstagramCaptionMaximumCharacters-1) })), images: []string{photoURL}},
		{name: "2200 four-byte characters, counted as characters", class: classBoundary, raw: items(with(photo("402"), func(w *mediaWire) { w.Caption = strings.Repeat("\U0001F355", core.InstagramCaptionMaximumCharacters) })), images: []string{photoURL}},
		{name: "2201 characters", class: classBoundary, raw: items(with(photo("403"), func(w *mediaWire) { w.Caption = strings.Repeat("a", core.InstagramCaptionMaximumCharacters+1) })), err: core.ErrInstagramResponse},
		{name: "65536 characters", class: classBoundary, raw: items(with(photo("404"), func(w *mediaWire) { w.Caption = strings.Repeat("a", 64<<10) })), err: core.ErrInstagramResponse},
		// Boundary: Primitive's media identity custody bound.
		{name: "id one byte under the custody bound", class: classBoundary, raw: items(reel(strings.Repeat("5", core.InstagramMediaIDCustodyMaximumBytes-1))), images: []string{coverURL}},
		{name: "id exactly at the custody bound", class: classBoundary, raw: items(reel(strings.Repeat("5", core.InstagramMediaIDCustodyMaximumBytes))), images: []string{coverURL}},
		{name: "id one byte over the custody bound", class: classBoundary, raw: items(reel(strings.Repeat("5", core.InstagramMediaIDCustodyMaximumBytes+1))), err: core.ErrInstagramResponse},
		{name: "a 4 KiB id", class: classBoundary, raw: items(reel(strings.Repeat("5", 4096))), err: core.ErrInstagramResponse},
		// Boundary: Primitive's page custody bound (Meta's observed default page is 25).
		{name: "one under the page bound", class: classBoundary, raw: items(reels(core.InstagramMediaPageCustodyMaximum - 1)...), images: repeat(coverURL, core.InstagramMediaPageCustodyMaximum-1)},
		{name: "exactly the page bound", class: classBoundary, raw: items(reels(core.InstagramMediaPageCustodyMaximum)...), images: repeat(coverURL, core.InstagramMediaPageCustodyMaximum)},
		{name: "one over the page bound", class: classBoundary, raw: items(reels(core.InstagramMediaPageCustodyMaximum + 1)...), err: core.ErrInstagramResponse},
		{name: "a thousand posts", class: classBoundary, raw: items(reels(1000)...), err: core.ErrInstagramResponse},
		// Boundary: Meta's closed media_type domain.
		{name: "IMAGE, the lowest kind", class: classBoundary, raw: items(photo("501")), images: []string{photoURL}},
		{name: "CAROUSEL_ALBUM, the highest kind", class: classBoundary, raw: items(with(photo("502"), func(w *mediaWire) { w.MediaType = "CAROUSEL_ALBUM" })), images: []string{photoURL}},
		{name: "CAROUSEL, the unknown next spelling", class: classBoundary, raw: items(with(photo("503"), func(w *mediaWire) { w.MediaType = "CAROUSEL" })), err: core.ErrInstagramResponse},
		{name: "VIDEO with a NUL suffix", class: classBoundary, raw: items(with(reel("504"), func(w *mediaWire) { w.MediaType = "VIDEO\x00" })), err: core.ErrInstagramResponse},
		// Boundary: Meta's image hosts.
		{name: "a regional cdninstagram host", class: classBoundary, raw: items(reel("601")), images: []string{coverURL}},
		{name: "the bare cdninstagram apex", class: classBoundary, raw: items(with(reel("602"), func(w *mediaWire) { w.ThumbnailURL = "https://cdninstagram.com/x.jpg" })), err: core.ErrInstagramBinding},
		{name: "an fbcdn host", class: classBoundary, raw: items(photo("603")), images: []string{photoURL}},
		{name: "an fbcdn lookalike", class: classBoundary, raw: items(with(photo("604"), func(w *mediaWire) { w.MediaURL = "https://fbcdn.net.evil.example/x.jpg" })), err: core.ErrInstagramBinding},
	}
}

// Which still a page shows depends only on kind and which URLs Meta sent.
// The domain is twelve combinations, so it is exhausted, not sampled.
func TestStillSelectionExhaustsKindAndURLPresence(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"IMAGE", "VIDEO", "CAROUSEL_ALBUM"} {
		for _, hasMedia := range []bool{false, true} {
			for _, hasThumbnail := range []bool{false, true} {
				wire := photo("700")
				wire.MediaType, wire.MediaURL, wire.ThumbnailURL = kind, "", ""
				if hasMedia {
					wire.MediaURL = photoURL
				}
				if hasThumbnail {
					wire.ThumbnailURL = coverURL
				}
				want := ""
				switch {
				case kind == "VIDEO" && hasThumbnail:
					want = coverURL
				case kind != "VIDEO" && hasMedia:
					want = photoURL
				}
				page, err := project(t, wire)
				if err != nil {
					t.Fatalf("%s media=%v thumbnail=%v refused: %v", kind, hasMedia, hasThumbnail, err)
				}
				image := page.Media[0].Image.HTTPURL()
				got := ""
				if page.Media[0].HasImage {
					got = image.String()
				}
				if got != want {
					t.Fatalf("%s media=%v thumbnail=%v shows %q, want %q", kind, hasMedia, hasThumbnail, got, want)
				}
			}
		}
	}
}

func repeat(value string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = value
	}
	return out
}

func TestMediaProducerClassifierMatrix(t *testing.T) {
	t.Parallel()
	rows := matrixRows()
	counts := map[matrixClass]int{}
	names := map[string]bool{}
	for _, row := range rows {
		counts[row.class]++
		if names[row.name] {
			t.Fatalf("duplicate row %q", row.name)
		}
		names[row.name] = true
	}
	if counts[classContradiction] != 10 || counts[classRefusal] != 10 || counts[classNeutral] != 10 || counts[classBoundary] != 20 {
		t.Fatalf("primary class quotas %v, want 10/10/10/20", counts)
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			wire, decodeErr := core.DecodeStrictJSONBytes[mediaListWire](row.raw(t), core.ExtensibleJSONLimits())
			if row.err == errDecoderRefusal {
				if decodeErr == nil {
					t.Fatal("the strict decoder admitted the row")
				}
				return
			}
			if decodeErr != nil {
				t.Fatalf("decode: %v", decodeErr)
			}
			page, err := wire.page()
			if row.err != nil {
				if !errors.Is(err, row.err) || !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
					t.Fatalf("got %v with %d media, want %v and nothing kept", err, len(page.Media), row.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if len(page.Media) != len(row.images) {
				t.Fatalf("admitted %d media, want %d", len(page.Media), len(row.images))
			}
			for i, media := range page.Media {
				image := media.Image.HTTPURL()
				got := ""
				if media.HasImage {
					got = image.String()
				}
				if got != row.images[i] || media.HasImage != (row.images[i] != "") || media.Kind.Validate() != nil || !media.Published.IsSet() {
					t.Fatalf("media %d still %q (has=%v), want %q", i, got, media.HasImage, row.images[i])
				}
			}
		})
	}
}
