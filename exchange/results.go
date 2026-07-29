package exchange

import "github.com/deliri/primitive/v2026/core"

// ResponseMetadata is the validated result shared by all client families.
type ResponseMetadata struct {
	Headers  CapturedHeaders
	Bytes    core.ByteLength
	Attempts uint64
	Status   core.HTTPStatusCode
}

// Validate checks status, bounded captured headers, and attempt count.
func (m ResponseMetadata) Validate() error {
	if err := m.Status.Validate(); err != nil {
		return responseError(err)
	}
	if err := m.Headers.Validate(); err != nil {
		return responseError(err)
	}
	if m.Attempts == 0 {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// JSONResponse is one strictly decoded and validated JSON response.
type JSONResponse[Body core.Validatable] struct {
	Body     Body
	Metadata ResponseMetadata
}

// Validate checks metadata and the decoded caller-owned body.
func (r JSONResponse[Body]) Validate() error {
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if err := validateCallerValue(r.Body); err != nil {
		return responseError(err)
	}
	return nil
}

// BoundedResponse is one aggregate byte response.
type BoundedResponse struct {
	Body     []byte
	Metadata ResponseMetadata
}

// Validate checks metadata and byte accounting.
func (r BoundedResponse) Validate() error {
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if r.Metadata.Bytes.Uint64() != uint64(len(r.Body)) {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// StreamResponse reports a completed streaming transfer.
type StreamResponse struct {
	Metadata ResponseMetadata
}

// Validate checks the completed transfer metadata.
func (r StreamResponse) Validate() error {
	return r.Metadata.Validate()
}

var (
	_ core.Validatable = ResponseMetadata{}
	_ core.Validatable = BoundedResponse{}
	_ core.Validatable = StreamResponse{}
)
