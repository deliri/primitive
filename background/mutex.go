package background

import "sync"

// Mutex delegates mutual exclusion to Go's sync.Mutex. Its zero value is ready
// to use. As with sync.Mutex, it must not be copied after first use.
// It owns no worker, queue, deadline, identity or product lifecycle.
type Mutex struct{ native sync.Mutex }

func (m *Mutex) Lock()         { m.native.Lock() }
func (m *Mutex) Unlock()       { m.native.Unlock() }
func (m *Mutex) TryLock() bool { return m.native.TryLock() }

var _ sync.Locker = (*Mutex)(nil)
