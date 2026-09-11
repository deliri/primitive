package upgrade

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// metadataBuffer owns exactly one canonical selector/trial document. Its
// storage cannot grow with the file, including on malformed input. These are
// the same nominal document bounds enforced by the persistence decoders.
// Artifact bytes never enter this buffer.
type metadataBuffer struct {
	data    [trialDocumentMaximumBytes]byte
	length  int
	maximum int
}

func (b *metadataBuffer) Write(p []byte) (int, error) {
	if b.maximum <= 0 || b.maximum > len(b.data) || b.length < 0 || b.length > b.maximum {
		return 0, contractError(core.ErrJSONContract, diagnosticJSON)
	}
	if len(p) > b.maximum-b.length {
		return 0, contractError(core.ErrJSONContract, diagnosticJSON)
	}
	n := copy(b.data[b.length:b.maximum], p)
	b.length += n
	return n, nil
}

func readMetadata(ctx context.Context, root *os.Root, path core.RelativePath, maximum int) ([]byte, error) {
	destination := metadataBuffer{maximum: maximum}
	_, err := filestore.Read(ctx, filestore.ReadRequest{Destination: &destination, Location: filestore.Location{Root: root, Path: path}})
	if err != nil {
		return nil, persistenceError(err)
	}
	return destination.data[:destination.length], nil
}
