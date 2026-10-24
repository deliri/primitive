package objectstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestS3NominalIngressLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		kind        uint8
		want        bool
	}{
		{name: "region ordinary", value: "us-east-1", want: true},
		{name: "region partition label", value: "us-gov-west-1", want: true},
		{name: "region future syntax is not a region registry", value: "future-region-99", want: true},
		{name: "region minimum scope label", value: "a", want: true},
		{name: "region neutral absent"},
		{name: "region uppercase refused", value: "US-east-1"},
		{name: "region scope slash refused", value: "us-east-1/s3"},
		{name: "region control refused", value: "us-east-1\n"},
		{name: "region leading dash refused", value: "-us-east-1"},
		{name: "region trailing dash refused", value: "us-east-1-"},
		{name: "bucket exact minimum", kind: 1, value: "abc", want: true},
		{name: "bucket below minimum", kind: 1, value: "ab"},
		{name: "bucket above minimum", kind: 1, value: "abcd", want: true},
		{name: "bucket below maximum", kind: 1, value: strings.Repeat("a", 62), want: true},
		{name: "bucket exact maximum", kind: 1, value: strings.Repeat("a", 63), want: true},
		{name: "bucket above maximum", kind: 1, value: strings.Repeat("a", 64)},
		{name: "bucket dotted general purpose", kind: 1, value: "retention.example", want: true},
		{name: "bucket adjacent periods refused", kind: 1, value: "retention..example"},
		{name: "bucket IPv4 shape refused", kind: 1, value: "192.168.5.4"},
		{name: "bucket uppercase refused", kind: 1, value: "Retention"},
		{name: "bucket illegal underscore refused", kind: 1, value: "retention_example"},
		{name: "bucket leading hyphen refused", kind: 1, value: "-retention"},
		{name: "bucket trailing period refused", kind: 1, value: "retention."},
		{name: "bucket punycode reservation", kind: 1, value: "xn--retention"},
		{name: "bucket native sthree reservation", kind: 1, value: "sthree-retention"},
		{name: "bucket documentation reservation", kind: 1, value: "amzn-s3-demo-retention"},
		{name: "bucket access point alias refused", kind: 1, value: "retention-s3alias"},
		{name: "bucket object lambda alias refused", kind: 1, value: "retention--ol-s3"},
		{name: "bucket multiregion access point refused", kind: 1, value: "retention.mrap"},
		{name: "bucket directory resource refused", kind: 1, value: "retention--x-s3"},
		{name: "bucket table resource refused", kind: 1, value: "retention--table-s3"},
		{name: "key neutral absent", kind: 2},
		{name: "key one byte", kind: 2, value: "a", want: true},
		{name: "key below maximum", kind: 2, value: strings.Repeat("a", 1023), want: true},
		{name: "key exact maximum", kind: 2, value: strings.Repeat("a", 1024), want: true},
		{name: "key above maximum", kind: 2, value: strings.Repeat("a", 1025)},
		{name: "key Unicode exact byte maximum", kind: 2, value: strings.Repeat("é", 512), want: true},
		{name: "key Unicode above byte maximum", kind: 2, value: strings.Repeat("é", 512) + "a"},
		{name: "key invalid UTF8 refused", kind: 2, value: string([]byte{0xff})},
		{name: "key reserved URL text preserved", kind: 2, value: "+%?# ", want: true},
		{name: "key controls preserved as encoded data", kind: 2, value: "a\x00\r\n", want: true},
		{name: "key slashes do not become filesystem hierarchy", kind: 2, value: "//a///b/", want: true},
		{name: "key dot prefix preserved", kind: 2, value: "./a", want: true},
		{name: "key balanced parents preserved", kind: 2, value: "a/b/../../c", want: true},
		{name: "key parent exceeds prior segments", kind: 2, value: "a/../../c"},
		{name: "key leading parent refused", kind: 2, value: "../a"},
		{name: "key dot does not authorize parent", kind: 2, value: "./../a"},
		{name: "key dotted text is not a parent", kind: 2, value: "..backup/.hidden", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			switch tc.kind {
			case 0:
				got, err := ParseS3Region(tc.value)
				if tc.want {
					if err != nil || got.String() != tc.value {
						t.Fatalf("ParseS3Region = (%q,%v), want exact %q", got.String(), err, tc.value)
					}
					s3NominalCodecClosure(t, got, []byte(tc.value), false)
				} else if !errors.Is(err, core.ErrObjectStoreContract) || got != (S3Region{}) {
					t.Fatalf("ParseS3Region error=%v value=%q, want typed zero refusal", err, got.String())
				}
			case 1:
				got, err := ParseS3Bucket(tc.value)
				if tc.want {
					if err != nil || got.String() != tc.value {
						t.Fatalf("ParseS3Bucket = (%q,%v), want exact %q", got.String(), err, tc.value)
					}
					s3NominalCodecClosure(t, got, []byte(tc.value), false)
				} else if !errors.Is(err, core.ErrObjectStoreContract) || got != (S3Bucket{}) {
					t.Fatalf("ParseS3Bucket error=%v value=%q, want typed zero refusal", err, got.String())
				}
			case 2:
				got, err := ParseS3Key(tc.value)
				if tc.want {
					if err != nil || got.String() != tc.value {
						t.Fatalf("ParseS3Key = (%q,%v), want exact %q", got.String(), err, tc.value)
					}
					s3NominalCodecClosure(t, got, []byte(tc.value), false)
				} else if !errors.Is(err, core.ErrObjectStoreContract) || got != (S3Key{}) {
					t.Fatalf("ParseS3Key error=%v value=%q, want typed zero refusal", err, got.String())
				}
			default:
				t.Fatalf("fixture kind=%d, want a declared nominal", tc.kind)
			}
		})
	}
}
