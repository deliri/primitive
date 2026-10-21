//go:build !unix

package process

import "io"

func pipeErrorDestination(destination io.Writer) (io.Writer, func() error, error) {
	return destination, nil, nil
}
