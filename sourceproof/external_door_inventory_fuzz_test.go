package sourceproof_test

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
	"github.com/deliri/primitive/v2026/sourceproof"
)

type proofJSONDoor uint8

const (
	proofJSONDoorUnknown proofJSONDoor = iota
	proofJSONDoorState
	proofJSONDoorEvidenceKind
	proofJSONDoorResult
	proofJSONDoorSummary
	proofJSONDoorLimit
)

func (d proofJSONDoor) receiverName() string {
	switch d {
	case proofJSONDoorState:
		return "State"
	case proofJSONDoorEvidenceKind:
		return "EvidenceKind"
	case proofJSONDoorResult:
		return "Result"
	case proofJSONDoorSummary:
		return "Summary"
	case proofJSONDoorUnknown, proofJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type proofJSONSeed struct {
	document []byte
	door     proofJSONDoor
}

func FuzzSourceProofExternalJSONDoorInventory(f *testing.F) {
	result, summary := proofResultForFuzz(f)
	for _, seed := range []proofJSONSeed{
		proofJSONSeedForFuzz(f, proofJSONDoorState, sourceproof.StateHumanReviewRequired),
		proofJSONSeedForFuzz(f, proofJSONDoorEvidenceKind, sourceproof.EvidenceSourceObservation),
		proofJSONSeedForFuzz(f, proofJSONDoorResult, result),
		proofJSONSeedForFuzz(f, proofJSONDoorSummary, summary),
	} {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(proofJSONDoorResult), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch proofJSONDoor(rawDoor) {
		case proofJSONDoorState:
			fuzzProofJSONDoor(t, data, sourceproof.StateHumanReviewRequired)
		case proofJSONDoorEvidenceKind:
			fuzzProofJSONDoor(t, data, sourceproof.EvidenceSourceObservation)
		case proofJSONDoorResult:
			fuzzProofJSONDoor(t, data, result)
		case proofJSONDoorSummary:
			fuzzProofJSONDoor(t, data, summary)
		case proofJSONDoorUnknown, proofJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type proofJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzProofJSONDoor[T proofJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("source proof seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("source proof JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrSourceProofContract) {
			t.Fatalf("source proof JSON door error = %v, want %v and %v", decodeErr, core.ErrJSONContract, core.ErrSourceProofContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected source proof JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted source proof JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("source proof canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("source proof round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("source proof canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("source proof JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func TestSourceProofExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, err := proofExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("proofExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var want []string
	for door := proofJSONDoorUnknown + 1; door < proofJSONDoorLimit; door++ {
		want = append(want, door.receiverName())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", got, want)
	}
}

func proofExportedJSONReceiverNames() ([]string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "UnmarshalJSON" || function.Recv == nil || len(function.Recv.List) != 1 {
				continue
			}
			pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			receiver, ok := pointer.X.(*ast.Ident)
			if ok && receiver.IsExported() {
				names = append(names, receiver.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func proofResultForFuzz(t testing.TB) (sourceproof.Result, sourceproof.Summary) {
	t.Helper()
	subject := proofSubject(t, core.SourceSubjectPackage, "exchange")
	claim := proofClaim(t, subject)
	result := proofResult(t, claim, proofCommit(t, "0123456789abcdef0123456789abcdef01234567"), sourceproof.StateHumanReviewRequired, nil)
	if err := result.ValidateAgainst(claim); err != nil {
		t.Fatalf("sourceproof.Result(seed).ValidateAgainst() error = %v, want nil", err)
	}
	resolver := proofResultResolver{results: map[proofResultKey]sourceproof.Result{
		{subject: claim.Subject, claim: claim.ID}: result,
	}}
	summary, err := sourceproof.VerifyClaims(context.Background(), func(emit sourceclaim.Emit) error {
		return emit(claim)
	}, resolver, func(sourceproof.Result) error { return nil })
	if err != nil {
		t.Fatalf("sourceproof.VerifyClaims(seed) error = %v, want nil", err)
	}
	return result, summary
}

func proofJSONSeedForFuzz(t testing.TB, door proofJSONDoor, value proofJSONValue) proofJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("source proof fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return proofJSONSeed{door: door, document: document}
}
