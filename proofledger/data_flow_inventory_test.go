package proofledger

import (
	"embed"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

//go:embed *.go
var proofLedgerSource embed.FS

func TestProofLedgerProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(proofLedgerSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(proofledger source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := proofLedgerSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse proofledger source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectProofLedgerRoles(file, structs, classified)
	}
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		t.Fatalf("proofledger production structs missing data-flow role = %q, want every struct classified", missing)
	}
}

// The compiler binds every decoder coordinate to its concrete call signature.
var proofLedgerIngress = struct {
	Ledger     func(*LedgerIdentity, []byte) error
	Event      func(*EventIdentity, []byte) error
	Sequence   func(*Sequence, []byte) error
	Position   func(*Position, []byte) error
	PageLimit  func(*PageLimit, []byte) error
	Receipt    func(*AppendReceipt, []byte) error
	Document   func(*AppendReceiptDocument, []byte) error
	Domain     func(*AppendReceiptSigningDomain, []byte) error
	DomainText func(AppendReceiptSigningDomain, []byte) (AppendReceiptSigningDomain, error)
	Envelope   func([]byte) (Envelope[ledgerTestPayload], error)
}{
	(*LedgerIdentity).UnmarshalJSON, (*EventIdentity).UnmarshalJSON,
	(*Sequence).UnmarshalJSON, (*Position).UnmarshalJSON, (*PageLimit).UnmarshalJSON,
	(*AppendReceipt).UnmarshalJSON, (*AppendReceiptDocument).UnmarshalJSON,
	(*AppendReceiptSigningDomain).UnmarshalJSON, AppendReceiptSigningDomain.ParseCanonicalText,
	DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload],
}

var proofLedgerFuzzInventory = struct {
	Envelope, JSON, Receipt, Replay, Domain func(*testing.F)
}{
	FuzzEnvelopeSemanticClosure, FuzzProofLedgerExternalJSONDoorsSemanticClosure,
	FuzzProofLedgerReceiptAuthentication, FuzzProofLedgerReplayAndAppendHead,
	FuzzProofLedgerSigningDomainText,
}

func TestProofLedgerExternalIngressInventory(t *testing.T) {
	t.Parallel()
	_ = proofLedgerIngress
	_ = proofLedgerFuzzInventory
	files, err := fs.Glob(proofLedgerSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob() error = %v, want nil", err)
	}
	var got []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := proofLedgerSource.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v, want nil", name, err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", name, err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !slices.Contains([]string{"UnmarshalJSON", "UnmarshalText", "ParseCanonicalText", "DecodeEnvelope"}, function.Name.Name) {
				continue
			}
			coordinate := function.Name.Name
			if function.Recv != nil {
				coordinate = roleReceiverName(function.Recv.List[0].Type) + "." + coordinate
			}
			got = append(got, coordinate)
		}
	}
	want := []string{"AppendReceipt.UnmarshalJSON", "AppendReceiptDocument.UnmarshalJSON", "AppendReceiptSigningDomain.ParseCanonicalText", "AppendReceiptSigningDomain.UnmarshalJSON", "DecodeEnvelope", "EventIdentity.UnmarshalJSON", "LedgerIdentity.UnmarshalJSON", "PageLimit.UnmarshalJSON", "Position.UnmarshalJSON", "Sequence.UnmarshalJSON"}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("external ingress = %v, want compiler-bound fuzz inventory %v", got, want)
	}
}

func TestProofLedgerSigningDomainExhaustsByteDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		value := AppendReceiptSigningDomain(raw)
		err := value.Validate()
		encoded, encodeErr := value.MarshalJSON()
		if value == AppendReceiptSigningDomainV1 {
			if err != nil || encodeErr != nil || !value.IsValid() {
				t.Fatalf("domain %d errors = %v/%v, want valid", raw, err, encodeErr)
			}
			var got AppendReceiptSigningDomain
			if err := got.UnmarshalJSON(encoded); err != nil || got != value {
				t.Fatalf("domain %d roundtrip = %v/%v, want exact", raw, got, err)
			}
		} else if !errors.Is(err, core.ErrProofLedgerContract) || !errors.Is(encodeErr, core.ErrJSONContract) || len(encoded) != 0 || value.IsValid() || value.String() != "" {
			t.Fatalf("domain %d errors = %v/%v bytes=%d, want typed refusal and no projection", raw, err, encodeErr, len(encoded))
		}
	}
}

func collectProofLedgerRoles(file *ast.File, structs, classified map[string]struct{}) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					structs[typeSpec.Name.Name] = struct{}{}
				}
			}
		case *ast.FuncDecl:
			if value.Recv == nil || !proofLedgerRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := roleReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func proofLedgerRoleMethod(name string) bool {
	return name == "proofLedgerProtocolFact" || name == "proofLedgerInternalFlow" || name == "proofLedgerCapabilityWrapper"
}
func roleReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.IndexExpr:
		return roleReceiverName(value.X)
	case *ast.IndexListExpr:
		return roleReceiverName(value.X)
	}
	return ""
}
