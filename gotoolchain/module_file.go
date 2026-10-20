package gotoolchain

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

type ModuleGoDeclaration uint8

const (
	ModuleGoDeclarationUnknown ModuleGoDeclaration = iota
	ModuleGoDeclarationAbsent
	ModuleGoDeclarationPresent
)

func (p ModuleGoDeclaration) IsValid() bool {
	return p == ModuleGoDeclarationAbsent || p == ModuleGoDeclarationPresent
}

func (p ModuleGoDeclaration) Validate() error {
	if !p.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}

func (p ModuleGoDeclaration) String() string {
	switch p {
	case ModuleGoDeclarationAbsent:
		return "absent"
	case ModuleGoDeclarationPresent:
		return "present"
	default:
		return core.UnknownEnumDiagnostic
	}
}

func (ModuleGoDeclaration) OffWireEnum() {}

// ModuleFileObservation projects only the declared Go version and replacement
// count. Go owns go.mod grammar; no module graph or directive catalog is copied.
type ModuleFileObservation struct {
	GoVersion     ToolchainVersion
	Replacements  uint64
	GoDeclaration ModuleGoDeclaration
}

func (o ModuleFileObservation) Validate() error {
	if err := o.GoDeclaration.Validate(); err != nil {
		return err
	}
	if o.GoDeclaration == ModuleGoDeclarationAbsent {
		if o.GoVersion.String() != "" {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	return o.GoVersion.Validate()
}

func (ModuleFileObservation) goToolchainProtocolFact() {}

// ObserveModuleFile asks the selected native Go command to parse go.mod without
// edits or dependency lookups. Native output is consumed inside its completed,
// extent-verified private file scope. Only a fixed scalar projection survives.
// Source: https://go.dev/ref/mod#go-mod-edit
func (c Capability) ObserveModuleFile(ctx context.Context, request ObservationRequest) (ModuleFileObservation, error) {
	if ctx == nil {
		return ModuleFileObservation{}, core.ErrGoToolchainContract
	}
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return ModuleFileObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return ModuleFileObservation{}, errors.Join(core.ErrGoToolchainExecution, err)
	}
	var observed ModuleFileObservation
	err := c.withCompletedCommandOutput(ctx, request.WorkingDirectory, []string{"mod", "edit", "-json"}, func(source io.Reader, _ process.ResultObservation) error {
		var err error
		observed, err = readModuleFileObservation(ctx, source)
		return err
	})
	if err != nil {
		return ModuleFileObservation{}, err
	}
	return observed, observed.Validate()
}

func readModuleFileObservation(ctx context.Context, source io.Reader) (ModuleFileObservation, error) {
	if ctx == nil || core.ReaderIsNil(source) {
		return ModuleFileObservation{}, core.ErrGoToolchainContract
	}
	decoder := jsontext.NewDecoder(source)
	read := func() (jsontext.Token, error) {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return jsontext.Token{}, err
		}
		value, err := decoder.ReadToken()
		return value, errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
	}
	skip := func() error {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return err
		}
		depth := decoder.StackDepth()
		// Borrow the native cursor through containers; never capture an
		// ignored requirement, replacement or other collection as one value.
		for {
			if _, err := read(); err != nil {
				return err
			}
			if decoder.StackDepth() == depth {
				return nil
			}
		}
	}
	refuse := func(err error) (ModuleFileObservation, error) {
		return ModuleFileObservation{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	start, err := read()
	if err != nil || start.Kind() != jsontext.KindBeginObject {
		return refuse(err)
	}
	observed := ModuleFileObservation{GoDeclaration: ModuleGoDeclarationAbsent}
	for {
		name, err := read()
		if err != nil {
			return refuse(err)
		}
		if name.Kind() == jsontext.KindEndObject {
			break
		}
		if name.Kind() != jsontext.KindString {
			return refuse(nil)
		}
		switch name.String() {
		case "Go":
			value, err := read()
			if err != nil || value.Kind() != jsontext.KindString {
				return refuse(err)
			}
			if value.String() != "" {
				observed.GoVersion, err = ParseToolchainVersion("go" + value.String())
				if err != nil {
					return refuse(err)
				}
				observed.GoDeclaration = ModuleGoDeclarationPresent
			}
		case "Replace":
			start, err := read()
			if err != nil {
				return refuse(err)
			}
			if start.Kind() == jsontext.KindNull {
				continue
			}
			if start.Kind() != jsontext.KindBeginArray {
				return refuse(nil)
			}
			for decoder.PeekKind() != jsontext.KindEndArray {
				if decoder.PeekKind() != jsontext.KindBeginObject {
					// PeekKind reports only a kind. Reading returns the owning
					// source or syntax error rather than erasing its identity.
					_, err := read()
					return refuse(err)
				}
				if err := skip(); err != nil {
					return refuse(err)
				}
				observed.Replacements++
			}
			if _, err := read(); err != nil {
				return refuse(err)
			}
		default:
			if err := skip(); err != nil {
				return refuse(err)
			}
		}
	}
	_, err = read()
	if !errors.Is(err, io.EOF) {
		return refuse(err)
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return refuse(err)
	}
	if err := observed.Validate(); err != nil {
		return refuse(err)
	}
	return observed, nil
}
