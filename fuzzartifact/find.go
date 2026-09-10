package fuzzartifact

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/filestore"
)

// Find visits every matching direct child in native directory order. Visit is
// synchronous backpressure; the caller owns any blocking work and must return.
// No names are retained or sorted. A callback may have effects even when it
// returns an error, so Delivered counts only callbacks returning nil.
func Find(ctx context.Context, request FindRequest) (Observation, error) {
	if err := request.Validate(); err != nil {
		return Observation{}, err
	}
	current := newFinder(request)
	err := filestore.Walk(ctx, filestore.WalkRequest{Location: request.Location, Visit: current.visit})
	err = errors.Join(err, contextstate.Validate(ctx))
	if err == nil {
		return current.finish()
	}
	if !current.observation.hasAccounting() {
		result := Observation{kind: request.Kind, format: request.Format, state: ObservationFailed}
		return result, errors.Join(result.Validate(), observationError(err))
	}
	return current.partial(observationError(err))
}

type finder struct {
	observation Observation
	visitor     func(GeneratedName) error
}

func newFinder(request FindRequest) finder {
	return finder{observation: Observation{kind: request.Kind, format: request.Format}, visitor: request.Visit}
}

func (f *finder) visit(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
	switch {
	case entry.Entry.IsDir():
		incrementSaturating(&f.observation.ignoredDirectories)
		return filestore.WalkSkipDirectory, nil
	case !entry.Entry.Type().IsRegular():
		incrementSaturating(&f.observation.nonRegular)
	default:
		return filestore.WalkContinue, f.observeRegular(entry.Entry.Name())
	}
	return filestore.WalkContinue, nil
}

func (f *finder) observeRegular(value string) error {
	name, err := ParseGeneratedName(f.observation.format, f.observation.kind, value)
	if err != nil {
		incrementSaturating(&f.observation.unsupportedRegular)
		return nil
	}
	incrementSaturating(&f.observation.matched)
	if err := f.visitor(name); err != nil {
		return err
	}
	incrementSaturating(&f.observation.delivered)
	return nil
}

func (f *finder) finish() (Observation, error) {
	f.observation.state = ObservationComplete
	var err error
	if f.observation.unsupportedRegular != 0 {
		f.observation.state = ObservationUnsupportedFormat
		err = formatError(errors.New("directory contains regular files outside the declared format"))
	}
	return f.observation, errors.Join(f.observation.Validate(), err)
}

func (f *finder) partial(cause error) (Observation, error) {
	f.observation.state = ObservationPartial
	var formatErr error
	if f.observation.unsupportedRegular != 0 {
		formatErr = formatError(errors.New("partial directory contains regular files outside the declared format"))
	}
	return f.observation, errors.Join(f.observation.Validate(), formatErr, cause)
}
