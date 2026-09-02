package receipt

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

// canonicalKeys returns every object member name in the exact order the encoder
// emitted it, depth-first, so a reordered structure is visible as data rather
// than as a length that happens to match.
func canonicalKeys(t *testing.T, data []byte) []string {
	t.Helper()

	type frame struct {
		object    bool
		expectKey bool
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(data))
	var got []string
	var stack []frame
	for {
		element, err := decoder.ReadToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode canonical JSON token: %v", err)
		}
		kind := element.Kind()
		if kind == jsontext.KindBeginObject || kind == jsontext.KindBeginArray ||
			kind == jsontext.KindEndObject || kind == jsontext.KindEndArray {
			switch kind {
			case jsontext.KindBeginObject:
				stack = append(stack, frame{object: true, expectKey: true})
			case jsontext.KindBeginArray:
				stack = append(stack, frame{})
			case jsontext.KindEndObject, jsontext.KindEndArray:
				if len(stack) == 0 {
					t.Fatalf("closing delimiter %q has no open container", kind)
				}
				stack = stack[:len(stack)-1]
				if len(stack) > 0 && stack[len(stack)-1].object {
					stack[len(stack)-1].expectKey = true
				}
			}
			continue
		}
		if len(stack) == 0 || !stack[len(stack)-1].object {
			continue
		}
		if stack[len(stack)-1].expectKey {
			if element.Kind() != jsontext.KindString {
				t.Fatalf("object member name = %v, want a string", element)
			}
			got = append(got, element.String())
			stack[len(stack)-1].expectKey = false
			continue
		}
		stack[len(stack)-1].expectKey = true
	}
	return got
}

// TestCanonicalMemberOrderIsPinnedIndependentlyOfMemoryLayout is the ratchet for
// the defect that field alignment can silently rewrite a signed contract.
//
// Every structure below is either signed through Attest or written durably. Its
// canonical member order is a wire contract, not an implementation detail. If a
// layout optimizer, a readability edit, or a new field changes this order, every
// previously signed receipt stops verifying and every durable watermark changes
// shape. That failure is silent at every other layer: lengths still match,
// round trips still succeed, and Validate still passes. Only this test sees it.
func TestCanonicalMemberOrderIsPinnedIndependentlyOfMemoryLayout(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 240)
	document := issueFixture(t, fixture)
	watermark := watermarkFixture(
		t,
		Scope{Principal: fixture.principal, Offering: fixture.offering},
		1,
		"wire-order",
	)
	cases := []struct {
		value json.Marshaler
		name  string
		want  []string
	}{
		{
			name:  "signed evidence body",
			value: document.Payload.Body,
			want: []string{
				"submission_identity", "object_identity", "extent_bytes",
				"sha256", "crc32c",
			},
		},
		{
			name:  "signed header",
			value: document.Payload.Header,
			want: []string{
				"receipt_identity", "principal_identity", "offering",
				"revision", "occurred_at_nanoseconds",
			},
		},
		{
			name:  "signed payload",
			value: document.Payload,
			want: []string{
				"header",
				"receipt_identity", "principal_identity", "offering",
				"revision", "occurred_at_nanoseconds",
				"body",
				"submission_identity", "object_identity", "extent_bytes",
				"sha256", "crc32c",
			},
		},
		{
			name:  "durable watermark scope",
			value: watermark.Scope,
			want:  []string{"principal_identity", "offering"},
		},
		{
			name:  "durable watermark",
			value: watermark,
			want: []string{
				"revision",
				"scope", "principal_identity", "offering",
				"generation", "cursor_digest", "chain_hash",
			},
		},
		{
			name:  "transported document",
			value: document,
			want: []string{
				"payload",
				"header",
				"receipt_identity", "principal_identity", "offering",
				"revision", "occurred_at_nanoseconds",
				"body",
				"submission_identity", "object_identity", "extent_bytes",
				"sha256", "crc32c",
				"attestation",
				"domain", "signer", "body_length_bytes", "body_sha256", "signature",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := tc.value.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v, want nil", err)
			}
			got := canonicalKeys(t, encoded)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("canonical member order = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestSignedPayloadBytesStayByteExact proves the exact bytes Attest signs for a
// fully determined fixture. A member rename, a member reorder, a changed
// encoding of any value type, or a new signed field all change these bytes and
// invalidate every receipt ever issued. The literal is the contract.
func TestSignedPayloadBytesStayByteExact(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 250)
	document := issueFixture(t, fixture)
	got, err := document.Payload.MarshalJSON()
	if err != nil {
		t.Fatalf("EvidencePayload.MarshalJSON() error = %v, want nil", err)
	}
	want := `{"header":{"receipt_identity":"ff000000000000000000000000000000",` +
		`"principal_identity":"fb000000000000000000000000000000",` +
		`"offering":"receipt-fixture-fc",` +
		`"revision":"v1","occurred_at_nanoseconds":"250"},` +
		`"body":{"submission_identity":"fd000000000000000000000000000000",` +
		`"object_identity":"fe000000000000000000000000000000",` +
		`"extent_bytes":1,` +
		`"sha256":"aa7225e7d5b0a2552bbb58880b3ec00c286995b801a7aeb69281e76a8b4908de",` +
		`"crc32c":"yvEUHA=="}}`
	if string(got) != want {
		t.Fatalf("signed payload bytes =\n%s\nwant\n%s", got, want)
	}
}

// TestCanonicalWireStructuresArePointerOnly proves the mechanism that makes the
// order above safe. A pointer-only structure gives every member the same size
// and alignment, so no layout optimizer has any reordering to perform. A wire
// structure that admits a non-pointer member has re-entered the exact hazard
// these projections exist to remove.
func TestCanonicalWireStructuresArePointerOnly(t *testing.T) {
	t.Parallel()

	wantWireStructures := []string{
		"documentWire", "evidenceBodyWire", "headerWire", "payloadWire",
		"scopeWire", "watermarkWire",
	}
	var gotWireStructures []string
	for name, structure := range productionStructures(t) {
		if !strings.HasSuffix(name, "Wire") {
			continue
		}
		gotWireStructures = append(gotWireStructures, name)
		for _, field := range structure.Fields.List {
			if _, ok := field.Type.(*ast.StarExpr); ok {
				continue
			}
			t.Errorf(
				"wire structure %q member %v is not a pointer, so layout may reorder the wire contract",
				name, field.Names,
			)
		}
		if structure.Fields.NumFields() == 0 {
			t.Errorf("wire structure %q declares no members", name)
		}
	}
	slices.Sort(gotWireStructures)
	if !slices.Equal(gotWireStructures, wantWireStructures) {
		t.Fatalf("canonical wire structures = %#v, want %#v", gotWireStructures, wantWireStructures)
	}
}

// TestNoProductionMarshalerReliesOnDeclarationOrder proves no MarshalJSON
// reaches the encoder through a locally defined alias of its own exported type,
// which is the shape that silently binds wire order to declaration order.
func TestNoProductionMarshalerReliesOnDeclarationOrder(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		ast.Inspect(file, func(node ast.Node) bool {
			function, ok := node.(*ast.FuncDecl)
			if !ok || function.Name.Name != "MarshalJSON" || function.Body == nil {
				return true
			}
			for _, statement := range function.Body.List {
				declaration, ok := statement.(*ast.DeclStmt)
				if !ok {
					continue
				}
				generic, ok := declaration.Decl.(*ast.GenDecl)
				if !ok || generic.Tok != token.TYPE {
					continue
				}
				t.Errorf(
					"MarshalJSON on %v declares a local type alias, binding wire order to declaration order",
					function.Recv.List[0].Type,
				)
			}
			return true
		})
	}
}

func productionFiles(t *testing.T) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v, want nil", err)
	}
	files := token.NewFileSet()
	var got []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(files, name, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", name, parseErr)
		}
		got = append(got, parsed)
	}
	if len(got) == 0 {
		t.Fatal("parsed Receipt production sources = 0, want at least one file")
	}
	return got
}

func productionStructures(t *testing.T) map[string]*ast.StructType {
	t.Helper()

	got := make(map[string]*ast.StructType)
	for _, file := range productionFiles(t) {
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				specification, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if structure, ok := specification.Type.(*ast.StructType); ok {
					got[specification.Name.Name] = structure
				}
			}
		}
	}
	return got
}
