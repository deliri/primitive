package attest_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// A fixed typed tuple keeps every generated member observable, including when
// the fuzzer changes its name. The old map-only branch lost the value oracle.
type canonicalScalarFacts struct {
	Text     string
	Signed   int64
	Unsigned uint64
	Flag     bool
}

func FuzzCanonicalObjectEmitsStrictlyDecodableAndStableDocuments(f *testing.F) {
	f.Add("text", "signed", "unsigned", "flag", "abc", int64(0), uint64(0), true)
	f.Add("a", "b", "c", "d", "", int64(-1), uint64(1), false)
	f.Add("run_id", "cpu_nanos", "count", "known", "a<b>&c", int64(math.MinInt64), uint64(math.MaxUint64), true)
	f.Add("", "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("_bad", "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("dup", "dup", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("UP", "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	f.Add("text", "signed", "unsigned", "flag", "\xff", int64(1), uint64(1), true)
	for _, size := range []int{attest.CanonicalFieldNameMaximumBytes - 1, attest.CanonicalFieldNameMaximumBytes, attest.CanonicalFieldNameMaximumBytes + 1} {
		f.Add(strings.Repeat("a", size), "signed", "unsigned", "flag", "x", int64(1), uint64(1), true)
	}
	// A regex is an independent grammar oracle, rather than a copy of the
	// builder's byte-by-byte state machine. The separator remains owner-defined.
	grammar := regexp.MustCompile(`^[a-z0-9]+(?:` + regexp.QuoteMeta(string(attest.CanonicalFieldNameSeparator)) + `[a-z0-9]+)*$`)
	f.Fuzz(func(t *testing.T, textName, signedName, unsignedName, flagName, text string, signed int64, unsigned uint64, flag bool) {
		names := [4]string{textName, signedName, unsignedName, flagName}
		facts := canonicalScalarFacts{Text: text, Signed: signed, Unsigned: unsigned, Flag: flag}
		object := attest.BeginCanonicalObject(nil)
		object.String(textName, text)
		object.Int64(signedName, signed)
		object.Uint64(unsignedName, unsigned)
		object.Bool(flagName, flag)
		got, gotErr := object.End()
		wantAdmitted := utf8.ValidString(text)
		for index, name := range names {
			wantAdmitted = wantAdmitted && len(name) <= attest.CanonicalFieldNameMaximumBytes && grammar.MatchString(name)
			for _, prior := range names[:index] {
				wantAdmitted = wantAdmitted && prior != name
			}
		}
		// Bound the independent encoder before allocating; escaped output below
		// this input ceiling is still checked against the exact emitted extent.
		wantAdmitted = wantAdmitted && len(text) <= attest.CanonicalBodyMaximumBytes
		var want []byte
		if wantAdmitted {
			want = canonicalScalarOracle(t, names, facts)
			wantAdmitted = len(want) <= attest.CanonicalBodyMaximumBytes
		}
		if !wantAdmitted {
			if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
				t.Fatalf("End() = (%d bytes, %v), want nil output and %v", len(got), gotErr, core.ErrAttestContract)
			}
			return
		}
		if gotErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("End() = (%q, %v), want independent token projection %q", got, gotErr, want)
		}
		// Decode every emitted token to prove all names, values, order and extent.
		decoder := jsontext.NewDecoder(bytes.NewReader(got))
		oracle := jsontext.NewDecoder(bytes.NewReader(want))
		for {
			wantToken, wantErr := oracle.ReadToken()
			gotToken, err := decoder.ReadToken()
			if errors.Is(wantErr, io.EOF) {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("trailing token = (%v, %v), want EOF", gotToken, err)
				}
				break
			}
			if err != nil || gotToken.Kind() != wantToken.Kind() || gotToken.String() != wantToken.String() {
				t.Fatalf("decoded token = (%v, %v), want %v", gotToken, err, wantToken)
			}
		}
	})
}

func canonicalScalarOracle(t testing.TB, names [4]string, facts canonicalScalarFacts) []byte {
	t.Helper()
	var destination bytes.Buffer
	encoder := jsontext.NewEncoder(&destination)
	tokens := [...]jsontext.Token{
		jsontext.BeginObject,
		jsontext.String(names[0]), jsontext.String(facts.Text),
		jsontext.String(names[1]), jsontext.Int(facts.Signed),
		jsontext.String(names[2]), jsontext.Uint(facts.Unsigned),
		jsontext.String(names[3]), jsontext.Bool(facts.Flag),
		jsontext.EndObject,
	}
	for _, token := range tokens {
		if err := encoder.WriteToken(token); err != nil {
			t.Fatalf("independent WriteToken(%v) error = %v, want nil", token, err)
		}
	}
	return bytes.TrimSuffix(destination.Bytes(), []byte{'\n'})
}
