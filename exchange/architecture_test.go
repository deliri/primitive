package exchange

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type (
	protocolContract[T any]  struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	typedFailure[T any]      struct{}
)

// The marker supplies the referenced Go type. A field's label cannot stand in
// for the contract the compiler actually bound to that field.
type inventoryTypeBinding interface {
	classifiedType() reflect.Type
}

func (protocolContract[T]) classifiedType() reflect.Type  { return reflect.TypeFor[T]() }
func (internalFlow[T]) classifiedType() reflect.Type      { return reflect.TypeFor[T]() }
func (capabilityWrapper[T]) classifiedType() reflect.Type { return reflect.TypeFor[T]() }
func (typedFailure[T]) classifiedType() reflect.Type      { return reflect.TypeFor[T]() }

// inventoryDocument is the caller-supplied document the generic server and
// client contracts are instantiated with throughout the inventory below.
// TestInventoryDocumentDrivesTheRealJSONWritePath proves it is a document the
// real write path accepts rather than a shape that only satisfies the compiler.
type inventoryDocument struct {
	Name string `json:"name"`
}

func (d inventoryDocument) Validate() error {
	if d.Name == "" {
		return core.ErrExchangeContract
	}
	return nil
}

func (d inventoryDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Name string `json:"name"`
	}{Name: d.Name})
}

// TestInventoryDocumentDrivesTheRealJSONWritePath runs the document the
// inventory names through production WriteJSON instead of asserting its methods
// in isolation, so the type the generic contracts are instantiated with is
// proved against the real encoder and the real ResponseWriter framing.
//
// An invalid caller document is refused by two independent gates: the pre-write
// JSONWriteCall.Validate gate and the encoder's own value validation. The
// rejection case below asserts the pre-write gate directly and then asserts the
// end-to-end effect, because removing either gate alone leaves the other
// covering it and would otherwise pass unnoticed.
func TestInventoryDocumentDrivesTheRealJSONWritePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		document   inventoryDocument
		underLimit bool
		wantErr    error
	}{
		{name: "valid nominal document crosses exact encoded ceiling", document: inventoryDocument{Name: "inventory"}},
		{name: "empty owner value cannot release framing", wantErr: core.ErrExchangeContract},
		{name: "escaped content retains Go JSON representation", document: inventoryDocument{Name: "quote\"\\\n"}},
		{name: "encoded size one above budget cannot release partial JSON", document: inventoryDocument{Name: "inventory"}, underLimit: true, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Independent plain Go struct has no production Validate/MarshalJSON method.
			expected, err := json.Marshal(struct {
				Name string `json:"name"`
			}{Name: tc.document.Name})
			if err != nil {
				t.Fatal(err)
			}
			extent := len(expected)
			if tc.underLimit {
				extent--
			}
			maximum, err := core.NewByteCount(uint64(extent))
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			socket, err := NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			call := JSONWriteCall[inventoryDocument]{Call: socket, Response: ServerJSONResponse[inventoryDocument]{Body: tc.document, Status: core.HTTPStatusOK()}, Policy: JSONWritePolicy{ResponseBodyLimit: maximum}}
			validation := call.Validate()
			if tc.document.Name == "" {
				if !errors.Is(validation, core.ErrExchangeContract) {
					t.Fatalf("empty document validation=%v,want contract refusal", validation)
				}
			} else if validation != nil {
				t.Fatal(validation)
			}
			gotErr := WriteJSON(call)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("inventory document error=%v,want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if recorder.Body.Len() != 0 || len(recorder.Header()) != 0 || recorder.Flushed {
					t.Fatalf("refused document emitted %q,%v", recorder.Body.Bytes(), recorder.Header())
				}
				return
			}
			if !bytes.Equal(recorder.Body.Bytes(), expected) || recorder.Code != http.StatusOK || recorder.Header().Get(core.HTTPHeaderContentLength().String()) != strconv.Itoa(len(expected)) || recorder.Header().Get(core.HTTPHeaderContentType().String()) != core.HTTPMediaTypeJSON().String() {
				t.Fatalf("inventory response=(%d,%q,%v),want exact Go encoding and framing %q", recorder.Code, recorder.Body.Bytes(), recorder.Header(), expected)
			}
		})
	}
}

// exchangeContractInventory classifies every production struct by its real
// data-flow role. Membership comes from compiler-bound marker types; field
// names are labels only.
type exchangeContractInventory struct {
	ResponseBufferRequest                     protocolContract[ResponseBufferRequest]
	ResponseBufferResult                      protocolContract[ResponseBufferResult]
	responseBuffer                            capabilityWrapper[responseBuffer]
	StatusError                               typedFailure[StatusError]
	RetryExhaustedError                       typedFailure[RetryExhaustedError]
	ServerErrorResponse                       protocolContract[ServerErrorResponse]
	ServerRedirectResponse                    protocolContract[ServerRedirectResponse]
	observedStandardResponseWriter            capabilityWrapper[observedStandardResponseWriter]
	BasicAuthorizationRequest                 protocolContract[BasicAuthorizationRequest]
	BearerAuthorization                       protocolContract[BearerAuthorization]
	OfficialSDKResponseBoundary               protocolContract[OfficialSDKResponseBoundary]
	OfficialSDKResponseBoundaryRequest        protocolContract[OfficialSDKResponseBoundaryRequest]
	OfficialSDKResponseCeilingRequest         protocolContract[OfficialSDKResponseCeilingRequest]
	OfficialSDKStreamingSuccessCeilingRequest protocolContract[OfficialSDKStreamingSuccessCeilingRequest]
	OfficialSDKResponseTransportRequest       protocolContract[OfficialSDKResponseTransportRequest]

	replayFact            internalFlow[replayFact]
	redirectFact          internalFlow[redirectFact]
	requestMetadata       internalFlow[requestMetadata]
	forwardedMemberCursor internalFlow[forwardedMemberCursor]

	RedirectPolicy         protocolContract[RedirectPolicy]
	IdempotencyKey         protocolContract[IdempotencyKey]
	HeaderValue            protocolContract[HeaderValue]
	Header                 protocolContract[Header]
	Headers                protocolContract[Headers]
	HeaderSelection        protocolContract[HeaderSelection]
	CapturedHeaders        protocolContract[CapturedHeaders]
	TrustedProxyPrefixes   protocolContract[TrustedProxyPrefixes]
	ClientAddressRequest   protocolContract[ClientAddressRequest]
	ClientAddress          protocolContract[ClientAddress]
	RequestSemantics       protocolContract[RequestSemantics]
	RetryPolicy            protocolContract[RetryPolicy]
	OperationPolicy        protocolContract[OperationPolicy]
	JSONPolicy             protocolContract[JSONPolicy]
	NoBodyJSONPolicy       protocolContract[NoBodyJSONPolicy]
	NoBodyBoundedPolicy    protocolContract[NoBodyBoundedPolicy]
	BoundedPolicy          protocolContract[BoundedPolicy]
	StreamPolicy           protocolContract[StreamPolicy]
	StreamReplayPolicy     protocolContract[StreamReplayPolicy]
	JSONRequest            protocolContract[JSONRequest[inventoryDocument]]
	NoBodyRequest          protocolContract[NoBodyRequest]
	NoBodyBoundedRequest   protocolContract[NoBodyBoundedRequest]
	BoundedRequest         protocolContract[BoundedRequest]
	UploadRequest          protocolContract[UploadRequest]
	DownloadRequest        protocolContract[DownloadRequest]
	StreamRoundTripRequest protocolContract[StreamRoundTripRequest]

	Client               capabilityWrapper[Client]
	SessionClientRequest protocolContract[SessionClientRequest]
	JSONCall             protocolContract[JSONCall[inventoryDocument]]
	NoBodyJSONCall       protocolContract[NoBodyJSONCall]
	BoundedCall          protocolContract[BoundedCall]
	NoBodyBoundedCall    protocolContract[NoBodyBoundedCall]

	aggregateRequest               internalFlow[aggregateRequest]
	aggregateCall                  internalFlow[aggregateCall]
	aggregateResponse              internalFlow[aggregateResponse]
	attemptResponse                internalFlow[attemptResponse]
	retryProgress                  internalFlow[retryProgress]
	aggregateAttempt               internalFlow[aggregateAttempt]
	aggregateReadRequest           internalFlow[aggregateReadRequest]
	aggregateAttemptResult         internalFlow[aggregateAttemptResult]
	retryWaitRequest               internalFlow[retryWaitRequest]
	redirectCheckRequest           internalFlow[redirectCheckRequest]
	officialSDKResponseTransport   internalFlow[officialSDKResponseTransport]
	officialSDKResponseReadRequest internalFlow[officialSDKResponseReadRequest]

	UploadCall              protocolContract[UploadCall]
	DownloadCall            protocolContract[DownloadCall]
	StreamRoundTripCall     protocolContract[StreamRoundTripCall]
	StreamReplayCall        protocolContract[StreamReplayCall]
	uploadHTTPRequest       internalFlow[uploadHTTPRequest]
	downloadHTTPRequest     internalFlow[downloadHTTPRequest]
	uploadResponseRequest   internalFlow[uploadResponseRequest]
	downloadResponseRequest internalFlow[downloadResponseRequest]
	streamDrainRequest      internalFlow[streamDrainRequest]
	boundedBodyRead         internalFlow[boundedBodyRead]
	boundedBodyDestination  internalFlow[boundedBodyDestination]
	downloadCopyRequest     internalFlow[downloadCopyRequest]
	progressReader          internalFlow[progressReader]
	observedStreamWriter    capabilityWrapper[observedStreamWriter]
	streamTransportFailure  internalFlow[streamTransportFailure]
	declaredBodyLength      internalFlow[declaredBodyLength]
	httpContentCoding       internalFlow[httpContentCoding]

	RouteSemantics           protocolContract[RouteSemantics]
	ServerPolicy             protocolContract[ServerPolicy]
	JSONWritePolicy          protocolContract[JSONWritePolicy]
	NoBody                   protocolContract[NoBody]
	Received                 protocolContract[Received[*inventoryDocument]]
	JSONReceiveCall          protocolContract[JSONReceiveCall]
	ProjectedJSONReceiveCall protocolContract[ProjectedJSONReceiveCall[inventoryDocument, *inventoryDocument]]
	NoBodyReceiveCall        protocolContract[NoBodyReceiveCall]
	projectionRequest        internalFlow[projectionRequest[inventoryDocument, *inventoryDocument]]
	ResponseHeaders          protocolContract[ResponseHeaders]
	ServerJSONResponse       protocolContract[ServerJSONResponse[inventoryDocument]]
	ServerNoBodyResponse     protocolContract[ServerNoBodyResponse]
	JSONWriteCall            protocolContract[JSONWriteCall[inventoryDocument]]
	NoBodyWriteCall          protocolContract[NoBodyWriteCall]
	jsonWriteRequest         internalFlow[jsonWriteRequest]

	ServerBoundedPolicy        protocolContract[ServerBoundedPolicy]
	ServerStreamPolicy         protocolContract[ServerStreamPolicy]
	BoundedReceiveCall         protocolContract[BoundedReceiveCall]
	StreamReceiveCall          protocolContract[StreamReceiveCall]
	ReceivedBytes              protocolContract[ReceivedBytes]
	ReceivedStream             protocolContract[ReceivedStream]
	rawRequestMetadata         internalFlow[rawRequestMetadata]
	ServerStreamResponse       protocolContract[ServerStreamResponse]
	StreamWriteCall            protocolContract[StreamWriteCall]
	ServerBoundedResponse      protocolContract[ServerBoundedResponse]
	BoundedWriteCall           protocolContract[BoundedWriteCall]
	JSONSocketContract         protocolContract[JSONSocketContract]
	SocketRoutePath            protocolContract[SocketRoutePath]
	ClientSocketConfiguration  protocolContract[ClientSocketConfiguration]
	ClientSocket               capabilityWrapper[ClientSocket]
	ServerSocket               capabilityWrapper[ServerSocket]
	SocketServerCall           protocolContract[SocketServerCall]
	ListenAddress              protocolContract[ListenAddress]
	ServerRuntimePolicy        protocolContract[ServerRuntimePolicy]
	ServerRuntimeConfiguration protocolContract[ServerRuntimeConfiguration]
	ServerListener             capabilityWrapper[ServerListener]
	ServerRuntime              capabilityWrapper[ServerRuntime]

	ResponseMetadata        protocolContract[ResponseMetadata]
	JSONResponse            protocolContract[JSONResponse[inventoryDocument]]
	BoundedResponse         protocolContract[BoundedResponse]
	StreamResponse          protocolContract[StreamResponse]
	StreamRoundTripResponse protocolContract[StreamRoundTripResponse]
}

func TestExchangeDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := productionStructNames(t)
	want := classifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Exchange production structs = %q, want classified %q", got, want)
	}
}

func TestSocketServerHasOnePublicAdmissionAndWriteDoor(t *testing.T) {
	t.Parallel()
	fileSet := token.NewFileSet()
	var file *ast.File
	for _, source := range exchangeArchitectureSources(t, fileSet) {
		if source.name == "socket.go" {
			file = source.syntax
		}
	}
	if file == nil {
		t.Fatalf("socket syntax=%v, want parsed socket.go", file)
	}
	got := make([]string, 0, 3)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || !function.Name.IsExported() {
			continue
		}
		name := function.Name.Name
		if strings.HasPrefix(name, "Receive") && strings.Contains(name, "SocketJSON") || strings.HasPrefix(name, "Write") && strings.Contains(name, "SocketJSON") {
			got = append(got, name)
		}
	}
	sort.Strings(got)
	want := []string{"ReceiveReplayBoundSocketJSON", "ReceiveSocketJSON", "WriteSocketJSON"}
	if !slices.Equal(got, want) {
		t.Fatalf("exported socket server JSON doors = %q, want %q", got, want)
	}
}

func TestRawHTTPAdmissionIsSealedSocketOrGoSDKTransport(t *testing.T) {
	t.Parallel()
	sources := exchangeArchitectureSources(t, token.NewFileSet())
	got := publicRawHTTPExposures(sources)
	// One Go request/writer admission seals their custody in SocketServerCall.
	// The SDK adapter must implement Go's http.RoundTripper signature; its
	// typed constructor is its admission boundary. Both exceptions are compiler
	// bound, and this fixed two-entry list cannot grow without changing the test.
	// Public mutable variables are forbidden too: they could replace a checked
	// function with an unclassified function-valued ingress after initialization.
	want := []string{IngressSymbolForTest(NewSocketServerCall), IngressSymbolForTest(officialSDKResponseTransport.RoundTrip)}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("public raw HTTP or mutable-global exposure=%q,want sealed socket and required Go transport contract %q", got, want)
	}
}

func publicRawHTTPExposures(sources []exchangeArchitectureSource) []string {
	var exposed []string
	bindings := rawHTTPBindings(sources)
	sealed := reflect.TypeFor[SocketServerCall]().Name()
	for _, source := range sources {
		file := source.syntax
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Name.IsExported() && rawHTTPType(file, typed.Type, bindings, rawHTTPTypeScope{}) {
					name := typed.Name.Name
					if typed.Recv != nil {
						receiver := typed.Recv.List[0].Type
						if pointer, ok := receiver.(*ast.StarExpr); ok {
							receiver = pointer.X
						}
						if nominal, ok := receiver.(*ast.Ident); ok {
							name = nominal.Name + "." + name
						} else {
							name = "unresolved receiver." + name
						}
					}
					exposed = append(exposed, name)
				}
			case *ast.GenDecl:
				for _, specification := range typed.Specs {
					switch spec := specification.(type) {
					case *ast.TypeSpec:
						if !spec.Name.IsExported() {
							continue
						}
						scope := (rawHTTPTypeScope{}).withTypeParameters(spec.TypeParams)
						if spec.Name.Name == sealed {
							structure, ok := spec.Type.(*ast.StructType)
							if !ok {
								exposed = append(exposed, spec.Name.Name)
								continue
							}
							for _, field := range structure.Fields.List {
								public := len(field.Names) == 0
								for _, name := range field.Names {
									public = public || name.IsExported()
								}
								if public && rawHTTPType(file, field.Type, bindings, scope) {
									exposed = append(exposed, spec.Name.Name)
								}
							}
							continue
						}
						if rawHTTPType(file, spec.Type, bindings, scope) {
							exposed = append(exposed, spec.Name.Name)
						}
					case *ast.ValueSpec:
						if typed.Tok != token.VAR {
							continue
						}
						for _, name := range spec.Names {
							if name.IsExported() {
								exposed = append(exposed, name.Name)
							}
						}
					}
				}
			}
		}
	}
	slices.Sort(exposed)
	return exposed
}

func TestPublicRawHTTPScanCannotLoseDeclarationShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         []string
	}{
		{name: "public function admits raw request", source: `import "net/http";func Door(*http.Request){}`, want: []string{"Door"}},
		{name: "public method cannot hide raw request", source: `import "net/http";type T struct{};func (T) Door(*http.Request){}`, want: []string{"T.Door"}},
		{name: "method of private receiver cannot hide raw request", source: `import "net/http";type private struct{};func (private) Door(*http.Request){}`, want: []string{"private.Door"}},
		{name: "exported alias cannot export raw custody", source: `import "net/http";type Door = *http.Request`, want: []string{"Door"}},
		{name: "defined request cannot evade alias scan", source: `import "net/http";type Door http.Request`, want: []string{"Door"}},
		{name: "exported interface admits raw writer", source: `import "net/http";type Door interface{Write(http.ResponseWriter)}`, want: []string{"Door"}},
		{name: "declared function variable cannot add unclassified ingress", source: `import "net/http";var Door func(*http.Request)`, want: []string{"Door"}},
		{name: "inferred function variable cannot add unclassified ingress", source: `import "net/http";var Door = http.NewRequest`, want: []string{"Door"}},
		{name: "closure initializer cannot hide function ingress", source: `import "net/http";var Door = func(*http.Request){}`, want: []string{"Door"}},
		{name: "mutable global lacks stable compiler-owned admission", source: `var Door = 1`, want: []string{"Door"}},
		{name: "sealed socket cannot grow exported raw field", source: `import "net/http";type SocketServerCall struct{request *http.Request;Writer http.ResponseWriter}`, want: []string{"SocketServerCall"}},
		{name: "sealed socket cannot grow embedded raw capability", source: `import "net/http";type SocketServerCall struct{*http.Request}`, want: []string{"SocketServerCall"}},
		{name: "sealed socket retains private Go handles", source: `import "net/http";type SocketServerCall struct{request *http.Request;writer http.ResponseWriter}`},
		{name: "private raw adapter remains implementation", source: `import "net/http";func private(*http.Request){}`},
		{name: "typed constant does not introduce mutable ingress", source: `const Door = 1`},
		{name: "local fixture name cannot impersonate Go request", source: `type Request struct{};func Door(*Request){}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package exchange;"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got := publicRawHTTPExposures([]exchangeArchitectureSource{{name: "fixture.go", syntax: file}})
			if !slices.Equal(got, tc.want) {
				t.Fatalf("raw/public exposure=%q,want %q", got, tc.want)
			}
		})
	}
}

func productionStructNames(t *testing.T) []string {
	t.Helper()

	names := make([]string, 0)
	fileSet := token.NewFileSet()
	for _, source := range exchangeArchitectureSources(t, fileSet) {
		file := source.syntax
		ast.Inspect(file, func(node ast.Node) bool {
			specification, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := specification.Type.(*ast.StructType); ok {
				names = append(names, specification.Name.Name)
			}
			return true
		})
	}
	sort.Strings(names)
	return names
}

func classifiedStructNames(t *testing.T) []string {
	t.Helper()
	inventory := exchangeContractInventory{
		observedStandardResponseWriter: capabilityWrapper[observedStandardResponseWriter]{},
		observedStreamWriter:           capabilityWrapper[observedStreamWriter]{},
	}
	got, err := boundInventoryStructNames(reflect.TypeOf(inventory))
	if err != nil {
		t.Fatalf("bound inventory error = %v, want nil", err)
	}
	return got
}

func boundInventoryStructNames(inventory reflect.Type) ([]string, error) {
	if inventory == nil || inventory.Kind() != reflect.Struct {
		return nil, core.ErrExchangeContract
	}
	names := make([]string, 0, inventory.NumField())
	for field := range inventory.Fields() {
		binding, ok := reflect.Zero(field.Type).Interface().(inventoryTypeBinding)
		if !ok {
			return nil, core.ErrExchangeContract
		}
		bound := binding.classifiedType()
		if bound.Kind() != reflect.Struct || bound.PkgPath() != reflect.TypeFor[Client]().PkgPath() || bound.Name() == "" {
			return nil, core.ErrExchangeContract
		}
		name, _, _ := strings.Cut(bound.Name(), "[")
		names = append(names, name)
	}
	slices.Sort(names)
	for index := 1; index < len(names); index++ {
		if names[index] == names[index-1] {
			return nil, core.ErrExchangeContract
		}
	}
	return names, nil
}

func TestInventoryBindingUsesCompilerTypesTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   reflect.Type
		want    []string
		wantErr error
	}{
		{name: "misleading field label cannot replace referenced protocol type", input: reflect.TypeFor[struct {
			Header protocolContract[StreamResponse]
		}](), want: []string{"StreamResponse"}},
		{name: "instantiated generic binds the actual generic declaration", input: reflect.TypeFor[struct {
			Misleading protocolContract[JSONRequest[inventoryDocument]]
		}](), want: []string{"JSONRequest"}},
		{name: "capability role retains its compiler-bound type", input: reflect.TypeFor[struct{ Misleading capabilityWrapper[Client] }](), want: []string{"Client"}},
		{name: "internal flow role retains its compiler-bound type", input: reflect.TypeFor[struct{ Misleading internalFlow[retryProgress] }](), want: []string{"retryProgress"}},
		{name: "typed failure role retains its compiler-bound type", input: reflect.TypeFor[struct{ Misleading typedFailure[StatusError] }](), want: []string{"StatusError"}},
		{name: "distinct bindings are ordered independently of field labels", input: reflect.TypeFor[struct {
			A internalFlow[retryProgress]
			Z capabilityWrapper[Client]
		}](), want: []string{"Client", "retryProgress"}},
		{name: "duplicate referenced type cannot hide behind distinct labels", input: reflect.TypeFor[struct {
			A protocolContract[Header]
			B protocolContract[Header]
		}](), wantErr: core.ErrExchangeContract},
		{name: "plain field without classification cannot enter inventory", input: reflect.TypeFor[struct{ A Header }](), wantErr: core.ErrExchangeContract},
		{name: "foreign package struct cannot substitute for local contract", input: reflect.TypeFor[struct {
			A protocolContract[http.Response]
		}](), wantErr: core.ErrExchangeContract},
		{name: "pointer binding cannot classify a struct declaration", input: reflect.TypeFor[struct{ A capabilityWrapper[*Client] }](), wantErr: core.ErrExchangeContract},
		{name: "anonymous shape cannot substitute for named contract", input: reflect.TypeFor[struct{ A protocolContract[struct{}] }](), wantErr: core.ErrExchangeContract},
		{name: "nonstruct input cannot masquerade as inventory", input: reflect.TypeFor[int](), wantErr: core.ErrExchangeContract},
		{name: "missing inventory type is refused", wantErr: core.ErrExchangeContract},
		{name: "empty inventory invents no classified structs", input: reflect.TypeFor[struct{}](), want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := boundInventoryStructNames(tc.input)
			if !errors.Is(gotErr, tc.wantErr) || !slices.Equal(got, tc.want) {
				t.Fatalf("bound inventory = %q/%v, want %q/%v", got, gotErr, tc.want, tc.wantErr)
			}
			if tc.wantErr != nil && got != nil {
				t.Fatalf("refused inventory = %q, want nil", got)
			}
		})
	}
}

var (
	_ core.Validatable            = inventoryDocument{}
	_ core.ValidatedJSONMarshaler = inventoryDocument{}
	_                             = exchangeContractInventory{}.replayFact
	_                             = exchangeContractInventory{}.redirectFact
	_                             = exchangeContractInventory{}.requestMetadata
	_                             = exchangeContractInventory{}.forwardedMemberCursor
	_                             = exchangeContractInventory{}.declaredBodyLength
	_                             = exchangeContractInventory{}.httpContentCoding
	_                             = exchangeContractInventory{}.aggregateRequest
	_                             = exchangeContractInventory{}.aggregateCall
	_                             = exchangeContractInventory{}.aggregateResponse
	_                             = exchangeContractInventory{}.attemptResponse
	_                             = exchangeContractInventory{}.retryProgress
	_                             = exchangeContractInventory{}.aggregateAttempt
	_                             = exchangeContractInventory{}.aggregateReadRequest
	_                             = exchangeContractInventory{}.aggregateAttemptResult
	_                             = exchangeContractInventory{}.retryWaitRequest
	_                             = exchangeContractInventory{}.redirectCheckRequest
	_                             = exchangeContractInventory{}.officialSDKResponseTransport
	_                             = exchangeContractInventory{}.officialSDKResponseReadRequest
	_                             = exchangeContractInventory{}.uploadHTTPRequest
	_                             = exchangeContractInventory{}.downloadHTTPRequest
	_                             = exchangeContractInventory{}.uploadResponseRequest
	_                             = exchangeContractInventory{}.downloadResponseRequest
	_                             = exchangeContractInventory{}.streamDrainRequest
	_                             = exchangeContractInventory{}.boundedBodyRead
	_                             = exchangeContractInventory{}.boundedBodyDestination
	_                             = exchangeContractInventory{}.downloadCopyRequest
	_                             = exchangeContractInventory{}.progressReader
	_                             = exchangeContractInventory{}.streamTransportFailure
	_                             = exchangeContractInventory{}.projectionRequest
	_                             = exchangeContractInventory{}.jsonWriteRequest
	_                             = exchangeContractInventory{}.rawRequestMetadata
)

var _ = exchangeContractInventory{}.responseBuffer

type exchangeArchitectureSource struct {
	name   string
	syntax *ast.File
}

// Setup reads the source through Filestore before handing bytes to Go's AST
// parser. Parsing needs the whole source; the per-file extent remains bounded.
func exchangeArchitectureSources(t testing.TB, fileSet *token.FileSet) []exchangeArchitectureSource {
	t.Helper()
	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("source directory setup error = %v, want nil", err)
	}
	absolute, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatalf("source directory admission error = %v, want nil", err)
	}
	root, err := filestore.OpenRoot(t.Context(), absolute)
	if err != nil {
		t.Fatalf("source root admission error = %v, want nil", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Errorf("source root close error = %v, want nil", err)
		}
	}()
	path, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("source root path admission error = %v, want nil", err)
	}
	var sources []exchangeArchitectureSource
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: path}, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				return filestore.WalkSkipDirectory, nil
			}
			name := entry.Path.String()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return filestore.WalkContinue, nil
			}
			var data bytes.Buffer
			_, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: filestore.Location{Root: root, Path: entry.Path}, Destination: &data})
			if err != nil {
				return filestore.WalkContinue, err
			}
			syntax, err := parser.ParseFile(fileSet, name, data.Bytes(), parser.SkipObjectResolution)
			if err != nil {
				return filestore.WalkContinue, err
			}
			sources = append(sources, exchangeArchitectureSource{name: name, syntax: syntax})
			return filestore.WalkContinue, nil
		},
	})
	if err != nil {
		t.Fatalf("source traversal/parse error = %v, want nil", err)
	}
	return sources
}
