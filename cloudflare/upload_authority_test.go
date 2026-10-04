package cloudflare

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestUploadAuthoritiesRejectCrossProviderAndForeignOrigins(t *testing.T) {
	t.Parallel()
	image, err := ParseImageID("tenant/photo")
	if err != nil {
		t.Fatal(err)
	}
	video, err := ParseStreamVideoID(strings.Repeat("b", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		wantErr error
		name    string
		url     string
		image   bool
	}{
		{name: "Images HTTPS upload origin is admitted", url: "https://" + core.CloudflareImagesUploadHost + "/one-use", image: true},
		{name: "Stream HTTPS upload origin is admitted", url: "https://" + core.CloudflareStreamUploadHost + "/one-use"},
		{name: "Images grant cannot be used as Stream grant", url: "https://" + core.CloudflareImagesUploadHost + "/one-use", wantErr: core.ErrCloudflareBinding},
		{name: "Stream grant cannot be used as Images grant", url: "https://" + core.CloudflareStreamUploadHost + "/one-use", image: true, wantErr: core.ErrCloudflareBinding},
		{name: "insecure Images transport is rejected", url: "http://" + core.CloudflareImagesUploadHost + "/one-use", image: true, wantErr: core.ErrCloudflareBinding},
		{name: "foreign subdomain cannot inherit upload authority", url: "https://" + core.CloudflareImagesUploadHost + ".attacker.invalid/one-use", image: true, wantErr: core.ErrCloudflareBinding},
		{name: "empty upload path cannot become a granted resource", url: "https://" + core.CloudflareStreamUploadHost + "/", wantErr: core.ErrCloudflareBinding},
		{name: "alternate port cannot redirect bearer authority", url: "https://" + core.CloudflareStreamUploadHost + ":8443/one-use", wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.image {
				got, err := ParseImageUpload(image, tc.url)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("image grant error=%v, want %v", err, tc.wantErr)
				}
				if err != nil && got != (ImageUpload{}) {
					t.Fatalf("image refusal=%v, want zero", got)
				}
			} else {
				got, err := ParseStreamUpload(video, tc.url)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Stream grant error=%v, want %v", err, tc.wantErr)
				}
				if err != nil && got != (StreamUpload{}) {
					t.Fatalf("Stream refusal=%v, want zero", got)
				}
			}
		})
	}
}

func TestImageExpiryAttacksBothDocumentedBoundaries(t *testing.T) {
	t.Parallel()
	now, err := temporal.InstantFromUnixSeconds(webhookTestSeconds)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		wantErr error
		name    string
		seconds uint64
	}{
		{name: "one below minimum expiry is rejected", seconds: core.CloudflareImageExpiryMinimumSeconds - 1, wantErr: core.ErrCloudflareContract},
		{name: "minimum expiry is accepted", seconds: core.CloudflareImageExpiryMinimumSeconds},
		{name: "one above minimum expiry is accepted", seconds: core.CloudflareImageExpiryMinimumSeconds + 1},
		{name: "one below maximum expiry is accepted", seconds: core.CloudflareImageExpiryMaximumSeconds - 1},
		{name: "maximum expiry is accepted", seconds: core.CloudflareImageExpiryMaximumSeconds},
		{name: "one above maximum expiry is rejected", seconds: core.CloudflareImageExpiryMaximumSeconds + 1, wantErr: core.ErrCloudflareContract},
		{name: "distant expiry does not bypass the six-hour ceiling", seconds: 1_000_000, wantErr: core.ErrCloudflareContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			duration, err := temporal.DurationFromSeconds(tc.seconds)
			if err != nil {
				t.Fatal(err)
			}
			expiry, err := now.Add(duration)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := (ImageDirectUploadRequest{ObservedAt: now, Expiry: expiry}).Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("expiry validation=%v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestMediaExtentCeilingIsDistinctFromCopyWindow(t *testing.T) {
	t.Parallel()
	for _, maximum := range []uint64{core.CloudflareImagesUploadMaximumBytes, core.CloudflareStreamBasicUploadMaximumBytes} {
		for _, size := range []uint64{0, maximum - 1, maximum, maximum + 1, 1<<63 - 1} {
			length, err := core.NewByteLength(size)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := (MediaUpload{Source: bytes.NewReader(nil), Response: io.Discard, Filename: "media.bin", Bytes: length}).validateMaximum(maximum)
			want := size <= maximum
			if (gotErr == nil) != want {
				t.Fatalf("source extent %d with ceiling %d error=%v, want admitted=%t", size, maximum, gotErr, want)
			}
		}
	}
}

func FuzzCloudflareUploadAuthorityClosure(f *testing.F) {
	image, err := ParseImageID("tenant/photo")
	if err != nil {
		f.Fatal(err)
	}
	video, err := ParseStreamVideoID(strings.Repeat("b", core.CloudflareIdentityCharacters))
	if err != nil {
		f.Fatal(err)
	}
	imageSeed, err := ParseImageUpload(image, "https://"+core.CloudflareImagesUploadHost+"/one-use")
	if err != nil {
		f.Fatal(err)
	}
	streamSeed, err := ParseStreamUpload(video, "https://"+core.CloudflareStreamUploadHost+"/one-use")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(imageSeed.endpoint.String())
	f.Add(streamSeed.endpoint.String())
	f.Add("")
	f.Add("https://user:secret@foreign.example/")
	f.Fuzz(func(t *testing.T, value string) {
		gotImage, imageErr := ParseImageUpload(image, value)
		if imageErr != nil {
			if !errors.Is(imageErr, core.ErrCloudflareContract) || gotImage != (ImageUpload{}) {
				t.Fatalf("image refusal=(%v,%v), want typed zero", gotImage, imageErr)
			}
		} else {
			u := gotImage.endpoint.HTTPURL()
			if gotImage.Validate() != nil || gotImage.ID != image || u.Scheme != core.SchemeHTTPS || u.Host != core.CloudflareImagesUploadHost || u.Path == "" || u.Path == "/" {
				t.Fatalf("image grant = (%v,%q,%q,%q), want exact input and Images authority", gotImage.Validate(), gotImage.ID.value, u.Scheme, u.Host)
			}
			again, err := ParseImageUpload(gotImage.ID, gotImage.endpoint.String())
			if err != nil || again != gotImage {
				t.Fatalf("image canonical closure error=%v, want identical authority", err)
			}
		}
		gotStream, streamErr := ParseStreamUpload(video, value)
		if streamErr != nil {
			if !errors.Is(streamErr, core.ErrCloudflareContract) || gotStream != (StreamUpload{}) {
				t.Fatalf("Stream refusal=(%v,%v), want typed zero", gotStream, streamErr)
			}
		} else {
			u := gotStream.endpoint.HTTPURL()
			if gotStream.Validate() != nil || gotStream.ID != video || u.Scheme != core.SchemeHTTPS || u.Host != core.CloudflareStreamUploadHost || u.Path == "" || u.Path == "/" {
				t.Fatalf("Stream grant = (%v,%q,%q,%q), want exact input and Stream authority", gotStream.Validate(), gotStream.ID.value, u.Scheme, u.Host)
			}
			again, err := ParseStreamUpload(gotStream.ID, gotStream.endpoint.String())
			if err != nil || again != gotStream {
				t.Fatalf("Stream canonical closure error=%v, want identical authority", err)
			}
		}
	})
}

func TestUploadAuthorityCanonicalProjectionPreservesRequestMeaning(t *testing.T) {
	t.Parallel()
	image, err := ParseImageID("tenant/photo")
	if err != nil {
		t.Fatal(err)
	}
	video, err := ParseStreamVideoID(strings.Repeat("b", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{name: "Furnace raw-space crasher has one nominal representation", path: "/ ", want: "/%20"},
		{name: "unescaped Unicode uses Go percent encoding", path: "/é", want: "/%C3%A9"},
		{name: "raw bracket retains Go path representation", path: "/[", want: "/["},
		{name: "escaped slash remains distinct from a path separator", path: "/a%2fb", want: "/a%2fb"},
		{name: "unreserved percent spelling remains part of signed request", path: "/%61", want: "/%61"},
		{name: "query order and plus spelling are preserved", path: "/one-use?b=a+b&a=%20", want: "/one-use?b=a+b&a=%20"},
		{name: "explicit empty query marker remains present", path: "/one-use?", want: "/one-use?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			imagePrefix := core.SchemeHTTPS + "://" + core.CloudflareImagesUploadHost
			gotImage, err := ParseImageUpload(image, imagePrefix+tc.path)
			if err != nil {
				t.Fatal(err)
			}
			imageAgain, err := ParseImageUpload(image, gotImage.endpoint.String())
			if err != nil || imageAgain != gotImage || gotImage.endpoint.String() != imagePrefix+tc.want {
				t.Fatalf("Images projection = (%q, stable=%t, error=%v), want (%q, true, nil)", gotImage.endpoint.String(), imageAgain == gotImage, err, imagePrefix+tc.want)
			}
			streamPrefix := core.SchemeHTTPS + "://" + core.CloudflareStreamUploadHost
			gotStream, err := ParseStreamUpload(video, streamPrefix+tc.path)
			if err != nil {
				t.Fatal(err)
			}
			streamAgain, err := ParseStreamUpload(video, gotStream.endpoint.String())
			if err != nil || streamAgain != gotStream || gotStream.endpoint.String() != streamPrefix+tc.want {
				t.Fatalf("Stream projection = (%q, stable=%t, error=%v), want (%q, true, nil)", gotStream.endpoint.String(), streamAgain == gotStream, err, streamPrefix+tc.want)
			}
		})
	}
}
