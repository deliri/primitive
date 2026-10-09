package gomodule

import (
	"context"
	"errors"
	"io"
	"iter"
	"strconv"
	"strings"
	"unicode"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

type DeclarationPresence uint8

const moduleDirectiveKeyword = "module"

const (
	DeclarationAbsent DeclarationPresence = iota + 1
	DeclarationPresent
)

func (p DeclarationPresence) Validate() error {
	if p != DeclarationAbsent && p != DeclarationPresent {
		return core.ErrGoModuleContract
	}
	return nil
}

// DeclarationObservation projects the first module directive. Absence means
// the source reached EOF; the caller decides whether a directive is required.
type DeclarationObservation struct {
	Path     DeclaredPath
	Presence DeclarationPresence
}

func (o DeclarationObservation) Validate() error {
	if err := o.Presence.Validate(); err != nil {
		return err
	}
	if o.Presence == DeclarationPresent {
		return o.Path.Validate()
	}
	if o.Path.value != "" {
		return core.ErrGoModuleContract
	}
	return nil
}

type DeclarationRequest struct{ Source io.Reader }

func (r DeclarationRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoModuleContract
	}
	return nil
}

// ObserveDeclaration streams Go's validated characters without collecting
// lines or unrelated tokens. Working memory is constant apart from the
// returned identity itself. No line or file extent ceiling applies. It is a
// directive projection, not validation of the remainder of a go.mod file.
func ObserveDeclaration(ctx context.Context, request DeclarationRequest) (DeclarationObservation, error) {
	if err := request.Validate(); err != nil {
		return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, err)
	}
	next, stop := iter.Pull2(lineio.Characters(ctx, lineio.CharacterRequest{Source: request.Source}))
	defer stop()
	input := declarationInput{next: next}
	for {
		found, delimiter, err := input.keyword()
		if found {
			return input.declaration(delimiter, err)
		}
		if err == io.EOF {
			return DeclarationObservation{Presence: DeclarationAbsent}, nil
		}
		if err != nil {
			return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, err)
		}
		if err := input.discardLine(delimiter); err != nil && err != io.EOF {
			return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, err)
		}
	}
}

type declarationInput struct {
	next func() (lineio.Character, error, bool)
}

func (r declarationInput) character() (rune, error) {
	value, err, ok := r.next()
	if !ok {
		return 0, io.EOF
	}
	return rune(value.Value), err
}

func (r declarationInput) keyword() (bool, rune, error) {
	var word [len(moduleDirectiveKeyword)]rune
	used := 0
	for {
		value, err := r.character()
		if err != nil {
			return moduleKeyword(word, used), 0, err
		}
		if unicode.IsSpace(value) {
			if used == 0 {
				continue
			}
			return moduleKeyword(word, used), value, nil
		}
		if used == len(word) {
			return false, '\n', r.discardLine(value)
		}
		word[used] = value
		used++
	}
}

func moduleKeyword(word [len(moduleDirectiveKeyword)]rune, used int) bool {
	if used != len(word) {
		return false
	}
	for index, value := range moduleDirectiveKeyword {
		if word[index] != value {
			return false
		}
	}
	return true
}

func (r declarationInput) discardLine(value rune) error {
	for value != '\n' {
		var err error
		value, err = r.character()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r declarationInput) declaration(delimiter rune, keywordErr error) (DeclarationObservation, error) {
	if keywordErr != nil || delimiter == '\n' {
		return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, keywordErr)
	}
	value, delimiter, err := r.identity()
	if err != nil {
		return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, err)
	}
	if err := r.tail(delimiter); err != nil {
		return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, err)
	}
	path, err := parseDeclaredPath(value)
	if err != nil {
		return DeclarationObservation{}, errors.Join(core.ErrGoModuleContract, err)
	}
	observation := DeclarationObservation{Presence: DeclarationPresent, Path: path}
	return observation, observation.Validate()
}

func (r declarationInput) identity() (string, rune, error) {
	var text strings.Builder
	for {
		value, err := r.character()
		if err == io.EOF {
			return decodeDeclarationIdentity(text.String(), 0)
		}
		if err != nil {
			return "", 0, err
		}
		if unicode.IsSpace(value) {
			if text.Len() != 0 || value == '\n' {
				return decodeDeclarationIdentity(text.String(), value)
			}
			continue
		}
		if _, err := text.WriteRune(value); err != nil {
			return "", 0, err
		}
	}
}

func decodeDeclarationIdentity(value string, delimiter rune) (string, rune, error) {
	if value == "" {
		return "", 0, core.ErrGoModuleContract
	}
	if strings.HasPrefix(value, "\"") {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return "", 0, errors.Join(core.ErrGoModuleContract, err)
		}
		value = decoded
	}
	return value, delimiter, nil
}

func (r declarationInput) tail(value rune) error {
	for value != 0 && value != '\n' {
		var err error
		value, err = r.character()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if unicode.IsSpace(value) {
			continue
		}
		return r.comment(value)
	}
	return nil
}

func (r declarationInput) comment(first rune) error {
	if first != '/' {
		return core.ErrGoModuleContract
	}
	second, err := r.character()
	if err != nil || second != '/' {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	err = r.discardLine(second)
	if err == io.EOF {
		return nil
	}
	return err
}

var _ core.Validatable = DeclarationPresence(0)
var _ core.Validatable = DeclarationObservation{}
var _ core.Validatable = DeclarationRequest{}
