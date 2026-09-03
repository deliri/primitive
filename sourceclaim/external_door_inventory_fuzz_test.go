package sourceclaim_test

import (
	"bytes"
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
)

type claimJSONDoor uint8

const (
	claimJSONDoorUnknown claimJSONDoor = iota
	claimJSONDoorID
	claimJSONDoorText
	claimJSONDoorReference
	claimJSONDoorRequirementMode
	claimJSONDoorExecutionKind
	claimJSONDoorCompilerPredicate
	claimJSONDoorClaim
	claimJSONDoorSummary
	claimJSONDoorLimit
)

func (d claimJSONDoor) receiverName() string {
	switch d {
	case claimJSONDoorID:
		return "ID"
	case claimJSONDoorText:
		return "Text"
	case claimJSONDoorReference:
		return "Reference"
	case claimJSONDoorRequirementMode:
		return "RequirementMode"
	case claimJSONDoorExecutionKind:
		return "ExecutionKind"
	case claimJSONDoorCompilerPredicate:
		return "CompilerPredicate"
	case claimJSONDoorClaim:
		return "Claim"
	case claimJSONDoorSummary:
		return "Summary"
	case claimJSONDoorUnknown, claimJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type claimJSONFixtures struct {
	claim     sourceclaim.Claim
	summary   sourceclaim.Summary
	id        sourceclaim.ID
	text      sourceclaim.Text
	reference sourceclaim.Reference
}

type claimJSONSeed struct {
	document []byte
	door     claimJSONDoor
}

func FuzzSourceClaimExternalJSONDoorInventory(f *testing.F) {
	fixtures := claimFixturesForFuzz(f)
	for _, seed := range claimJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(claimJSONDoorClaim), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch claimJSONDoor(rawDoor) {
		case claimJSONDoorID:
			fuzzClaimJSONDoor(t, data, fixtures.id)
		case claimJSONDoorText:
			fuzzClaimJSONDoor(t, data, fixtures.text)
		case claimJSONDoorReference:
			fuzzClaimJSONDoor(t, data, fixtures.reference)
		case claimJSONDoorRequirementMode:
			fuzzClaimJSONDoor(t, data, sourceclaim.RequirementCompiler)
		case claimJSONDoorExecutionKind:
			fuzzClaimJSONDoor(t, data, sourceclaim.ExecutionTest)
		case claimJSONDoorCompilerPredicate:
			fuzzClaimJSONDoor(t, data, sourceclaim.CompilerSubjectPresent)
		case claimJSONDoorClaim:
			fuzzClaimJSONDoor(t, data, fixtures.claim)
		case claimJSONDoorSummary:
			fuzzClaimJSONDoor(t, data, fixtures.summary)
		case claimJSONDoorUnknown, claimJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type claimJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzClaimJSONDoor[T claimJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("source claim seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("source claim JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrSourceClaimContract) {
			t.Fatalf("source claim JSON door error = %v, want %v and %v", decodeErr, core.ErrJSONContract, core.ErrSourceClaimContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected source claim JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted source claim JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("source claim canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("source claim round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("source claim canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("source claim JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func TestSourceClaimExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, err := exportedJSONReceiverNames(".")
	if err != nil {
		t.Fatalf("exportedJSONReceiverNames() error = %v, want nil", err)
	}
	var want []string
	for door := claimJSONDoorUnknown + 1; door < claimJSONDoorLimit; door++ {
		want = append(want, door.receiverName())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", got, want)
	}
}

func exportedJSONReceiverNames(directory string) ([]string, error) {
	files, err := os.ReadDir(directory)
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

func claimFixturesForFuzz(t testing.TB) claimJSONFixtures {
	t.Helper()
	path, err := core.ParseSourcePath("exchange")
	if err != nil {
		t.Fatalf("core.ParseSourcePath(seed) error = %v, want nil", err)
	}
	subject, err := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if err != nil {
		t.Fatalf("core.NewSourceSubject(seed) error = %v, want nil", err)
	}
	identifier := mustClaimID(t, "exchange-bounds-http")
	text := mustClaimText(t, "Bounded HTTP exchange")
	reference, err := sourceclaim.NewReference("exchange.Client.RoundTrip")
	if err != nil {
		t.Fatalf("sourceclaim.NewReference(seed) error = %v, want nil", err)
	}
	claim := sourceclaim.Claim{
		ID: identifier, Author: mustClaimAuthority(t, 1), Subject: subject, Title: text,
		Problem:      sourceclaim.Narrative{Summary: mustClaimText(t, "Unbounded transports lose one mechanical truth.")},
		Solution:     sourceclaim.Narrative{Summary: mustClaimText(t, "Own bounded transport behind typed structures.")},
		Benefit:      sourceclaim.Narrative{Summary: mustClaimText(t, "Consumers cross one validated transport wall.")},
		Removal:      sourceclaim.Narrative{Summary: mustClaimText(t, "Remove when Go owns the complete bounded exchange contract.")},
		Owns:         []sourceclaim.Boundary{{ID: mustClaimID(t, "http-mechanics"), Detail: mustClaimText(t, "HTTP shape and bounded execution.")}},
		DoesNotOwn:   []sourceclaim.Boundary{{ID: mustClaimID(t, "product-policy"), Detail: mustClaimText(t, "Whether an operation is meaningful.")}},
		Requirements: []sourceclaim.Requirement{{ID: mustClaimID(t, "human-value-review"), Statement: mustClaimText(t, "A human confirms the boundary remains useful."), Mode: sourceclaim.RequirementHumanReview}},
	}
	if err := claim.Validate(); err != nil {
		t.Fatalf("sourceclaim.Claim(seed).Validate() error = %v, want nil", err)
	}
	summary, err := sourceclaim.Consume(func(emit sourceclaim.Emit) error {
		return emit(claim)
	}, func(sourceclaim.Claim) error { return nil })
	if err != nil {
		t.Fatalf("sourceclaim.Consume(seed) error = %v, want nil", err)
	}
	return claimJSONFixtures{claim: claim, summary: summary, id: identifier, text: text, reference: reference}
}

func claimJSONSeedsForFuzz(t testing.TB, fixtures claimJSONFixtures) []claimJSONSeed {
	t.Helper()
	return []claimJSONSeed{
		claimJSONSeedForFuzz(t, claimJSONDoorID, fixtures.id),
		claimJSONSeedForFuzz(t, claimJSONDoorText, fixtures.text),
		claimJSONSeedForFuzz(t, claimJSONDoorReference, fixtures.reference),
		claimJSONSeedForFuzz(t, claimJSONDoorRequirementMode, sourceclaim.RequirementCompiler),
		claimJSONSeedForFuzz(t, claimJSONDoorExecutionKind, sourceclaim.ExecutionTest),
		claimJSONSeedForFuzz(t, claimJSONDoorCompilerPredicate, sourceclaim.CompilerSubjectPresent),
		claimJSONSeedForFuzz(t, claimJSONDoorClaim, fixtures.claim),
		claimJSONSeedForFuzz(t, claimJSONDoorSummary, fixtures.summary),
	}
}

func claimJSONSeedForFuzz(t testing.TB, door claimJSONDoor, value claimJSONValue) claimJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("source claim fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return claimJSONSeed{door: door, document: document}
}
