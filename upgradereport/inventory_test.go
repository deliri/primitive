package upgradereport

import (
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestProductionStructInventory(t *testing.T) {
	t.Parallel()
	roles := map[string]string{
		"EvidencePayload": "authenticated attempt-to-submission commitment binding",
		"EvidenceRequest": "shared signed attempt binding around the existing submission agreement",
		"Payload":         "device-owned observation crossing the signed reporting boundary",
		"Acknowledgment":  "authority-owned durable recording fact bound to an exact request digest",
		"Request":         "shared authenticated request, certificate and evidence receipt",
		"Response":        "shared authenticated durable acknowledgment; no product acceptance",
	}
	seen := make(map[string]bool)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range general.Specs {
				named, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := named.Type.(*ast.StructType); !ok {
					continue
				}
				if roles[named.Name.Name] == "" {
					t.Errorf("unclassified production struct %s", named.Name.Name)
				}
				seen[named.Name.Name] = true
			}
		}
	}
	for name := range roles {
		if !seen[name] {
			t.Errorf("inventory names absent struct %s", name)
		}
	}
}

func FuzzClosedEnumCanonicalAdmission(f *testing.F) {
	for o := Succeeded; o <= Interrupted; o++ {
		f.Add(value[[]byte](f)(o.MarshalJSON()))
	}
	for s := Bootstrap; s <= Complete; s++ {
		f.Add(value[[]byte](f)(s.MarshalJSON()))
	}
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`"future"`))
	f.Add([]byte(`123`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var text string
		textErr := json.Unmarshal(data, &text)
		outcome := Failed
		err := outcome.UnmarshalJSON(data)
		if err != nil {
			if outcome != Failed || !errors.Is(err, core.ErrReportContract) {
				t.Fatalf("outcome refusal = (%v,%v), want preserved and ErrReportContract", outcome, err)
			}
		} else {
			if textErr != nil || outcome.String() != text {
				t.Fatalf("admitted outcome = %q for input %q, want exact token", outcome.String(), data)
			}
			if err := outcome.Validate(); err != nil {
				t.Fatal(err)
			}
			var again Outcome
			if err := again.UnmarshalJSON(value[[]byte](t)(outcome.MarshalJSON())); err != nil || again != outcome {
				t.Fatalf("outcome canonical closure = (%v,%v), want %v", again, err, outcome)
			}
		}
		stage := Trial
		err = stage.UnmarshalJSON(data)
		if err != nil {
			if stage != Trial || !errors.Is(err, core.ErrReportContract) {
				t.Fatalf("stage refusal = (%v,%v), want preserved and ErrReportContract", stage, err)
			}
		} else {
			if textErr != nil || stage.String() != text {
				t.Fatalf("admitted stage = %q for input %q, want exact token", stage.String(), data)
			}
			if err := stage.Validate(); err != nil {
				t.Fatal(err)
			}
			var again Stage
			if err := again.UnmarshalJSON(value[[]byte](t)(stage.MarshalJSON())); err != nil || again != stage {
				t.Fatalf("stage canonical closure = (%v,%v), want %v", again, err, stage)
			}
		}
	})
}
