package keygen

import (
	"crypto/ed25519"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// These are method expressions, so pointer-only replacements cannot retain
// the value-receiver contracts by implicitly taking a test variable's address.
var (
	_ core.Validatable                                 = SecretRequest{}
	_ core.Validatable                                 = RandomTokenRequest{}
	_ core.Validatable                                 = EntropyReader{}
	_ core.Validatable                                 = SigningKey{}
	_ core.Validatable                                 = Token{}
	_ io.Reader                                        = EntropyReader{}
	_ fmt.Formatter                                    = SigningKey{}
	_ func() EntropyReader                             = NewEntropyReader
	_ func() (uint64, error)                           = RandomUint64
	_ func(RandomTokenRequest) (Token, error)          = RandomToken
	_ func(SecretRequest) (core.SecretMaterial, error) = GenerateSecret
	_ func() (SigningKey, error)                       = GenerateSigningKey
	_ func(SigningKey) ([SeedSize]byte, error)         = SigningKey.Seed
	_ func(SigningKey) (ed25519.PrivateKey, error)     = SigningKey.PrivateKey
	_ func(SigningKey) (core.Ed25519PublicKey, error)  = SigningKey.PublicKey
	_ func(SigningKey) error                           = SigningKey.Destroy
	_ func(SigningKey) error                           = SigningKey.Validate
	_ func(Token) ([]byte, error)                      = Token.Bytes
	_ func(Token) error                                = Token.Validate
	_ func(EntropyReader, []byte) (int, error)         = EntropyReader.Read
)
