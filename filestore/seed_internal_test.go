package filestore

import (
	"bytes"
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
)

// Fixture construction emits bytes through the same public writer and reader
// whose semantic closure is fuzzed. The caller visibly owns its directory.
func FilestoreRoundTripSeedForTest(ctx context.Context, directory string) (emitted []byte, err error) {
	absolute, err := core.ParseAbsolutePath(directory)
	if err != nil {
		return nil, err
	}
	root, err := OpenRoot(ctx, absolute)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, root.Close())
		if err != nil {
			emitted = nil
		}
	}()
	target, err := core.ParseRelativePath("seed")
	if err != nil {
		return nil, err
	}
	stage, err := core.ParseRelativePath("stage")
	if err != nil {
		return nil, err
	}
	input := []byte{0, 255, 1, 127}
	maximum, err := core.NewByteCount(uint64(len(input)))
	if err != nil {
		return nil, err
	}
	request := WriteRequest{Source: bytes.NewReader(input), Location: Location{Root: root, Path: target}, Temporary: stage, Mode: 0o600, Install: InstallCreate, MaximumBytes: maximum}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	recovery, err := Write(ctx, request)
	if err != nil {
		return nil, err
	}
	if recovery != (CommitRequest{}) {
		return nil, core.ErrFilestoreContract
	}
	var output bytes.Buffer
	count, err := Read(ctx, ReadRequest{Location: request.Location, Destination: &output, MaximumBytes: maximum})
	if err != nil {
		return nil, err
	}
	if count.Uint64() != uint64(len(input)) || !bytes.Equal(output.Bytes(), input) {
		return nil, core.ErrFilestoreContract
	}
	return output.Bytes(), nil
}
