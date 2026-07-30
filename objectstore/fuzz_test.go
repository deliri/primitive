package objectstore_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func FuzzParseSignedURL(f *testing.F) {
	f.Add(
		"https://s3.amazonaws.com/bucket/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=host",
	)
	f.Add(
		"https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=host",
	)
	f.Add("https://upload.imagedelivery.net/image-id")
	f.Add("")
	f.Add("http://example.com/object?signature=secret")
	f.Add("https://example.com/")

	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := objectstore.ParseSignedURL(value)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf(
					"ParseSignedURL(%q) error = %v, want %v",
					value,
					gotErr,
					core.ErrObjectStoreContract,
				)
			}
			if gotValidateErr := got.Validate(); gotValidateErr == nil {
				t.Fatalf(
					"ParseSignedURL(%q) rejected value Validate() error = nil, want rejection",
					value,
				)
			}
			return
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf(
				"ParseSignedURL(%q) accepted value Validate() error = %v, want nil",
				value,
				gotValidateErr,
			)
		}
		if formatted := fmt.Sprintf("%v", got); formatted != core.RedactedValueText {
			t.Fatalf(
				"ParseSignedURL(%q) formatted = %q, want %q",
				value,
				formatted,
				core.RedactedValueText,
			)
		}
	})
}
