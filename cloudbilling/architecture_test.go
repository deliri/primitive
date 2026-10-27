package cloudbilling

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"
)

//go:embed *.go
var sources embed.FS

type capabilityRole[T any] struct{}
type protocolRole[T any] struct{}
type wireRole[T any] struct{}
type scalarRole[T any] struct{}
type bufferRole[T any] struct{}

// The compiler binds each classification to the actual production carrier;
// the AST inventory fails if a future carrier is added without an owner.
type billingInventory struct {
	Quota       scalarRole[QuotaProject]
	ADCRequest  protocolRole[ADCClientRequest]
	Credentials protocolRole[CredentialSource]
	ADC         capabilityRole[ADCClient]
	Options     protocolRole[ServerOptions]
	Server      capabilityRole[Server]
	Request     protocolRole[ListSKUsRequest]
	Source      protocolRole[SourceReceipt]
	Receipt     protocolRole[PageReceipt]
	Limits      protocolRole[PageLimits]
	Money       protocolRole[Money]
	MoneyWire   wireRole[moneyWire]
	Page        protocolRole[Page]
	SKU         protocolRole[SKU]
	Category    protocolRole[Category]
	Geography   protocolRole[Geography]
	Pricing     protocolRole[PricingInfo]
	Aggregation protocolRole[Aggregation]
	Expression  protocolRole[PricingExpression]
	Tier        protocolRole[TierRate]
	Buffer      bufferRole[pageBuffer]
}

func TestBillingCompilerOwnedStructAndSDKExecutionBoundary(t *testing.T) {
	t.Parallel()
	_ = billingInventory{}
	entries, err := sources.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := sources.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if spec, ok := node.(*ast.TypeSpec); ok {
				if spec.Assign.IsValid() {
					t.Errorf("type alias %s, want actual nominal carrier", spec.Name.Name)
				}
				if _, ok := spec.Type.(*ast.StructType); ok {
					got = append(got, spec.Name.Name)
				}
			}
			return true
		})
	}
	slices.Sort(got)
	want := []string{"ADCClient", "ADCClientRequest", "Aggregation", "Category", "CredentialSource", "Geography", "ListSKUsRequest", "Money", "Page", "PageLimits", "PageReceipt", "PricingExpression", "PricingInfo", "QuotaProject", "SKU", "Server", "ServerOptions", "SourceReceipt", "TierRate", "moneyWire", "pageBuffer"}
	if !slices.Equal(got, want) || reflect.TypeFor[billingInventory]().NumField() != len(want) {
		t.Fatalf("production carriers=%q/inventory fields=%d, want %q", got, reflect.TypeFor[billingInventory]().NumField(), want)
	}
}
