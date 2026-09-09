package controlplane_test

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
)

//go:embed *.go
var controlplaneIngressSources embed.FS

type ingressProof struct {
	owner reflect.Type
	fuzz  func(*testing.F)
}

// Every JSON decoder has a compiler-linked semantic campaign. A new decoder
// cannot silently inherit coverage merely because its sibling has a fuzzer.
func TestExternalJSONDoorsHaveSemanticFuzzProof(t *testing.T) {
	t.Parallel()
	inventory := []ingressProof{
		{reflect.TypeFor[controlplane.SigningDomain](), FuzzSigningDomainExternalDecoders},
		{reflect.TypeFor[controlplane.ProductStatus](), FuzzProductStatusExternalDecoders},
		{reflect.TypeFor[controlplane.ResponseHeaderField](), FuzzResponseHeaderFieldExternalDecoders},
		{reflect.TypeFor[controlplane.UsageDisposition](), FuzzUsageDispositionExternalDecoders},
		{reflect.TypeFor[controlplane.UsageClass](), FuzzUsageClassExternalDecoders},
		{reflect.TypeFor[controlplane.OutcomeClass](), FuzzOutcomeClassExternalDecoders},
		{reflect.TypeFor[controlplane.RegistrationRequest](), FuzzRegistrationRequestExternalDecoder},
		{reflect.TypeFor[controlplane.InstallationCertificateBody](), FuzzInstallationCertificateBodyDecodeAndVerify},
		{reflect.TypeFor[controlplane.InstallationCertificateDocument](), FuzzInstallationCertificateDocumentDecodeAndVerify},
		{reflect.TypeFor[controlplane.RegistrationPayload](), FuzzRegistrationPayloadExternalDecoder},
		{reflect.TypeFor[controlplane.RegistrationDocument](), FuzzRegistrationDocumentDecodeAndVerify},
		{reflect.TypeFor[controlplane.CheckInPayload](), FuzzCheckInPayloadExternalDecoder},
		{reflect.TypeFor[controlplane.CheckInRequest](), FuzzCheckInRequestDecodeAndVerify},
		{reflect.TypeFor[controlplane.CheckInResponsePayload](), FuzzCheckInResponsePayloadExternalDecoder},
		{reflect.TypeFor[controlplane.CheckInResponseDocument](), FuzzCheckInResponseDocumentDecodeAndVerify},
		{reflect.TypeFor[controlplane.ResponseHeader](), FuzzResponseHeaderExternalDecoder},
		{reflect.TypeFor[controlplane.UsageWatermark](), FuzzUsageWatermarkExternalDecoder},
		{reflect.TypeFor[controlplane.UsageWindow](), FuzzUsageWindowDecode},
		{reflect.TypeFor[controlplane.ResponseCommitment](), FuzzResponseCommitmentExternalDecoder},
		{reflect.TypeFor[controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]](), FuzzAuthenticatedResponseExternalSemanticClosure},
	}
	want := make(map[string]bool, len(inventory))
	for _, entry := range inventory {
		name, _, _ := strings.Cut(entry.owner.Name(), "[")
		if entry.fuzz == nil || want[name] {
			t.Fatalf("ingress %s proof = duplicate or nil, want one semantic campaign", name)
		}
		want[name] = true
	}
	entries, err := controlplaneIngressSources.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(sources) error = %v, want nil", err)
	}
	got := make(map[string]bool, len(want))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, err := controlplaneIngressSources.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v, want nil", entry.Name(), err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", entry.Name(), err)
		}
		for _, declaration := range file.Decls {
			method, ok := declaration.(*ast.FuncDecl)
			if !ok || method.Recv == nil || method.Name.Name != "UnmarshalJSON" {
				continue
			}
			name := ingressReceiverName(method.Recv.List[0].Type)
			if !want[name] {
				t.Errorf("external JSON decoder %s has no compiler-linked semantic fuzz proof", name)
			}
			got[name] = true
		}
	}
	for name := range want {
		if !got[name] {
			t.Errorf("inventory owner %s has no external JSON decoder", name)
		}
	}
}

func ingressReceiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return ingressReceiverName(value.X)
	case *ast.IndexExpr:
		return ingressReceiverName(value.X)
	case *ast.IndexListExpr:
		return ingressReceiverName(value.X)
	default:
		return ""
	}
}

func TestIngressReceiverMatcherCoversPointerAndGenericSyntax(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"value receiver", "Document", "Document"},
		{"pointer receiver", "*Document", "Document"},
		{"one generic parameter", "Document[Body]", "Document"},
		{"two generic parameters", "*Document[Body, BodyPtr]", "Document"},
		{"non-receiver syntax", "[]byte", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expression, err := parser.ParseExpr(tc.source)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v, want nil", err)
			}
			if got := ingressReceiverName(expression); got != tc.want {
				t.Fatalf("receiver name = %q, want %q", got, tc.want)
			}
		})
	}
}
