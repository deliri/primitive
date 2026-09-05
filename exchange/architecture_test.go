package exchange

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type (
	protocolContract[T any]  struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	typedFailure[T any]      struct{}
)

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

	limit, err := core.NewByteCount(1 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	status := core.HTTPStatusOK()
	policy := JSONWritePolicy{ResponseBodyLimit: limit}

	t.Run("valid document is encoded and framed", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		serverCall := SocketServerCall{writer: recorder, request: httptest.NewRequest(http.MethodGet, "/", nil)}
		writeErr := WriteJSON(JSONWriteCall[inventoryDocument]{
			Call: serverCall,
			Response: ServerJSONResponse[inventoryDocument]{
				Body:   inventoryDocument{Name: "inventory"},
				Status: status,
			},
			Policy: policy,
		})
		if writeErr != nil {
			t.Fatalf("WriteJSON() error = %v, want nil", writeErr)
		}
		wantBody := `{"name":"inventory"}`
		if got := recorder.Body.String(); got != wantBody {
			t.Fatalf("WriteJSON() body = %q, want %q", got, wantBody)
		}
		if got := recorder.Code; got != 200 {
			t.Fatalf("WriteJSON() status = %d, want 200", got)
		}
		if got := recorder.Header().Get("Content-Length"); got != strconv.Itoa(len(wantBody)) {
			t.Fatalf("WriteJSON() Content-Length = %q, want %d", got, len(wantBody))
		}
	})

	t.Run("invalid document writes no response", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		serverCall := SocketServerCall{writer: recorder, request: httptest.NewRequest(http.MethodGet, "/", nil)}
		call := JSONWriteCall[inventoryDocument]{
			Call: serverCall,
			Response: ServerJSONResponse[inventoryDocument]{
				Body:   inventoryDocument{},
				Status: status,
			},
			Policy: policy,
		}
		if gotErr := call.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("JSONWriteCall.Validate() error = %v, want %v", gotErr, core.ErrExchangeContract)
		}
		writeErr := WriteJSON(call)
		if !errors.Is(writeErr, core.ErrExchangeContract) {
			t.Fatalf("WriteJSON(invalid document) error = %v, want %v", writeErr, core.ErrExchangeContract)
		}
		if got := recorder.Body.Len(); got != 0 {
			t.Fatalf("WriteJSON(invalid document) wrote %d body bytes, want 0", got)
		}
	})
}

// exchangeContractInventory classifies every production struct by its real
// data-flow role. Field names deliberately equal the classified type names.
type exchangeContractInventory struct {
	ResponseBufferRequest                     protocolContract[ResponseBufferRequest]
	ResponseBufferResult                      protocolContract[ResponseBufferResult]
	responseBuffer                            capabilityWrapper[responseBuffer]
	StatusError                               typedFailure[StatusError]
	RetryExhaustedError                       typedFailure[RetryExhaustedError]
	ServerErrorResponse                       protocolContract[ServerErrorResponse]
	ServerRedirectResponse                    protocolContract[ServerRedirectResponse]
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

	Client            capabilityWrapper[Client]
	JSONCall          protocolContract[JSONCall[inventoryDocument]]
	NoBodyJSONCall    protocolContract[NoBodyJSONCall]
	BoundedCall       protocolContract[BoundedCall]
	NoBodyBoundedCall protocolContract[NoBodyBoundedCall]

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
	file, err := parser.ParseFile(fileSet, "socket.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parser.ParseFile(socket.go) error = %v, want nil", err)
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

func TestSocketServerCallIsOnlyPublicRawHTTPAdmission(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v, want nil", err)
	}
	gotFunctions := make([]string, 0, 1)
	gotFields := make([]string, 0)
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileSet, filepath.Clean(entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Recv == nil && typed.Name.IsExported() && rawHTTPType(fileSet, typed.Type) {
					gotFunctions = append(gotFunctions, typed.Name.Name)
				}
			case *ast.GenDecl:
				for _, specification := range typed.Specs {
					typeSpecification, ok := specification.(*ast.TypeSpec)
					if !ok || !typeSpecification.Name.IsExported() {
						continue
					}
					structure, ok := typeSpecification.Type.(*ast.StructType)
					if !ok {
						continue
					}
					if typeSpecification.Name.Name == "SocketServerCall" {
						continue
					}
					for _, field := range structure.Fields.List {
						if rawHTTPType(fileSet, field.Type) {
							gotFields = append(gotFields, typeSpecification.Name.Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(gotFunctions)
	sort.Strings(gotFields)
	wantFunctions := []string{"NewSocketServerCall"}
	if !slices.Equal(gotFunctions, wantFunctions) || len(gotFields) != 0 {
		t.Fatalf("public raw HTTP functions/struct fields = (%q, %q), want (%q, none)", gotFunctions, gotFields, wantFunctions)
	}
}

func rawHTTPType(fileSet *token.FileSet, expression ast.Expr) bool {
	var encoded bytes.Buffer
	if err := format.Node(&encoded, fileSet, expression); err != nil {
		return false
	}
	value := encoded.String()
	return strings.Contains(value, "*http.Request") || strings.Contains(value, "http.ResponseWriter")
}

func productionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Clean(entry.Name()),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
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

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(
		fileSet,
		"architecture_test.go",
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parser.ParseFile(architecture_test.go) error = %v, want nil", err)
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			specification := raw.(*ast.TypeSpec)
			if specification.Name.Name != "exchangeContractInventory" {
				continue
			}
			structure := specification.Type.(*ast.StructType)
			names := make([]string, 0, len(structure.Fields.List))
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			sort.Strings(names)
			return names
		}
	}
	t.Fatal("exchangeContractInventory declarations found = 0, want 1")
	return nil
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
