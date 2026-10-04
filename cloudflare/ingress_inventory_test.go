package cloudflare

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type ingressProof struct {
	fuzz func(*testing.F)
	name string
}

func bindIngress[Door any](door Door, fuzz func(*testing.F)) ingressProof {
	name := runtime.FuncForPC(reflect.ValueOf(door).Pointer()).Name()
	name = strings.TrimPrefix(name, "github.com/deliri/primitive/v2026/cloudflare.")
	return ingressProof{name: name, fuzz: fuzz}
}
func cloudflareIngressProofs() []ingressProof {
	return []ingressProof{
		bindIngress(ParseAccountID, FuzzCloudflareIdentityRepresentationClosure),
		bindIngress(ParseImageID, FuzzCloudflareIdentityRepresentationClosure),
		bindIngress(ParseStreamVideoID, FuzzCloudflareIdentityRepresentationClosure),
		bindIngress(ParseAPIToken, FuzzCloudflareSecretCustody),
		bindIngress(APITokenFromSecret, FuzzCloudflareSecretCustody),
		bindIngress(ParseNotificationSecret, FuzzCloudflareSecretCustody),
		bindIngress(ParseStreamWebhookSecret, FuzzCloudflareSecretCustody),
		bindIngress(ParseR2Credentials, FuzzCloudflareSecretCustody),
		bindIngress(ParseR2Bucket, FuzzR2ObjectRepresentationClosure),
		bindIngress(ParseR2Key, FuzzR2ObjectRepresentationClosure),
		bindIngress(ParseR2Grant, FuzzR2GrantRepresentationAndProviderVerification),
		bindIngress(ParseImageUpload, FuzzCloudflareUploadAuthorityClosure),
		bindIngress(ParseStreamUpload, FuzzCloudflareUploadAuthorityClosure),
		bindIngress(ImagesServer.CreateDirectUpload, FuzzImagesDirectUploadResponseSemanticClosure),
		bindIngress(StreamServer.CreateDirectUpload, FuzzStreamDirectUploadResponseSemanticClosure),
		bindIngress(ImagesWebhookReceiver.Receive, FuzzImagesWebhookExactSecretAndRoute),
		bindIngress(StreamWebhookReceiver.Receive, FuzzStreamWebhookSignatureRepresentation),
		bindIngress(R2Server.Presign, FuzzR2GrantSignatureAndAuthority),
	}
}

// These are Cloudflare's representation-admission doors. Raw stream transfer
// stays in Exchange and has its own public ingress/fuzz inventory; Cloudflare
// does not decode an R2 object body or claim it is a provider protocol document.
func TestCloudflareExternalDecodersHaveCompilerBoundSemanticFuzz(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(cloudflareSource, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := cloudflareSource.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, cloudflareDecoderDoors(file)...)
	}
	var want []string
	for _, proof := range cloudflareIngressProofs() {
		if proof.fuzz == nil {
			t.Fatalf("%s fuzz target=nil, want compiler bound target", proof.name)
		}
		want = append(want, proof.name)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("Cloudflare external decoder doors=%q, want exact fuzz inventory %q", got, want)
	}
}
func cloudflareDecoderDoors(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !fn.Name.IsExported() {
			continue
		}
		name := fn.Name.Name
		if !(strings.HasPrefix(name, "Parse") || strings.HasPrefix(name, "Decode") || strings.HasPrefix(name, "Unmarshal") || name == "APITokenFromSecret" || name == "CreateDirectUpload" || name == "Receive" || name == "Presign") {
			continue
		}
		if fn.Recv != nil {
			name = cloudflareReceiverName(fn.Recv.List[0].Type) + "." + name
		}
		names = append(names, name)
	}
	return names
}
func TestCloudflareIngressMatcherLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{name: "new public decoder cannot evade inventory", source: `func DecodeFuture([]byte){}`, want: []string{"DecodeFuture"}},
		{name: "receiver method remains a distinct door", source: `type T struct{};func (*T) UnmarshalJSON([]byte){}`, want: []string{"T.UnmarshalJSON"}},
		{name: "private parser is covered through its public owner", source: `func parsePrivate([]byte){}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\n"+tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := cloudflareDecoderDoors(file); !slices.Equal(got, tc.want) {
				t.Fatalf("discovered doors=%q, want %q", got, tc.want)
			}
		})
	}
}
