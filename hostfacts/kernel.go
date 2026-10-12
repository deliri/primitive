package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// KernelText is one native kernel identity field, without framing or controls.
// Linux supplies fixed uname fields; Darwin supplies native sysctl scalars.
// Neither path reads or limits a process-output stream.
type KernelText struct{ value string }

func (v KernelText) String() string { return v.value }

func (v KernelText) Validate() error {
	if v.value == "" || !utf8.ValidString(v.value) || strings.ContainsFunc(v.value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return core.ErrHostFactsContract
	}
	return nil
}

// KernelObservation excludes the host name and detailed build description.
// Products decide how much of the native release and machine facts to publish.
type KernelObservation struct {
	System  KernelText
	Release KernelText
	Machine KernelText
}

func (o KernelObservation) Validate() error {
	return errors.Join(o.System.Validate(), o.Release.Validate(), o.Machine.Validate())
}

// ObserveKernel observes uname through the native platform API, without PATH,
// shell configuration, subprocess output or a retained command transcript.
// Cancellation and unavailable platforms return no observation.
func ObserveKernel(ctx context.Context) (KernelObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return KernelObservation{}, err
	}
	observation, err := observeKernel()
	if err := errors.Join(err, contextstate.Validate(ctx)); err != nil {
		identity := core.ErrHostFactsObservation
		if errors.Is(err, core.ErrHostFactsUnsupported) {
			identity = core.ErrHostFactsUnsupported
		}
		return KernelObservation{}, fail(OperationKernel, identity, err)
	}
	if err := observation.Validate(); err != nil {
		return KernelObservation{}, fail(OperationKernel, core.ErrHostFactsObservation, err)
	}
	return observation, nil
}

func admitKernelField(field []byte) (KernelText, error) {
	terminator := bytes.IndexByte(field, 0)
	if terminator < 0 {
		return KernelText{}, core.ErrHostFactsObservation
	}
	text := KernelText{value: string(field[:terminator])}
	if err := text.Validate(); err != nil {
		return KernelText{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return text, nil
}
