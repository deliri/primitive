package cloudflare

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzImageDeliveryTokensClosure(f *testing.F) {
	seed := imageDeliveryFixture(f)
	f.Add(seed.Account.String())
	f.Add(seed.Variant.String())
	f.Add("")
	f.Add("../unowned")
	f.Add("a%2Fb")
	f.Add(strings.Repeat("x", core.CloudflareImageVariantMaximumCharacters))
	f.Add(strings.Repeat("x", 129))
	f.Fuzz(func(t *testing.T, raw string) {
		account, err := ParseImageDeliveryAccount(raw)
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || account != (ImageDeliveryAccount{}) {
				t.Fatalf("account refusal=(%v,%v), want zero and typed binding refusal", account, err)
			}
		} else {
			again, parseErr := ParseImageDeliveryAccount(account.String())
			if account.String() != raw || account.Validate() != nil || again != account || parseErr != nil {
				t.Fatalf("account closure=(%v,%v), want exact %q", again, parseErr, raw)
			}
		}
		variant, err := ParseImageVariantName(raw)
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || variant != (ImageVariantName{}) {
				t.Fatalf("variant refusal=(%v,%v), want zero and typed binding refusal", variant, err)
			}
			return
		}
		again, parseErr := ParseImageVariantName(variant.String())
		if variant.String() != raw || variant.Validate() != nil || again != variant || parseErr != nil {
			t.Fatalf("variant closure=(%v,%v), want exact %q", again, parseErr, raw)
		}
	})
}

func FuzzImageDeliveryAddressBinding(f *testing.F) {
	seed := imageDeliveryFixture(f)
	if _, err := seed.Address(); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Origin.String(), seed.Image.String())
	f.Add("https://media.example.test?", "tenant/../photo")
	f.Add("https://media.example.test", "tenant/a?b#c,d%2Fe")
	f.Fuzz(func(t *testing.T, origin, id string) {
		r := seed
		var err error
		r.Origin, err = core.ParseHTTPEndpoint(origin)
		if err != nil {
			if !errors.Is(err, core.ErrPrimitiveContract) || r.Origin != (core.HTTPEndpoint{}) {
				t.Fatalf("origin=(%v,%v), want zero HTTP refusal", r.Origin, err)
			}
			return
		}
		r.Image, err = ParseImageID(id)
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || r.Image != (ImageID{}) {
				t.Fatalf("image=(%v,%v), want zero binding refusal", r.Image, err)
			}
			return
		}
		got, err := r.Address()
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (core.HTTPEndpoint{}) {
				t.Fatalf("address=(%v,%v), want zero binding refusal", got, err)
			}
			return
		}
		u := got.HTTPURL()
		wantPath := core.CloudflareImagesCustomDeliveryPath + r.Account.String() + "/" + id + "/" + r.Variant.String()
		if got.Validate() != nil || !got.SameOrigin(r.Origin) || u.Path != wantPath || u.Scheme != core.SchemeHTTPS || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.User != nil || strings.ContainsAny(got.String(), " ,\\\t\r\n") {
			t.Fatalf("address=%q, want exact escaped identity and origin %q", got.String(), wantPath)
		}
		again, parseErr := core.ParseHTTPEndpoint(got.String())
		if parseErr != nil || again.String() != got.String() {
			t.Fatalf("canonical address=(%v,%v), want %v", again, parseErr, got)
		}
	})
}
