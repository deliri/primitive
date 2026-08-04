package release

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

// TestMetadataKindExhaustsItsEntireBackingDomain proves the closed wire enum
// admits exactly the three customer documents and nothing else.
func TestMetadataKindExhaustsItsEntireBackingDomain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		kind := MetadataKind(raw)
		wantValid := raw >= int(MetadataKindDependencies) && raw <= int(MetadataKindReleaseNotes)
		if got := kind.IsValid(); got != wantValid {
			t.Fatalf("MetadataKind(%d).IsValid() = %t, want %t", raw, got, wantValid)
		}
		if gotErr := kind.Validate(); (gotErr == nil) != wantValid {
			t.Fatalf("MetadataKind(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if wantValid {
			continue
		}
		if got := kind.String(); got != core.UnknownEnumDiagnostic {
			t.Fatalf("MetadataKind(%d).String() = %q, want %q", raw, got, core.UnknownEnumDiagnostic)
		}
		if _, gotErr := kind.MarshalJSON(); !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("MetadataKind(%d).MarshalJSON() error = %v, want %v", raw, gotErr, core.ErrJSONContract)
		}
		if _, gotErr := metadataContentType(kind); gotErr == nil {
			t.Fatalf("metadataContentType(MetadataKind(%d)) error = nil, want rejection", raw)
		}
		if _, gotErr := metadataFilenameSuffix(kind); gotErr == nil {
			t.Fatalf("metadataFilenameSuffix(MetadataKind(%d)) error = nil, want rejection", raw)
		}
	}
}

func TestBuildProvenanceWireFieldsRejectInvalidDomainsAndRetainSignedDigests(t *testing.T) {
	t.Parallel()

	plan := provenanceBuildPlan(t)
	assignment, err := NewLinkerAssignment("github.com/offGridSoft/bug/internal/release.embeddedServerKey", "0123456789abcdef")
	if err != nil {
		t.Fatalf("NewLinkerAssignment() error = %v, want nil", err)
	}
	assignments, err := NewLinkerAssignments([]LinkerAssignment{assignment})
	if err != nil {
		t.Fatalf("NewLinkerAssignments() error = %v, want nil", err)
	}
	request := plan.request
	request.LinkerAssignments = assignments
	plan, err = PrepareBuildPlan(request)
	if err != nil {
		t.Fatalf("PrepareBuildPlan(nonempty linker set) error = %v, want nil", err)
	}
	provenance, err := NewBuildProvenance(BuildProvenanceRequest{
		Plan: plan, Tools: provenanceVerifiedTools(t),
		DerivationGeneration: garble.CurrentDerivationGeneration(),
	})
	if err != nil {
		t.Fatalf("NewBuildProvenance() error = %v, want nil", err)
	}
	wire, err := provenance.wire()
	if err != nil {
		t.Fatalf("BuildProvenance.wire() error = %v, want nil", err)
	}
	if len(wire.LinkerAssignments) != 1 {
		t.Fatalf("BuildProvenance linker assignments = %d, want 1 nonempty mutation target", len(wire.LinkerAssignments))
	}
	alternateGoDigest := sha256.Sum256([]byte("different go executable"))
	alternateGarbleDigest := sha256.Sum256([]byte("different garble executable"))

	cases := []struct {
		mutate       func(*buildProvenanceWire)
		name         string
		wantAccepted bool
	}{
		{name: "go toolchain version substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GoToolchain = "go1.26.6" }},
		{name: "garble module substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleModule += "/fork" }},
		{name: "garble version substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleVersion += "1" }},
		{name: "garble revision substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleRevision = strings.Repeat("0", len(w.GarbleRevision)) }},
		{name: "garble module sum substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleModuleSum += "x" }},
		{name: "future derivation substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleDerivation = "two" }},
		{name: "literal policy substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleLiterals = "future" }},
		{name: "diagnostic policy substitution is rejected", mutate: func(w *buildProvenanceWire) { w.GarbleDiagnostics = "future" }},
		{name: "flag shaped main package substitution is rejected", mutate: func(w *buildProvenanceWire) { w.MainPackage = "-buildmode=exe/cmd" }},
		{name: "module mode substitution is rejected", mutate: func(w *buildProvenanceWire) { w.ModuleMode = "mod" }},
		{name: "linker symbol substitution is rejected", mutate: func(w *buildProvenanceWire) { w.LinkerAssignments[0].Symbol = "-ldflags/y.Value" }},
		{name: "linker value control byte substitution is rejected", mutate: func(w *buildProvenanceWire) { w.LinkerAssignments[0].Value = "one\x00two" }},
		{name: "duplicated linker assignment is rejected", mutate: func(w *buildProvenanceWire) {
			w.LinkerAssignments = append(w.LinkerAssignments, w.LinkerAssignments[0])
		}},
		{name: "go executable digest substitution is retained as signed provenance", mutate: func(w *buildProvenanceWire) { w.GoExecutableSHA256 = core.NewSHA256Digest(alternateGoDigest) }, wantAccepted: true},
		{name: "garble executable digest substitution is retained as signed provenance", mutate: func(w *buildProvenanceWire) { w.GarbleExecutableSHA256 = core.NewSHA256Digest(alternateGarbleDigest) }, wantAccepted: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidate := wire
			candidate.LinkerAssignments = append([]linkerAssignmentWire(nil), wire.LinkerAssignments...)
			tc.mutate(&candidate)
			encoded, err := json.Marshal(candidate)
			if err != nil {
				t.Fatalf("json.Marshal(mutated BuildProvenance) error = %v, want nil", err)
			}
			got := provenance
			gotErr := json.Unmarshal(encoded, &got)
			if tc.wantAccepted {
				if gotErr != nil {
					t.Fatalf("json.Unmarshal(signed digest substitution) error = %v, want nil", gotErr)
				}
				if got == provenance {
					t.Fatalf("%s: json.Unmarshal() provenance = original, want changed signed fact", tc.name)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(mutated BuildProvenance) error = %v, want %v", gotErr, core.ErrJSONContract)
			}
			if got != provenance {
				t.Fatalf("%s: json.Unmarshal() receiver changed, want original provenance on rejection", tc.name)
			}
		})
	}
}

// TestBuildProvenanceHistoricalValidationDoesNotConsultCurrentSelectors is a
// structural ratchet for a contract that cannot have a runtime red case until
// a second reviewed tool identity exists. Signed historical provenance must be
// validated by its recorded closed-domain identity, never by today's selector.
func TestBuildProvenanceHistoricalValidationDoesNotConsultCurrentSelectors(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "publication_metadata.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parser.ParseFile(publication_metadata.go) error = %v, want nil", err)
	}
	targets := [...]struct {
		name  string
		found bool
	}{
		{name: "BuildProvenance.Validate"},
		{name: "buildProvenanceFromWire"},
		{name: "parseBuildProvenanceTools"},
	}
	var forbidden []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := function.Name.Name
		if function.Recv != nil {
			name = "BuildProvenance." + name
		}
		targetIndex := -1
		for index := range targets {
			if targets[index].name == name {
				targetIndex = index
				break
			}
		}
		if targetIndex == -1 {
			continue
		}
		targets[targetIndex].found = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			called := calledFunctionName(call.Fun)
			switch called {
			case "CurrentGoToolchain", "garble.CurrentTool", "garble.CurrentDerivationGeneration":
				forbidden = append(forbidden, name+" -> "+called)
			}
			return true
		})
	}
	for _, target := range targets {
		if !target.found {
			t.Fatalf("historical provenance structural target %q was not found", target.name)
		}
	}
	if len(forbidden) != 0 {
		t.Fatalf("historical provenance validation current-selector calls = %v, want none", forbidden)
	}
}

func calledFunctionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix, ok := value.X.(*ast.Ident)
		if ok {
			return prefix.Name + "." + value.Sel.Name
		}
	}
	return ""
}

// TestMetadataKindUnmarshalJSONRejectsEveryNonCanonicalToken proves the wire
// decoder accepts one exact byte sequence per role and refuses every equivalent
// encoding, so a signed manifest cannot carry two spellings of one role.
func TestMetadataKindUnmarshalJSONRejectsEveryNonCanonicalToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		encoded   string
		want      MetadataKind
		wantErr   bool
		wantTyped bool
	}{
		{name: "canonical dependencies token is accepted", encoded: `"dependencies"`, want: MetadataKindDependencies},
		{name: "canonical documentation token is accepted", encoded: `"documentation"`, want: MetadataKindDocumentation},
		{name: "canonical release notes token is accepted", encoded: `"release_notes"`, want: MetadataKindReleaseNotes},

		{name: "unicode escaped token is rejected", encoded: "\"\\u0064ependencies\"", wantErr: true, wantTyped: true},
		{name: "escaped release notes separator is rejected", encoded: "\"release\\u005fnotes\"", wantErr: true, wantTyped: true},
		{name: "uppercase token is rejected", encoded: `"Dependencies"`, wantErr: true, wantTyped: true},
		{name: "hyphenated token is rejected", encoded: `"release-notes"`, wantErr: true, wantTyped: true},
		{name: "spaced token is rejected", encoded: `"release notes"`, wantErr: true, wantTyped: true},
		{name: "unknown token is rejected", encoded: `"changelog"`, wantErr: true, wantTyped: true},
		{name: "empty token is rejected", encoded: `""`, wantErr: true, wantTyped: true},
		{name: "unknown zero label is rejected", encoded: `"unknown"`, wantErr: true, wantTyped: true},
		{name: "numeric ordinal is rejected", encoded: `1`, wantErr: true, wantTyped: true},
		{name: "zero ordinal is rejected", encoded: `0`, wantErr: true, wantTyped: true},
		{name: "null is rejected", encoded: `null`, wantErr: true, wantTyped: true},
		{name: "boolean is rejected", encoded: `true`, wantErr: true, wantTyped: true},
		{name: "object is rejected", encoded: `{"kind":"dependencies"}`, wantErr: true, wantTyped: true},
		{name: "array is rejected", encoded: `["dependencies"]`, wantErr: true, wantTyped: true},
		{name: "truncated string is rejected", encoded: `"dependencies`, wantErr: true},
		{name: "trailing garbage is rejected", encoded: `"dependencies"x`, wantErr: true},
		{name: "prefix token is rejected", encoded: `"depend"`, wantErr: true, wantTyped: true},
		{name: "padded token is rejected", encoded: `" dependencies "`, wantErr: true, wantTyped: true},
		{name: "oversized token is rejected", encoded: `"` + strings.Repeat("d", 4096) + `"`, wantErr: true, wantTyped: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got MetadataKind
			gotErr := json.Unmarshal([]byte(tc.encoded), &got)
			if tc.wantErr {
				if gotErr == nil {
					t.Fatalf("json.Unmarshal(%s, MetadataKind) error = nil, want rejection", tc.encoded)
				}
				if tc.wantTyped && !errors.Is(gotErr, core.ErrJSONContract) {
					t.Fatalf("json.Unmarshal(%s, MetadataKind) error = %v, want %v", tc.encoded, gotErr, core.ErrJSONContract)
				}
				if got != MetadataKindUnknown {
					t.Fatalf("json.Unmarshal(%s, MetadataKind) = %v, want the unset receiver", tc.encoded, got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("json.Unmarshal(%s, MetadataKind) error = %v, want nil", tc.encoded, gotErr)
			}
			if got != tc.want {
				t.Fatalf("json.Unmarshal(%s, MetadataKind) = %v, want %v", tc.encoded, got, tc.want)
			}
			reencoded, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("json.Marshal(%v) error = %v, want nil", got, err)
			}
			if string(reencoded) != tc.encoded {
				t.Fatalf("MetadataKind re-encoding = %s, want %s", reencoded, tc.encoded)
			}
		})
	}
}

// TestMetadataSetRejectsEveryCardinalityAndRoleSubstitutionOnTheWire proves the
// fixed three-slot projection cannot be grown, shrunk, reordered, or duplicated
// by a decoded document.
func TestMetadataSetRejectsEveryCardinalityAndRoleSubstitutionOnTheWire(t *testing.T) {
	t.Parallel()

	valid := fixtureMetadataSet(t)
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("json.Marshal(MetadataSet) error = %v, want nil", err)
	}
	assets := make([]json.RawMessage, 0, MetadataAssetCount)
	if err := json.Unmarshal(encoded, &assets); err != nil {
		t.Fatalf("json.Unmarshal(MetadataSet, raw assets) error = %v, want nil", err)
	}
	if len(assets) != MetadataAssetCount {
		t.Fatalf("encoded MetadataSet length = %d, want %d", len(assets), MetadataAssetCount)
	}
	joined := func(values ...json.RawMessage) string {
		parts := make([]string, len(values))
		for index, value := range values {
			parts[index] = string(value)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}

	cases := []struct {
		name    string
		encoded string
	}{
		{name: "empty array is rejected", encoded: "[]"},
		{name: "one asset is rejected", encoded: joined(assets[0])},
		{name: "two assets are rejected", encoded: joined(assets[0], assets[1])},
		{name: "four assets are rejected", encoded: joined(assets[0], assets[1], assets[2], assets[2])},
		{name: "reordered assets are rejected", encoded: joined(assets[2], assets[1], assets[0])},
		{name: "adjacent swap is rejected", encoded: joined(assets[1], assets[0], assets[2])},
		{name: "duplicated first asset is rejected", encoded: joined(assets[0], assets[0], assets[2])},
		{name: "duplicated last asset is rejected", encoded: joined(assets[0], assets[2], assets[2])},
		{name: "null members are rejected", encoded: "[null,null,null]"},
		{name: "null document is rejected", encoded: "null"},
		{name: "object document is rejected", encoded: `{"dependencies":{}}`},
		{name: "nested array is rejected", encoded: joined(json.RawMessage(joined(assets[0], assets[1], assets[2])))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got MetadataSet
			gotErr := json.Unmarshal([]byte(tc.encoded), &got)
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(MetadataSet) error = %v, want %v", gotErr, core.ErrJSONContract)
			}
			if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrReleaseManifest) {
				t.Fatalf("rejected MetadataSet.Validate() error = %v, want %v", gotErr, core.ErrReleaseManifest)
			}
			if _, ok := got.At(0); ok {
				t.Fatal("rejected MetadataSet.At(0) ok = true, want false")
			}
		})
	}
}
