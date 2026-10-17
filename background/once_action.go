package background

import (
	"context"
	"sync"

	"github.com/deliri/primitive/v2026/core"
)

// OnceActionRequest transfers one action to native sync.Once custody. It adds
// no goroutine, deadline, retry or product state. Run must not recursively run
// the same action, as with Go's sync.Once.
type OnceActionRequest struct{ Run func() error }

func (r OnceActionRequest) Validate() error {
	if r.Run == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

// OnceAction is a copyable handle to one shared native action and its first
// observation. Every Run waits for that action to finish; copies cannot create
// another execution or replace its first failure with a later success.
type OnceAction struct{ action *onceAction }

type onceAction struct {
	once   sync.Once
	run    func() error
	result error
}

func NewOnceAction(request OnceActionRequest) (OnceAction, error) {
	if err := request.Validate(); err != nil {
		return OnceAction{}, err
	}
	return OnceAction{action: &onceAction{run: request.Run, result: core.ErrBackgroundStopped}}, nil
}

func (a OnceAction) Validate() error {
	if a.action == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

// Run preserves native error causes and contains a panic. If an action calls
// Goexit, that caller unwinds normally, every waiting caller is released by
// sync.Once, and later callers observe Stopped rather than invented success.
func (a OnceAction) Run() error {
	if err := a.Validate(); err != nil {
		return err
	}
	a.action.once.Do(func() {
		defer func() { a.action.run = nil }()
		a.action.result = invoke(context.Background(), func(context.Context) error { return a.action.run() })
	})
	return a.action.result
}
