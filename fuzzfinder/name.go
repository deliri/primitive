package fuzzfinder

import (
	"bytes"
	"encoding/hex"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const generatedNameLowerHexErrorText = "generated name is not lowercase hexadecimal"

// GeneratedName is one exact Go-generated fuzz cache or crasher filename.
type GeneratedName struct {
	value  [generatedNameBytesGo1_27]byte
	kind   ArtifactKind
	format CacheFormat
}

// GeneratedName derives the exact Go-generated filename for one content
// digest under f. Go's corpus writer names persisted entries from the leading
// digest bytes; the selected cache format owns the exact width.
func (f CacheFormat) GeneratedName(kind ArtifactKind, digest core.SHA256Digest) (GeneratedName, error) {
	width, err := f.generatedNameBytes(kind)
	if err != nil {
		return GeneratedName{}, err
	}
	digestBytes, err := digest.Bytes()
	if err != nil {
		return GeneratedName{}, contractError(err)
	}

	name := GeneratedName{kind: kind, format: f}
	hex.Encode(name.value[:width], digestBytes[:width/2])
	if err := name.Validate(); err != nil {
		return GeneratedName{}, err
	}
	return name, nil
}

// ParseGeneratedName parses one filename under the declared Go cache format.
func ParseGeneratedName(format CacheFormat, kind ArtifactKind, value string) (GeneratedName, error) {
	width, err := format.generatedNameBytes(kind)
	if err != nil {
		return GeneratedName{}, err
	}
	if len(value) != int(width) {
		return GeneratedName{}, formatError(errors.New("generated name has the wrong width"))
	}
	name := GeneratedName{kind: kind, format: format}
	for index := range len(value) {
		if !isLowerHex(value[index]) {
			return GeneratedName{}, formatError(errors.New(generatedNameLowerHexErrorText))
		}
		name.value[index] = value[index]
	}
	return name, nil
}

// Validate rejects unset, cross-kind, cross-format, or malformed names.
func (n GeneratedName) Validate() error {
	width, err := n.format.generatedNameBytes(n.kind)
	if err != nil {
		return err
	}
	for _, value := range n.value[:width] {
		if !isLowerHex(value) {
			return formatError(errors.New(generatedNameLowerHexErrorText))
		}
	}
	for _, value := range n.value[width:] {
		if value != 0 {
			return formatError(errors.New("generated name has data outside its declared width"))
		}
	}
	return nil
}

// String returns the exact generated filename.
func (n GeneratedName) String() string {
	width, err := n.format.generatedNameBytes(n.kind)
	if err != nil {
		return ""
	}
	return string(n.value[:width])
}

// Kind returns the artifact class whose Go-generated naming rule produced n.
func (n GeneratedName) Kind() ArtifactKind {
	return n.kind
}

// Format returns the exact Go toolchain format that produced n.
func (n GeneratedName) Format() CacheFormat {
	return n.format
}

func (n GeneratedName) compare(other GeneratedName) int {
	return bytes.Compare(n.value[:], other.value[:])
}

func isLowerHex(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f'
}

var _ core.Validatable = GeneratedName{}
