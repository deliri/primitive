package keygen_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func openSSHFixture(t testing.TB, comment string, label keygen.PEMLabel) (keygen.SigningKey, []byte) {
	t.Helper()
	var seed [keygen.SeedSize]byte
	for i := range seed {
		seed[i] = 7
	}
	key, err := keygen.AdoptSigningKey(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = key.Destroy() })
	var output bytes.Buffer
	if err := keygen.WriteOpenSSH(t.Context(), keygen.OpenSSHWriteRequest{Destination: &output, Key: key, Label: label, Comment: comment}); err != nil {
		t.Fatal(err)
	}
	return key, output.Bytes()
}

func TestOpenSSHStreamingKeyConservationAcrossPaddingAndCommentExtents(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 65535, 65536, 65537, 1048577} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			original, data := openSSHFixture(t, strings.Repeat("c", size), keygen.PEMLabel("CALLER PRIVATE KEY"))
			got, err := keygen.ReadOpenSSH(t.Context(), keygen.OpenSSHReadRequest{Source: iotest.OneByteReader(bytes.NewReader(data)), AdditionalLabel: "CALLER PRIVATE KEY"})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = got.Destroy() }()
			wantSeed, wantErr := original.Seed()
			gotSeed, gotErr := got.Seed()
			clear(data)
			if wantErr != nil || gotErr != nil || gotSeed != wantSeed {
				t.Fatalf("streamed key material changed: %v/%v", wantErr, gotErr)
			}
			clear(wantSeed[:])
			clear(gotSeed[:])
		})
	}
}

func TestOpenSSHCodecInteroperatesWithNativeOpenSSH(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	generated := filepath.Join(dir, "native")
	if output, err := exec.CommandContext(t.Context(), "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "native format oracle", "-f", generated).CombinedOutput(); err != nil {
		t.Fatalf("native key generation: %v %s", err, output)
	}
	source, err := os.Open(generated)
	if err != nil {
		t.Fatal(err)
	}
	key, decodeErr := keygen.ReadOpenSSH(t.Context(), keygen.OpenSSHReadRequest{Source: source})
	closeErr := source.Close()
	if err := errors.Join(decodeErr, closeErr); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = key.Destroy() }()
	public, err := key.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, err := public.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	encoded := filepath.Join(dir, "encoded")
	destination, err := os.OpenFile(encoded, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := keygen.WriteOpenSSH(t.Context(), keygen.OpenSSHWriteRequest{Key: key, Destination: destination, Label: keygen.OpenSSHPrivateKeyLabel, Comment: "native round trip"})
	if err := errors.Join(writeErr, destination.Close()); err != nil {
		t.Fatal(err)
	}
	output, err := exec.CommandContext(t.Context(), "ssh-keygen", "-y", "-P", "", "-f", encoded).CombinedOutput()
	if err != nil {
		t.Fatalf("native key decoding: %v %s", err, output)
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 || fields[0] != "ssh-ed25519" {
		t.Fatal("native key oracle returned an unexpected public-key format")
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil || len(blob) != 51 || !bytes.Equal(blob[19:], publicBytes) {
		t.Fatalf("native public identity disagrees: %v", err)
	}
}

func TestOpenSSHDecoderWithdrawsMalformedFramingAndIdentity(t *testing.T) {
	t.Parallel()
	_, encoded := openSSHFixture(t, "", keygen.OpenSSHPrivateKeyLabel)
	block, _ := pem.Decode(encoded)
	if block == nil {
		t.Fatal("fixture is not native PEM")
	}
	// Independent offsets in the single-key Ed25519 grammar: outer public
	// begins at 43, private string length at 94, private check integers at 98.
	for _, test := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{name: "magic", edit: func(b []byte) []byte { b[0] ^= 1; return b }},
		{name: "cipher", edit: func(b []byte) []byte { b[19] ^= 1; return b }},
		{name: "KDF", edit: func(b []byte) []byte { b[27] ^= 1; return b }},
		{name: "KDF options", edit: func(b []byte) []byte { b[34] = 1; return b }},
		{name: "zero keys", edit: func(b []byte) []byte { binary.BigEndian.PutUint32(b[35:39], 0); return b }},
		{name: "two keys", edit: func(b []byte) []byte { binary.BigEndian.PutUint32(b[35:39], 2); return b }},
		{name: "outer algorithm", edit: func(b []byte) []byte { b[47] ^= 1; return b }},
		{name: "outer identity", edit: func(b []byte) []byte { b[62] ^= 1; return b }},
		{name: "private extent", edit: func(b []byte) []byte { b[97] ^= 1; return b }},
		{name: "check integers", edit: func(b []byte) []byte { b[98] ^= 1; return b }},
		{name: "inner algorithm", edit: func(b []byte) []byte { b[110] ^= 1; return b }},
		{name: "inner identity", edit: func(b []byte) []byte { b[125] ^= 1; return b }},
		{name: "seed identity", edit: func(b []byte) []byte { b[161] ^= 1; return b }},
		{name: "private public suffix", edit: func(b []byte) []byte { b[193] ^= 1; return b }},
		{name: "comment extent", edit: func(b []byte) []byte { binary.BigEndian.PutUint32(b[225:229], ^uint32(0)); return b }},
		{name: "padding", edit: func(b []byte) []byte { b[len(b)-1] ^= 1; return b }},
		{name: "trailing binary", edit: func(b []byte) []byte { return append(b, 0) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := test.edit(bytes.Clone(block.Bytes))
			defer clear(data)
			wire := pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: data})
			got, err := keygen.ReadOpenSSH(t.Context(), keygen.OpenSSHReadRequest{Source: bytes.NewReader(wire)})
			if got != (keygen.SigningKey{}) || !errors.Is(err, core.ErrKeygenContract) {
				t.Fatalf("malformed wire emitted a key: %t/%v", got != (keygen.SigningKey{}), err)
			}
		})
	}
}

func TestOpenSSHDecoderPreservesSourceRefusalAfterCompleteKey(t *testing.T) {
	t.Parallel()
	_, data := openSSHFixture(t, "", keygen.OpenSSHPrivateKeyLabel)
	for _, source := range []io.Reader{
		iotest.ErrReader(io.ErrClosedPipe),
		io.MultiReader(bytes.NewReader(data), iotest.ErrReader(io.ErrClosedPipe)),
	} {
		got, err := keygen.ReadOpenSSH(t.Context(), keygen.OpenSSHReadRequest{Source: source})
		if got != (keygen.SigningKey{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrKeygenContract) {
			t.Fatalf("source refusal emitted a key or lost native cause: %t/%v", got != (keygen.SigningKey{}), err)
		}
	}
	for _, wire := range [][]byte{
		data[:len(data)-1-38],
		bytes.ReplaceAll(data, []byte("END OPENSSH"), []byte("END FOREIGN")),
		append(bytes.Clone(data), []byte("trailing record")...),
		bytes.ReplaceAll(data, []byte("OPENSSH PRIVATE KEY"), []byte("FOREIGN PRIVATE KEY")),
	} {
		got, err := keygen.ReadOpenSSH(t.Context(), keygen.OpenSSHReadRequest{Source: bytes.NewReader(wire)})
		if got != (keygen.SigningKey{}) || !errors.Is(err, core.ErrKeygenContract) {
			t.Fatalf("invalid envelope emitted a key: %t/%v", got != (keygen.SigningKey{}), err)
		}
	}
}

type openSSHWriteRefusal struct{}

func (openSSHWriteRefusal) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestOpenSSHCodecAdmissionAndCancellation(t *testing.T) {
	t.Parallel()
	key, data := openSSHFixture(t, "", keygen.OpenSSHPrivateKeyLabel)
	for _, label := range []keygen.PEMLabel{"", "leading\nline", " PRIVATE KEY", "PRIVATE KEY ", "lowercase"} {
		var output bytes.Buffer
		request := keygen.OpenSSHWriteRequest{Key: key, Destination: &output, Label: label}
		if err := keygen.WriteOpenSSH(t.Context(), request); !errors.Is(err, core.ErrKeygenContract) || output.Len() != 0 {
			t.Fatalf("invalid label wrote bytes=%d error=%v", output.Len(), err)
		}
	}
	if err := keygen.WriteOpenSSH(t.Context(), keygen.OpenSSHWriteRequest{Key: key, Destination: openSSHWriteRefusal{}, Label: keygen.OpenSSHPrivateKeyLabel}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("native writer refusal lost cause: %v", err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	got, err := keygen.ReadOpenSSH(ctx, keygen.OpenSSHReadRequest{Source: bytes.NewReader(data)})
	if got != (keygen.SigningKey{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("canceled source emitted a key: %t/%v", got != (keygen.SigningKey{}), err)
	}
	var output bytes.Buffer
	if err := keygen.WriteOpenSSH(ctx, keygen.OpenSSHWriteRequest{Key: key, Destination: &output, Label: keygen.OpenSSHPrivateKeyLabel}); !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) || output.Len() != 0 {
		t.Fatalf("canceled destination wrote bytes=%d error=%v", output.Len(), err)
	}
	got, err = keygen.ReadOpenSSH(nil, keygen.OpenSSHReadRequest{Source: bytes.NewReader(data)})
	if got != (keygen.SigningKey{}) || !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context admitted: %v", err)
	}
	if err := keygen.WriteOpenSSH(nil, keygen.OpenSSHWriteRequest{Key: key, Destination: &output, Label: keygen.OpenSSHPrivateKeyLabel}); !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil write context admitted: %v", err)
	}
}

func FuzzOpenSSHNativeSeedAndCommentConservation(f *testing.F) {
	f.Add([]byte("comment"), uint8(7))
	f.Add([]byte{}, uint8(1))
	f.Add([]byte{0, 255, '\n'}, uint8(255))
	f.Fuzz(func(t *testing.T, comment []byte, value uint8) {
		if value == 0 {
			value = 1
		}
		var seed [keygen.SeedSize]byte
		for i := range seed {
			seed[i] = value
		}
		key, err := keygen.AdoptSigningKey(seed)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = key.Destroy() }()
		var output bytes.Buffer
		if err := keygen.WriteOpenSSH(t.Context(), keygen.OpenSSHWriteRequest{Key: key, Label: keygen.OpenSSHPrivateKeyLabel, Destination: &output, Comment: string(comment)}); err != nil {
			t.Fatal(err)
		}
		decoded, err := keygen.ReadOpenSSH(t.Context(), keygen.OpenSSHReadRequest{Source: &output})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = decoded.Destroy() }()
		actual, err := decoded.Seed()
		defer clear(actual[:])
		if err != nil || actual != seed {
			t.Fatalf("native OpenSSH changed the signing seed: %v", err)
		}
		private, err := decoded.PrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		defer clear(private)
		message := []byte("decode native key identity")
		signature := ed25519.Sign(private, message)
		if !ed25519.Verify(private.Public().(ed25519.PublicKey), message, signature) {
			t.Fatal("decoded native identity cannot verify its own signature")
		}
	})
}

func BenchmarkOpenSSHDecoderStreamingComment(b *testing.B) {
	for _, size := range []int{8, 1048577} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			_, data := openSSHFixture(b, strings.Repeat("c", size), keygen.OpenSSHPrivateKeyLabel)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				key, err := keygen.ReadOpenSSH(b.Context(), keygen.OpenSSHReadRequest{Source: bytes.NewReader(data)})
				if err != nil {
					b.Fatal(err)
				}
				if err := key.Destroy(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
