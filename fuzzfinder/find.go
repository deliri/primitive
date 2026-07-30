package fuzzfinder

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
)

const directoryReadBatchEntries = 64

// Find opens, streams, and closes one real rooted directory. It retains only a
// bounded canonical prefix and preserves native open, stat, read, and close
// errors through the stable Fuzzfinder identities.
func Find(request FindRequest) (Observation, error) {
	if err := request.Validate(); err != nil {
		return Observation{}, err
	}
	directory, err := request.Location.Root.Open(request.Location.Path.String())
	if err != nil {
		return failedFind(request, observationError(err))
	}
	info, statErr := directory.Stat()
	if statErr != nil {
		return failedFind(request, errors.Join(observationError(statErr), closeDirectory(directory)))
	}
	if !info.IsDir() {
		return failedFind(request, errors.Join(contractError(fs.ErrInvalid), closeDirectory(directory)))
	}
	result, findErr := findDirectory(directory, request)
	closeErr := closeDirectory(directory)
	return result, errors.Join(findErr, closeErr)
}

func failedFind(request FindRequest, err error) (Observation, error) {
	result := failedObservation(request.Kind, request.Retention)
	return result, errors.Join(result.Validate(), err)
}

func closeDirectory(directory *os.File) error {
	if err := directory.Close(); err != nil {
		return observationError(err)
	}
	return nil
}

type finder struct {
	observation Observation
	format      CacheFormat
}

func newFinder(request FindRequest) finder {
	return finder{
		observation: Observation{limit: request.Retention, kind: request.Kind},
		format:      request.Format,
	}
}

func findDirectory(directory fs.ReadDirFile, request FindRequest) (Observation, error) {
	if directory == nil {
		return Observation{}, contractError(errors.New("directory reader is nil"))
	}
	current := newFinder(request)
	for {
		entries, readErr := directory.ReadDir(directoryReadBatchEntries)
		if err := current.observe(entries); err != nil {
			return current.partial(observationError(err))
		}
		if errors.Is(readErr, io.EOF) {
			return current.finish()
		}
		if readErr != nil {
			return current.partial(observationError(readErr))
		}
		if len(entries) == 0 {
			return current.partial(observationError(io.ErrNoProgress))
		}
	}
}

func (f *finder) observe(entries []fs.DirEntry) error {
	for _, entry := range entries {
		if entry == nil {
			return errors.New("directory returned a nil entry")
		}
		switch {
		case entry.IsDir():
			incrementSaturating(&f.observation.ignoredDirectories)
		case !entry.Type().IsRegular():
			incrementSaturating(&f.observation.nonRegular)
		default:
			f.observeRegular(entry.Name())
		}
	}
	return nil
}

func (f *finder) observeRegular(value string) {
	name, err := ParseGeneratedName(f.format, value)
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
