package capabilities

import (
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/firestoredocuments"
	"github.com/deliri/primitive/v2026/github"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/keygen"
)

var (
	_ func(context.Context, googleidentity.BrowserAuthorizationCompleteRequest) (googleidentity.BrowserVerifiedIdentity, error) = googleidentity.CompleteBrowserAuthorization
	_ func(context.Context, github.BrowserAuthorizationCompleteRequest) (github.BrowserVerifiedIdentity, error)                 = github.CompleteBrowserAuthorization
)

func TestBrowserAuthorizationOperationsAreCompilerBound(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name      string
		operation Operation
		function  reflect.Value
	}{
		{name: "Google subject verification owns declared operation", operation: OperationGoogleBrowserAuthorization, function: reflect.ValueOf(googleidentity.CompleteBrowserAuthorization)},
		{name: "GitHub subject verification owns declared operation", operation: OperationGitHubBrowserAuthorization, function: reflect.ValueOf(github.CompleteBrowserAuthorization)},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			contract, found, err := row.operation.Contract()
			if err != nil || !found {
				t.Fatalf("browser capability = (%+v,%t,%v), want offered", contract, found, err)
			}
			signature := row.function.Type()
			want := runtime.FuncForPC(row.function.Pointer()).Name()
			got := contract.Function.ImportPath.String() + "." + contract.Function.Selector.String()
			resultPath, pathErr := contract.ResultPackage.ImportPath()
			if got != want || contract.Request.String() != signature.In(1).Name() || contract.Result.String() != signature.Out(0).Name() || resultPath != signature.Out(0).PkgPath() || pathErr != nil {
				t.Fatalf("browser capability signature = %+v/%v, want exact %s", contract, pathErr, want)
			}
			effect, err := row.operation.effect()
			if err != nil || effect != EffectTransport {
				t.Fatalf("browser capability effect = (%v,%v), want transport", effect, err)
			}
		})
	}
}

func TestOAuthProofAndFirestorePrimaryOperationsAreCompilerBound(t *testing.T) {
	t.Parallel()
	proof, found, err := OperationGenerateOAuthBrowserProof.Contract()
	if err != nil || !found || proof.HasRequest || proof.Result.String() != reflect.TypeFor[keygen.OAuthBrowserProof]().Name() || proof.Function.ImportPath.String()+"."+proof.Function.Selector.String() != runtime.FuncForPC(reflect.ValueOf(keygen.GenerateOAuthBrowserProof).Pointer()).Name() {
		t.Fatalf("OAuth proof capability = (%+v,%t,%v), want no-argument native entropy producer", proof, found, err)
	}
	transaction, found, err := OperationRunFirestoreTransaction.Contract()
	if err != nil || !found || !transaction.HasRequest || transaction.Request.String() != reflect.TypeFor[firestoredocuments.TransactionRequest]().Name() || transaction.Result.String() != reflect.TypeFor[firestoredocuments.TransactionResult]().Name() || transaction.Function.ImportPath.String()+"."+transaction.Function.Selector.String() != runtime.FuncForPC(reflect.ValueOf(firestoredocuments.RunTransaction).Pointer()).Name() {
		t.Fatalf("Firestore primary capability = (%+v,%t,%v), want concrete transaction execution", transaction, found, err)
	}
}
