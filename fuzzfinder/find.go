package fuzzfinder

import (
	"context"
	"errors"
	"slices"

	"github.com/deliri/primitive/v2026/filestore"
)

// Find streams one real rooted directory through Filestore. It retains only a
// bounded canonical prefix and preserves Filestore's external-door identity
// beneath the stable Fuzzfinder observation identity.
func Find(ctx context.Context, request FindRequest) (Observation, error) {
	if err := request.Validate(); err != nil {
		return Observation{}, err
	}
	current := newFinder(request)
	err := filestore.Walk(ctx, filestore.WalkRequest{
		Location: request.Location,
		Order:    filestore.WalkOrderNative,
		Visit:    current.visit,
	})
	if err == nil {
		return current.finish()
	}
	if current.observation.retained == 0 && !current.observation.hasAccounting() {
		return failedFind(request, observationError(err))
	}
	return current.partial(observationError(err))
}

func failedFind(request FindRequest, err error) (Observation, error) {
	result := failedObservation(request.Kind, request.Format, request.Retention)
	return result, errors.Join(result.Validate(), err)
}

type finder struct {
	observation Observation
}

func newFinder(request FindRequest) finder {
	return finder{
		observation: Observation{limit: request.Retention, kind: request.Kind, format: request.Format},
	}
}

func (f *finder) visit(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
	switch {
	case entry.Entry.IsDir():
		incrementSaturating(&f.observation.ignoredDirectories)
		return filestore.WalkSkipDirectory, nil
	case !entry.Entry.Type().IsRegular():
		incrementSaturating(&f.observation.nonRegular)
	default:
		f.observeRegular(entry.Entry.Name())
	}
	return filestore.WalkContinue, nil
}

func (f *finder) observeRegular(value string) {
	name, err := ParseGeneratedName(f.observation.Format(), f.observation.kind, value)
	if err != nil {
		incrementSaturating(&f.observation.unsupportedRegular)
		return
	}
	f.observeGenerated(name)
}

func (f *finder) observeGenerated(name GeneratedName) {
	count := int(f.observation.retained)
	position, found := slices.BinarySearchFunc(f.observation.names[:count], name, GeneratedName.compare)
	if found {
		return
	}
	if count < int(f.observation.limit.value) {
		copy(f.observation.names[position+1:count+1], f.observation.names[position:count])
		f.observation.names[position] = name
		f.observation.retained++
		return
	}
	incrementSaturating(&f.observation.overLimit)
	if position < count {
		copy(f.observation.names[position+1:count], f.observation.names[position:count-1])
		f.observation.names[position] = name
	}
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
