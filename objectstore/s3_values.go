package objectstore

import (
	"errors"
	"net/netip"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// S3Region is an explicit signing-region label. Validation checks scope
// syntax, not whether AWS has provisioned a region or enabled an account.
type S3Region struct{ value string }

func ParseS3Region(value string) (S3Region, error) {
	r := S3Region{value: value}
	if err := r.Validate(); err != nil {
		return S3Region{}, err
	}
	return r, nil
}

func (r S3Region) Validate() error {
	if r.value == "" || !s3LowerDigit(r.value[0]) || !s3LowerDigit(r.value[len(r.value)-1]) {
		return core.ErrObjectStoreContract
	}
	for _, c := range r.value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}
func (r S3Region) String() string {
	if r.Validate() != nil {
		return ""
	}
	return r.value
}
func (r S3Region) MarshalText() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return []byte(r.value), nil
}
func (r *S3Region) UnmarshalText(data []byte) error {
	if r == nil {
		return core.ErrObjectStoreContract
	}
	got, err := ParseS3Region(string(data))
	if err == nil {
		*r = got
	}
	return err
}
func (r S3Region) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return jsonio.MarshalText(r.value)
}
func (r *S3Region) UnmarshalJSON(data []byte) error {
	value, err := jsonio.DecodeText(data)
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := r.UnmarshalText([]byte(value)); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	return nil
}

// S3Bucket names a general-purpose bucket, including dotted names. Access
// point aliases, directory buckets and other S3 resource kinds are separate
// protocols and are not admitted by this path-style PutObject door.
// Source: https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html
type S3Bucket struct{ value string }

func ParseS3Bucket(value string) (S3Bucket, error) {
	b := S3Bucket{value: value}
	if err := b.Validate(); err != nil {
		return S3Bucket{}, err
	}
	return b, nil
}
func (b S3Bucket) Validate() error {
	if len(b.value) < 3 || len(b.value) > 63 || !s3LowerDigit(b.value[0]) || !s3LowerDigit(b.value[len(b.value)-1]) || strings.Contains(b.value, "..") {
		return core.ErrObjectStoreContract
	}
	for _, c := range b.value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
			return core.ErrObjectStoreContract
		}
	}
	if _, err := netip.ParseAddr(b.value); err == nil {
		return core.ErrObjectStoreContract
	}
	for _, prefix := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(b.value, prefix) {
			return core.ErrObjectStoreContract
		}
	}
	for _, suffix := range []string{"-s3alias", "--ol-s3", ".mrap", "--x-s3", "--table-s3"} {
		if strings.HasSuffix(b.value, suffix) {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}
func (b S3Bucket) String() string {
	if b.Validate() != nil {
		return ""
	}
	return b.value
}
func (b S3Bucket) MarshalText() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return []byte(b.value), nil
}
func (b *S3Bucket) UnmarshalText(data []byte) error {
	if b == nil {
		return core.ErrObjectStoreContract
	}
	got, err := ParseS3Bucket(string(data))
	if err == nil {
		*b = got
	}
	return err
}
func (b S3Bucket) MarshalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return jsonio.MarshalText(b.value)
}
func (b *S3Bucket) UnmarshalJSON(data []byte) error {
	value, err := jsonio.DecodeText(data)
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := b.UnmarshalText([]byte(value)); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	return nil
}

// S3Key retains the exact UTF-8 object identity. It is not a filesystem path:
// repeated slashes, leading slashes, spaces, NUL, Unicode and balanced relative
// elements are preserved. AWS's 1024-byte bound includes every prefix byte.
// Source: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-keys.html
type S3Key struct{ value string }

func ParseS3Key(value string) (S3Key, error) {
	k := S3Key{value: value}
	if err := k.Validate(); err != nil {
		return S3Key{}, err
	}
	return k, nil
}
func (k S3Key) Validate() error {
	if len(k.value) == 0 || len(k.value) > 1024 || !utf8.ValidString(k.value) {
		return core.ErrObjectStoreContract
	}
	depth := 0
	for segment := range strings.SplitSeq(k.value, "/") {
		switch segment {
		case "", ".":
		case "..":
			depth--
			if depth < 0 {
				return core.ErrObjectStoreContract
			}
		default:
			depth++
		}
	}
	return nil
}
func (k S3Key) String() string {
	if k.Validate() != nil {
		return ""
	}
	return k.value
}
func (k S3Key) MarshalText() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	return []byte(k.value), nil
}
func (k *S3Key) UnmarshalText(data []byte) error {
	if k == nil {
		return core.ErrObjectStoreContract
	}
	got, err := ParseS3Key(string(data))
	if err == nil {
		*k = got
	}
	return err
}
func (k S3Key) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return jsonio.MarshalText(k.value)
}
func (k *S3Key) UnmarshalJSON(data []byte) error {
	value, err := jsonio.DecodeText(data)
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := k.UnmarshalText([]byte(value)); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	return nil
}

func s3LowerDigit(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

var (
	_ core.Validatable = S3Region{}
	_ core.Validatable = S3Bucket{}
	_ core.Validatable = S3Key{}
)
