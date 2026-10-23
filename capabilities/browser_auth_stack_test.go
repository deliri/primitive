package capabilities

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/aesgcm"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
	"testing"
)

// Method-shaped execution doors have actual compiler witnesses, rather than
// fabricated package-function aliases in OperationContract.
var (
	_ func(aesgcm.Cipher, aesgcm.SealTextRequest) (aesgcm.SealedText, error)                                        = aesgcm.Cipher.SealText
	_ func(aesgcm.Cipher, aesgcm.OpenTextRequest) (aesgcm.OpenResult, error)                                        = aesgcm.Cipher.OpenText
	_ func(exchange.SocketServerCall, exchange.CookieName, core.ByteCount) (exchange.ScalarValue, error)            = exchange.SocketServerCall.UniqueCookie
	_ func(exchange.SocketServerCall, exchange.QueryParameterName, core.ByteCount) (exchange.ScalarValue, error)    = exchange.SocketServerCall.UniqueQuery
	_ func(*secretstore.GoogleReader, context.Context, secretstore.AccessRequest) (secretstore.AccessResult, error) = (*secretstore.GoogleReader).Access
)

func TestBrowserAuthStackResolvesCompilerOwnedPackages(t *testing.T) {
	t.Parallel()
	for _, identity := range [...]core.PackageIdentity{core.PackageGoogleIdentity, core.PackageGitHub, core.PackageExchange, core.PackageKeygen, core.PackageAESGCM, core.PackageSecretStore, core.PackageFirestoreDocuments} {
		t.Run(identity.String(), func(t *testing.T) {
			t.Parallel()
			got, err := Resolve(ForPackage(ScopeProduction, identity))
			wantPath, pathErr := identity.ImportPath()
			gotPath, gotPathErr := got.Capability.ImportPath()
			if err != nil || pathErr != nil || gotPathErr != nil || got.Validate() != nil || got.Capability.Package != identity || gotPath != wantPath {
				t.Fatalf("auth capability %v = (%+v,%v,%q),want exact compiled package%q", identity, got, err, gotPath, wantPath)
			}
		})
	}
	t.Run("unknown package has no execution authority", func(t *testing.T) {
		t.Parallel()
		got, err := Resolve(ForPackage(ScopeProduction, core.PackageUnknown))
		if !errors.Is(err, core.ErrCapabilitiesContract) || got != (Match{}) {
			t.Fatalf("unknown capability=(%+v,%v),want zero typed refusal", got, err)
		}
	})
	t.Run("unavailable operation fabricates no contract", func(t *testing.T) {
		t.Parallel()
		got, found, err := OperationUnavailable.Contract()
		if err != nil || found || got != (OperationContract{}) {
			t.Fatalf("absent operation=(%+v,%t,%v),want zero unavailable", got, found, err)
		}
	})
}
