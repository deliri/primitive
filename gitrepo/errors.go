package gitrepo

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(err error) error {
	return errors.Join(core.ErrGitRepositoryContract, err)
}

func executionError(err error) error {
	return errors.Join(core.ErrGitRepositoryExecution, err)
}

func outputError(err error) error {
	return errors.Join(core.ErrGitRepositoryOutput, err)
}
