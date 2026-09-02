package gcsobjects

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	json "encoding/json/v2"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/auth/credentials"
	"github.com/deliri/primitive/v2026/core"
)

type gcsCredentialFileShape uint8

const (
	gcsCredentialFileRegular gcsCredentialFileShape = iota + 1
	gcsCredentialFileAbsent
	gcsCredentialFileAbsentParent
	gcsCredentialFileDirectory
	gcsCredentialFileEscapingSymlink
	gcsCredentialFileLoopSymlink
	gcsCredentialFileNestedRegular
)

type gcsCredentialFileCase struct {
	name       string
	content    []byte
	wantErrors []error
	shape      gcsCredentialFileShape
	cancelled  bool
}

// TestGCSCredentialFileIngressHostileBoundaryMatrix is a direct ratchet for
// the package-private custody leaf. Public SDK construction is separately
// pressured by FuzzNewGCSClientCredentialFileSemanticBoundary below.
func TestGCSCredentialFileIngressHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()

	validCases := []gcsCredentialFileCase{
		{name: "empty regular credential file is read exactly", shape: gcsCredentialFileRegular},
		{name: "one ASCII byte is read exactly", shape: gcsCredentialFileRegular, content: []byte{'a'}},
		{name: "JSON-shaped bytes are read without reinterpretation", shape: gcsCredentialFileRegular, content: []byte(`{"type":"service_account"}`)},
		{name: "line-delimited bytes remain exact", shape: gcsCredentialFileRegular, content: []byte("first\nsecond\n")},
		{name: "UTF-8 bytes remain exact", shape: gcsCredentialFileRegular, content: []byte("crédential")},
		{name: "embedded zero byte remains exact", shape: gcsCredentialFileRegular, content: []byte{'a', 0, 'b'}},
		{name: "invalid UTF-8 remains exact for the provider parser", shape: gcsCredentialFileRegular, content: []byte{0xff, 0xfe}},
		{name: "four KiB regular file is read exactly", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(4 << 10)},
		{name: "eight KiB regular file is read exactly", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(8 << 10)},
		{name: "sixteen KiB regular file is read exactly", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(16 << 10)},
	}
	rejectionCases := []gcsCredentialFileCase{
		{name: "one byte above maximum is refused", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes + 1), wantErrors: credentialSizeErrors()},
		{name: "two bytes above maximum are refused", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes + 2), wantErrors: credentialSizeErrors()},
		{name: "one page above maximum is refused", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes + 4096), wantErrors: credentialSizeErrors()},
		{name: "twice the maximum is refused without partial bytes", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(2 * GCSCredentialJSONMaximumBytes), wantErrors: credentialSizeErrors()},
		{name: "absent credential file preserves Filestore source identity", shape: gcsCredentialFileAbsent, wantErrors: credentialSourceErrors()},
		{name: "absent credential parent preserves Filestore source identity", shape: gcsCredentialFileAbsentParent, wantErrors: credentialSourceErrors()},
		{name: "directory cannot stand in for a credential file", shape: gcsCredentialFileDirectory, wantErrors: credentialSourceErrors()},
		{name: "escaping symlink cannot leave the rooted parent", shape: gcsCredentialFileEscapingSymlink, wantErrors: credentialSourceErrors()},
		{name: "symlink loop cannot stand in for a credential file", shape: gcsCredentialFileLoopSymlink, wantErrors: credentialSourceErrors()},
		{name: "pre-cancelled read releases no credential bytes", shape: gcsCredentialFileRegular, content: []byte("unreleased"), cancelled: true, wantErrors: []error{core.ErrObjectStoreContract, context.Canceled}},
	}
	boundaryCases := []gcsCredentialFileCase{
		{name: "zero-byte lower bound is admitted", shape: gcsCredentialFileRegular},
		{name: "one-byte lower neighbour is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(1)},
		{name: "two-byte lower neighbour is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(2)},
		{name: "thirty-one bytes below stream word are admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(31)},
		{name: "thirty-two byte stream word is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(32)},
		{name: "thirty-three bytes above stream word are admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(33)},
		{name: "one below KiB boundary is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes((1 << 10) - 1)},
		{name: "exact KiB boundary is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(1 << 10)},
		{name: "one above KiB boundary is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes((1 << 10) + 1)},
		{name: "one below Filestore buffer boundary is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes((32 << 10) - 1)},
		{name: "exact Filestore buffer boundary is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(32 << 10)},
		{name: "one above Filestore buffer boundary is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes((32 << 10) + 1)},
		{name: "two below credential ceiling are admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes - 2)},
		{name: "one below credential ceiling is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes - 1)},
		{name: "exact credential ceiling is admitted", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes)},
		{name: "one above credential ceiling is refused", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes + 1), wantErrors: credentialSizeErrors()},
		{name: "two above credential ceiling are refused", shape: gcsCredentialFileRegular, content: repeatedCredentialBytes(GCSCredentialJSONMaximumBytes + 2), wantErrors: credentialSizeErrors()},
		{name: "binary byte spectrum below ceiling remains exact", shape: gcsCredentialFileRegular, content: byteSpectrum()},
		{name: "whitespace-only provider input remains exact", shape: gcsCredentialFileRegular, content: []byte(" \t\r\n")},
		{name: "one nested parent remains confined and exact", shape: gcsCredentialFileNestedRegular, content: []byte("nested")},
	}

	runGCSCredentialFileCases(t, validCases)
	runGCSCredentialFileCases(t, rejectionCases)
	runGCSCredentialFileCases(t, boundaryCases)
}

func runGCSCredentialFileCases(t *testing.T, cases []gcsCredentialFileCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			path := prepareGCSCredentialFile(t, directory, testCase)
			ctx := context.Background()
			if testCase.cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			absolute, parseErr := core.ParseAbsolutePath(path)
			if parseErr != nil {
				t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", path, parseErr)
			}
			got, gotErr := gcsCredentialJSON(ctx, GCSClientConfig{
				Authentication: GCSAuthenticationServiceAccountFile,
				CredentialFile: absolute,
			})
			if len(testCase.wantErrors) != 0 {
				for _, wantErr := range testCase.wantErrors {
					if !errors.Is(gotErr, wantErr) {
						t.Fatalf("gcsCredentialJSON() error = %v, want errors.Is(..., %v)", gotErr, wantErr)
					}
				}
				if got != nil {
					t.Fatalf("gcsCredentialJSON() bytes = %d, want nil after refusal", len(got))
				}
				return
			}
			if gotErr != nil || !bytes.Equal(got, testCase.content) {
				t.Fatalf("gcsCredentialJSON() = (%d bytes, %v), want exact %d bytes and nil", len(got), gotErr, len(testCase.content))
			}
		})
	}
}

func prepareGCSCredentialFile(t testing.TB, directory string, testCase gcsCredentialFileCase) string {
	t.Helper()

	path := filepath.Join(directory, "credential.json")
	switch testCase.shape {
	case gcsCredentialFileRegular:
		if err := os.WriteFile(path, testCase.content, 0o600); err != nil {
			t.Fatalf("os.WriteFile(credential) error = %v, want nil", err)
		}
	case gcsCredentialFileAbsent:
	case gcsCredentialFileAbsentParent:
		path = filepath.Join(directory, "absent", "credential.json")
	case gcsCredentialFileDirectory:
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("os.Mkdir(credential path) error = %v, want nil", err)
		}
	case gcsCredentialFileEscapingSymlink:
		if err := os.Symlink(filepath.Join("..", "outside-credential.json"), path); err != nil {
			t.Fatalf("os.Symlink(escaping credential) error = %v, want nil", err)
		}
	case gcsCredentialFileLoopSymlink:
		if err := os.Symlink(filepath.Base(path), path); err != nil {
			t.Fatalf("os.Symlink(loop credential) error = %v, want nil", err)
		}
	case gcsCredentialFileNestedRegular:
		parent := filepath.Join(directory, "nested")
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatalf("os.Mkdir(nested credential parent) error = %v, want nil", err)
		}
		path = filepath.Join(parent, "credential.json")
		if err := os.WriteFile(path, testCase.content, 0o600); err != nil {
			t.Fatalf("os.WriteFile(nested credential) error = %v, want nil", err)
		}
	default:
		t.Fatalf("credential file shape = %d, want a closed test shape", testCase.shape)
	}
	return path
}

func repeatedCredentialBytes(size int) []byte {
	return bytes.Repeat([]byte{'c'}, size)
}

func byteSpectrum() []byte {
	value := make([]byte, 256)
	for index := range value {
		value[index] = byte(index)
	}
	return value
}

func credentialSizeErrors() []error {
	return []error{core.ErrObjectStoreContract, core.ErrFilestoreSize}
}

func credentialSourceErrors() []error {
	return []error{core.ErrObjectStoreContract, core.ErrFilestoreSource}
}

type gcsServiceAccountCredentialDocument struct {
	Type         credentials.CredType `json:"type"`
	ProjectID    string               `json:"project_id"`
	PrivateKeyID string               `json:"private_key_id"`
	PrivateKey   string               `json:"private_key"`
	ClientEmail  string               `json:"client_email"`
	TokenURI     string               `json:"token_uri"`
}

func canonicalGCSServiceAccountCredential(t testing.TB) []byte {
	t.Helper()
	document := gcsServiceAccountCredentialDocument{
		Type:         credentials.ServiceAccount,
		ProjectID:    "primitive-provider-proof",
		PrivateKeyID: "provider-owned-key-id",
		PrivateKey:   string(canonicalGCSCredentialPrivateKey(t)),
		ClientEmail:  "provider-proof@primitive-provider-proof.iam.gserviceaccount.com",
		TokenURI:     "https://oauth2.googleapis.com/token",
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(service account credential) error = %v, want nil", err)
	}
	return encoded
}

func canonicalGCSCredentialPrivateKey(t testing.TB) []byte {
	t.Helper()

	primeP := gcsCredentialPrime(t, "efec2274843234968624da171fd2cd6efe38412a6d9e8ce955ef4b3d732c2aeed3fbbc8f04600623807c3eb7455f465bc438719c1d92195c7c9a56a5e4a01005")
	primeQ := gcsCredentialPrime(t, "cf6f8f185a3860ba605d524586a52002bbefbfcd6b8226387653cc227892e98e903f310f246faafb77089dd93b026437480ce0053c0090d6f65f3174f64b4d21")
	modulus := new(big.Int).Mul(primeP, primeQ)
	primePMinusOne := new(big.Int).Sub(primeP, big.NewInt(1))
	primeQMinusOne := new(big.Int).Sub(primeQ, big.NewInt(1))
	totient := new(big.Int).Mul(primePMinusOne, primeQMinusOne)
	privateExponent := new(big.Int).ModInverse(big.NewInt(65537), totient)
	if privateExponent == nil {
		t.Fatal("RSA private exponent = nil, want invertible deterministic primes")
	}
	key := &rsa.PrivateKey{
		N: modulus, E: 65537,
		D:      privateExponent,
		Primes: []*big.Int{primeP, primeQ},
	}
	if err := key.Validate(); err != nil {
		t.Fatalf("deterministic RSA fixture Validate() error = %v, want nil", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func gcsCredentialPrime(t testing.TB, value string) *big.Int {
	t.Helper()
	prime, ok := new(big.Int).SetString(value, 16)
	if !ok {
		t.Fatalf("deterministic RSA prime parsed = false, want true")
	}
	return prime
}

func FuzzNewGCSClientCredentialFileSemanticBoundary(f *testing.F) {
	canonical := canonicalGCSServiceAccountCredential(f)
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{"type":"service_account"}`))
	f.Add(repeatedCredentialBytes(GCSCredentialJSONMaximumBytes - 1))
	f.Add(repeatedCredentialBytes(GCSCredentialJSONMaximumBytes))
	f.Add(repeatedCredentialBytes(GCSCredentialJSONMaximumBytes + 1))

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > GCSCredentialJSONMaximumBytes+1 {
			input = input[:GCSCredentialJSONMaximumBytes+1]
		}
		directory := t.TempDir()
		path := filepath.Join(directory, "credential.json")
		if err := os.WriteFile(path, input, 0o600); err != nil {
			t.Fatalf("os.WriteFile(fuzz credential) error = %v, want nil", err)
		}
		absolute, parseErr := core.ParseAbsolutePath(path)
		if parseErr != nil {
			t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", path, parseErr)
		}
		config := GCSClientConfig{
			Authentication: GCSAuthenticationServiceAccountFile,
			CredentialFile: absolute,
		}
		if validateErr := config.Validate(); validateErr != nil {
			t.Fatalf("GCSClientConfig.Validate() error = %v, want nil", validateErr)
		}
		gotBytes, gotReadErr := gcsCredentialJSON(context.Background(), config)
		client, gotClientErr := NewGCSClient(context.Background(), config)
		if len(input) > GCSCredentialJSONMaximumBytes {
			if gotBytes != nil || client != nil ||
				!errors.Is(gotReadErr, core.ErrFilestoreSize) ||
				!errors.Is(gotClientErr, core.ErrObjectStoreContract) ||
				!errors.Is(gotClientErr, core.ErrFilestoreSize) {
				t.Fatalf("oversized credential ingress = (%d bytes, %v, %v, %v), want nil bytes, nil client, object-store contract, and Filestore size", len(gotBytes), gotReadErr, client, gotClientErr)
			}
			return
		}
		if gotReadErr != nil || !bytes.Equal(gotBytes, input) {
			t.Fatalf("bounded credential custody = (%d bytes, %v), want exact %d bytes and nil", len(gotBytes), gotReadErr, len(input))
		}
		if gotClientErr != nil {
			if client != nil || !errors.Is(gotClientErr, core.ErrObjectStoreContract) {
				t.Fatalf("rejected provider credential = (%v, %v), want nil client and %v", client, gotClientErr, core.ErrObjectStoreContract)
			}
			return
		}
		if client == nil || client.Validate() != nil {
			t.Fatalf("accepted provider credential client = %v, want validated client", client)
		}
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("accepted provider credential client close error = %v, want nil", closeErr)
		}
	})
}
