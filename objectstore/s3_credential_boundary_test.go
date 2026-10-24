package objectstore

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestS3StaticCredentialNativeBoundariesLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, access, secret string
		want                 bool
	}{
		{name: "below IAM access ID minimum", access: strings.Repeat("A", 15), secret: s3FixtureSecret},
		{name: "exact IAM access ID minimum", access: strings.Repeat("A", 16), secret: s3FixtureSecret, want: true},
		{name: "above IAM access ID minimum", access: strings.Repeat("A", 17), secret: s3FixtureSecret, want: true},
		{name: "below IAM access ID maximum", access: strings.Repeat("A", 127), secret: s3FixtureSecret, want: true},
		{name: "exact IAM access ID maximum", access: strings.Repeat("A", 128), secret: s3FixtureSecret, want: true},
		{name: "above IAM access ID maximum", access: strings.Repeat("A", 129), secret: s3FixtureSecret},
		{name: "IAM word character retained", access: "AKIA_EXAMPLE_1234", secret: s3FixtureSecret, want: true},
		{name: "scope delimiter cannot enter access ID", access: "AKIA/EXAMPLE/1234", secret: s3FixtureSecret},
		{name: "neutral secret still required", access: s3FixtureAccess},
		{name: "one byte secret is not assigned invented AWS minimum", access: s3FixtureAccess, secret: "x", want: true},
		{name: "secret above Core storage maximum is not an AWS rejection", access: s3FixtureAccess, secret: strings.Repeat("x", 65), want: true},
		{name: "native HMAC long key continues across JSON window", access: s3FixtureAccess, secret: strings.Repeat("x", 32769), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var data bytes.Buffer
			if err := jsonio.WriteProjection(t.Context(), &data, s3CredentialFixture{AccessKeyID: tc.access, SecretAccessKey: tc.secret}, jsonio.OwnedCodecPolicy{}, jsonio.LayoutCompact); err != nil {
				t.Fatal(err)
			}
			got, gotErr := decodeS3FileCredentials(t.Context(), &data)
			defer got.destroy()
			if tc.want {
				if gotErr != nil || string(got.AccessKeyID.value) != tc.access || string(got.SecretAccessKey.value) != tc.secret {
					t.Fatalf("credential admission error=%v sizes=(%d,%d), want exact native credential sizes(%d,%d)", gotErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value), len(tc.access), len(tc.secret))
				}
				access, secret := got.AccessKeyID.value, got.SecretAccessKey.value
				got.destroy()
				for _, value := range [][]byte{access, secret} {
					for _, b := range value {
						if b != 0 {
							t.Fatalf("destroyed credential nonzero=%v, want false after clearing owned storage", b != 0)
						}
					}
				}
				return
			}
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 {
				t.Fatalf("credential rejection error=%v sizes=(%d,%d), want typed zero result", gotErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
			}
		})
	}
}
