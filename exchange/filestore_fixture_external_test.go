package exchange_test

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func exchangeFixtureLocation(t testing.TB, path string) (filestore.Location, error) {
	t.Helper()
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		return filestore.Location{}, err
	}
	location, err := filestore.OpenParent(t.Context(), absolute)
	if err != nil {
		return filestore.Location{}, err
	}
	t.Cleanup(func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("fixture root Close() = %v, want nil", err)
		}
	})
	return location, nil
}

func openExchangeFixtureFile(t testing.TB, path string) (*os.File, error) {
	t.Helper()
	location, err := exchangeFixtureLocation(t, path)
	if err != nil {
		return nil, err
	}
	return filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
}

func createExchangeFixtureFile(t testing.TB, path string) (*os.File, error) {
	t.Helper()
	location, err := exchangeFixtureLocation(t, path)
	if err != nil {
		return nil, err
	}
	return filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: location, Mode: 0o600, Append: filestore.AppendCreate})
}

func writeExchangeFixtureFile(t testing.TB, path string, data []byte) error {
	t.Helper()
	file, err := createExchangeFixtureFile(t, path)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	if written != len(data) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	return errors.Join(writeErr, file.Close())
}
