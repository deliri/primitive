package submissionauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const authWhitespaceFixtureBytes = 1 << 20

type authJSONReceiver[T any] interface {
	*T
	UnmarshalJSON([]byte) error
}

func authPreservingDecode[T comparable, P authJSONReceiver[T]](want T) func([]byte) (bool, error) {
	return func(data []byte) (bool, error) {
		got := want
		err := P(&got).UnmarshalJSON(data)
		return got == want, err
	}
}
func TestSubmissionAuthJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	requestWire, err := fixture.request.document.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	completionWire, err := fixture.credentialed.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, door := range []struct {
		decode func([]byte) (bool, error)
		name   string
		data   []byte
	}{
		{name: "request", data: requestWire, decode: authPreservingDecode(fixture.request.document)},
		{name: "completion", data: completionWire, decode: authPreservingDecode(fixture.credentialed)},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			gap := bytes.Repeat([]byte(" "), authWhitespaceFixtureBytes)
			for _, tc := range []struct {
				wantErr error
				name    string
				data    []byte
			}{
				{name: "canonical", data: door.data, wantErr: nil},
				{name: "large_prefix", data: append(bytes.Clone(gap), door.data...), wantErr: nil},
				{name: "large_interior", data: append(append([]byte{'{'}, gap...), door.data[1:]...), wantErr: nil},
				{name: "large_suffix", data: append(bytes.Clone(door.data), gap...), wantErr: nil},
				{name: "neutral_whitespace", data: gap, wantErr: core.ErrJSONContract},
				{name: "trailing_document", data: append(append(bytes.Clone(door.data), gap...), []byte("{}")...), wantErr: core.ErrJSONContract},
				{name: "truncated_after_large_prefix", data: append(bytes.Clone(gap), door.data[:len(door.data)-1]...), wantErr: core.ErrJSONContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					same, err := door.decode(tc.data)
					if !same || !errors.Is(err, tc.wantErr) || tc.wantErr != nil && !errors.Is(err, core.ErrControlPlaneContract) {
						t.Fatalf("preserved=%t error=%v, want exact document and %v", same, err, tc.wantErr)
					}
				})
			}
		})
	}
}
