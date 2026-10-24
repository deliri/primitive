package objectstore

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/smithy-go/encoding/httpbinding"
	"github.com/deliri/primitive/v2026/core"
)

// This checks the pinned provider serializer, before the generic signer sees
// a URL. A signer oracle given an already misescaped URL cannot prove this seam.
func TestS3NativePathEncodingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, key string
		wantErr   bool
	}{
		{name: "positive AWS unreserved bytes", key: "Az09-._~"},
		{name: "reserved path punctuation uses AWS percent encoding", key: "a+b:c@d&e=f;$g"},
		{name: "Unicode percent and NUL are exact key bytes", key: "東京/%+#?\x00"},
		{name: "neutral slash and dot spelling is never cleaned", key: "//./a/../"},
		{name: "negative missing key cannot issue target", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r := s3FileFixture(t, dir, []byte("sealed"))
			if tc.wantErr {
				r.Key = S3Key{}
			} else {
				var err error
				r.Key, err = ParseS3Key(tc.key)
				if err != nil {
					t.Fatal(err)
				}
			}
			credentials, err := decodeS3FileCredentials(t.Context(), bytes.NewReader(s3CredentialFixtureJSON(t)))
			if err != nil {
				t.Fatal(err)
			}
			defer credentials.destroy()
			got, gotErr := presignS3File(t.Context(), r, credentials)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrObjectStoreContract) || got.URL.value != nil || got.ExpiresAt.IsSet() {
					t.Fatalf("missing key presign error=%v target=%v, want typed zero target", gotErr, got)
				}
				return
			}
			want := httpbinding.EscapePath("/"+r.Bucket.String()+"/"+tc.key, false)
			if gotErr != nil || got.URL.value == nil {
				t.Fatalf("native path presign error=%v target=%v, want valid target", gotErr, got)
			}
			if path := got.URL.value.EscapedPath(); path != want || got.URL.value.Path != "/"+r.Bucket.String()+"/"+tc.key {
				t.Fatalf("S3 wire path=%q decoded=%q, want pinned native path=%q and exact key=%q", path, got.URL.value.Path, want, tc.key)
			}
		})
	}
}
