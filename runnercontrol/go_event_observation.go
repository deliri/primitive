package runnercontrol

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// GoEventObservation carries one native cmd/go metadata record. Labels are
// source data; consumer policy owns their role in cross-record accounting.
// Output remains a borrowed stream and is not materialized in this value.
type GoEventObservation struct {
	Time        time.Time
	FailedBuild string
	ImportPath  string
	Package     string
	Test        string
	Frame       GoEventFrame
}

func (GoEventObservation) runnerControlProtocolFact() {}

func (o GoEventObservation) Validate() error {
	if err := o.Frame.Validate(); err != nil {
		return err
	}
	for _, text := range [...]string{o.FailedBuild, o.ImportPath, o.Package, o.Test} {
		if !utf8.ValidString(text) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

// GoEventObservationRequest projects metadata through the existing native
// scalar decoder. Output fragments remain provisional until OnEvent. Each
// metadata atom allocates its full admitted text; no event population is held.
type GoEventObservationRequest struct {
	Source   io.Reader
	OnOutput func(GoEventStringFragment) error
	OnEvent  func(GoEventObservation) error
}

func (GoEventObservationRequest) runnerControlCapabilityWrapper() {}

func (r GoEventObservationRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.OnOutput == nil || r.OnEvent == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type goEventMetadataAtoms struct {
	timestamp   strings.Builder
	failedBuild strings.Builder
	importPath  strings.Builder
	packagePath strings.Builder
	testName    strings.Builder
	timePresent bool
}

func (goEventMetadataAtoms) runnerControlInternalFlow() {}

// ReadGoEventObservations owns native metadata assembly and Go's time JSON
// decoding. The framing/enum/string authority remains ReadGoEventStream.
// Callback, source and cancellation refusals retain their native identities.
// A delivered event certifies only its prefix; clean EOF closes the source.
func ReadGoEventObservations(ctx context.Context, request GoEventObservationRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	var atoms goEventMetadataAtoms
	return ReadGoEventStream(ctx, GoEventStreamRequest{
		Source: request.Source,
		OnString: func(fragment GoEventStringFragment) error {
			var destination *strings.Builder
			switch fragment.Field {
			case GoEventFieldTime:
				destination = &atoms.timestamp
				atoms.timePresent = true
			case GoEventFieldFailedBuild:
				destination = &atoms.failedBuild
			case GoEventFieldImportPath:
				destination = &atoms.importPath
			case GoEventFieldPackage:
				destination = &atoms.packagePath
			case GoEventFieldTest:
				destination = &atoms.testName
			case GoEventFieldOutput:
				return request.OnOutput(fragment)
			case GoEventFieldAction, GoEventFieldOutputType, GoEventFieldKey, GoEventFieldValue, GoEventFieldPath:
				return nil
			default:
				return core.ErrPrimitiveContract
			}
			_, err := destination.Write(fragment.Data)
			return err
		},
		OnEvent: func(frame GoEventFrame) error {
			var timestamp time.Time
			if atoms.timePresent {
				encoded, err := jsonio.MarshalText(atoms.timestamp.String())
				if err != nil {
					return err
				}
				timestamp, err = jsonio.DecodeValue[time.Time](encoded, jsonio.ProjectionReadPolicy{Members: jsonio.MembersDeclaredOnly, Root: jsonio.RootOwnedMethods})
				if err != nil {
					return err
				}
			}
			observation := GoEventObservation{Time: timestamp, FailedBuild: atoms.failedBuild.String(), ImportPath: atoms.importPath.String(), Package: atoms.packagePath.String(), Test: atoms.testName.String(), Frame: frame}
			if err := observation.Validate(); err != nil {
				return errors.Join(core.ErrPrimitiveContract, err)
			}
			atoms = goEventMetadataAtoms{}
			return request.OnEvent(observation)
		},
	})
}
