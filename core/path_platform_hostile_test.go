package core

import (
	"bytes"
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAbsoluteFilesystemPathHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	maximumRunePath := strings.Repeat(
		string(filepath.Separator)+strings.Repeat("a", filesystemPathComponentMaximumBytes),
		16,
	)
	oneBelowMaximumRunePath := maximumRunePath[:len(maximumRunePath)-1]
	overMaximumRunePath := strings.Repeat(
		string(filepath.Separator)+strings.Repeat("a", filesystemPathComponentMaximumBytes),
		15,
	) + string(filepath.Separator) + strings.Repeat("a", filesystemPathComponentMaximumBytes-1) +
		string(filepath.Separator) + "a"
	maximumComponents := "/" + strings.Repeat("a/", FilesystemPathMaximumComponents-1) + "a"
	overMaximumComponents := maximumComponents + "/a"
	cases := []struct {
		name        string
		value       string
		disposition boundaryDisposition
	}{
		{name: "root lexical path is accepted without false file or directory claim", value: string(filepath.Separator), disposition: boundaryAccept},
		{name: "minimum one-component absolute path", value: "/a", disposition: boundaryAccept},
		{name: "ordinary two-component path", value: "/a/b", disposition: boundaryAccept},
		{name: "hidden final component", value: "/a/.hidden", disposition: boundaryAccept},
		{name: "hyphen and underscore components", value: "/a-b/c_d", disposition: boundaryAccept},
		{name: "space inside component", value: "/a b/c", disposition: boundaryAccept},
		{name: "Unicode component", value: "/é/世界", disposition: boundaryAccept},
		{name: "JSON escaped control byte 0x01 inside component", value: "/a\x01b", disposition: boundaryAccept},
		{name: "JSON escaped vertical tab inside component", value: "/a\x0bb", disposition: boundaryAccept},
		{name: "JSON escaped delete byte inside component", value: "/a\x7fb", disposition: boundaryAccept},
		{name: "raw ampersand inside component", value: "/a&b", disposition: boundaryAccept},
		{name: "raw less-than inside component", value: "/a<b", disposition: boundaryAccept},
		{name: "raw greater-than inside component", value: "/a>b", disposition: boundaryAccept},
		{name: "one below component maximum", value: strings.TrimSuffix(maximumComponents, "/a"), disposition: boundaryAccept},
		{name: "exact component maximum", value: maximumComponents, disposition: boundaryAccept},
		{name: "one rune below complete-path maximum", value: oneBelowMaximumRunePath, disposition: boundaryAccept},
		{name: "exact complete-path rune maximum", value: maximumRunePath, disposition: boundaryAccept},
		{name: "empty path is rejected", value: ""},
		{name: "relative component is rejected", value: "a"},
		{name: "relative multi-component path is rejected", value: "a/b"},
		{name: "dot relative path is rejected", value: "."},
		{name: "parent relative path is rejected", value: ".."},
		{name: "absolute dot component is rejected", value: "/a/./b"},
		{name: "absolute parent component is rejected", value: "/a/../b"},
		{name: "repeated separator is rejected", value: "/a//b"},
		{name: "trailing separator is rejected", value: "/a/"},
		{name: "NUL final byte is rejected", value: "/a\x00"},
		{name: "NUL middle byte is rejected", value: "/a\x00/b"},
		{name: "invalid UTF8 byte is rejected", value: string([]byte{'/', 0xff})},
		{name: "one above component maximum is rejected", value: overMaximumComponents},
		{name: "one above complete-path rune maximum is rejected", value: overMaximumRunePath},
		{name: "far above rune maximum is rejected", value: "/" + strings.Repeat("界", filesystemPathMaximumRunes*2)},
		{name: "one component one byte above filesystem maximum is rejected", value: "/" + strings.Repeat("a", filesystemPathComponentMaximumBytes+1)},
		{name: "leading ASCII space prevents absolute path", value: " /a"},
		{name: "leading newline prevents absolute path", value: "\n/a"},
		{name: "backslash is not native absolute separator", value: `\a\b`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseAbsolutePath(tc.value)
			wantAccept := tc.disposition == boundaryAccept
			if (gotErr == nil) != wantAccept {
				t.Fatalf("ParseAbsolutePath(%q) = (%v, %v), want accept %t", tc.value, got, gotErr, wantAccept)
			}
			if gotErr == nil {
				if got.String() != tc.value || got.Validate() != nil {
					t.Fatalf("accepted absolute path = %q validation %v, want %q/nil", got.String(), got.Validate(), tc.value)
				}
				directWire, directMarshalErr := got.MarshalJSON()
				if directMarshalErr != nil {
					t.Fatalf("AbsolutePath.MarshalJSON(%q) error = %v, want nil", got, directMarshalErr)
				}
				if !jsontext.Value(directWire).IsValid() {
					t.Fatalf("AbsolutePath.MarshalJSON(%q) wire = %q, want valid JSON", got, directWire)
				}
				directRoundTrip, directDecodeErr := DecodeStrictJSON[AbsolutePath](
					bytes.NewReader(directWire),
					DefaultStrictJSONLimits(),
				)
				if directDecodeErr != nil || directRoundTrip != got {
					t.Fatalf(
						"AbsolutePath direct JSON round trip = (%v, %v), want (%v, nil)",
						directRoundTrip,
						directDecodeErr,
						got,
					)
				}
				gotWire, gotEncodeErr := EncodeValidatedJSON(got, DefaultStrictJSONLimits())
				if gotEncodeErr != nil {
					t.Fatalf("EncodeValidatedJSON(AbsolutePath %q) error = %v, want nil", got, gotEncodeErr)
				}
				gotRoundTrip, gotDecodeErr := DecodeStrictJSON[AbsolutePath](
					bytes.NewReader(gotWire),
					DefaultStrictJSONLimits(),
				)
				if gotDecodeErr != nil || gotRoundTrip != got {
					t.Fatalf(
						"AbsolutePath strict JSON round trip = (%v, %v), want (%v, nil)",
						gotRoundTrip,
						gotDecodeErr,
						got,
					)
				}
			} else if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("ParseAbsolutePath(%q) error = %v, want %v", tc.value, gotErr, ErrPrimitiveContract)
			}
		})
	}

	if got := utf8.RuneCountInString(maximumRunePath); got != filesystemPathMaximumRunes {
		t.Fatalf("maximum-rune fixture count = %d, want %d", got, filesystemPathMaximumRunes)
	}
	if got := utf8.RuneCountInString(overMaximumRunePath); got != filesystemPathMaximumRunes+1 {
		t.Fatalf("over-maximum-rune fixture count = %d, want %d", got, filesystemPathMaximumRunes+1)
	}
}

// TestAbsolutePathStrictJSONInteroperableEscaping proves that the public
// decoder accepts both raw JSON-safe bytes emitted by non-Go producers and
// standard JSON escapes. Raw documents are intentional at this ingress
// boundary because no typed value exists until decoding succeeds.
func TestAbsolutePathStrictJSONInteroperableEscaping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		wire string
		want string
	}{
		{name: "raw ampersand", wire: `"/a&b"`, want: "/a&b"},
		{name: "raw less-than", wire: `"/a<b"`, want: "/a<b"},
		{name: "raw greater-than", wire: `"/a>b"`, want: "/a>b"},
		{name: "raw HTML punctuation sequence", wire: `"/a&<b>"`, want: "/a&<b>"},
		{name: "escaped ampersand", wire: `"/a\u0026b"`, want: "/a&b"},
		{name: "escaped less-than", wire: `"/a\u003cb"`, want: "/a<b"},
		{name: "escaped greater-than", wire: `"/a\u003eb"`, want: "/a>b"},
		{name: "escaped control byte 0x01", wire: `"/a\u0001b"`, want: "/a\x01b"},
		{name: "escaped vertical tab", wire: `"/a\u000bb"`, want: "/a\x0bb"},
		{name: "escaped delete byte", wire: `"/a\u007fb"`, want: "/a\x7fb"},
		{name: "raw replacement rune", wire: `"/a�b"`, want: "/a�b"},
		{name: "escaped replacement rune", wire: `"/a\ufffdb"`, want: "/a�b"},
		{name: "paired surrogate escape", wire: `"/a\ud83d\ude42b"`, want: "/a🙂b"},
		{name: "escaped backslash before surrogate text", wire: `"/a\\ud800b"`, want: `/a\ud800b`},
		{name: "leading document whitespace", wire: " \t\"/aAb\"", want: "/aAb"},
		{name: "trailing document whitespace", wire: "\"/aAb\"\r\n", want: "/aAb"},
		{name: "escaped path separators", wire: `"\/a\/aAb"`, want: "/a/aAb"},
		{name: "escaped basic Latin letter", wire: `"/a\u0041b"`, want: "/aAb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want, wantErr := ParseAbsolutePath(tc.want)
			if wantErr != nil {
				t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", tc.want, wantErr)
			}
			got, gotErr := DecodeStrictJSON[AbsolutePath](bytes.NewReader([]byte(tc.wire)), DefaultStrictJSONLimits())
			if gotErr != nil || got != want {
				t.Fatalf(
					"DecodeStrictJSON[AbsolutePath](%q) = (%v, %v), want (%v, nil)",
					tc.wire,
					got,
					gotErr,
					want,
				)
			}
		})
	}
}

func TestAbsolutePathStrictJSONRejectsLossySurrogateRepair(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		wire string
	}{
		{name: "minimum lone high surrogate", wire: `"/a\ud800b"`},
		{name: "maximum lone high surrogate", wire: `"/a\udbffb"`},
		{name: "minimum lone low surrogate", wire: `"/a\udc00b"`},
		{name: "maximum lone low surrogate", wire: `"/a\udfffb"`},
		{name: "high surrogate followed by high surrogate", wire: `"/a\ud800\ud800b"`},
		{name: "low surrogate followed by low surrogate", wire: `"/a\udfff\udfffb"`},
		{name: "high surrogate followed by basic escape", wire: `"/a\ud800\u0041b"`},
		{name: "high surrogate followed by raw byte", wire: `"/a\ud800xb"`},
		{name: "raw byte followed by low surrogate", wire: `"/ax\udfffb"`},
		{name: "maximum high surrogate followed by minimum high surrogate", wire: `"/a\udbff\ud800b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var refused string
			gotStdlibErr := json.Unmarshal([]byte(tc.wire), &refused)
			var gotSyntax *jsontext.SyntacticError
			if !errors.As(gotStdlibErr, &gotSyntax) || refused != "" {
				t.Fatalf(
					"json.Unmarshal(%q) = (%q, %v), want empty and JSON v2 syntactic refusal",
					tc.wire,
					refused,
					gotStdlibErr,
				)
			}
			got, gotErr := DecodeStrictJSON[AbsolutePath](
				bytes.NewReader([]byte(tc.wire)),
				DefaultStrictJSONLimits(),
			)
			if !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf(
					"DecodeStrictJSON[AbsolutePath](%q) error = %v, want %v",
					tc.wire,
					gotErr,
					ErrJSONContract,
				)
			}
			if got != (AbsolutePath{}) {
				t.Fatalf("DecodeStrictJSON[AbsolutePath](%q) = %v, want zero", tc.wire, got)
			}
			var direct AbsolutePath
			gotDirectErr := direct.UnmarshalJSON([]byte(tc.wire))
			if !errors.Is(gotDirectErr, ErrJSONContract) {
				t.Fatalf(
					"AbsolutePath.UnmarshalJSON(%q) error = %v, want %v",
					tc.wire,
					gotDirectErr,
					ErrJSONContract,
				)
			}
			if direct != (AbsolutePath{}) {
				t.Fatalf("AbsolutePath.UnmarshalJSON(%q) receiver = %v, want zero", tc.wire, direct)
			}
		})
	}
}

func TestTypedAbsolutePathCompositionPreservesLexicalContract(t *testing.T) {
	t.Parallel()

	root, rootErr := ParseAbsolutePath(string(filepath.Separator))
	if rootErr != nil {
		t.Fatalf("ParseAbsolutePath(root) error = %v, want nil", rootErr)
	}
	parentName, parentNameErr := ParsePathComponent("parent")
	if parentNameErr != nil {
		t.Fatalf("ParsePathComponent(parent) error = %v, want nil", parentNameErr)
	}
	fileName, fileNameErr := ParsePathComponent("file.txt")
	if fileNameErr != nil {
		t.Fatalf("ParsePathComponent(file.txt) error = %v, want nil", fileNameErr)
	}
	parent, joinParentErr := root.Join(parentName)
	if joinParentErr != nil {
		t.Fatalf("root.Join(parent) error = %v, want nil", joinParentErr)
	}
	child, joinChildErr := parent.Join(fileName)
	if joinChildErr != nil {
		t.Fatalf("parent.Join(file.txt) error = %v, want nil", joinChildErr)
	}
	wantChild := filepath.Join(string(filepath.Separator), parentName.String(), fileName.String())
	if child.String() != wantChild {
		t.Fatalf("joined absolute path = %q, want %q", child.String(), wantChild)
	}
	gotParent, gotParentErr := child.Parent()
	if gotParentErr != nil || gotParent != parent {
		t.Fatalf("child.Parent() = (%v, %v), want (%v, nil)", gotParent, gotParentErr, parent)
	}
	gotBase, gotBaseErr := child.Base()
	if gotBaseErr != nil || gotBase != fileName {
		t.Fatalf("child.Base() = (%v, %v), want (%v, nil)", gotBase, gotBaseErr, fileName)
	}
	gotRootParent, gotRootParentErr := root.Parent()
	if gotRootParentErr != nil || gotRootParent != root {
		t.Fatalf("root.Parent() = (%v, %v), want (%v, nil)", gotRootParent, gotRootParentErr, root)
	}
	if _, gotRootBaseErr := root.Base(); !errors.Is(gotRootBaseErr, ErrPrimitiveContract) {
		t.Fatalf("root.Base() error = %v, want %v", gotRootBaseErr, ErrPrimitiveContract)
	}
}

func TestTypedAbsolutePathCompositionRejectsEveryZeroOwnershipBoundary(t *testing.T) {
	t.Parallel()

	root, rootErr := ParseAbsolutePath(string(filepath.Separator))
	if rootErr != nil {
		t.Fatalf("ParseAbsolutePath(root) error = %v, want nil", rootErr)
	}
	component, componentErr := ParsePathComponent("child")
	if componentErr != nil {
		t.Fatalf("ParsePathComponent(child) error = %v, want nil", componentErr)
	}
	cases := []struct {
		wantErr error
		run     func() error
		name    string
	}{
		{name: "zero path component validation is rejected", run: func() error { return (PathComponent{}).Validate() }, wantErr: ErrPrimitiveContract},
		{name: "zero absolute path validation is rejected", run: func() error { return (AbsolutePath{}).Validate() }, wantErr: ErrPrimitiveContract},
		{name: "zero absolute path parent is rejected", run: func() error { _, err := (AbsolutePath{}).Parent(); return err }, wantErr: ErrPrimitiveContract},
		{name: "zero absolute path base is rejected", run: func() error { _, err := (AbsolutePath{}).Base(); return err }, wantErr: ErrPrimitiveContract},
		{name: "zero absolute path cannot join a valid component", run: func() error { _, err := (AbsolutePath{}).Join(component); return err }, wantErr: ErrPrimitiveContract},
		{name: "valid absolute path cannot join a zero component", run: func() error { _, err := root.Join(PathComponent{}); return err }, wantErr: ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("typed path operation error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestPathComponentHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		value       string
		disposition boundaryDisposition
	}{
		{name: "minimum one-letter component is accepted", value: "a", disposition: boundaryAccept},
		{name: "minimum one-digit component is accepted", value: "0", disposition: boundaryAccept},
		{name: "single hyphen component is accepted", value: "-", disposition: boundaryAccept},
		{name: "single underscore component is accepted", value: "_", disposition: boundaryAccept},
		{name: "two-letter component is accepted", value: "ab", disposition: boundaryAccept},
		{name: "leading dot without dot identity is accepted", value: ".hidden", disposition: boundaryAccept},
		{name: "trailing dot without dot identity is accepted", value: "name.", disposition: boundaryAccept},
		{name: "three dots are an ordinary component", value: "...", disposition: boundaryAccept},
		{name: "internal repeated dots are accepted", value: "a..b", disposition: boundaryAccept},
		{name: "internal ASCII space is accepted", value: "a b", disposition: boundaryAccept},
		{name: "leading ASCII space is preserved", value: " name", disposition: boundaryAccept},
		{name: "trailing ASCII space is preserved", value: "name ", disposition: boundaryAccept},
		{name: "tab byte is a valid native filename byte", value: "a\tb", disposition: boundaryAccept},
		{name: "newline byte is a valid native filename byte", value: "a\nb", disposition: boundaryAccept},
		{name: "control byte 0x01 is a valid native filename byte", value: "a\x01b", disposition: boundaryAccept},
		{name: "bell byte is a valid native filename byte", value: "a\x07b", disposition: boundaryAccept},
		{name: "vertical tab is a valid native filename byte", value: "a\x0bb", disposition: boundaryAccept},
		{name: "delete byte is a valid native filename byte", value: "a\x7fb", disposition: boundaryAccept},
		{name: "Latin Unicode component is accepted", value: "é", disposition: boundaryAccept},
		{name: "multi-rune Unicode component is accepted", value: "世界", disposition: boundaryAccept},
		{name: "four-byte Unicode rune is accepted", value: "🙂", disposition: boundaryAccept},
		{name: "punctuation without native separator is accepted", value: "!@#$%^&()[]{}=+,;'", disposition: boundaryAccept},
		{name: "less-than and greater-than are accepted", value: "a<b>c", disposition: boundaryAccept},
		{name: "one byte below component maximum is accepted", value: strings.Repeat("a", filesystemPathComponentMaximumBytes-1), disposition: boundaryAccept},
		{name: "exact component byte maximum is accepted", value: strings.Repeat("a", filesystemPathComponentMaximumBytes), disposition: boundaryAccept},
		{name: "multibyte value one byte below maximum is accepted", value: strings.Repeat("é", 127), disposition: boundaryAccept},
		{name: "multibyte value at exact maximum is accepted", value: strings.Repeat("é", 127) + "a", disposition: boundaryAccept},
		{name: "empty component is rejected"},
		{name: "exact current-directory identity is rejected", value: "."},
		{name: "exact parent-directory identity is rejected", value: ".."},
		{name: "native separator alone is rejected", value: string(filepath.Separator)},
		{name: "native separator in middle is rejected", value: "a" + string(filepath.Separator) + "b"},
		{name: "native separator prefix is rejected", value: string(filepath.Separator) + "a"},
		{name: "native separator suffix is rejected", value: "a" + string(filepath.Separator)},
		{name: "repeated native separators are rejected", value: "a" + strings.Repeat(string(filepath.Separator), 2) + "b"},
		{name: "absolute multi-component path is rejected", value: filepath.Join(string(filepath.Separator), "a", "b")},
		{name: "NUL at first byte is rejected", value: "\x00ab"},
		{name: "NUL in middle is rejected", value: "a\x00b"},
		{name: "NUL at final byte is rejected", value: "ab\x00"},
		{name: "invalid UTF8 first byte is rejected", value: string([]byte{0xff, 'a'})},
		{name: "invalid UTF8 middle byte is rejected", value: string([]byte{'a', 0xff, 'b'})},
		{name: "invalid UTF8 final byte is rejected", value: string([]byte{'a', 0xff})},
		{name: "one byte above component maximum is rejected", value: strings.Repeat("a", filesystemPathComponentMaximumBytes+1)},
		{name: "multibyte value one byte above maximum is rejected", value: strings.Repeat("é", 128)},
		{name: "far above component maximum is rejected", value: strings.Repeat("界", filesystemPathComponentMaximumBytes*2)},
		{name: "maximum bytes plus separator is rejected", value: strings.Repeat("a", filesystemPathComponentMaximumBytes) + string(filepath.Separator)},
		{name: "dot identity with separator suffix is rejected", value: "." + string(filepath.Separator)},
		{name: "parent identity with separator suffix is rejected", value: ".." + string(filepath.Separator)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParsePathComponent(tc.value)
			wantAccept := tc.disposition == boundaryAccept
			if (gotErr == nil) != wantAccept {
				t.Fatalf("ParsePathComponent(%q) = (%v, %v), want accept %t", tc.value, got, gotErr, wantAccept)
			}
			if !wantAccept && !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("ParsePathComponent(%q) error = %v, want %v", tc.value, gotErr, ErrPrimitiveContract)
			}
			if wantAccept {
				if got.String() != tc.value {
					t.Fatalf("ParsePathComponent(%q).String() = %q, want %q", tc.value, got.String(), tc.value)
				}
				if gotValidateErr := got.Validate(); gotValidateErr != nil {
					t.Fatalf("ParsePathComponent(%q).Validate() error = %v, want nil", tc.value, gotValidateErr)
				}
				directWire, directMarshalErr := got.MarshalJSON()
				if directMarshalErr != nil {
					t.Fatalf("PathComponent.MarshalJSON(%q) error = %v, want nil", got, directMarshalErr)
				}
				if !jsontext.Value(directWire).IsValid() {
					t.Fatalf("PathComponent.MarshalJSON(%q) wire = %q, want valid JSON", got, directWire)
				}
				directRoundTrip, directDecodeErr := DecodeStrictJSON[PathComponent](
					bytes.NewReader(directWire),
					DefaultStrictJSONLimits(),
				)
				if directDecodeErr != nil || directRoundTrip != got {
					t.Fatalf(
						"PathComponent direct JSON round trip = (%v, %v), want (%v, nil)",
						directRoundTrip,
						directDecodeErr,
						got,
					)
				}
				gotWire, gotEncodeErr := EncodeValidatedJSON(got, DefaultStrictJSONLimits())
				if gotEncodeErr != nil {
					t.Fatalf("EncodeValidatedJSON(PathComponent %q) error = %v, want nil", got, gotEncodeErr)
				}
				gotRoundTrip, gotDecodeErr := DecodeStrictJSON[PathComponent](
					bytes.NewReader(gotWire),
					DefaultStrictJSONLimits(),
				)
				if gotDecodeErr != nil || gotRoundTrip != got {
					t.Fatalf(
						"PathComponent strict JSON round trip = (%v, %v), want (%v, nil)",
						gotRoundTrip,
						gotDecodeErr,
						got,
					)
				}
			}
		})
	}
}
