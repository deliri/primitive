package hostfacts

import (
	"context"
	"iter"
	"runtime"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// GoProgramCounter is one ephemeral return-program coordinate in this process.
type GoProgramCounter uintptr

// Validate refuses the absence of a recorded program coordinate.
func (pc GoProgramCounter) Validate() error {
	if pc == 0 {
		return core.ErrHostFactsObservation
	}
	return nil
}

// ResolveGoProgramCounter resolves an already recorded return-program coordinate
// in this process. Go owns inline-frame expansion and symbol lookup; Primitive
// returns the first observed frame without retaining a stack inventory.
func ResolveGoProgramCounter(ctx context.Context, pc GoProgramCounter) (GoStackFrame, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return GoStackFrame{}, err
	}
	if err := pc.Validate(); err != nil {
		return GoStackFrame{}, err
	}
	frame, _ := runtime.CallersFrames([]uintptr{uintptr(pc)}).Next()
	observation := GoStackFrame{PC: GoProgramCounter(frame.PC), Function: GoFunctionName(frame.Function), File: GoSourceFile(frame.File), Line: frame.Line}
	if err := contextstate.Validate(ctx); err != nil {
		return GoStackFrame{}, err
	}
	if err := observation.Validate(); err != nil {
		return GoStackFrame{}, err
	}
	return observation, nil
}

// GoFunctionName is the runtime's symbol coordinate; an empty value means that
// Go could not resolve the symbol for this observed program counter.
type GoFunctionName string

// GoSourceFile is the runtime's source coordinate; it may be empty when Go
// cannot resolve a source file. It is not a filesystem execution authority.
type GoSourceFile string

// GoStackFrame is one runtime observation, with Go's expanded inline
// frame semantics. Line zero means unavailable; the program counter is required.
type GoStackFrame struct {
	PC       GoProgramCounter
	Function GoFunctionName
	File     GoSourceFile
	Line     int
}

// Validate refuses fabricated zero frames and impossible negative line numbers.
func (f GoStackFrame) Validate() error {
	if err := f.PC.Validate(); err != nil {
		return err
	}
	if f.Line < 0 {
		return core.ErrHostFactsObservation
	}
	return nil
}

const goStackWindowFrames = 32

// CurrentGoStackFrames observes the consuming goroutine's real Go stack in
// fixed windows. It retains no stack inventory and imposes no total-frame
// ceiling. Stop consuming to stop observation. Go owns frame expansion and
// symbol lookup; callers own formatting, retention and diagnostic policy.
func CurrentGoStackFrames(ctx context.Context) iter.Seq2[GoStackFrame, error] {
	return func(yield func(GoStackFrame, error) bool) {
		var pcs [goStackWindowFrames]uintptr
		for skip := 1; ; {
			if err := contextstate.Validate(ctx); err != nil {
				yield(GoStackFrame{}, err)
				return
			}
			n := runtime.Callers(skip, pcs[:])
			if n == 0 {
				return
			}
			frames := runtime.CallersFrames(pcs[:n])
			for {
				frame, more := frames.Next()
				observation := GoStackFrame{PC: GoProgramCounter(frame.PC), Function: GoFunctionName(frame.Function), File: GoSourceFile(frame.File), Line: frame.Line}
				if err := contextstate.Validate(ctx); err != nil {
					yield(GoStackFrame{}, err)
					return
				}
				if err := observation.Validate(); err != nil {
					yield(GoStackFrame{}, err)
					return
				}
				if !yield(observation, nil) {
					return
				}
				if !more {
					break
				}
			}
			skip += n
		}
	}
}

var _ core.Validatable = GoStackFrame{}
var _ core.Validatable = GoProgramCounter(0)
