package gotoolchain

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/mod/sumdb/dirhash"
)

func moduleClosureTestSum(t testing.TB, content string) string {
	t.Helper()
	sum, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(content)), nil })
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestModuleClosureFrozenSumGrammarAndConflict(t *testing.T) {
	t.Parallel()
	hash := moduleClosureTestSum(t, "module example.com/dependency\n")
	other := moduleClosureTestSum(t, "module example.com/other\n")
	line := "example.com/dependency v1.0.0/go.mod " + hash
	for _, row := range []struct {
		name, input string
		entries     int
		refuse      bool
	}{
		{"native_hash", line + "\n", 1, false},
		{"no_final_newline", line, 1, false},
		{"blank_lines_and_native_whitespace", "\n\t" + line + "\r\n\n", 1, false},
		{"same_duplicate", line + "\n" + line, 1, false},
		{"distinct_tree_and_mod", line + "\nexample.com/dependency v1.0.0 " + other, 2, false},
		{"empty_history", "\n", 0, false},
		{"conflicting_duplicate", line + "\nexample.com/dependency v1.0.0/go.mod " + other, 0, true},
		{"missing_hash", "example.com/dependency v1.0.0/go.mod", 0, true},
		{"extra_field", line + " extra", 0, true},
		{"path_traversal", "../dependency v1.0.0 " + hash, 0, true},
		{"version_query", "example.com/dependency latest " + hash, 0, true},
		{"major_mismatch", "example.com/dependency v2.0.0 " + hash, 0, true},
		{"hash_algorithm", strings.Replace(line, "h1:", "h2:", 1), 0, true},
		{"hash_truncated", line[:len(line)-1], 0, true},
		{"invalid_utf8", line + string([]byte{255}), 0, true},
		{"mod_suffix_repeated", "example.com/dependency v1.0.0/go.mod/go.mod " + hash, 0, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := readModuleSums(t.Context(), strings.NewReader(row.input))
			if row.refuse {
				if !errors.Is(err, core.ErrGoToolchainOutput) || len(got.order) != 0 {
					t.Fatalf("sums=%+v/%v, want zero output refusal", got, err)
				}
			} else if err != nil || len(got.order) != row.entries || len(got.expected) != row.entries {
				t.Fatalf("sums=%+v/%v, want %d immutable identities", got, err, row.entries)
			}
		})
	}
}

func TestModuleClosureNativeJSONStreamingBoundaries(t *testing.T) {
	t.Parallel()
	// The typed native producer shape is marshaled by Go's real JSON encoder.
	// Mutations below are invalid ingress, never handwritten valid JSON seeds.
	type native struct {
		Path     string
		Version  string
		Main     bool
		Replace  *struct{ Path string } `json:",omitempty"`
		Versions []string
	}
	base, err := json.Marshal(native{Path: "example.com/main", Main: true, Versions: []string{"v1.0.0", "v1.1.0"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name   string
		input  []byte
		refuse bool
	}{
		{"complete_main", base, false},
		{"ignored_population", bytes.Replace(base, []byte(`"v1.1.0"`), []byte(`{"nested":[null,true,1,{}]}`), 1), false},
		{"truncated_final_object", base[:len(base)-1], true},
		{"duplicate_member", bytes.Replace(base, []byte(`"Main":true`), []byte(`"Main":true,"Main":true`), 1), true},
		{"wrong_scalar_type", bytes.Replace(base, []byte(`"Main":true`), []byte(`"Main":1`), 1), true},
		{"foreign_top_array", append(append([]byte("["), base...), ']'), true},
		{"empty_stream", nil, true},
		{"second_main", append(slices.Clone(base), base...), true},
		{"missing_main", bytes.Replace(base, []byte(`"Main":true`), []byte(`"Main":false`), 1), true},
		{"main_version", bytes.Replace(base, []byte(`"Version":""`), []byte(`"Version":"v1.0.0"`), 1), true},
		{"trailing_invalid", append(slices.Clone(base), 'x'), true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			got, err := visitSelectedModules(t.Context(), moduleSelectionVisit{source: bytes.NewReader(row.input), visit: func(context.Context, SelectedModuleObservation) error { calls++; return nil }})
			if (err != nil) != row.refuse || got != 0 || calls != 0 {
				t.Fatalf("selection=%d calls=%d error=%v, want refusal=%t and no dependency loan", got, calls, err, row.refuse)
			}
		})
	}
	for _, name := range []string{"replacement", "native_error", "truncated_ignored"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input := append([]byte(nil), base...)
			switch name {
			case "replacement":
				encoded, encodeErr := json.Marshal(native{Path: "example.com/main", Main: true, Replace: &struct{ Path string }{"example.com/other"}})
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				input = encoded
			case "native_error":
				input = bytes.Replace(input, []byte(`"Versions":`), []byte(`"Error":{"Err":"unavailable"},"Versions":`), 1)
			case "truncated_ignored":
				input = bytes.Replace(input, []byte(`"Versions":`), []byte(`"Versions":[`), 1)
			}
			got, err := visitSelectedModules(t.Context(), moduleSelectionVisit{source: bytes.NewReader(input), visit: func(context.Context, SelectedModuleObservation) error { return nil }})
			if got != 0 || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("unsafe native=%d/%v, want zero refusal", got, err)
			}
		})
	}
}

func TestModuleClosureIngressPreservesFailureAndCancellation(t *testing.T) {
	t.Parallel()
	cause := errors.New("frozen sum source failed")
	if got, err := readModuleSums(t.Context(), io.MultiReader(strings.NewReader("\n"), moduleClosureFailingReader{cause})); !errors.Is(err, cause) || len(got.order) != 0 {
		t.Fatalf("source failure=%+v/%v, want exact cause and zero", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := readModuleSums(ctx, strings.NewReader("")); !errors.Is(err, context.Canceled) || len(got.order) != 0 {
		t.Fatalf("cancellation=%+v/%v, want zero context cancellation", got, err)
	}
}

type moduleClosureFailingReader struct{ cause error }

func (r moduleClosureFailingReader) Read([]byte) (int, error) { return 0, r.cause }

func FuzzModuleClosureFrozenSumSemanticBoundary(f *testing.F) {
	for _, body := range []string{"module example.com/dependency\n", "module example.com/other\n\ngo 1.16\n"} {
		f.Add(body, uint8(0))
		f.Add(body, uint8(1))
	}
	f.Fuzz(func(t *testing.T, body string, operation uint8) {
		hash := moduleClosureTestSum(t, body)
		text := "example.com/dependency v1.0.0/go.mod " + hash + "\n"
		refuse := false
		switch operation % 4 {
		case 1:
			text += text
		case 2:
			text += "example.com/dependency v1.0.0/go.mod " + moduleClosureTestSum(t, body+"\x00") + "\n"
			refuse = true
		case 3:
			text += "unexpected"
			refuse = true
		}
		got, err := readModuleSums(t.Context(), strings.NewReader(text))
		if refuse {
			if !errors.Is(err, core.ErrGoToolchainOutput) || len(got.order) != 0 {
				t.Fatalf("mutated sum=%+v/%v, want zero conflict/grammar refusal", got, err)
			}
			return
		}
		if err != nil || len(got.order) != 1 || got.expected[got.order[0]].String() != hash {
			t.Fatalf("sum=%+v/%v, want exact native file-list h1 %s", got, err, hash)
		}
		value, err := ParseModuleChecksum(hash)
		if err != nil || value.String() != hash || value.Validate() != nil {
			t.Fatalf("checksum=%v/%v, want native hash round trip", value, err)
		}
	})
}

func FuzzModuleClosureJSONCursorSemanticBoundary(f *testing.F) {
	f.Add("example.com/main", true)
	f.Add("quoted\"path", false)
	f.Fuzz(func(t *testing.T, path string, main bool) {
		path = strings.ToValidUTF8(path, "\ufffd")
		data, err := json.Marshal(struct {
			Path string
			Main bool
		}{path, main})
		if err != nil {
			t.Fatal(err)
		}
		var reference struct {
			Path string
			Main bool
		}
		if err := json.Unmarshal(data, &reference); err != nil {
			t.Fatal(err)
		}
		got, err := readModuleSelection(moduleJSONCursor{ctx: t.Context(), decoder: jsontext.NewDecoder(bytes.NewReader(data))})
		if err != nil || got.path != reference.Path || got.main != main || got.refusal {
			t.Fatalf("native scalar=%+v/%v, want encoded %+v", got, err, reference)
		}
		if _, err := readModuleSelection(moduleJSONCursor{ctx: t.Context(), decoder: jsontext.NewDecoder(bytes.NewReader(data[:len(data)-1]))}); !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("truncated object error=%v, want output refusal", err)
		}
	})
}
