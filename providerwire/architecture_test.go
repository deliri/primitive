package providerwire

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

type (
	providerProtocolFact[T any] struct{}
	providerInternalFlow[T any] struct{}
	providerCapability[T any]   struct{}
)

// providerWireContractInventory classifies every production data carrier by
// its actual wall-socket role. The field name must equal the classified type.
type providerWireContractInventory struct {
	StripeCredential                    providerCapability[StripeCredential]
	stripeCredentialKindFact            providerInternalFlow[stripeCredentialKindFact]
	PlunkCredential                     providerCapability[PlunkCredential]
	PlunkWebhookSecret                  providerCapability[PlunkWebhookSecret]
	TwilioCredential                    providerCapability[TwilioCredential]
	PayPalAccessToken                   providerCapability[PayPalAccessToken]
	PayPalClientCredential              providerCapability[PayPalClientCredential]
	PayPalAccessGrant                   providerCapability[PayPalAccessGrant]
	PayPalAccessGrantRequest            providerProtocolFact[PayPalAccessGrantRequest]
	StripeClient                        providerCapability[StripeClient]
	PlunkClient                         providerCapability[PlunkClient]
	TwilioClient                        providerCapability[TwilioClient]
	PayPalClient                        providerCapability[PayPalClient]
	stripeClient                        providerInternalFlow[stripeClient]
	plunkClient                         providerInternalFlow[plunkClient]
	twilioClient                        providerInternalFlow[twilioClient]
	payPalClient                        providerInternalFlow[payPalClient]
	jsonProviderRequestContract         providerInternalFlow[jsonProviderRequestContract]
	InboundObservation                  providerProtocolFact[InboundObservation]
	providerFact                        providerInternalFlow[providerFact]
	StripeWebhookSecret                 providerCapability[StripeWebhookSecret]
	StripeWebhookReceiver               providerCapability[StripeWebhookReceiver]
	StripeWebhookReceiveRequest         providerProtocolFact[StripeWebhookReceiveRequest]
	stripeWebhookReceiver               providerInternalFlow[stripeWebhookReceiver]
	TwilioAuthToken                     providerCapability[TwilioAuthToken]
	twilioWebhookRepresentationFact     providerInternalFlow[twilioWebhookRepresentationFact]
	TwilioWebhookReceiverRequest        providerProtocolFact[TwilioWebhookReceiverRequest]
	TwilioWebhookReceiver               providerCapability[TwilioWebhookReceiver]
	twilioWebhookReceiver               providerInternalFlow[twilioWebhookReceiver]
	PlunkWebhookReceiver                providerCapability[PlunkWebhookReceiver]
	plunkWebhookReceiver                providerInternalFlow[plunkWebhookReceiver]
	PayPalWebhookReceiver               providerCapability[PayPalWebhookReceiver]
	PayPalWebhookReceiveRequest         providerProtocolFact[PayPalWebhookReceiveRequest]
	payPalWebhookReceiver               providerInternalFlow[payPalWebhookReceiver]
	payPalOAuthResponse                 providerProtocolFact[payPalOAuthResponse]
	payPalOAuthRequest                  providerInternalFlow[payPalOAuthRequest]
	payPalWebhookVerificationResponse   providerProtocolFact[payPalWebhookVerificationResponse]
	payPalWebhookVerificationProjection providerProtocolFact[payPalWebhookVerificationProjection]
	payPalWebhookVerificationStatusFact providerInternalFlow[payPalWebhookVerificationStatusFact]
}

func TestProviderWireDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := providerWireProductionStructNames(t)
	want := providerWireClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Providerwire production structs = %q, want classified %q", got, want)
	}
}

func providerWireProductionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileSet, filepath.Clean(entry.Name()), nil, parser.SkipObjectResolution)
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

func providerWireClassifiedStructNames(t *testing.T) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "architecture_test.go", nil, parser.SkipObjectResolution)
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
			if specification.Name.Name != "providerWireContractInventory" {
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
	t.Fatal("providerWireContractInventory declarations found = 0, want 1")
	return nil
}

var (
	_ = providerWireContractInventory{}.stripeClient
	_ = providerWireContractInventory{}.stripeCredentialKindFact
	_ = providerWireContractInventory{}.plunkClient
	_ = providerWireContractInventory{}.twilioClient
	_ = providerWireContractInventory{}.payPalClient
	_ = providerWireContractInventory{}.jsonProviderRequestContract
	_ = providerWireContractInventory{}.providerFact
	_ = providerWireContractInventory{}.stripeWebhookReceiver
	_ = providerWireContractInventory{}.twilioWebhookReceiver
	_ = providerWireContractInventory{}.twilioWebhookRepresentationFact
	_ = providerWireContractInventory{}.plunkWebhookReceiver
	_ = providerWireContractInventory{}.payPalWebhookReceiver
	_ = providerWireContractInventory{}.payPalOAuthResponse
	_ = providerWireContractInventory{}.payPalOAuthRequest
	_ = providerWireContractInventory{}.payPalWebhookVerificationResponse
	_ = providerWireContractInventory{}.payPalWebhookVerificationProjection
	_ = providerWireContractInventory{}.payPalWebhookVerificationStatusFact
)
