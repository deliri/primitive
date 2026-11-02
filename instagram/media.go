package instagram

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	userIDMaximumBytes = 32
	// Meta writes media timestamps with a colon-free UTC offset.
	timestampLayout = "2006-01-02T15:04:05-0700"
	mediaFields     = "id,media_type,media_url,thumbnail_url,permalink,caption,timestamp"
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

// MediaListRequest asks for an account's newest media, newest first.
type MediaListRequest struct {
	Policy exchange.OperationPolicy
	User   UserID
	Limit  uint8
}

func (r MediaListRequest) Validate() error {
	if err := r.User.Validate(); err != nil {
		return err
	}
	if r.Limit == 0 || r.Limit > core.InstagramMediaPageMaximum {
		return core.ErrInstagramContract
	}
	if err := r.Policy.Validate(); err != nil {
		return errors.Join(core.ErrInstagramContract, err)
	}
	return nil
}

// Endpoint is the exact Graph media-list URL. The SDK owns host, version,
// path and fields; the caller owns only the account and page size.
func (r MediaListRequest) Endpoint() (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.InstagramGraphHost + "/" + core.InstagramGraphVersion + "/" + r.User.value + "/media?fields=" + mediaFields + "&limit=" + strconv.Itoa(int(r.Limit)))
}

// MediaKind is the closed media family Meta reports.
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

// Media is one published post. Image is the still a page can show: the frame
// itself, or a video's cover. Meta withholds it for some media; HasImage then
// reports false and Image is unset.
type Media struct {
	Published temporal.Instant
	Permalink core.HTTPEndpoint
	Image     core.HTTPEndpoint
	ID        string
	Caption   string
	Kind      MediaKind
	HasImage  bool
}

// MediaPage is one response page, in Meta's order (newest first).
type MediaPage struct {
	Media []Media
}

func (p MediaPage) Validate() error {
	if len(p.Media) > core.InstagramMediaPageMaximum {
		return core.ErrInstagramResponse
	}
	return nil
}

// mediaListWire is Meta's response document. Unknown members such as paging
// are tolerated; every member this package reads is checked before projection.
type mediaListWire struct {
	Data []mediaWire `json:"data"`
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
	if w.Data == nil || len(w.Data) > core.InstagramMediaPageMaximum {
		return core.ErrInstagramResponse
	}
	return nil
}

func (w mediaListWire) page(limit uint8) (MediaPage, error) {
	if err := w.Validate(); err != nil {
		return MediaPage{}, err
	}
	if len(w.Data) > int(limit) {
		return MediaPage{}, core.ErrInstagramResponse
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
		for _, prior := range page.Media {
			if prior.ID == media.ID {
				return MediaPage{}, core.ErrInstagramResponse
			}
		}
		page.Media = append(page.Media, media)
	}
	return page, nil
}

func (w mediaWire) media(utc temporal.Location, layout temporal.TimeLayout) (Media, error) {
	if w.ID == "" || len(w.ID) > core.InstagramMediaIDMaximumBytes || strings.Trim(w.ID, "0123456789_") != "" {
		return Media{}, core.ErrInstagramResponse
	}
	if len(w.Caption) > core.InstagramCaptionMaximumBytes || !utf8.ValidString(w.Caption) {
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

func instagramPageHost(host string) bool {
	return host == "www.instagram.com" || host == "instagram.com"
}

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
