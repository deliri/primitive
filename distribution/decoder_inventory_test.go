package distribution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

type distributionFuzzOwner[T any] struct{}
type distributionIngressInventory struct {
	SigningDomain                 distributionFuzzOwner[SigningDomain]
	RequestCommitment             distributionFuzzOwner[RequestCommitment]
	PublicationRequestPayload     distributionFuzzOwner[PublicationRequestPayload]
	PublicationRequestDocument    distributionFuzzOwner[PublicationRequestDocument]
	PublicationGrantPayload       distributionFuzzOwner[PublicationGrantPayload]
	PublicationGrantDocument      distributionFuzzOwner[PublicationGrantDocument]
	PublicationCompletionPayload  distributionFuzzOwner[PublicationCompletionPayload]
	PublicationCompletionDocument distributionFuzzOwner[PublicationCompletionDocument]
	UpdateRequestPayload          distributionFuzzOwner[UpdateRequestPayload]
	UpdateRequestDocument         distributionFuzzOwner[UpdateRequestDocument]
	UpdateResponsePayload         distributionFuzzOwner[UpdateResponsePayload]
	UpdateResponseDocument        distributionFuzzOwner[UpdateResponseDocument]
	UpgradeRequestPayload         distributionFuzzOwner[UpgradeRequestPayload]
	UpgradeRequestDocument        distributionFuzzOwner[UpgradeRequestDocument]
	UpgradeGrantPayload           distributionFuzzOwner[UpgradeGrantPayload]
	UpgradeGrantDocument          distributionFuzzOwner[UpgradeGrantDocument]
}

var _ distributionIngressInventory

func TestDistributionExternalDecoderInventory(t *testing.T) {
	t.Parallel()
	files, err := distributionProductionFiles()
	if err != nil {
		t.Fatalf("source inventory=%v, want nil", err)
	}
	var got []string
	for _, name := range files {
		file, err := distributionParseSource(token.NewFileSet(), name)
		if err != nil {
			t.Fatalf("parse source=%v, want nil", err)
		}
		got = append(got, distributionDecoderOwners(file)...)
	}
	want := []string{
		"SigningDomain", "RequestCommitment", "PublicationRequestPayload", "PublicationRequestDocument",
		"PublicationGrantPayload", "PublicationGrantDocument", "PublicationCompletionPayload", "PublicationCompletionDocument",
		"UpdateRequestPayload", "UpdateRequestDocument", "UpdateResponsePayload", "UpdateResponseDocument",
		"UpgradeRequestPayload", "UpgradeRequestDocument", "UpgradeGrantPayload", "UpgradeGrantDocument"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("external decoder owners=%v, want %v", got, want)
	}
	// Read compiler-embedded external tests, not the mutable checkout.
	file, err := distributionParseSource(token.NewFileSet(), "external_decoder_fuzz_test.go")
	if err != nil {
		t.Fatalf("parse fuzz owners=%v, want nil", err)
	}
	var fuzz []string
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if ok && f.Recv == nil && len(f.Name.Name) >= 4 && f.Name.Name[:4] == "Fuzz" {
			fuzz = append(fuzz, f.Name.Name)
		}
	}
	var targets []string
	for _, owner := range want {
		suffix := "ExternalDecoder"
		if owner == "SigningDomain" {
			suffix = "ExternalDecoders"
		}
		targets = append(targets, "Fuzz"+owner+suffix)
	}
	slices.Sort(fuzz)
	slices.Sort(targets)
	if !slices.Equal(fuzz, targets) {
		t.Fatalf("fuzz functions=%v, want %v", fuzz, targets)
	}
}

func distributionDecoderOwners(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Recv == nil || f.Name.Name != "UnmarshalJSON" {
			continue
		}
		receiver := f.Recv.List[0].Type
		if p, ok := receiver.(*ast.StarExpr); ok {
			receiver = p.X
		}
		if n, ok := receiver.(*ast.Ident); ok {
			names = append(names, n.Name)
		}
	}
	return names
}

func TestDistributionDecoderMatcherLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         []string
	}{
		{"pointer decoder", "package p; type Socket struct{}; func (*Socket) UnmarshalJSON([]byte) error{return nil}", []string{"Socket"}},
		{"value decoder", "package p; type Socket struct{}; func (Socket) UnmarshalJSON([]byte) error{return nil}", []string{"Socket"}},
		{"encoder is not ingress", "package p; type Socket struct{}; func (Socket) MarshalJSON()([]byte,error){return nil,nil}", nil},
		{"free function is not a type boundary", "package p; func UnmarshalJSON([]byte) error{return nil}", nil},
		{"unrelated method", "package p; type Socket struct{}; func (*Socket) Validate()error{return nil}", nil},
		{"two distinct owners", "package p; type A struct{}; type B struct{}; func (*A) UnmarshalJSON([]byte)error{return nil};func(B)UnmarshalJSON([]byte)error{return nil}", []string{"A", "B"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, 0)
			if err != nil {
				t.Fatalf("parse fixture=%v, want nil", err)
			}
			got := distributionDecoderOwners(file)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("decoder owners=%v, want %v", got, tc.want)
			}
		})
	}
}
