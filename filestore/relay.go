package filestore

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// RelayRequest lends both directions of a byte relay. Near is one side's
// input and output, Far the other's: a relay child passes its own standard
// streams as Near and its parent's lent streams as Far. Relay closes only
// FarOutput, once NearInput ends; every other stream stays the caller's.
type RelayRequest struct {
	NearInput  io.Reader
	NearOutput io.Writer
	FarInput   io.Reader
	FarOutput  io.WriteCloser
}

func (r RelayRequest) Validate() error {
	if r.NearInput == nil || r.NearOutput == nil || r.FarInput == nil || r.FarOutput == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// RelayResult records each direction's native outcome after both joined.
// Its zero value is invalid: it proves no joined relay.
type RelayResult struct {
	UpwardError   error
	DownwardError error
	CloseError    error
	joined        bool
}

func (r RelayResult) Validate() error {
	if !r.joined {
		return core.ErrFilestoreContract
	}
	return nil
}

type relayUpwardResult struct {
	copyError  error
	closeError error
}

// Relay copies NearInput to FarOutput and FarInput to NearOutput at once, as
// a protocol that interleaves requests and replies requires. The streams own
// the lifetime, as with cat: the downward copy ends when FarInput ends, the
// upward copy when NearInput ends, and Relay returns only after both joined.
// A blocked read on a shared descriptor cannot be interrupted without
// changing its other owners, so there is no cancellation path.
func Relay(request RelayRequest) (RelayResult, error) {
	if err := request.Validate(); err != nil {
		return RelayResult{}, err
	}
	upward := make(chan relayUpwardResult, 1)
	go func() {
		_, copyErr := io.Copy(request.FarOutput, request.NearInput)
		upward <- relayUpwardResult{copyError: copyErr, closeError: request.FarOutput.Close()}
	}()
	_, downErr := io.Copy(request.NearOutput, request.FarInput)
	up := <-upward
	result := RelayResult{UpwardError: up.copyError, DownwardError: downErr, CloseError: up.closeError, joined: true}
	return result, errors.Join(result.Validate(), result.UpwardError, result.DownwardError, result.CloseError)
}

var _ core.Validatable = RelayRequest{}
var _ core.Validatable = RelayResult{}
