package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

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
		page, err := wire.page()
		if err != nil {
			if !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
				t.Fatalf("refusal kept facts or lost its family: %v %+v", err, page)
			}
			return
		}
		seen := map[string]bool{}
		for _, media := range page.Media {
			if seen[media.ID] || !validMediaID(media.ID) {
				t.Fatalf("bad identity admitted: %q", media.ID)
			}
			seen[media.ID] = true
			if !utf8.ValidString(media.Caption) || utf8.RuneCountInString(media.Caption) > core.InstagramCaptionMaximumCharacters {
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

// ParseUserID admits external text into a request path.
func FuzzParseUserID(f *testing.F) {
	user, err := ParseUserID("17841432921003907")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(user.String())
	for _, hostile := range []string{"", "1/2", "1?x=y", "-1", "１", strings.Repeat("9", 33), "1\x00", " 1"} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got, err := ParseUserID(raw)
		want := raw != "" && len(raw) <= userIDMaximumBytes && strings.Trim(raw, "0123456789") == ""
		if want != (err == nil) {
			t.Fatalf("ParseUserID(%q) = %v, oracle says valid=%v", raw, err, want)
		}
		if err == nil {
			request := MediaListRequest{User: got, Policy: testPolicy(t)}
			endpoint, err := request.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			address := endpoint.HTTPURL()
			if address.Host != core.InstagramGraphHost || address.Path != "/"+core.InstagramGraphVersion+"/"+raw+"/media" || address.RawQuery != "fields="+mediaFields {
				t.Fatalf("an admitted id moved the request to %q", address.String())
			}
		}
	})
}

// ParseCredential admits secret material from outside the process.
func FuzzParseCredential(f *testing.F) {
	f.Add([]byte(testToken))
	for _, hostile := range []string{"", "Bearer " + testToken, testToken + "\n", strings.Repeat("a", core.InstagramCredentialCustodyMaximumBytes+1), "\xff\xfe"} {
		f.Add([]byte(hostile))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		credential, err := ParseCredential(raw)
		want := len(raw) >= core.InstagramCredentialMinimumBytes && len(raw) <= core.InstagramCredentialCustodyMaximumBytes
		for _, b := range raw {
			want = want && b > ' ' && b <= '~'
		}
		if want != (err == nil) {
			t.Fatalf("ParseCredential = %v, oracle says valid=%v", err, want)
		}
		if err == nil && len(raw) > 0 {
			before := string(raw)
			clear(raw)
			if credential.Validate() != nil || string(credential.value) != before {
				t.Fatal("the credential shares or lost the caller's bytes")
			}
		}
	})
}

// ListMedia is the public door for Meta's HTTP answer: any status, any body.
func FuzzListMediaResponse(f *testing.F) {
	canonical, err := json.Marshal(mediaListWire{Data: []mediaWire{validWire()}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(200, canonical)
	f.Add(400, []byte(`{"error":{"code":190}}`))
	f.Add(401, []byte{})
	f.Add(500, []byte(`not json`))
	f.Add(200, []byte(`{"data":null}`))
	f.Fuzz(func(t *testing.T, status int, body []byte) {
		if status < 200 || status > 599 {
			return
		}
		fake := &metaFake{status: status, body: string(body)}
		page, err := fakeClient(t, fake).ListMedia(context.Background(), MediaListRequest{User: testUser(t), Policy: testPolicy(t)})
		if err != nil {
			if !errors.Is(err, core.ErrInstagramContract) || len(page.Media) != 0 {
				t.Fatalf("a failure kept media or lost its family: %v", err)
			}
			if errors.Is(err, core.ErrInstagramAuthentication) && !strings.Contains(string(body), "190") {
				t.Fatalf("authentication claimed without Meta's code 190: %d %q", status, body)
			}
			return
		}
		if status != 200 || len(page.Media) > core.InstagramMediaPageCustodyMaximum {
			t.Fatalf("status %d admitted %d media", status, len(page.Media))
		}
	})
}
