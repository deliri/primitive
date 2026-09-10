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
		name   string
		data   []byte
		decode func([]byte) (bool, error)
	}{
		{"request", requestWire, authPreservingDecode(fixture.request.document)},
		{"completion", completionWire, authPreservingDecode(fixture.credentialed)},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			gap := bytes.Repeat([]byte(" "), authWhitespaceFixtureBytes)
			for _, tc := range []struct {
				name    string
				data    []byte
				wantErr error
			}{
				{"canonical", door.data, nil},
				{"large_prefix", append(bytes.Clone(gap), door.data...), nil},
				{"large_interior", append(append([]byte{'{'}, gap...), door.data[1:]...), nil},
				{"large_suffix", append(bytes.Clone(door.data), gap...), nil},
				{"neutral_whitespace", gap, core.ErrJSONContract},
				{"trailing_document", append(append(bytes.Clone(door.data), gap...), []byte("{}")...), core.ErrJSONContract},
				{"truncated_after_large_prefix", append(bytes.Clone(gap), door.data[:len(door.data)-1]...), core.ErrJSONContract},
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
