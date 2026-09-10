package exchange

import (
	json "encoding/json/v2"
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// These exported test-only contracts connect the internal and external test
// packages. Every door and target is an actual Go function value, not a name
// standing in for a compiler binding.
type IngressCoverageKindForTest uint8

const (
	IngressFuzzForTest IngressCoverageKindForTest = iota + 1
	IngressCapabilityForTest
	IngressProjectionForTest
)

type IngressCoverageForTest struct {
	Door   any
	Kind   IngressCoverageKindForTest
	Fuzz   func(*testing.F)
	Proof  func(*testing.T)
	Reason string
}
type IngressDeclarationForTest struct {
	Symbol, File string
	Line         int
}

func IngressSymbolForTest(function any) string {
	value := reflect.ValueOf(function)
	if !value.IsValid() || value.Kind() != reflect.Func || value.IsNil() {
		return ""
	}
	bound := runtime.FuncForPC(value.Pointer())
	if bound == nil {
		return ""
	}
	// Runtime generic names include type arguments containing dots and slashes.
	// Remove balanced instantiations before separating the package and receiver.
	var name strings.Builder
	depth := 0
	for _, r := range bound.Name() {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		default:
			if depth == 0 {
				name.WriteRune(r)
			}
		}
	}
	symbol, owned := strings.CutPrefix(name.String(), reflect.TypeFor[SocketServerCall]().PkgPath()+".")
	if !owned {
		return ""
	}
	return strings.NewReplacer("(*", "", "(", "", ")", "").Replace(symbol)
}

func ExchangeIngressDeclarationsForTest(t testing.TB) []IngressDeclarationForTest {
	t.Helper()
	fset := token.NewFileSet()
	var declarations []IngressDeclarationForTest
	for _, source := range exchangeArchitectureSources(t, fset) {
		for _, declaration := range source.syntax.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() {
				continue
			}
			symbol := fn.Name.Name
			if fn.Recv != nil {
				receiver := fn.Recv.List[0].Type
				for {
					switch typed := receiver.(type) {
					case *ast.StarExpr:
						receiver = typed.X
					case *ast.IndexExpr:
						receiver = typed.X
					case *ast.IndexListExpr:
						receiver = typed.X
					default:
						goto resolved
					}
				}
			resolved:
				name, ok := receiver.(*ast.Ident)
				if !ok || !name.IsExported() {
					continue
				}
				// These owning validation/representation methods are covered at their
				// admitting doors. New methods with different signatures remain visible.
				if owningProjectionMethodForTest(source.syntax, fn) {
					continue
				}
				symbol = name.Name + "." + symbol
			}
			declarations = append(declarations, IngressDeclarationForTest{Symbol: symbol, File: source.name, Line: fset.Position(fn.Pos()).Line})
		}
	}
	return declarations
}

func owningProjectionMethodForTest(file *ast.File, fn *ast.FuncDecl) bool {
	contracts := []reflect.Type{
		reflect.TypeFor[core.Validatable](), reflect.TypeFor[fmt.Stringer](), reflect.TypeFor[json.Marshaler](),
		reflect.TypeFor[interface{ IsValid() bool }](), reflect.TypeFor[interface{ IsZero() bool }](),
		reflect.TypeFor[interface{ Value() (string, error) }](), reflect.TypeFor[fmt.Formatter](),
	}
	for _, contract := range contracts {
		method := contract.Method(0)
		if fn.Name.Name == method.Name && ingressSignatureMatchesForTest(file, fn.Type, method.Type) {
			return true
		}
	}
	return false
}

// This matches only the explicitly exempted Go projection signatures. Unknown
// AST shapes stay in the ingress inventory and therefore require a bound proof.
func ingressSignatureMatchesForTest(file *ast.File, signature *ast.FuncType, contract reflect.Type) bool {
	if signature.TypeParams.NumFields() != 0 || signature.Params.NumFields() != contract.NumIn() || signature.Results.NumFields() != contract.NumOut() {
		return false
	}
	for side, fields := range []*ast.FieldList{signature.Params, signature.Results} {
		if fields == nil {
			continue
		}
		at := 0
		for _, field := range fields.List {
			for range max(1, len(field.Names)) {
				var wanted reflect.Type
				if side == 0 {
					wanted = contract.In(at)
				} else {
					wanted = contract.Out(at)
				}
				if !ingressProjectionTypeForTest(file, field.Type, wanted) {
					return false
				}
				at++
			}
		}
	}
	return true
}

func ingressProjectionTypeForTest(file *ast.File, expression ast.Expr, wanted reflect.Type) bool {
	if wanted.Kind() == reflect.Slice {
		array, ok := expression.(*ast.ArrayType)
		return ok && array.Len == nil && ingressProjectionTypeForTest(file, array.Elt, wanted.Elem())
	}
	if wanted.PkgPath() == "" {
		identifier, ok := expression.(*ast.Ident)
		if !ok {
			return false
		}
		return identifier.Name == wanted.Name() || wanted.Kind() == reflect.Uint8 && identifier.Name == "byte" || wanted.Kind() == reflect.Int32 && identifier.Name == "rune"
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != wanted.Name() {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	for _, imported := range file.Imports {
		importedPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil || importedPath != wanted.PkgPath() {
			continue
		}
		name := path.Base(importedPath)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		if qualifier.Name == name {
			return true
		}
	}
	return false
}

func InternalIngressCoverageForTest() []IngressCoverageForTest {
	return []IngressCoverageForTest{
		{Door: NewOfficialSDKResponseBoundary, Kind: IngressFuzzForTest, Fuzz: FuzzOfficialSDKBoundaryCeilingAndStreamingConfiguration},
		{Door: NewOfficialSDKMethodResponseBoundary, Kind: IngressFuzzForTest, Fuzz: FuzzOfficialSDKBoundaryCeilingAndStreamingConfiguration},
		{Door: NewOfficialSDKStreamingResponseBoundary, Kind: IngressFuzzForTest, Fuzz: FuzzOfficialSDKBoundaryCeilingAndStreamingConfiguration},
		{Door: NewStandardOfficialSDKResponseTransport, Kind: IngressFuzzForTest, Fuzz: FuzzOfficialSDKBoundaryCeilingAndStreamingConfiguration},
		{Door: ParseBasicAuthorizationIdentity, Kind: IngressFuzzForTest, Fuzz: FuzzParseBasicAuthorizationIdentitySemanticClosure},
		{Door: NewBasicAuthorizationHeader, Kind: IngressFuzzForTest, Fuzz: FuzzNewBasicAuthorizationHeaderCustody},
		{Door: ParseIdempotencyKey, Kind: IngressFuzzForTest, Fuzz: FuzzParseIdempotencyKeySemanticClosure},
		{Door: ParseSocketRoutePath, Kind: IngressFuzzForTest, Fuzz: FuzzParseSocketRoutePathSemanticClosure},
		{Door: (*Method).UnmarshalJSON, Kind: IngressFuzzForTest, Fuzz: FuzzMethodJSONSemanticClosure},
		{Door: ParseTrustedProxyPrefixes, Kind: IngressFuzzForTest, Fuzz: FuzzTrustedProxyPrefixesSemanticClosure},
		{Door: ParseListenAddress, Kind: IngressFuzzForTest, Fuzz: FuzzParseListenAddressSemanticClosure},
		{Door: NewServerRuntime, Kind: IngressFuzzForTest, Fuzz: FuzzServerRuntimeConfigurationAdmission},
		{Door: BufferResponse, Kind: IngressFuzzForTest, Fuzz: FuzzResponseBufferSemanticExtent},
		{Door: Upload, Kind: IngressFuzzForTest, Fuzz: FuzzUploadResponseDeclarationCustody},
		{Door: SendBounded, Kind: IngressFuzzForTest, Fuzz: FuzzBoundedClientsPreserveCompletedProducerCause},
		{Door: Client.WithoutProxy, Kind: IngressCapabilityForTest, Proof: TestClientWithoutProxyLayerTriad, Reason: "Admits a Go client/transport capability; no external representation decoding."},
		{Door: NewSessionClient, Kind: IngressFuzzForTest, Fuzz: FuzzSessionClientGoCookieCustody},
		{Door: Error, Kind: IngressFuzzForTest, Fuzz: FuzzErrorNotFoundRedirectAndSetCookieGoParity},
		{Door: NotFound, Kind: IngressFuzzForTest, Fuzz: FuzzErrorNotFoundRedirectAndSetCookieGoParity},
		{Door: Redirect, Kind: IngressFuzzForTest, Fuzz: FuzzErrorNotFoundRedirectAndSetCookieGoParity},
		{Door: SetCookie, Kind: IngressFuzzForTest, Fuzz: FuzzErrorNotFoundRedirectAndSetCookieGoParity},
		{Door: WriteJSON[admissionJSONDocument], Kind: IngressFuzzForTest, Fuzz: FuzzWriteJSONNoBodyBoundedStreamAndSocketCustody},
		{Door: WriteNoBody, Kind: IngressFuzzForTest, Fuzz: FuzzWriteJSONNoBodyBoundedStreamAndSocketCustody},
		{Door: WriteBounded, Kind: IngressFuzzForTest, Fuzz: FuzzWriteJSONNoBodyBoundedStreamAndSocketCustody},
		{Door: WriteStream, Kind: IngressFuzzForTest, Fuzz: FuzzWriteJSONNoBodyBoundedStreamAndSocketCustody},
		{Door: WriteSocketJSON[admissionJSONDocument], Kind: IngressFuzzForTest, Fuzz: FuzzWriteJSONNoBodyBoundedStreamAndSocketCustody},
		{Door: Listen, Kind: IngressCapabilityForTest, Proof: TestServerListenerConfigurationAgreementTable, Reason: "Go acquires a listener from validated typed configuration; address representation is fuzzed at ParseListenAddress."},
		{Door: (*ServerListener).Address, Kind: IngressCapabilityForTest, Proof: TestServerListenerConfigurationAgreementTable, Reason: "Observes the OS-assigned address of the owned Go listener."},
		{Door: (*ServerListener).Close, Kind: IngressCapabilityForTest, Proof: TestServerListenerConfigurationAgreementTable, Reason: "Closes the owned Go listener; no external document ingestion."},
	}
}
