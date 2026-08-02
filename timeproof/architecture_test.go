package timeproof

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type (
	protocolFact[T any]     struct{ Value T }
	sealedProjection[T any] struct{ Value T }
	wireProjection[T any]   struct{ Value T }
	internalFlow[T any]     struct{ Value T }
	operationRequest[T any] struct{ Value T }
)

type timeproofContractInventory struct {
	TimestampChainRequest      internalFlow[timestampChainRequest]
	MessageImprint             protocolFact[messageImprint]
	TimestampRequestFields     internalFlow[timestampRequestFields]
	CMSEncapsulatedContent     protocolFact[cmsEncapsulatedContent]
	CMSIssuerAndSerial         protocolFact[cmsIssuerAndSerial]
	CMSAttribute               protocolFact[cmsAttribute]
	RefusalCodeFact            protocolFact[refusalCodeFact]
	ParsedToken                internalFlow[parsedToken]
	CMSSignerInfo              protocolFact[cmsSignerInfo]
	ParsedSignedData           internalFlow[parsedSignedData]
	AuthorityContract          internalFlow[authorityContract]
	TimestampPolicyContract    internalFlow[timestampPolicyContract]
	RefusalStatusFact          protocolFact[refusalStatusFact]
	ParsedTSTInfo              internalFlow[parsedTSTInfo]
	VerifiedToken              internalFlow[verifiedToken]
	RequestWire                wireProjection[requestWire]
	AuthorityEvidence          sealedProjection[AuthorityEvidence]
	AuthorityEvidenceInput     internalFlow[authorityEvidenceInput]
	AuthorityEvidenceWire      wireProjection[authorityEvidenceWire]
	Request                    sealedProjection[Request]
	VerifyRequest              operationRequest[VerifyRequest]
	AuthoritativeTimestamp     sealedProjection[AuthoritativeTimestamp]
	AuthoritativeTimestampWire wireProjection[authoritativeTimestampWire]
	AccuracyWire               protocolFact[accuracyWire]
	AuthorityConclusion        protocolFact[authorityConclusion]
	Refusal                    sealedProjection[Refusal]
	RefusalCodeSet             internalFlow[refusalCodeSet]
	PrepareRequest             operationRequest[PrepareRequest]
	SerialNumber               sealedProjection[SerialNumber]
	Nonce                      sealedProjection[Nonce]
}

var _ = timeproofContractInventory{}

func TestTimeproofProductionStructsHaveCompilerVisibleDataFlowRoles(
	t *testing.T,
) {
	t.Parallel()

	gotStructs, gotImports, gotErr := scanTimeproofProduction(".")
	if gotErr != nil {
		t.Fatalf("scanTimeproofProduction() error = %v, want nil", gotErr)
	}
	wantStructs := classifiedTimeproofStructs()
	if !slices.Equal(gotStructs, wantStructs) {
		t.Fatalf(
			"Timeproof production structs = %q, want classified %q",
			gotStructs,
			wantStructs,
		)
	}
	wantImports := []string{
		"bytes",
		"crypto/sha1",
		"crypto/sha256",
		"crypto/sha512",
		"crypto/subtle",
		"crypto/x509",
		"crypto/x509/pkix",
		"embed",
		"encoding/asn1",
		"encoding/base64",
		"encoding/hex",
		"encoding/json",
		"encoding/pem",
		"errors",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/keygen",
		"github.com/deliri/primitive/v2026/temporal",
		"math/big",
		"strconv",
		"strings",
		"sync",
		"time",
	}
	if !slices.Equal(gotImports, wantImports) {
		t.Fatalf(
			"Timeproof production imports = %q, want exact transport-free imports %q",
			gotImports,
			wantImports,
		)
	}
}

func classifiedTimeproofStructs() []string {
	inventory := reflect.TypeFor[timeproofContractInventory]()
	classified := make([]string, 0, inventory.NumField())
	for field := range inventory.Fields() {
		role := field.Type
		classified = append(
			classified,
			role.Field(0).Type.Name(),
		)
	}
	sort.Strings(classified)
	return classified
}

func scanTimeproofProduction(root string) ([]string, []string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	structs := make([]string, 0, len(entries))
	imports := make([]string, 0, len(entries))
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			files,
			filepath.Join(root, entry.Name()),
			nil,
			0,
		)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		structs = append(structs, timeproofStructNames(file)...)
		for _, spec := range file.Imports {
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return nil, nil, unquoteErr
			}
			imports = append(imports, path)
		}
	}
	sort.Strings(structs)
	sort.Strings(imports)
	imports = slices.Compact(imports)
	return structs, imports, nil
}

func timeproofStructNames(file *ast.File) []string {
	names := make([]string, 0)
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, spec := range generic.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				names = append(names, typeSpec.Name.Name)
			}
		}
	}
	return names
}
