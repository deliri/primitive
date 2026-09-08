package attest_test

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json/jsontext"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// chunkedLiteralBody exposes arbitrary external bytes without building a second
// body. Each callback writes the supplied bytes in independently varied chunks.
type chunkedLiteralBody struct {
	literalBody
	chunkSize int
}

func (b chunkedLiteralBody) WriteCanonical(destination io.Writer) error {
	for offset := 0; offset < len(b.value); {
		end := min(offset+b.chunkSize, len(b.value))
		written, err := destination.Write(b.value[offset:end])
		if err != nil {
			return err
		}
		if written != end-offset {
			return io.ErrShortWrite
		}
		offset = end
	}
	return nil
}

func FuzzSignCanonicalBodyStreaming(f *testing.F) {
	seedBody := builtBody{commit: "fuzz-stream", count: 1}
	canonical, err := seedBody.canonical()
	if err != nil {
		f.Fatalf("canonical seed error = %v, want nil", err)
	}
	f.Add(canonical, uint16(1))
	f.Add([]byte{}, uint16(1))
	for _, size := range []int{attest.CanonicalBodyMaximumBytes - 1, attest.CanonicalBodyMaximumBytes, attest.CanonicalBodyMaximumBytes + 1} {
		f.Add(bytes.Repeat([]byte{0xa5}, size), uint16(8192))
	}
	key := deterministicPrivateKey(f, "fuzz-stream")
	trust := mustTrustedKeys(f, mustPublicKey(f, key))
	f.Fuzz(func(t *testing.T, data []byte, chunk uint16) {
		body := chunkedLiteralBody{value: data, domain: testDomainPrimary, chunkSize: int(chunk) + 1}
		got, gotErr := attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: key})
		if len(data) == 0 || len(data) > attest.CanonicalBodyMaximumBytes {
			if !errors.Is(gotErr, core.ErrAttestContract) || got != (attest.Envelope[testDomain]{}) {
				t.Fatalf("Sign() = (%+v, %v), want zero envelope and %v", got, gotErr, core.ErrAttestContract)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("Sign() error = %v, want nil", gotErr)
		}
		length, err := got.BodyLength.Uint64()
		if err != nil || length != uint64(len(data)) || got.BodySHA256 != core.NewSHA256Digest(sha256.Sum256(data)) {
			t.Fatalf("signed facts = (%d bytes, %v, %v), want %d bytes and SHA-256 of input", length, got.BodySHA256, err, len(data))
		}
		signature, err := got.Signature.Bytes()
		if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), independentAttestationFrame(t, got), signature[:]) {
			t.Fatalf("independent signature verification error = %v, want authentic frame", err)
		}
		// A different write partition must preserve the signed agreement.
		verified, err := attest.Verify(attest.VerifyRequest[testDomain]{Body: body.literalBody, Envelope: got, TrustedKeys: trust})
		if err != nil {
			t.Fatalf("Verify(one-write body) error = %v, want nil", err)
		}
		retained, err := verified.Envelope()
		if err != nil || retained != got {
			t.Fatalf("Verified.Envelope() = (%+v, %v), want %+v", retained, err, got)
		}
	})
}

func FuzzSigningDomainAdmission(f *testing.F) {
	canonical, err := testDomainPrimary.MarshalText()
	if err != nil {
		f.Fatalf("domain seed error = %v, want nil", err)
	}
	f.Add(string(canonical))
	for _, seed := range []string{"", "a--b", "a_b", "\xff", strings.Repeat("a", attest.SigningDomainMaximumBytes-1), strings.Repeat("a", attest.SigningDomainMaximumBytes), strings.Repeat("a", attest.SigningDomainMaximumBytes+1)} {
		f.Add(seed)
	}
	grammar := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	key := deterministicPrivateKey(f, "fuzz-domain")
	trust := mustTrustedKeys(f, mustPublicKey(f, key))
	f.Fuzz(func(t *testing.T, text string) {
		body := textDomainBody{domain: textDomain{text: text}}
		request := attest.SignRequest[textDomain]{Body: body, Signer: key}
		shapeErr := request.Validate()
		got, gotErr := attest.Sign(request)
		wantAdmitted := len(text) <= attest.SigningDomainMaximumBytes && grammar.MatchString(text)
		if !wantAdmitted {
			if !errors.Is(shapeErr, core.ErrAttestContract) || !errors.Is(gotErr, core.ErrAttestContract) || got != (attest.Envelope[textDomain]{}) {
				t.Fatalf("domain admission = (%v, %+v, %v), want typed refusal and zero envelope", shapeErr, got, gotErr)
			}
			return
		}
		if shapeErr != nil || gotErr != nil || got.Domain != body.domain {
			t.Fatalf("domain admission = (%v, %+v, %v), want domain %+v", shapeErr, got, gotErr, body.domain)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v, want nil", err)
		}
		var decoded attest.Envelope[textDomain]
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
			t.Fatalf("domain JSON reconstruction = (%+v, %v), want %+v", decoded, err, got)
		}
		proof, err := attest.Verify(attest.VerifyRequest[textDomain]{Body: body, Envelope: decoded, TrustedKeys: trust})
		if err != nil {
			t.Fatalf("Verify(reconstructed domain) error = %v, want nil", err)
		}
		retained, err := proof.Envelope()
		if err != nil || retained != got {
			t.Fatalf("verified envelope = (%+v, %v), want %+v", retained, err, got)
		}
	})
}

// returnedSignatureSigner models external signer response bytes, including a
// provider returning a structurally valid signature for a different frame.
type returnedSignatureSigner struct {
	public   ed25519.PublicKey
	response []byte
}

func (s returnedSignatureSigner) Public() crypto.PublicKey { return s.public }
func (s returnedSignatureSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return bytes.Clone(s.response), nil
}

func FuzzSignExternalSignerResponse(f *testing.F) {
	key := deterministicPrivateKey(f, "fuzz-provider")
	body := literalBody{value: []byte("provider response"), domain: testDomainPrimary}
	canonical := mustEnvelope(f, body, key)
	seed, err := canonical.Signature.Bytes()
	if err != nil {
		f.Fatalf("Signature.Bytes() error = %v, want nil", err)
	}
	f.Add(seed[:])
	f.Add([]byte{})
	f.Add(seed[:len(seed)-1])
	f.Add(append(bytes.Clone(seed[:]), 0))
	foreign := mustEnvelope(f, literalBody{value: []byte("foreign response"), domain: testDomainPrimary}, key)
	foreignBytes, err := foreign.Signature.Bytes()
	if err != nil {
		f.Fatalf("foreign Signature.Bytes() error = %v, want nil", err)
	}
	f.Add(foreignBytes[:])
	frame := independentAttestationFrame(f, canonical)
	public := key.Public().(ed25519.PublicKey)
	f.Fuzz(func(t *testing.T, response []byte) {
		got, gotErr := attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: returnedSignatureSigner{public: public, response: response}})
		if !ed25519.Verify(public, frame, response) {
			if !errors.Is(gotErr, core.ErrAttestContract) || got != (attest.Envelope[testDomain]{}) {
				t.Fatalf("Sign(provider response) = (%+v, %v), want zero envelope and %v", got, gotErr, core.ErrAttestContract)
			}
			return
		}
		if gotErr != nil || got != canonical {
			t.Fatalf("Sign(provider response) = (%+v, %v), want authentic envelope %+v", got, gotErr, canonical)
		}
	})
}

func FuzzSignLocalPrivateKey(f *testing.F) {
	seed := deterministicPrivateKey(f, "fuzz-private-key")
	f.Add([]byte(seed))
	f.Add([]byte{})
	f.Add([]byte(seed[:ed25519.SeedSize]))
	f.Add([]byte(seed[:ed25519.PrivateKeySize-1]))
	f.Add(append(bytes.Clone(seed), 0))
	corrupted := bytes.Clone(seed)
	corrupted[ed25519.SeedSize] ^= 1
	f.Add(corrupted)
	body := builtBody{commit: "private-key-ingress", count: 1}
	f.Fuzz(func(t *testing.T, raw []byte) {
		original := sha256.Sum256(raw)
		request := attest.SignRequest[testDomain]{Body: body, Signer: ed25519.PrivateKey(raw)}
		shapeErr := request.Validate()
		got, gotErr := attest.Sign(request)
		if sha256.Sum256(raw) != original {
			t.Fatalf("caller private key digest = %x, want %x", sha256.Sum256(raw), original)
		}
		wantAdmitted := len(raw) == ed25519.PrivateKeySize
		if wantAdmitted {
			wantAdmitted = bytes.Equal(ed25519.NewKeyFromSeed(raw[:ed25519.SeedSize]), raw)
		}
		if !wantAdmitted {
			if !errors.Is(shapeErr, core.ErrAttestContract) || !errors.Is(gotErr, core.ErrAttestContract) || got != (attest.Envelope[testDomain]{}) {
				t.Fatalf("private key admission = (%v, %+v, %v), want typed refusal and zero envelope", shapeErr, got, gotErr)
			}
			return
		}
		if shapeErr != nil || gotErr != nil {
			t.Fatalf("private key admission errors = (%v, %v), want nil", shapeErr, gotErr)
		}
		signature, err := got.Signature.Bytes()
		if err != nil || !ed25519.Verify(ed25519.PublicKey(raw[ed25519.SeedSize:]), independentAttestationFrame(t, got), signature[:]) {
			t.Fatalf("independent signature verification error = %v, want authentic caller key signature", err)
		}
	})
}

// externalJSONMember is the deliberate raw provider output seam. The builder
// owns JSON admission; the fixture neither parses nor copies external bytes.
type externalJSONMember struct{ value jsontext.Value }

func (externalJSONMember) Validate() error                { return nil }
func (m externalJSONMember) MarshalJSON() ([]byte, error) { return m.value, nil }

func FuzzCanonicalObjectNestedMember(f *testing.F) {
	canonical, err := (builtBody{commit: "nested-member", count: 1}).canonical()
	if err != nil {
		f.Fatalf("nested seed error = %v, want nil", err)
	}
	f.Add(canonical)
	for _, raw := range [][]byte{[]byte(`{"a":1,"a":2}`), append(bytes.Clone(canonical), 0)} {
		f.Add(raw)
	}
	for _, boundary := range nestedBoundaryCases() {
		f.Add([]byte(boundary.input))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		object := attest.BeginCanonicalObject(nil)
		object.Value("nested", externalJSONMember{value: jsontext.Value(raw)})
		got, gotErr := object.End()
		wantAdmitted := nestedJSONAdmitted(raw)
		if !wantAdmitted {
			if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
				t.Fatalf("nested End() = (%q, %v), want nil output and %v", got, gotErr, core.ErrAttestContract)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("independently admitted nested input rejected: %v", gotErr)
		}
		if len(got) > attest.CanonicalBodyMaximumBytes {
			t.Fatalf("nested object bytes = %d, want <= %d", len(got), attest.CanonicalBodyMaximumBytes)
		}
		decoder := jsontext.NewDecoder(bytes.NewReader(got))
		start, err := decoder.ReadToken()
		if err != nil || start.Kind() != jsontext.BeginObject.Kind() {
			t.Fatalf("object start = (%v, %v), want object", start, err)
		}
		name, err := decoder.ReadToken()
		if err != nil || name.String() != "nested" {
			t.Fatalf("member name = (%v, %v), want nested", name, err)
		}
		value, err := decoder.ReadValue()
		if err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) || !bytes.Equal(bytes.TrimSpace(value), bytes.TrimSpace(raw)) {
			t.Fatalf("nested value = (%q, %v), want original non-null JSON %q", value, err, raw)
		}
		end, err := decoder.ReadToken()
		if err != nil || end.Kind() != jsontext.EndObject.Kind() {
			t.Fatalf("object end = (%v, %v), want object close", end, err)
		}
		if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
			t.Fatalf("trailing token error = %v, want EOF", err)
		}
	})
}
