package instagram

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// Primitive's bound for an account identity placed in the request path.
	// Graph IDs are numeric strings; digits-only keeps the path closed.
	userIDMaximumBytes = 32
	// Meta: "ISO 8601-formatted creation date in UTC", example
	// "2019-09-26T22:36:43+0000" (offset without a colon).
	timestampLayout = "2006-01-02T15:04:05-0700"
	// The documented IG Media fields this package reads.
	mediaFields = "id,media_type,media_url,thumbnail_url,permalink,caption,timestamp"
)

// UserID is one Instagram professional account's numeric Graph identity.
type UserID struct{ value string }

func ParseUserID(raw string) (UserID, error) {
	if raw == "" || len(raw) > userIDMaximumBytes || strings.Trim(raw, "0123456789") != "" {
		return UserID{}, core.ErrInstagramContract
	}
	return UserID{value: raw}, nil
}

func (u UserID) Validate() error {
	_, err := ParseUserID(u.value)
	return err
}

func (u UserID) String() string { return u.value }

// MediaListRequest asks for one page of an account's media. Meta documents no
// page-size parameter for this edge, so none is sent; Meta chooses the page.
// https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/media
type MediaListRequest struct {
	Policy exchange.OperationPolicy
	User   UserID
}

func (r MediaListRequest) Validate() error {
	if err := r.User.Validate(); err != nil {
		return err
	}
	if err := r.Policy.Validate(); err != nil {
		return errors.Join(core.ErrInstagramContract, err)
	}
	return nil
}

// Endpoint is the exact Graph media-list URL. The SDK owns host, version,
// path and fields; the caller owns only the account.
func (r MediaListRequest) Endpoint() (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.InstagramGraphHost + "/" + core.InstagramGraphVersion + "/" + r.User.value + "/media?fields=" + mediaFields)
}

// MediaKind is the closed media family. Meta: media_type "Can be
// CAROUSEL_ALBUM, IMAGE, or VIDEO."
type MediaKind uint8

const (
	MediaKindUnknown MediaKind = iota
	MediaKindImage
	MediaKindVideo
	MediaKindCarousel
)

var mediaKindWire = [...]string{"", "IMAGE", "VIDEO", "CAROUSEL_ALBUM"}

func parseMediaKind(raw string) (MediaKind, error) {
	for index := 1; index < len(mediaKindWire); index++ {
		if mediaKindWire[index] == raw {
			return MediaKind(index), nil
		}
	}
	return MediaKindUnknown, core.ErrInstagramResponse
}

func (k MediaKind) Validate() error {
	if k == MediaKindUnknown || int(k) >= len(mediaKindWire) {
		return core.ErrInstagramContract
	}
	return nil
}

// Media is one published post. Image is the still a page can show. Meta:
// thumbnail_url is "Only available on VIDEO media", and media_url may be
// omitted for licensed audio. A video shows its thumbnail; anything else shows
// its media_url. Without that still, HasImage is false and Image is unset; a
// video file or another kind's thumbnail is never borrowed as a still.
type Media struct {
	Published temporal.Instant
	Permalink core.HTTPEndpoint
	Image     core.HTTPEndpoint
	ID        string
	Caption   string
	Kind      MediaKind
	HasImage  bool
}

// MediaPage is one response page, in Meta's order.
type MediaPage struct {
	Media []Media
}

func (p MediaPage) Validate() error {
	if len(p.Media) > core.InstagramMediaPageCustodyMaximum {
		return core.ErrInstagramResponse
	}
	return nil
}

// mediaListWire is Meta's response document. Exchange decodes strictly, so
// each documented member is declared: data, and cursor paging.
type mediaListWire struct {
	Paging *mediaPagingWire `json:"paging,omitempty"`
	Data   []mediaWire      `json:"data"`
}

// mediaPagingWire is cursor pagination. Meta: next and previous may be
// absent ("If not included, this is the last/first page of data").
// https://developers.facebook.com/docs/graph-api/results
type mediaPagingWire struct {
	Cursors  mediaCursorsWire `json:"cursors"`
	Next     string           `json:"next,omitempty"`
	Previous string           `json:"previous,omitempty"`
}

type mediaCursorsWire struct {
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type mediaWire struct {
	ID           string `json:"id"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Permalink    string `json:"permalink"`
	Caption      string `json:"caption"`
	Timestamp    string `json:"timestamp"`
}

func (w mediaListWire) Validate() error {
	if w.Data == nil || len(w.Data) > core.InstagramMediaPageCustodyMaximum {
		return core.ErrInstagramResponse
	}
	return nil
}

func (w mediaListWire) page() (MediaPage, error) {
	if err := w.Validate(); err != nil {
		return MediaPage{}, err
	}
	utc, err := temporal.NewLocation(temporal.LocationUTC)
	if err != nil {
		return MediaPage{}, errors.Join(core.ErrInstagramContract, err)
	}
	layout, err := temporal.NewTimeLayout(timestampLayout)
	if err != nil {
		return MediaPage{}, errors.Join(core.ErrInstagramContract, err)
	}
	page := MediaPage{Media: make([]Media, 0, len(w.Data))}
	for _, item := range w.Data {
		media, err := item.media(utc, layout)
		if err != nil {
			return MediaPage{}, err
		}
		// One media identity, and one permanent post URL, name exactly one post.
		for _, prior := range page.Media {
			if prior.ID == media.ID || prior.Permalink == media.Permalink {
				return MediaPage{}, core.ErrInstagramResponse
			}
		}
		page.Media = append(page.Media, media)
	}
	return page, nil
}

func (w mediaWire) media(utc temporal.Location, layout temporal.TimeLayout) (Media, error) {
	if !validMediaID(w.ID) {
		return Media{}, core.ErrInstagramResponse
	}
	if !utf8.ValidString(w.Caption) || utf8.RuneCountInString(w.Caption) > core.InstagramCaptionMaximumCharacters {
		return Media{}, core.ErrInstagramResponse
	}
	kind, err := parseMediaKind(w.MediaType)
	if err != nil {
		return Media{}, err
	}
	published, err := utc.ParseLocalTime(layout, w.Timestamp)
	if err != nil {
		return Media{}, errors.Join(core.ErrInstagramResponse, err)
	}
	permalink, err := hostedEndpoint(w.Permalink, instagramPageHost)
	if err != nil {
		return Media{}, err
	}
	media := Media{ID: w.ID, Kind: kind, Caption: w.Caption, Published: published, Permalink: permalink}
	still := w.MediaURL
	if kind == MediaKindVideo {
		still = w.ThumbnailURL
	}
	if still == "" {
		return media, nil
	}
	media.Image, err = hostedEndpoint(still, instagramImageHost)
	if err != nil {
		return Media{}, err
	}
	media.HasImage = true
	return media, nil
}

// validMediaID admits Meta's opaque "Media ID" inside Primitive's custody
// bound: non-empty visible ASCII. Meta documents no finer grammar.
func validMediaID(id string) bool {
	if id == "" || len(id) > core.InstagramMediaIDCustodyMaximumBytes {
		return false
	}
	for index := 0; index < len(id); index++ {
		if id[index] <= ' ' || id[index] > '~' {
			return false
		}
	}
	return true
}

func instagramPageHost(host string) bool {
	return host == "www.instagram.com" || host == "instagram.com"
}

// instagramImageHost admits Meta's media CDNs: observed serving this account's
// stills (scontent-*.cdninstagram.com), and Meta's shared CDN (*.fbcdn.net).
func instagramImageHost(host string) bool {
	return strings.HasSuffix(host, ".cdninstagram.com") || strings.HasSuffix(host, ".fbcdn.net")
}

// hostedEndpoint admits only an HTTPS URL on a host Meta serves this fact from.
func hostedEndpoint(raw string, admitted func(string) bool) (core.HTTPEndpoint, error) {
	endpoint, err := core.ParseHTTPEndpoint(raw)
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrInstagramResponse, err)
	}
	address := endpoint.HTTPURL()
	if address.Scheme != core.SchemeHTTPS || !admitted(address.Hostname()) {
		return core.HTTPEndpoint{}, core.ErrInstagramBinding
	}
	return endpoint, nil
}
