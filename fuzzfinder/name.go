package fuzzfinder

import (
	"bytes"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const generatedNameLowerHexErrorText = "generated name is not lowercase hexadecimal"

// GeneratedName is one exact Go-generated fuzz cache or crasher filename.
type GeneratedName struct {
	value [generatedNameBytesGo1_26]byte
}

// ParseGeneratedName parses one filename under the declared Go cache format.
func ParseGeneratedName(format CacheFormat, value string) (GeneratedName, error) {
	width, err := format.generatedNameBytes()
	if err != nil {
		return GeneratedName{}, err
	}
	if len(value) != int(width) {
		return GeneratedName{}, formatError(errors.New("generated name has the wrong width"))
	}
	var name GeneratedName
	for index := range len(value) {
		if !isLowerHex(value[index]) {
			return GeneratedName{}, formatError(errors.New(generatedNameLowerHexErrorText))
		}
		name.value[index] = value[index]
	}
	return name, nil
}

// Validate rejects unset or malformed generated names. It inspects the stored
// bytes rather than reparsing a projected string under an assumed format: a
// name carries no format of its own, so guessing one here would validate a
// future format's name against Go 1.26's rule.
func (n GeneratedName) Validate() error {
	for _, value := range n.value {
		if !isLowerHex(value) {
			return formatError(errors.New(generatedNameLowerHexErrorText))
		}
	}
	return nil
}

// String returns the exact generated filename.
func (n GeneratedName) String() string {
	return string(n.value[:])
}

func (n GeneratedName) compare(other GeneratedName) int {
	return bytes.Compare(n.value[:], other.value[:])
}

func isLowerHex(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f'
}

var _ core.Validatable = GeneratedName{}
