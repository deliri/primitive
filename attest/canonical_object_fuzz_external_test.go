package attest_test

import (
	"bytes"
	"errors"
	"testing"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// fuzzedCanonicalMembers is the decoded shape of the document the fuzz target
// builds. Naming it lets the oracle prove strict decoding really consumed the
// emitted members instead of accepting an arbitrary object.
type fuzzedCanonicalMembers struct {
	Text     *string `json:"text"`
	Signed   *int64  `json:"signed"`
	Unsigned *uint64 `json:"unsigned"`
	Flag     *bool   `json:"flag"`
}

// Validate proves every member the builder wrote survived decoding.
func (m fuzzedCanonicalMembers) Validate() error {
	if m.Text == nil || m.Signed == nil || m.Unsigned == nil || m.Flag == nil {
		return core.ErrAttestContract
	}
	return nil
}

// FuzzCanonicalObjectEmitsStrictlyDecodableAndStableDocuments drives arbitrary
// member names and values through the real builder. The oracle is not
// "did not panic": an accepted document must decode under Core's strict JSON
// contract, must preserve every typed member exactly, and must rebuild to the
// same bytes, while a rejection must carry a stable typed identity and return
// no bytes at all.
func FuzzCanonicalObjectEmitsStrictlyDecodableAndStableDocuments(f *testing.F) {
	f.Add("text", "signed", "unsigned", "flag", "abc", int64(0), uint64(0), true)
	f.Add("a", "b", "c", "d", "", int64(-1), uint64(1), false)
	f.Add("run_id", "cpu_nanos", "count", "known", "a<b>&c", int64(-9223372036854775808), uint64(18446744073709551615), true)
	f.Add("", "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("_bad", "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("dup", "dup", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("UP", "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("text", "signed", "unsigned", "flag", "\xff", int64(1), uint64(1), true)

	f.Fuzz(func(
		t *testing.T,
		textName string, signedName string, unsignedName string, flagName string,
		text string, signed int64, unsigned uint64, flag bool,
	) {
		build := func() ([]byte, error) {
			object := attest.BeginCanonicalObject(nil)
			object.String(textName, text)
			object.Int64(signedName, signed)
			object.Uint64(unsignedName, unsigned)
			object.Bool(flagName, flag)
			return object.End()
		}

		encoded, err := build()
		if err != nil {
			if !errors.Is(err, core.ErrAttestContract) {
				t.Fatalf("End() error = %v, want %v", err, core.ErrAttestContract)
			}
			if encoded != nil {
				t.Fatalf("End() after rejection = %q, want nil", encoded)
			}
			return
		}

		// An accepted document is byte-stable: the same typed inputs rebuild to
		// exactly the same bytes, which is the property a signature depends on.
		rebuilt, rebuildErr := build()
		if rebuildErr != nil {
			t.Fatalf("second End() error = %v, want nil", rebuildErr)
		}
		if string(rebuilt) != string(encoded) {
			t.Fatalf("rebuilt canonical object = %q, want %q", rebuilt, encoded)
		}

		// Only the fixed-name shape can be decoded into the typed oracle; other
		// accepted names are still required to be strictly decodable objects.
		if textName != "text" || signedName != "signed" ||
			unsignedName != "unsigned" || flagName != "flag" {
			if _, decodeErr := core.DecodeStrictJSONStructure[map[string]jsontext.Value](
				encoded, core.DefaultStrictJSONLimits(),
			); decodeErr != nil {
				t.Fatalf("DecodeStrictJSONStructure() error = %v, want nil for %q", decodeErr, encoded)
			}
			return
		}

		decoded, decodeErr := core.DecodeStrictJSON[fuzzedCanonicalMembers](
			bytes.NewReader(encoded), core.DefaultStrictJSONLimits(),
		)
		if decodeErr != nil {
			t.Fatalf("DecodeStrictJSON() error = %v, want nil for %q", decodeErr, encoded)
		}
		if *decoded.Text != text {
			t.Fatalf("decoded text = %q, want %q", *decoded.Text, text)
		}
		if *decoded.Signed != signed {
			t.Fatalf("decoded signed = %d, want %d", *decoded.Signed, signed)
		}
		if *decoded.Unsigned != unsigned {
			t.Fatalf("decoded unsigned = %d, want %d", *decoded.Unsigned, unsigned)
		}
		if *decoded.Flag != flag {
			t.Fatalf("decoded flag = %v, want %v", *decoded.Flag, flag)
		}
	})
}
