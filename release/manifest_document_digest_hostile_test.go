package release

import (
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestManifestDocumentDigestJSONBoundaryPreservesExactSignedDocumentIdentity(t *testing.T) {
	t.Parallel()

	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 8, 10), 1)
	want := fixture.verified.DocumentDigest()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(ManifestDocumentDigest) error = %v, want nil", err)
	}
	if got, wantText := string(encoded), `"`+want.String()+`"`; got != wantText {
		t.Fatalf("json.Marshal(ManifestDocumentDigest) = %q, want %q", got, wantText)
	}
	var got ManifestDocumentDigest
	if err := json.Unmarshal(encoded, &got); err != nil || got != want {
		t.Fatalf("json.Unmarshal(ManifestDocumentDigest) = (%v, %v), want (%v, nil)", got, err, want)
	}
}

func TestManifestDocumentDigestOwnsJSONFailureIdentityAndPreservesReceiver(t *testing.T) {
	t.Parallel()

	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 8, 10), 1)
	want := fixture.verified.DocumentDigest()
	cases := []struct {
		name string
		wire []byte
	}{
		{name: "null is rejected", wire: []byte("null")},
		{name: "number is rejected", wire: []byte("1")},
		{name: "object is rejected", wire: []byte("{}")},
		{name: "array is rejected", wire: []byte("[]")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := want
			gotErr := json.Unmarshal(tc.wire, &got)
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("json.Unmarshal(hostile ManifestDocumentDigest) error = %v, want %v and %v", gotErr, core.ErrJSONContract, core.ErrReleaseContract)
			}
			if got != want {
				t.Fatalf("rejected ManifestDocumentDigest receiver = %v, want preserved %v", got, want)
			}
		})
	}
	if _, err := json.Marshal(ManifestDocumentDigest{}); !errors.Is(err, core.ErrJSONContract) ||
		!errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("json.Marshal(ManifestDocumentDigest{}) error = %v, want %v and %v", err, core.ErrJSONContract, core.ErrReleaseContract)
	}
	if err := (*ManifestDocumentDigest)(nil).UnmarshalJSON(nil); !errors.Is(err, core.ErrJSONContract) ||
		!errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("nil ManifestDocumentDigest.UnmarshalJSON() error = %v, want %v and %v", err, core.ErrJSONContract, core.ErrReleaseContract)
	}
}
