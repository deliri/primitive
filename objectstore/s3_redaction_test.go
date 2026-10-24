package objectstore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestS3PrivateCredentialRedactionLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive populated owners redact every formatting verb", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		request := s3FileFixture(t, dir, []byte("sealed"))
		credentials, err := decodeS3FileCredentials(t.Context(), bytes.NewReader(s3CredentialFixtureJSON(t)))
		if err != nil {
			t.Fatal(err)
		}
		defer credentials.destroy()
		for _, owner := range []struct {
			name  string
			value fmt.Formatter
		}{
			{name: "request", value: request},
			{name: "credential pair", value: credentials},
			{name: "access key", value: credentials.AccessKeyID},
			{name: "secret key", value: credentials.SecretAccessKey},
		} {
			for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
				if got := fmt.Sprintf(verb, owner.value); got != core.RedactedValueText {
					t.Fatalf("%s formatter %s redacted=%v output bytes=%d, want exact redacted projection", owner.name, verb, got == core.RedactedValueText, len(got))
				}
			}
		}
	})
	t.Run("negative credential JSON export refuses without secret bytes", func(t *testing.T) {
		t.Parallel()
		credentials, err := decodeS3FileCredentials(t.Context(), bytes.NewReader(s3CredentialFixtureJSON(t)))
		if err != nil {
			t.Fatal(err)
		}
		defer credentials.destroy()
		got, gotErr := jsonio.MarshalValidated(credentials, jsonio.OwnedCodecPolicy{})
		if len(got) != 0 || !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf("credential JSON bytes=%d error=%v, want zero bytes and typed export refusal", len(got), gotErr)
		}
		cause := fmt.Errorf("private secret %s: %w", s3FixtureSecret, io.ErrClosedPipe)
		refusal := s3FileRefusal(cause)
		if !errors.Is(refusal, io.ErrClosedPipe) || !errors.Is(refusal, core.ErrObjectStoreContract) {
			t.Fatalf("redacted error native-pipe=%v typed=%v, want both retained identities", errors.Is(refusal, io.ErrClosedPipe), errors.Is(refusal, core.ErrObjectStoreContract))
		}
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			if got := fmt.Sprintf(verb, refusal); got != "S3 file upload refused" {
				t.Fatalf("error formatter %s safe projection=%v output bytes=%d, want fixed redacted diagnostic", verb, got == "S3 file upload refused", len(got))
			}
		}
	})
	t.Run("neutral absent owners never expose internal representation", func(t *testing.T) {
		t.Parallel()
		for _, owner := range []fmt.Formatter{S3FileUpload{}, s3FileCredentials{}, s3CredentialValue{}} {
			if got := fmt.Sprintf("%#v", owner); got != core.RedactedValueText {
				t.Fatalf("absent owner redacted=%v bytes=%d, want redacted neutral projection", got == core.RedactedValueText, len(got))
			}
		}
		if got := s3FileRefusal(nil); got != nil {
			t.Fatalf("absent refusal=%v, want nil without manufactured error", got)
		}
	})
}
