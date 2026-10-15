package process

import (
	"context"
	"errors"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
)

// ExecutionScope owns native children from admission through the completion of
// their single wait, including pending stream cleanup. Its zero value is ready
// for use and it must not be copied after use. Completed executions are removed;
// it retains only capabilities for currently borrowed native resources.
type ExecutionScope struct {
	mu     sync.Mutex
	active []*Execution
}

// ActiveExecutionRequest borrows each active child synchronously. Visit must
// not call Begin, Wait, ActiveCount or VisitActive on this same scope. The
// callback may inspect or signal the execution; returning an error stops visits.
type ActiveExecutionRequest struct{ Visit func(*Execution) error }

// ExecutionWaitRequest keeps optional native cleanup inside the same custody
// lifetime. Complete runs after the single wait and before removal; its error
// joins the native wait cause. It is never called for a refused wait.
type ExecutionWaitRequest struct {
	Execution *Execution
	Complete  func(Result, error) error
}

func (r ExecutionWaitRequest) Validate() error { return r.Execution.usable() }

func (r ActiveExecutionRequest) Validate() error {
	if r.Visit == nil {
		return contractError("active execution visitor is nil")
	}
	return nil
}

// Begin registers a successfully started native child before a visitor can
// observe it. Failed admission retains no capability. Use this scope's Wait
// to complete the child; calling Execution.Wait directly bypasses its custody.
func (s *ExecutionScope) Begin(ctx context.Context, request Request) (*Execution, error) {
	if s == nil {
		return nil, contractError("execution scope is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	execution, err := Begin(ctx, request)
	if err != nil {
		if ctx != nil {
			err = errors.Join(err, context.Cause(ctx))
		}
		return nil, err
	}
	s.active = append(s.active, execution)
	return execution, nil
}

// Wait claims the native single wait while admission is locked. A duplicate or
// foreign wait cannot remove another caller's still-borrowed stream resources.
func (s *ExecutionScope) Wait(request ExecutionWaitRequest) (Result, error) {
	if s == nil {
		return Result{}, contractError("execution scope is nil")
	}
	if err := request.Validate(); err != nil {
		return Result{}, err
	}
	execution := request.Execution
	s.mu.Lock()
	held := false
	for _, active := range s.active {
		if active == execution {
			held = true
			break
		}
	}
	if !held {
		s.mu.Unlock()
		return Result{}, contractError("execution is not owned by this scope")
	}
	if err := execution.admitWait(); err != nil {
		s.mu.Unlock()
		return Result{}, err
	}
	s.mu.Unlock()
	result, err := execution.wait()
	defer s.release(execution)
	if request.Complete != nil {
		err = errors.Join(err, request.Complete(result, err))
	}
	if err != nil {
		err = errors.Join(err, context.Cause(execution.parent))
	}
	return result, err
}

func (s *ExecutionScope) release(execution *Execution) {
	s.mu.Lock()
	for index, active := range s.active {
		if active != execution {
			continue
		}
		last := len(s.active) - 1
		s.active[index], s.active[last] = s.active[last], nil
		s.active = s.active[:last]
		// Release the registry allocation when the last native resource completes.
		if last == 0 {
			s.active = nil
		}
		break
	}
	s.mu.Unlock()
}

// ActiveCount includes children whose process is gone but native stream cleanup
// is still in flight. It does not infer completion from signal delivery.
func (s *ExecutionScope) ActiveCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active)
}

// VisitActive lends one native capability at a time without cloning a registry.
// Admission and removal wait for the synchronous visitor to return. No historical
// execution population or iterator coroutine is created.
func (s *ExecutionScope) VisitActive(ctx context.Context, request ActiveExecutionRequest) error {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		if ctx != nil {
			err = errors.Join(err, context.Cause(ctx))
		}
		return err
	}
	if s == nil {
		return contractError("execution scope is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, execution := range s.active {
		if err := contextstate.Validate(ctx); err != nil {
			return errors.Join(err, context.Cause(ctx))
		}
		if err := request.Visit(execution); err != nil {
			return errors.Join(err, contextstate.Validate(ctx), context.Cause(ctx))
		}
	}
	return errors.Join(contextstate.Validate(ctx), context.Cause(ctx))
}
