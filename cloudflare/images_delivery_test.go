package cloudflare

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestImageDeliveryIdentityBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value      string
		account, variant bool
	}{
		{name: "lowercase path component", value: "public", account: true, variant: true},
		{name: "uppercase path component", value: "A", account: true, variant: true},
		{name: "numeric path component", value: "0", account: true, variant: true},
		{name: "delivery hash alphabet", value: "ZWd9g1K7eljCn_KDTu_MWA", account: true, variant: true},
		{name: "hyphen remains identity", value: "square-256", account: true, variant: true},
		{name: "underscore remains identity", value: "square_256", account: true, variant: true},
		{name: "empty never binds", value: ""},
		{name: "slash cannot add a path level", value: "a/b"},
		{name: "query cannot add transform options", value: "a?width=10"},
		{name: "fragment cannot truncate identity", value: "a#b"},
		{name: "percent cannot hide a separator", value: "a%2Fb"},
		{name: "dot cannot normalize path", value: ".."},
		{name: "space cannot split srcset", value: "a b"},
		{name: "comma cannot split srcset", value: "a,b"},
		{name: "control cannot enter HTTP", value: "a\nb"},
		{name: "backslash cannot normalize in browser", value: `a\b`},
		{name: "non ASCII is outside delivery token grammar", value: "café"},
		{name: "variant one below maximum", value: strings.Repeat("a", core.CloudflareImageVariantMaximumCharacters-1), account: true, variant: true},
		{name: "variant maximum", value: strings.Repeat("a", core.CloudflareImageVariantMaximumCharacters), account: true, variant: true},
		{name: "variant one above maximum", value: strings.Repeat("a", core.CloudflareImageVariantMaximumCharacters+1), account: true},
		{name: "account below former SDK quota", value: strings.Repeat("a", 128-1), account: true},
		{name: "account at former SDK quota", value: strings.Repeat("a", 128), account: true},
		{name: "account above former SDK quota", value: strings.Repeat("a", 128+1), account: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			account, accountErr := ParseImageDeliveryAccount(tc.value)
			variant, variantErr := ParseImageVariantName(tc.value)
			if tc.account {
				if accountErr != nil || account.String() != tc.value || account.Validate() != nil {
					t.Fatalf("account=(%q,%v), want exact admitted token %q", account.String(), accountErr, tc.value)
				}
			} else if !errors.Is(accountErr, core.ErrCloudflareBinding) || account != (ImageDeliveryAccount{}) {
				t.Fatalf("account=(%v,%v), want zero and binding refusal", account, accountErr)
			}
			if tc.variant {
				if variantErr != nil || variant.String() != tc.value || variant.Validate() != nil {
					t.Fatalf("variant=(%q,%v), want exact admitted token %q", variant.String(), variantErr, tc.value)
				}
			} else if !errors.Is(variantErr, core.ErrCloudflareBinding) || variant != (ImageVariantName{}) {
				t.Fatalf("variant=(%v,%v), want zero and binding refusal", variant, variantErr)
			}
		})
	}
}

func imageDeliveryFixture(t testing.TB) ImageDeliveryRequest {
	t.Helper()
	origin, err := core.ParseHTTPEndpoint("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	account, err := ParseImageDeliveryAccount("ZWd9g1K7eljCn_KDTu_MWA")
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseImageID("tenant/photo")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := ParseImageVariantName("square-256")
	if err != nil {
		t.Fatal(err)
	}
	return ImageDeliveryRequest{Origin: origin, Account: account, Image: id, Variant: variant}
}

func TestImageDeliveryAddressLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, origin, id, want string
		mutate                 func(*ImageDeliveryRequest)
		wantErr                error
	}{
		{name: "custom origin owns delivery", want: "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/photo/square-256"},
		{name: "root slash is an origin", origin: "https://media.example.test/", want: "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/photo/square-256"},
		{name: "unicode image ID retains exact identity", id: "tenant/café", want: "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/caf%C3%A9/square-256"},
		{name: "literal percent is escaped once", id: "tenant/a%2Fb", want: "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/a%252Fb/square-256"},
		{name: "image query bytes remain path", id: "tenant/a?b#c", want: "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/a%3Fb%23c/square-256"},
		{name: "image whitespace and comma remain escaped path", id: "tenant/a b,c", want: "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/a%20b%2Cc/square-256"},
		{name: "absent origin refuses", mutate: func(r *ImageDeliveryRequest) { r.Origin = core.HTTPEndpoint{} }, wantErr: core.ErrCloudflareBinding},
		{name: "absent account refuses", mutate: func(r *ImageDeliveryRequest) { r.Account = ImageDeliveryAccount{} }, wantErr: core.ErrCloudflareBinding},
		{name: "absent image refuses", mutate: func(r *ImageDeliveryRequest) { r.Image = ImageID{} }, wantErr: core.ErrCloudflareBinding},
		{name: "absent variant refuses", mutate: func(r *ImageDeliveryRequest) { r.Variant = ImageVariantName{} }, wantErr: core.ErrCloudflareBinding},
		{name: "cleartext origin refuses", origin: "http://media.example.test", wantErr: core.ErrCloudflareBinding},
		{name: "origin path is never discarded", origin: "https://media.example.test/unowned", wantErr: core.ErrCloudflareBinding},
		{name: "origin query is never discarded", origin: "https://media.example.test?x=1", wantErr: core.ErrCloudflareBinding},
		{name: "empty query is never discarded", origin: "https://media.example.test?", wantErr: core.ErrCloudflareBinding},
		{name: "alternate port refuses", origin: "https://media.example.test:444", wantErr: core.ErrCloudflareBinding},
		{name: "parent path segment refuses", id: "tenant/../photo", wantErr: core.ErrCloudflareBinding},
		{name: "current path segment refuses", id: "tenant/./photo", wantErr: core.ErrCloudflareBinding},
		{name: "empty path segment refuses", id: "tenant//photo", wantErr: core.ErrCloudflareBinding},
		{name: "browser backslash refuses", id: `tenant\photo`, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := imageDeliveryFixture(t)
			var err error
			if tc.origin != "" {
				r.Origin, err = core.ParseHTTPEndpoint(tc.origin)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.id != "" {
				r.Image, err = ParseImageID(tc.id)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(&r)
			}
			got, gotErr := r.Address()
			if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.want {
				t.Fatalf("Address()=(%q,%v), want (%q,%v)", got.String(), gotErr, tc.want, tc.wantErr)
			}
			if gotErr != nil && got != (core.HTTPEndpoint{}) {
				t.Fatalf("refused address=%v, want zero", got)
			}
			if gotErr == nil && (got.Validate() != nil || !got.SameOrigin(r.Origin)) {
				t.Fatalf("address=%v, want valid same-origin delivery", got)
			}
		})
	}
}

func TestImageDetailsPublicAddressBindingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		change  func(*imageDetailsWire, *ImageDeliveryRequest)
		wantErr error
	}{
		{name: "uploaded exact account image variant projects custom domain"},
		{name: "other variant before exact variant does not select first", change: func(w *imageDetailsWire, _ *ImageDeliveryRequest) {
			w.Variants = append([]string{"https://imagedelivery.net/account/session/photo.jpg/other"}, w.Variants...)
		}},
		{name: "draft has no public delivery proof", change: func(w *imageDetailsWire, _ *ImageDeliveryRequest) { w.Draft = true }, wantErr: core.ErrCloudflareResponse},
		{name: "absent variants have no public delivery proof", change: func(w *imageDetailsWire, _ *ImageDeliveryRequest) { w.Variants = nil }, wantErr: core.ErrCloudflareResponse},
		{name: "private metadata cannot emit unsigned delivery", change: func(w *imageDetailsWire, _ *ImageDeliveryRequest) { w.RequireSignedURLs = true }, wantErr: core.ErrCloudflareResponse},
		{name: "wrong account refuses without rewritten provider identity", change: func(w *imageDetailsWire, _ *ImageDeliveryRequest) {
			w.Variants[0] = "https://imagedelivery.net/foreign/session/photo.jpg/public"
		}, wantErr: core.ErrCloudflareBinding},
		{name: "wrong variant refuses", change: func(w *imageDetailsWire, _ *ImageDeliveryRequest) {
			w.Variants[0] = "https://imagedelivery.net/account/session/photo.jpg/other"
		}, wantErr: core.ErrCloudflareBinding},
		{name: "request for different image refuses", change: func(_ *imageDetailsWire, r *ImageDeliveryRequest) { r.Image = ImageID{value: "other"} }, wantErr: core.ErrCloudflareBinding},
		{name: "zero request cannot acquire delivery", change: func(_ *imageDetailsWire, r *ImageDeliveryRequest) { *r = ImageDeliveryRequest{} }, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := imageDetailsFixture()
			r := imageDeliveryFixture(t)
			r.Account, r.Image, r.Variant = ImageDeliveryAccount{value: "account"}, ImageID{value: wire.ID}, ImageVariantName{value: "public"}
			if tc.change != nil {
				tc.change(&wire, &r)
			}
			details, err := wire.details()
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := details.PublicAddress(r)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("PublicAddress() error=%v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (core.HTTPEndpoint{}) {
					t.Fatalf("refused address=%v, want zero", got)
				}
				return
			}
			const want = "https://media.example.test/cdn-cgi/imagedelivery/account/session/photo.jpg/public"
			if got.String() != want {
				t.Fatalf("public address=%q, want %q", got.String(), want)
			}
		})
	}
}
