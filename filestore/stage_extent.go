package filestore

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

func observeStageLength(file *os.File) (core.ByteLength, error) {
	info, err := file.Stat()
	if err != nil {
		return core.ByteLength{}, activationError(err)
	}
	extent, err := core.CheckedUint64FromInt64(info.Size())
	if err != nil {
		return core.ByteLength{}, sizeError(err)
	}
	length, err := core.NewByteLength(extent)
	if err != nil {
		return core.ByteLength{}, sizeError(err)
	}
	return length, nil
}
