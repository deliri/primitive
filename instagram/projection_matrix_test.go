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
	limit  uint8
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
	r1, r2 := reel("101"), reel("102")
	return []matrixRow{
		// Contradiction: admissible facts that cannot all be true of one post.
		{name: "the same post twice", class: classContradiction, raw: items(r1, r1), err: core.ErrInstagramResponse},
		{name: "one id, two kinds", class: classContradiction, raw: items(r1, with(photo("101"), func(w *mediaWire) { w.Permalink = "https://www.instagram.com/p/other/" })), err: core.ErrInstagramResponse},
		{name: "one id, two captions", class: classContradiction, raw: items(r1, with(r1, func(w *mediaWire) { w.Caption = "edited"; w.Permalink = "https://www.instagram.com/reel/x/" })), err: core.ErrInstagramResponse},
		{name: "one id repeated two places apart", class: classContradiction, raw: items(r1, r2, with(r1, func(w *mediaWire) { w.Permalink = "https://www.instagram.com/reel/y/" })), err: core.ErrInstagramResponse},
		{name: "two ids, one post link", class: classContradiction, raw: items(r1, with(r2, func(w *mediaWire) { w.Permalink = r1.Permalink })), err: core.ErrInstagramResponse},
		{name: "more posts than were asked for", class: classContradiction, raw: items(reels(3)...), limit: 2, err: core.ErrInstagramResponse},
		{name: "a photo carrying only a video cover", class: classContradiction, raw: items(with(photo("201"), func(w *mediaWire) { w.MediaURL = ""; w.ThumbnailURL = coverURL })), images: []string{""}},
		{name: "a reel carrying only its video file", class: classContradiction, raw: items(with(reel("202"), func(w *mediaWire) { w.ThumbnailURL = "" })), images: []string{""}},
		{name: "a carousel carrying only a cover", class: classContradiction, raw: items(with(photo("203"), func(w *mediaWire) { w.MediaType = "CAROUSEL_ALBUM"; w.MediaURL = ""; w.ThumbnailURL = coverURL })), images: []string{""}},
		{name: "a reel with both a file and a cover shows the cover", class: classContradiction, raw: items(reel("204")), images: []string{coverURL}},

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
		{name: "paging links are accepted and ignored", class: classNeutral, raw: rawJSON(`{"data":[],"paging":{"cursors":{"before":"a","after":"b"},"next":"https://graph.facebook.com/next","previous":"https://graph.facebook.com/prev"}}`), images: []string{}},
		{name: "an empty cursors block", class: classNeutral, raw: rawJSON(`{"data":[],"paging":{"cursors":{}}}`), images: []string{}},
		{name: "a photo Meta withholds", class: classNeutral, raw: items(with(photo("303"), func(w *mediaWire) { w.MediaURL = "" })), images: []string{""}},
		{name: "a reel with no file and no cover", class: classNeutral, raw: items(with(reel("304"), func(w *mediaWire) { w.MediaURL, w.ThumbnailURL = "", "" })), images: []string{""}},
		{name: "a whitespace-only caption is kept exactly", class: classNeutral, raw: items(with(photo("305"), func(w *mediaWire) { w.Caption = " \n " })), images: []string{photoURL}},
		{name: "a post link with a query is kept, not normalized", class: classNeutral, raw: items(with(photo("306"), func(w *mediaWire) { w.Permalink += "?igsh=abc" })), images: []string{photoURL}},

		// Boundary: caption bytes.
		{name: "caption one byte under the bound", class: classBoundary, raw: items(with(photo("401"), func(w *mediaWire) { w.Caption = strings.Repeat("a", core.InstagramCaptionMaximumBytes-1) })), images: []string{photoURL}},
		{name: "caption exactly at the bound", class: classBoundary, raw: items(with(photo("402"), func(w *mediaWire) { w.Caption = strings.Repeat("a", core.InstagramCaptionMaximumBytes) })), images: []string{photoURL}},
		{name: "caption one byte over the bound", class: classBoundary, raw: items(with(photo("403"), func(w *mediaWire) { w.Caption = strings.Repeat("a", core.InstagramCaptionMaximumBytes+1) })), err: core.ErrInstagramResponse},
		{name: "a 64 KiB caption", class: classBoundary, raw: items(with(photo("404"), func(w *mediaWire) { w.Caption = strings.Repeat("a", 64<<10) })), err: core.ErrInstagramResponse},
		// Boundary: id length.
		{name: "id one byte under the bound", class: classBoundary, raw: items(reel(strings.Repeat("5", core.InstagramMediaIDMaximumBytes-1))), images: []string{coverURL}},
		{name: "id exactly at the bound", class: classBoundary, raw: items(reel(strings.Repeat("5", core.InstagramMediaIDMaximumBytes))), images: []string{coverURL}},
		{name: "id one byte over the bound", class: classBoundary, raw: items(reel(strings.Repeat("5", core.InstagramMediaIDMaximumBytes+1))), err: core.ErrInstagramResponse},
		{name: "a 4 KiB id", class: classBoundary, raw: items(reel(strings.Repeat("5", 4096))), err: core.ErrInstagramResponse},
		// Boundary: posts against the requested page size.
		{name: "eleven of twelve", class: classBoundary, raw: items(reels(11)...), limit: 12, images: repeat(coverURL, 11)},
		{name: "twelve of twelve", class: classBoundary, raw: items(reels(12)...), limit: 12, images: repeat(coverURL, 12)},
		{name: "thirteen of twelve", class: classBoundary, raw: items(reels(13)...), limit: 12, err: core.ErrInstagramResponse},
		{name: "fifty at the maximum page", class: classBoundary, raw: items(reels(50)...), limit: 50, images: repeat(coverURL, 50)},
		// Boundary: the closed kind domain.
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
			limit := row.limit
			if limit == 0 {
				limit = core.InstagramMediaPageMaximum
			}
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
			page, err := wire.page(limit)
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
