package runnercontrol

import (
	"bytes"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type boundedLineWrite struct {
	pending *[]byte
	consume func([]byte) error
	data    []byte
	maximum int
}

func writeBoundedLines(request boundedLineWrite) (int, bool, error) {
	if request.pending == nil || request.maximum <= 0 || request.consume == nil {
		return 0, false, errors.Join(core.ErrPrimitiveContract, errors.New("bounded line compiler contract is invalid"))
	}
	written := 0
	for len(request.data) != 0 {
		end := bytes.IndexByte(request.data, '\n')
		if end < 0 {
			end = len(request.data)
		}
		if end > request.maximum-len(*request.pending) {
			return written, true, observationFailure("observation line exceeds the byte ceiling", core.ErrJSONContract)
		}
		*request.pending = append(*request.pending, request.data[:end]...)
		written += end
		request.data = request.data[end:]
		if len(request.data) == 0 {
			return written, false, nil
		}
		written++
		request.data = request.data[1:]
		if len(*request.pending) == 0 {
			return written, false, observationFailure("observation stream contains an empty line", core.ErrJSONContract)
		}
		if err := request.consume(*request.pending); err != nil {
			return written, false, err
		}
		*request.pending = (*request.pending)[:0]
	}
	return written, false, nil
}
