package cloudflare

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func TestImageCustomIDRejectsUUIDAndPrivateDelivery(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		text    string
		private bool
	}{
		{name: "custom UTF8 path is accepted", text: "tenant/café"},
		{name: "UUID belongs to provider generated identity", text: "3f4aeabc-a222-11ee-8c90-0242ac120002", wantErr: core.ErrCloudflareBinding},
		{name: "custom private identity is unsupported", text: "tenant/photo", private: true, wantErr: core.ErrCloudflareBinding},
		{name: "ordinary hyphenated custom text is not a UUID", text: "owner-photo-123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, err := ParseImageID(tc.text)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := (ImageDirectUploadRequest{CustomID: id, RequireSignedURLs: tc.private}).Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("custom ID request error=%v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestStreamIDAdmitsPublishedOpaqueIdentifier(t *testing.T) {
	t.Parallel()
	// The public schema defines maxLength=32, not an exactly-32 hex contract.
	// https://developers.cloudflare.com/api/resources/stream/methods/get/
	const published = "ea95132c15732412d22c1476fa83f27a"
	got, err := ParseStreamVideoID(published)
	if err != nil || got.String() != published {
		t.Fatalf("published Stream ID=(%v,%v), want exact admitted ID", got, err)
	}
}

func TestAccountIDExhaustsByteAlphabetAndLengthBoundary(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		input := strings.Repeat("a", core.CloudflareIdentityCharacters-1) + string([]byte{byte(raw)})
		got, err := ParseAccountID(input)
		want := raw >= '0' && raw <= '9' || raw >= 'a' && raw <= 'f'
		if (err == nil) != want {
			t.Fatalf("account byte %d error=%v, want admitted=%t", raw, err, want)
		}
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (AccountID{}) {
				t.Fatalf("refusal=(%v,%v), want zero typed binding refusal", got, err)
			}
		} else if got.String() != input || got.Validate() != nil {
			t.Fatalf("admitted account=%v, want exact %q", got, input)
		}
	}
	for _, size := range []int{0, core.CloudflareIdentityCharacters - 1, core.CloudflareIdentityCharacters, core.CloudflareIdentityCharacters + 1, 4096} {
		got, err := ParseAccountID(strings.Repeat("a", size))
		want := size == core.CloudflareIdentityCharacters
		if (err == nil) != want {
			t.Fatalf("account length %d=(%v,%v), want admitted=%t", size, got, err, want)
		}
	}
}

func FuzzCloudflareIdentityRepresentationClosure(f *testing.F) {
	seed, err := ParseImageID("tenant/café")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("/leading")
	f.Add("trailing/")
	f.Add(string([]byte{0xff}))
	f.Add(strings.Repeat("a", core.CloudflareImageIDMaximumCharacters+1))
	f.Fuzz(func(t *testing.T, value string) {
		image, imageErr := ParseImageID(value)
		if imageErr != nil {
			if !errors.Is(imageErr, core.ErrCloudflareBinding) || image != (ImageID{}) {
				t.Fatalf("image refusal=(%v,%v), want zero typed refusal", image, imageErr)
			}
		} else {
			if image.Validate() != nil || image.String() != value || !utf8.ValidString(value) || utf8.RuneCountInString(value) > core.CloudflareImageIDMaximumCharacters || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.ContainsAny(value, "\x00\r\n") {
				t.Fatalf("accepted image=%v, want exact valid bounded ID", image)
			}
		}
		account, accountErr := ParseAccountID(value)
		if accountErr != nil {
			if !errors.Is(accountErr, core.ErrCloudflareBinding) || account != (AccountID{}) {
				t.Fatalf("account refusal=(%v,%v), want zero typed refusal", account, accountErr)
			}
		} else {
			if account.Validate() != nil || account.String() != value || len(value) != core.CloudflareIdentityCharacters || strings.Trim(value, "0123456789abcdef") != "" {
				t.Fatalf("accepted account=%v, want lowercase hex identity", account)
			}
		}
		video, videoErr := ParseStreamVideoID(value)
		if videoErr != nil {
			if !errors.Is(videoErr, core.ErrCloudflareBinding) || video != (StreamVideoID{}) {
				t.Fatalf("video refusal=(%v,%v), want zero typed refusal", video, videoErr)
			}
		} else if video.Validate() != nil || video.String() != value {
			t.Fatalf("accepted video=%v, want exact input", video)
		}
	})
}
