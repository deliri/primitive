package upgrade

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestTrialDocumentCanonicalClosure(t *testing.T) {
	t.Parallel()

	prior := artifactForTest(t, []byte("installed"), 1)
	candidate := artifactForTest(t, []byte("candidate"), 2)
	document := trialDocument{
		Revision: trialRevisionCurrent,
		Prior: selectionDocument{
			Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: prior,
		},
		Candidate: candidate,
	}
	encoded, err := encodeTrial(document)
	if err != nil {
		t.Fatalf("encodeTrial() error = %v, want nil", err)
	}
	decoded, err := decodeTrial(encoded)
	if err != nil || decoded != document {
		t.Fatalf("decodeTrial(encode) = (%v, %v), want (%v, nil)",
			decoded, err, document)
	}
	if len(encoded) >= trialDocumentMaximumBytes {
		t.Fatalf("canonical trial extent = %d, want < owned bound %d",
			len(encoded), trialDocumentMaximumBytes)
	}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "nil document", data: nil},
		{name: "empty document", data: []byte{}},
		{name: "empty object omits every field", data: []byte(`{}`)},
		{name: "json null", data: []byte(`null`)},
		{name: "json array", data: []byte(`[]`)},
		{name: "json string", data: []byte(`"trial"`)},
		{name: "json number", data: []byte(`1`)},
		{name: "truncated final byte", data: encoded[:len(encoded)-1]},
		{name: "truncated to half", data: encoded[:len(encoded)/2]},
		{name: "leading space breaks canonical equality", data: append([]byte{' '}, encoded...)},
		{name: "trailing space breaks canonical equality", data: append(slices.Clone(encoded), ' ')},
		{name: "trailing newline breaks canonical equality", data: append(slices.Clone(encoded), '\n')},
		{name: "two concatenated receipts", data: append(slices.Clone(encoded), encoded...)},
		{name: "trailing object start", data: append(slices.Clone(encoded), '{')},
		{name: "duplicate revision", data: bytes.Replace(encoded,
			[]byte(`{"revision":1,`), []byte(`{"revision":1,"revision":1,`), 1)},
		{name: "unknown top-level field", data: bytes.Replace(encoded,
			[]byte(`{"revision":1,`), []byte(`{"future":true,"revision":1,`), 1)},
		{name: "missing revision", data: bytes.Replace(encoded,
			[]byte(`"revision":1,`), nil, 1)},
		{name: "revision below current", data: bytes.Replace(encoded,
			[]byte(`"revision":1,`), []byte(`"revision":0,`), 1)},
		{name: "revision above current", data: bytes.Replace(encoded,
			[]byte(`"revision":1,`), []byte(`"revision":2,`), 1)},
		{name: "revision is a string", data: bytes.Replace(encoded,
			[]byte(`"revision":1,`), []byte(`"revision":"1",`), 1)},
		{name: "revision is a decimal", data: bytes.Replace(encoded,
			[]byte(`"revision":1,`), []byte(`"revision":1.0,`), 1)},
		{name: "missing prior", data: bytes.Replace(encoded,
			[]byte(`"prior":`), []byte(`"removed":`), 1)},
		{name: "null prior", data: bytes.Replace(encoded,
			encodedFieldValue(t, encoded, `"prior":`), []byte(`"prior":null`), 1)},
		{name: "missing candidate", data: bytes.Replace(encoded,
			[]byte(`"candidate":`), []byte(`"removed":`), 1)},
		{name: "null candidate", data: bytes.Replace(encoded,
			encodedFieldValue(t, encoded, `"candidate":`), []byte(`"candidate":null`), 1)},
		{name: "nested prior revision is zero", data: bytes.Replace(encoded,
			[]byte(`"prior":{"revision":1,`), []byte(`"prior":{"revision":0,`), 1)},
		{name: "nested prior slot is unknown", data: bytes.Replace(encoded,
			[]byte(`"slot":"slot-a"`), []byte(`"slot":"slot-c"`), 1)},
		{name: "document exceeds the owned bound", data: append(
			slices.Clone(encoded),
			bytes.Repeat([]byte("x"), trialDocumentMaximumBytes)...,
		)},
		{name: "nesting bomb", data: append(
			bytes.Repeat([]byte(`{"prior":`), 200),
			bytes.Repeat([]byte(`}`), 200)...,
		)},
		{name: "invalid utf8 in a field name", data: []byte("{\"\xff\":1}")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, decodeErr := decodeTrial(tc.data)
			requireRejection(
				t, decodeErr, core.ErrUpgradeContract, diagnosticUnknown,
			)
			if !errors.Is(decodeErr, core.ErrJSONContract) {
				t.Fatalf("decodeTrial() error = %v, want %v",
					decodeErr, core.ErrJSONContract)
			}
			if got != (trialDocument{}) {
				t.Fatalf("decodeTrial() = %v, want the zero document", got)
			}
		})
	}
}

func encodedFieldValue(
	t *testing.T,
	document []byte,
	field string,
) []byte {
	t.Helper()

	start := bytes.Index(document, []byte(field))
	if start < 0 {
		t.Fatalf("encoded fixture has no field %s", field)
	}
	valueStart := start + len(field)
	depth := 0
	for index := valueStart; index < len(document); index++ {
		switch document[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return document[start : index+1]
			}
		}
	}
	t.Fatalf("encoded fixture field %s has no complete object", field)
	return nil
}
