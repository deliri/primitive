package objectstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type s3Nominal interface {
	comparable
	Validate() error
	String() string
	MarshalJSON() ([]byte, error)
	MarshalText() ([]byte, error)
}

type s3NominalDecoder[T s3Nominal] interface {
	*T
	UnmarshalJSON([]byte) error
	UnmarshalText([]byte) error
}

func s3NominalCodecClosure[T s3Nominal, P s3NominalDecoder[T]](t *testing.T, before T, data []byte, isJSON bool) {
	t.Helper()
	got := before
	var err error
	if isJSON {
		err = P(&got).UnmarshalJSON(data)
	} else {
		err = P(&got).UnmarshalText(data)
	}
	if err != nil {
		if !errors.Is(err, core.ErrObjectStoreContract) || got != before {
			t.Fatalf("nominal rejection error=%v preserved=%v, want typed and transactional", err, got == before)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("accepted nominal validation=%v, want nil", err)
	}
	var want string
	if isJSON {
		want, err = jsonio.DecodeText(data)
	} else {
		want = string(data)
	}
	if err != nil || got.String() != want {
		t.Fatalf("accepted nominal=%q decode=%v, want exact input %q", got.String(), err, want)
	}
	first, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("nominal MarshalJSON error=%v, want nil", err)
	}
	var round T
	if err := P(&round).UnmarshalJSON(first); err != nil || round != got {
		t.Fatalf("nominal JSON roundtrip error=%v equality=%v, want exact identity", err, round == got)
	}
	second, err := round.MarshalJSON()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("canonical nominal second bytes=%q error=%v, want %q", second, err, first)
	}
	text, err := got.MarshalText()
	if err != nil || string(text) != want {
		t.Fatalf("nominal text=%q error=%v, want %q", text, err, want)
	}
	if err := P(&round).UnmarshalText(text); err != nil || round != got {
		t.Fatalf("nominal text roundtrip error=%v equality=%v, want exact identity", err, round == got)
	}
	var absent P
	if err := absent.UnmarshalText(text); !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("nil nominal text receiver error=%v, want typed refusal", err)
	}
	if err := absent.UnmarshalJSON(first); !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("nil nominal JSON receiver error=%v, want typed refusal", err)
	}
}

func s3NominalFuzzSeeds(f *testing.F, isJSON bool) {
	f.Helper()
	region, _ := ParseS3Region("us-east-1")
	bucket, _ := ParseS3Bucket("retention.example")
	key, _ := ParseS3Key("//東京/+%?#\x00/a/../archive")
	if isJSON {
		data, err := region.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(uint8(0), data)
		data, err = bucket.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(uint8(1), data)
		data, err = key.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(uint8(2), data)
		f.Add(uint8(1), []byte("null"))
		f.Add(uint8(2), []byte(`"a" "b"`))
		f.Add(uint8(0), []byte("31"))
	} else {
		data, err := region.MarshalText()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(uint8(0), data)
		data, err = bucket.MarshalText()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(uint8(1), data)
		data, err = key.MarshalText()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(uint8(2), data)
		f.Add(uint8(2), []byte{0xff})
		f.Add(uint8(1), []byte("xn--reserved"))
		f.Add(uint8(0), []byte("us/evil"))
	}
	f.Add(uint8(2), []byte{})
}

func FuzzS3NominalJSONSemanticClosure(f *testing.F) {
	s3NominalFuzzSeeds(f, true)
	f.Fuzz(func(t *testing.T, kind uint8, data []byte) {
		// Campaign allocation bound; this is not a production transfer quota.
		if len(data) > 64*1024 {
			t.Skip("fuzz campaign input budget")
		}
		switch kind % 3 {
		case 0:
			before, _ := ParseS3Region("us-east-1")
			s3NominalCodecClosure(t, before, data, true)
		case 1:
			before, _ := ParseS3Bucket("retention.example")
			s3NominalCodecClosure(t, before, data, true)
		case 2:
			before, _ := ParseS3Key("prior")
			s3NominalCodecClosure(t, before, data, true)
		}
	})
}

func FuzzS3NominalTextSemanticClosure(f *testing.F) {
	s3NominalFuzzSeeds(f, false)
	f.Fuzz(func(t *testing.T, kind uint8, data []byte) {
		if len(data) > 64*1024 {
			t.Skip("fuzz campaign input budget")
		}
		switch kind % 3 {
		case 0:
			before, _ := ParseS3Region("us-east-1")
			got, err := ParseS3Region(string(data))
			if err != nil && got != (S3Region{}) {
				t.Fatalf("region parse refusal populated=%v, want zero", got != (S3Region{}))
			}
			s3NominalCodecClosure(t, before, data, false)
		case 1:
			before, _ := ParseS3Bucket("retention.example")
			got, err := ParseS3Bucket(string(data))
			if err != nil && got != (S3Bucket{}) {
				t.Fatalf("bucket parse refusal populated=%v, want zero", got != (S3Bucket{}))
			}
			s3NominalCodecClosure(t, before, data, false)
		case 2:
			before, _ := ParseS3Key("prior")
			got, err := ParseS3Key(string(data))
			if err != nil && got != (S3Key{}) {
				t.Fatalf("key parse refusal populated=%v, want zero", got != (S3Key{}))
			}
			s3NominalCodecClosure(t, before, data, false)
		}
	})
}

func FuzzS3StaticCredentialFileSemanticClosure(f *testing.F) {
	f.Add(s3CredentialFixtureJSON(f))
	f.Add([]byte(`{"access_key_id":"ASIAIOSFODNN7EXAMPLE","secret_access_key":"unsupported temporary pair"}`))
	f.Add([]byte(`{"access_key_id":"secret-looking unknown field"}`))
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			t.Skip("fuzz campaign input budget")
		}
		got, err := decodeS3FileCredentials(t.Context(), bytes.NewReader(data))
		defer got.destroy()
		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreContract) || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 {
				t.Fatalf("credential refusal error=%v sizes=(%d,%d), want typed zero", err, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted credential validation=%v, want nil", err)
		}
		// The native hook-free foreign projection is an independent decoder of
		// the original representation, not a copy of credential validation.
		wire, err := jsonio.DecodeProjectionReader[s3CredentialFixture](context.Background(), bytes.NewReader(data), jsonio.ProjectionReadPolicy{})
		if err != nil || wire.AccessKeyID != string(got.AccessKeyID.value) || wire.SecretAccessKey != string(got.SecretAccessKey.value) {
			t.Fatalf("credential native projection error=%v sizes=(%d,%d), want exact accepted key fields", err, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
		}
		if strings.HasPrefix(wire.AccessKeyID, "ASIA") {
			t.Fatalf("accepted temporary credential flag=%v, want false for static-only admission", strings.HasPrefix(wire.AccessKeyID, "ASIA"))
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
	})
}
