package process

import (
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// WorkingDirectory reports the calling process's current working directory
// as the absolute path Request.WorkingDirectory demands.
//
// It exists because this package requires an absolute working directory on
// every request and must therefore supply a way to make one: the directory
// the caller is already in is the value nearly every one-shot tool run
// wants, and without this door every consumer reaches for os.Getwd and
// re-parses the answer by hand.
func WorkingDirectory() (core.AbsolutePath, error) {
	directory, err := os.Getwd()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessObservation, err)
	}
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessObservation, err)
	}
	return path, nil
}
