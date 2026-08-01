package exchange

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
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

type inventoryDocument struct{}

func (inventoryDocument) Validate() error { return nil }

func (inventoryDocument) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{}{})
}

// exchangeContractInventory classifies every production struct by its real
// data-flow role. Field names deliberately equal the classified type names.
type exchangeContractInventory struct {
	StatusError         typedFailure[StatusError]
	RetryExhaustedError typedFailure[RetryExhaustedError]

	replayFact      internalFlow[replayFact]
	redirectFact    internalFlow[redirectFact]
	requestMetadata internalFlow[requestMetadata]

	APIRequestID protocolContract[APIRequestID]
	APIErrorBody protocolContract[APIErrorBody]
	APINoBody    protocolContract[APINoBody]
	APIEnvelope  protocolContract[APIEnvelope[inventoryDocument]]

	RedirectPolicy       protocolContract[RedirectPolicy]
	IdempotencyKey       protocolContract[IdempotencyKey]
	Header               protocolContract[Header]
	Headers              protocolContract[Headers]
	HeaderSelection      protocolContract[HeaderSelection]
	CapturedHeaders      protocolContract[CapturedHeaders]
	RequestSemantics     protocolContract[RequestSemantics]
	RetryPolicy          protocolContract[RetryPolicy]
	OperationPolicy      protocolContract[OperationPolicy]
	JSONPolicy           protocolContract[JSONPolicy]
	NoBodyJSONPolicy     protocolContract[NoBodyJSONPolicy]
	NoBodyBoundedPolicy  protocolContract[NoBodyBoundedPolicy]
	BoundedPolicy        protocolContract[BoundedPolicy]
	StreamPolicy         protocolContract[StreamPolicy]
	StreamReplayPolicy   protocolContract[StreamReplayPolicy]
	JSONRequest          protocolContract[JSONRequest[inventoryDocument]]
	NoBodyRequest        protocolContract[NoBodyRequest]
	NoBodyBoundedRequest protocolContract[NoBodyBoundedRequest]
	BoundedRequest       protocolContract[BoundedRequest]
	UploadRequest        protocolContract[UploadRequest]
	DownloadRequest      protocolContract[DownloadRequest]

	Client            capabilityWrapper[Client]
	JSONCall          protocolContract[JSONCall[inventoryDocument]]
	NoBodyJSONCall    protocolContract[NoBodyJSONCall]
	BoundedCall       protocolContract[BoundedCall]
	NoBodyBoundedCall protocolContract[NoBodyBoundedCall]

	aggregateRequest       internalFlow[aggregateRequest]
	aggregateCall          internalFlow[aggregateCall]
	aggregateResponse      internalFlow[aggregateResponse]
	attemptResponse        internalFlow[attemptResponse]
	retryProgress          internalFlow[retryProgress]
	aggregateAttempt       internalFlow[aggregateAttempt]
	aggregateReadRequest   internalFlow[aggregateReadRequest]
	aggregateAttemptResult internalFlow[aggregateAttemptResult]
	retryWaitRequest       internalFlow[retryWaitRequest]
	redirectCheckRequest   internalFlow[redirectCheckRequest]

	UploadCall              protocolContract[UploadCall]
	DownloadCall            protocolContract[DownloadCall]
	StreamReplayCall        protocolContract[StreamReplayCall]
	uploadHTTPRequest       internalFlow[uploadHTTPRequest]
	downloadHTTPRequest     internalFlow[downloadHTTPRequest]
	uploadResponseRequest   internalFlow[uploadResponseRequest]
	downloadResponseRequest internalFlow[downloadResponseRequest]
	streamDrainRequest      internalFlow[streamDrainRequest]
	boundedBodyRead         internalFlow[boundedBodyRead]
	boundedBodyDestination  internalFlow[boundedBodyDestination]
	downloadCopyRequest     internalFlow[downloadCopyRequest]
	streamTransportFailure  internalFlow[streamTransportFailure]

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

	ServerBoundedPolicy   protocolContract[ServerBoundedPolicy]
	ServerStreamPolicy    protocolContract[ServerStreamPolicy]
	BoundedReceiveCall    protocolContract[BoundedReceiveCall]
	StreamReceiveCall     protocolContract[StreamReceiveCall]
	ReceivedBytes         protocolContract[ReceivedBytes]
	ReceivedStream        protocolContract[ReceivedStream]
	rawRequestMetadata    internalFlow[rawRequestMetadata]
	ServerStreamResponse  protocolContract[ServerStreamResponse]
	StreamWriteCall       protocolContract[StreamWriteCall]
	ServerBoundedResponse protocolContract[ServerBoundedResponse]
	BoundedWriteCall      protocolContract[BoundedWriteCall]

	ResponseMetadata protocolContract[ResponseMetadata]
	JSONResponse     protocolContract[JSONResponse[inventoryDocument]]
	BoundedResponse  protocolContract[BoundedResponse]
	StreamResponse   protocolContract[StreamResponse]
}

func TestExchangeDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := productionStructNames(t)
	want := classifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Exchange production structs = %q, want classified %q", got, want)
	}
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
	_                             = exchangeContractInventory{}.uploadHTTPRequest
	_                             = exchangeContractInventory{}.downloadHTTPRequest
	_                             = exchangeContractInventory{}.uploadResponseRequest
	_                             = exchangeContractInventory{}.downloadResponseRequest
	_                             = exchangeContractInventory{}.streamDrainRequest
	_                             = exchangeContractInventory{}.boundedBodyRead
	_                             = exchangeContractInventory{}.boundedBodyDestination
	_                             = exchangeContractInventory{}.downloadCopyRequest
	_                             = exchangeContractInventory{}.streamTransportFailure
	_                             = exchangeContractInventory{}.projectionRequest
	_                             = exchangeContractInventory{}.jsonWriteRequest
	_                             = exchangeContractInventory{}.rawRequestMetadata
)
