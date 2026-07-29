package core

import (
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAbsoluteFilesystemPathHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	maximumRunePath := strings.Repeat(
		string(filepath.Separator)+strings.Repeat("a", FilesystemPathComponentMaximumBytes),
		16,
	)
	oneBelowMaximumRunePath := maximumRunePath[:len(maximumRunePath)-1]
	overMaximumRunePath := strings.Repeat(
		string(filepath.Separator)+strings.Repeat("a", FilesystemPathComponentMaximumBytes),
		15,
	) + string(filepath.Separator) + strings.Repeat("a", FilesystemPathComponentMaximumBytes-1) +
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
		{name: "far above rune maximum is rejected", value: "/" + strings.Repeat("界", FilesystemPathMaximumRunes*2)},
		{name: "one component one byte above filesystem maximum is rejected", value: "/" + strings.Repeat("a", FilesystemPathComponentMaximumBytes+1)},
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
				if !json.Valid(directWire) {
					t.Fatalf("AbsolutePath.MarshalJSON(%q) wire = %q, want valid JSON", got, directWire)
				}
				directRoundTrip, directDecodeErr := DecodeStrictJSON[AbsolutePath](
					directWire,
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
					gotWire,
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

	if got := utf8.RuneCountInString(maximumRunePath); got != FilesystemPathMaximumRunes {
		t.Fatalf("maximum-rune fixture count = %d, want %d", got, FilesystemPathMaximumRunes)
	}
	if got := utf8.RuneCountInString(overMaximumRunePath); got != FilesystemPathMaximumRunes+1 {
		t.Fatalf("over-maximum-rune fixture count = %d, want %d", got, FilesystemPathMaximumRunes+1)
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
			got, gotErr := DecodeStrictJSON[AbsolutePath]([]byte(tc.wire), DefaultStrictJSONLimits())
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

			var repaired string
			gotStdlibErr := json.Unmarshal([]byte(tc.wire), &repaired)
			if gotStdlibErr != nil || !strings.ContainsRune(repaired, utf8.RuneError) {
				t.Fatalf(
					"json.Unmarshal(%q) = (%q, %v), want lossy replacement/nil fixture proof",
					tc.wire,
					repaired,
					gotStdlibErr,
				)
			}
			got, gotErr := DecodeStrictJSON[AbsolutePath](
				[]byte(tc.wire),
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

func TestPlatformClosedDomainAndJSONBoundary(t *testing.T) {
	t.Parallel()

	wantOperatingSystem, wantOperatingSystemErr := ParseOperatingSystem(runtime.GOOS)
	wantArchitecture, wantArchitectureErr := ParseCPUArchitecture(runtime.GOARCH)
	gotCurrent, gotCurrentErr := CurrentSupportedPlatform()
	if wantOperatingSystemErr != nil || wantArchitectureErr != nil {
		if !errors.Is(gotCurrentErr, ErrPrimitiveContract) {
			t.Fatalf("CurrentSupportedPlatform() error = %v, want %v", gotCurrentErr, ErrPrimitiveContract)
		}
		if gotCurrent != (Platform{}) {
			t.Fatalf("CurrentSupportedPlatform() value = %v, want zero on unsupported host", gotCurrent)
		}
	} else {
		wantCurrent, wantCurrentErr := NewPlatform(wantOperatingSystem, wantArchitecture)
		if wantCurrentErr != nil {
			t.Fatalf("NewPlatform(runtime source facts) error = %v, want nil", wantCurrentErr)
		}
		if gotCurrentErr != nil || gotCurrent != wantCurrent {
			t.Fatalf("CurrentSupportedPlatform() = (%v, %v), want (%v, nil)", gotCurrent, gotCurrentErr, wantCurrent)
		}
	}

	valid := []struct {
		name            string
		wantWire        string
		operatingSystem OperatingSystem
		architecture    CPUArchitecture
	}{
		{name: "Darwin AMD64", operatingSystem: OperatingSystemDarwin, architecture: CPUArchitectureAMD64, wantWire: "darwin-amd64"},
		{name: "Darwin ARM64", operatingSystem: OperatingSystemDarwin, architecture: CPUArchitectureARM64, wantWire: "darwin-arm64"},
		{name: "Linux AMD64", operatingSystem: OperatingSystemLinux, architecture: CPUArchitectureAMD64, wantWire: "linux-amd64"},
		{name: "Linux ARM64", operatingSystem: OperatingSystemLinux, architecture: CPUArchitectureARM64, wantWire: "linux-arm64"},
		{name: "Windows AMD64", operatingSystem: OperatingSystemWindows, architecture: CPUArchitectureAMD64, wantWire: "windows-amd64"},
		{name: "Windows ARM64", operatingSystem: OperatingSystemWindows, architecture: CPUArchitectureARM64, wantWire: "windows-arm64"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewPlatform(tc.operatingSystem, tc.architecture)
			if gotErr != nil || got.String() != tc.wantWire {
				t.Fatalf("NewPlatform() = (%v, %v), want wire %q/nil", got, gotErr, tc.wantWire)
			}
			gotParsed, gotParseErr := ParsePlatform(tc.wantWire)
			if gotParseErr != nil || gotParsed != got {
				t.Fatalf("ParsePlatform(%q) = (%v, %v), want (%v, nil)", tc.wantWire, gotParsed, gotParseErr, got)
			}
			gotJSON, gotJSONErr := json.Marshal(got)
			if gotJSONErr != nil {
				t.Fatalf("json.Marshal(Platform) error = %v, want nil", gotJSONErr)
			}
			var gotRoundTrip Platform
			gotRoundTripErr := json.Unmarshal(gotJSON, &gotRoundTrip)
			if gotRoundTripErr != nil || gotRoundTrip != got {
				t.Fatalf("Platform JSON round trip = (%v, %v), want (%v, nil)", gotRoundTrip, gotRoundTripErr, got)
			}
		})
	}

	invalid := []struct {
		name string
		wire string
	}{
		{name: "empty token", wire: ""},
		{name: "missing architecture", wire: "linux"},
		{name: "missing operating system", wire: "-amd64"},
		{name: "empty architecture", wire: "linux-"},
		{name: "extra separator", wire: "linux-amd64-extra"},
		{name: "slash separator", wire: "linux/amd64"},
		{name: "underscore separator", wire: "linux_amd64"},
		{name: "uppercase operating system", wire: "Linux-amd64"},
		{name: "uppercase architecture", wire: "linux-AMD64"},
		{name: "unknown operating system", wire: "freebsd-amd64"},
		{name: "unknown architecture", wire: "linux-386"},
		{name: "space prefix", wire: " linux-amd64"},
		{name: "space suffix", wire: "linux-amd64 "},
		{name: "newline suffix", wire: "linux-amd64\n"},
		{name: "NUL suffix", wire: "linux-amd64\x00"},
		{name: "duplicate token", wire: "linux-amd64linux-amd64"},
		{name: "JSON null spelling", wire: "null"},
		{name: "JSON quoted spelling", wire: `"linux-amd64"`},
		{name: "future architecture", wire: "linux-riscv64"},
		{name: "far oversized token", wire: strings.Repeat("linux-amd64", 100)},
	}
	for _, tc := range invalid {
		t.Run("reject "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParsePlatform(tc.wire)
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("ParsePlatform(%q) error = %v, want %v", tc.wire, gotErr, ErrPrimitiveContract)
			}
			if got != (Platform{}) {
				t.Fatalf("ParsePlatform(%q) value = %v, want zero", tc.wire, got)
			}
		})
	}
}

func TestPlatformComponentEnumsExhaustClosedDomainsAndJSON(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		operatingSystem := OperatingSystem(raw)
		wantOperatingSystemValid := operatingSystem > OperatingSystemUnknown &&
			operatingSystem < operatingSystemLimit
		if gotValid := operatingSystem.IsValid(); gotValid != wantOperatingSystemValid {
			t.Fatalf("OperatingSystem(%d).IsValid() = %t, want %t", raw, gotValid, wantOperatingSystemValid)
		}
		gotOperatingSystemWire, gotOperatingSystemErr := json.Marshal(operatingSystem)
		if !wantOperatingSystemValid {
			if !errors.Is(gotOperatingSystemErr, ErrJSONContract) || gotOperatingSystemWire != nil {
				t.Fatalf(
					"json.Marshal(OperatingSystem(%d)) = (%s, %v), want (nil, %v)",
					raw,
					gotOperatingSystemWire,
					gotOperatingSystemErr,
					ErrJSONContract,
				)
			}
		} else {
			if gotOperatingSystemErr != nil {
				t.Fatalf("json.Marshal(OperatingSystem(%d)) error = %v, want nil", raw, gotOperatingSystemErr)
			}
			var gotOperatingSystem OperatingSystem
			gotUnmarshalErr := json.Unmarshal(gotOperatingSystemWire, &gotOperatingSystem)
			if gotUnmarshalErr != nil || gotOperatingSystem != operatingSystem {
				t.Fatalf(
					"OperatingSystem(%d) JSON round trip = (%v, %v), want (%v, nil)",
					raw,
					gotOperatingSystem,
					gotUnmarshalErr,
					operatingSystem,
				)
			}
		}

		architecture := CPUArchitecture(raw)
		wantArchitectureValid := architecture > CPUArchitectureUnknown &&
			architecture < cpuArchitectureLimit
		if gotValid := architecture.IsValid(); gotValid != wantArchitectureValid {
			t.Fatalf("CPUArchitecture(%d).IsValid() = %t, want %t", raw, gotValid, wantArchitectureValid)
		}
		gotArchitectureWire, gotArchitectureErr := json.Marshal(architecture)
		if !wantArchitectureValid {
			if !errors.Is(gotArchitectureErr, ErrJSONContract) || gotArchitectureWire != nil {
				t.Fatalf(
					"json.Marshal(CPUArchitecture(%d)) = (%s, %v), want (nil, %v)",
					raw,
					gotArchitectureWire,
					gotArchitectureErr,
					ErrJSONContract,
				)
			}
			continue
		}
		if gotArchitectureErr != nil {
			t.Fatalf("json.Marshal(CPUArchitecture(%d)) error = %v, want nil", raw, gotArchitectureErr)
		}
		var gotArchitecture CPUArchitecture
		gotUnmarshalErr := json.Unmarshal(gotArchitectureWire, &gotArchitecture)
		if gotUnmarshalErr != nil || gotArchitecture != architecture {
			t.Fatalf(
				"CPUArchitecture(%d) JSON round trip = (%v, %v), want (%v, nil)",
				raw,
				gotArchitecture,
				gotUnmarshalErr,
				architecture,
			)
		}
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
		{name: "one byte below component maximum is accepted", value: strings.Repeat("a", FilesystemPathComponentMaximumBytes-1), disposition: boundaryAccept},
		{name: "exact component byte maximum is accepted", value: strings.Repeat("a", FilesystemPathComponentMaximumBytes), disposition: boundaryAccept},
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
		{name: "one byte above component maximum is rejected", value: strings.Repeat("a", FilesystemPathComponentMaximumBytes+1)},
		{name: "multibyte value one byte above maximum is rejected", value: strings.Repeat("é", 128)},
		{name: "far above component maximum is rejected", value: strings.Repeat("界", FilesystemPathComponentMaximumBytes*2)},
		{name: "maximum bytes plus separator is rejected", value: strings.Repeat("a", FilesystemPathComponentMaximumBytes) + string(filepath.Separator)},
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
				if !json.Valid(directWire) {
					t.Fatalf("PathComponent.MarshalJSON(%q) wire = %q, want valid JSON", got, directWire)
				}
				directRoundTrip, directDecodeErr := DecodeStrictJSON[PathComponent](
					directWire,
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
					gotWire,
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

func TestPlatformTokenBoundCoversEveryAdmittedCombination(t *testing.T) {
	t.Parallel()

	for operatingSystem := OperatingSystemDarwin; operatingSystem < operatingSystemLimit; operatingSystem++ {
		for architecture := CPUArchitectureAMD64; architecture < cpuArchitectureLimit; architecture++ {
			platform, err := NewPlatform(operatingSystem, architecture)
			if err != nil {
				t.Fatalf("NewPlatform(%v, %v) error = %v, want nil", operatingSystem, architecture, err)
			}
			if len(platform.String()) > PlatformTokenMaximumBytes {
				t.Fatalf("admitted platform %q length = %d, exceeds %d", platform, len(platform.String()), PlatformTokenMaximumBytes)
			}
		}
	}
}
