package objectstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// A credential value is valid exactly when it holds bytes; it still never
// serializes. Values arrive through the real JSON door.
func TestS3CredentialValueValidateThroughJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		json string
		want bool
	}{
		{"empty string", `""`, false},
		{"one byte", `"a"`, true},
		{"an escaped byte", `"A"`, true},
		{"a long secret", `"` + strings.Repeat("s", 4096) + `"`, true},
	} {
		var value s3CredentialValue
		if err := value.UnmarshalJSON([]byte(tc.json)); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		err := value.Validate()
		if tc.want != (err == nil) || (err != nil && !errors.Is(err, core.ErrObjectStoreContract)) {
			t.Fatalf("%s: Validate = %v, want valid=%v", tc.name, err, tc.want)
		}
		if out, err := value.MarshalJSON(); out != nil || !errors.Is(err, core.ErrObjectStoreContract) {
			t.Fatalf("%s: a credential value serialized", tc.name)
		}
	}
	if (s3CredentialValue{}).Validate() == nil {
		t.Fatal("the zero credential value validated")
	}
}
