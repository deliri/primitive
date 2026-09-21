package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeAnalyzerKeepsPublishedBytesAndRefusesNewLayoutDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		declaration string
		extra       string
		wantError   error
	}{
		{name: "published declaration remains exact", declaration: "struct {\n\tFirst byte `json:\"first\"`\n\tSecond *int `json:\"second\"`\n}"},
		{name: "unlisted scratch layout still fails", declaration: "struct {\n\tFirst byte `json:\"first\"`\n\tSecond *int `json:\"second\"`\n}", extra: "type Scratch struct { Flag byte; Pointer *int }", wantError: errUnpinnedLayout},
		{name: "reordered wire fields cannot hide behind admission", declaration: "struct {\n\tSecond *int `json:\"second\"`\n\tFirst byte `json:\"first\"`\n}", wantError: errWireLayoutChanged},
		{name: "changed wire spelling cannot hide behind admission", declaration: "struct {\n\tFirst byte `json:\"different\"`\n\tSecond *int `json:\"second\"`\n}", wantError: errWireLayoutChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeFixture(t, root, "go.mod", []byte("module fieldalignment-fixture\n\ngo 1.27.1\n"))
			writeFixture(t, root, "wire.go", []byte("package fixture\n\ntype Wire "+tc.declaration+"\n"+tc.extra+"\n"))
			pins := []wireLayout{{File: "wire.go", Type: "Wire", Declaration: "struct {\n\tFirst  byte `json:\"first\"`\n\tSecond *int `json:\"second\"`\n}", Reason: "Published JSON field order is part of canonical bytes."}}
			encoded, err := json.Marshal(pins)
			if err != nil {
				t.Fatalf("encode pinned fixture = %v, want nil", err)
			}
			writeFixture(t, root, "layouts.json", encoded)
			var output, diagnostics bytes.Buffer
			err = run(root, "layouts.json", &output, &diagnostics)
			if !errors.Is(err, tc.wantError) {
				t.Fatalf("native analyzer gate error = %v, want %v; stdout=%s stderr=%s", err, tc.wantError, output.Bytes(), diagnostics.Bytes())
			}
			if tc.wantError == nil && !strings.Contains(output.String(), "struct with 16 pointer bytes could be 8") {
				t.Fatalf("retained analyzer output = %q, want the admitted native pointer-layout diagnostic", output.Bytes())
			}
			if tc.extra != "" && !strings.Contains(output.String(), "fieldalignment-fixture") {
				t.Fatalf("unlisted layout output = %q, want native analysis of fixture module", output.Bytes())
			}
		})
	}
}

func writeFixture(t *testing.T, root, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
		t.Fatalf("write fixture %s = %v, want nil", name, err)
	}
}

func TestAnalyzerLoadFailureCannotBecomeAnEmptyPassingResult(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`null`, `{`, `{"fixture":{"error":"type checking failed"}}`, `{"fixture":{}}`, `{"fixture":{"fieldalignment":null}}`} {
		if err := checkFindings([]byte(raw), nil); !errors.Is(err, errAnalyzerFailure) {
			t.Fatalf("check native failure %q = %v, want analyzer failure", raw, err)
		}
	}
}

func TestASN1SequenceOrderRemainsPinned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixture(t, root, "go.mod", []byte("module fieldalignment-asn1-fixture\n\ngo 1.27.1\n"))
	const original = "struct {\n\tStatus byte\n\tToken *int `asn1:\"optional\"`\n}"
	writeFixture(t, root, "wire.go", []byte("package fixture\ntype Wire "+original+"\n"))
	pins := []wireLayout{{File: "wire.go", Type: "Wire", Declaration: "struct {\n\tStatus byte\n\tToken  *int `asn1:\"optional\"`\n}", Reason: "ASN.1 SEQUENCE fields are positional."}}
	encoded, err := json.Marshal(pins)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "layouts.json", encoded)
	var output, diagnostics bytes.Buffer
	if err := run(root, "layouts.json", &output, &diagnostics); err != nil {
		t.Fatalf("original ASN.1 agreement: %v; %s", err, diagnostics.Bytes())
	}
	if !strings.Contains(output.String(), "struct with 16 pointer bytes could be 8") {
		t.Fatalf("missing native diagnostic: %s", output.Bytes())
	}
	writeFixture(t, root, "wire.go", []byte("package fixture\ntype Wire struct { Token *int `asn1:\"optional\"`; Status byte }\n"))
	if err := run(root, "layouts.json", &output, &diagnostics); !errors.Is(err, errWireLayoutChanged) {
		t.Fatalf("reordered ASN.1 agreement = %v, want wire change", err)
	}
}
