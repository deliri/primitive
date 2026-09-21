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
		{owner: reflect.TypeFor[controlplane.SigningDomain](), fuzz: FuzzSigningDomainExternalDecoders},
		{owner: reflect.TypeFor[controlplane.ProductStatus](), fuzz: FuzzProductStatusExternalDecoders},
		{owner: reflect.TypeFor[controlplane.ResponseHeaderField](), fuzz: FuzzResponseHeaderFieldExternalDecoders},
		{owner: reflect.TypeFor[controlplane.UsageDisposition](), fuzz: FuzzUsageDispositionExternalDecoders},
		{owner: reflect.TypeFor[controlplane.UsageClass](), fuzz: FuzzUsageClassExternalDecoders},
		{owner: reflect.TypeFor[controlplane.OutcomeClass](), fuzz: FuzzOutcomeClassExternalDecoders},
		{owner: reflect.TypeFor[controlplane.RegistrationRequest](), fuzz: FuzzRegistrationRequestExternalDecoder},
		{owner: reflect.TypeFor[controlplane.AccessRegistrationRequest](), fuzz: FuzzAccessRegistrationSemanticClosure},
		{owner: reflect.TypeFor[controlplane.InstallationCertificateBody](), fuzz: FuzzInstallationCertificateBodyDecodeAndVerify},
		{owner: reflect.TypeFor[controlplane.InstallationCertificateDocument](), fuzz: FuzzInstallationCertificateDocumentDecodeAndVerify},
		{owner: reflect.TypeFor[controlplane.RegistrationPayload](), fuzz: FuzzRegistrationPayloadExternalDecoder},
		{owner: reflect.TypeFor[controlplane.RegistrationDocument](), fuzz: FuzzRegistrationDocumentDecodeAndVerify},
		{owner: reflect.TypeFor[controlplane.CheckInPayload](), fuzz: FuzzCheckInPayloadExternalDecoder},
		{owner: reflect.TypeFor[controlplane.CheckInRequest](), fuzz: FuzzCheckInRequestDecodeAndVerify},
		{owner: reflect.TypeFor[controlplane.CheckInResponsePayload](), fuzz: FuzzCheckInResponsePayloadExternalDecoder},
		{owner: reflect.TypeFor[controlplane.CheckInResponseDocument](), fuzz: FuzzCheckInResponseDocumentDecodeAndVerify},
		{owner: reflect.TypeFor[controlplane.ResponseHeader](), fuzz: FuzzResponseHeaderExternalDecoder},
		{owner: reflect.TypeFor[controlplane.UsageWatermark](), fuzz: FuzzUsageWatermarkExternalDecoder},
		{owner: reflect.TypeFor[controlplane.UsageWindow](), fuzz: FuzzUsageWindowDecode},
		{owner: reflect.TypeFor[controlplane.ResponseCommitment](), fuzz: FuzzResponseCommitmentExternalDecoder},
		{owner: reflect.TypeFor[controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]](), fuzz: FuzzAuthenticatedResponseExternalSemanticClosure},
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
		{name: "value receiver", source: "Document", want: "Document"},
		{name: "pointer receiver", source: "*Document", want: "Document"},
		{name: "one generic parameter", source: "Document[Body]", want: "Document"},
		{name: "two generic parameters", source: "*Document[Body, BodyPtr]", want: "Document"},
		{name: "non-receiver syntax", source: "[]byte", want: ""},
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
