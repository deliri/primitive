package background_test

import (
	"github.com/deliri/primitive/v2026/background"
	"sync"
	"testing"
)

func TestMutexNativeZeroValueAndExclusion(t *testing.T) {
	t.Parallel()
	var first, second background.Mutex
	if !first.TryLock() {
		t.Fatal("zero mutex refused ownership")
	}
	if first.TryLock() {
		t.Fatal("held mutex admitted a second owner")
	}
	if !second.TryLock() {
		t.Fatal("independent mutex shared another mutex's ownership")
	}
	second.Unlock()
	first.Unlock()
	if !first.TryLock() {
		t.Fatal("unlocked mutex refused reacquisition")
	}
	first.Unlock()
	first.Lock()
	if first.TryLock() {
		t.Fatal("Lock failed to exclude a second owner")
	}
	first.Unlock()
}

func TestMutexNativeConcurrentReadModifyWrite(t *testing.T) {
	t.Parallel()
	var mutex background.Mutex
	var wait sync.WaitGroup
	value := 0
	const workers, operations = 32, 256
	for range workers {
		wait.Go(func() {
			for range operations {
				mutex.Lock()
				value++
				mutex.Unlock()
			}
		})
	}
	wait.Wait()
	if value != workers*operations {
		t.Fatalf("native protected total = %d, want %d", value, workers*operations)
	}
}

func TestMutexNativeTryLockObservesOtherGoroutineOwner(t *testing.T) {
	t.Parallel()
	var mutex background.Mutex
	owned, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { defer close(done); mutex.Lock(); close(owned); <-release; mutex.Unlock() }()
	<-owned
	acquired := mutex.TryLock()
	if acquired {
		mutex.Unlock()
	}
	close(release)
	<-done
	if acquired {
		t.Fatal("TryLock admitted a concurrent owner")
	}
	if !mutex.TryLock() {
		t.Fatal("joined owner's Unlock did not release mutex")
	}
	mutex.Unlock()
}

func FuzzMutexNativeTryLockMatchesGo(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 0, 1, 0, 1, 1, 0})
	f.Fuzz(func(t *testing.T, operations []byte) {
		var got background.Mutex
		var want sync.Mutex
		held := false
		for _, operation := range operations {
			if operation&1 != 0 && held {
				got.Unlock()
				want.Unlock()
				held = false
				continue
			}
			actual, expected := got.TryLock(), want.TryLock()
			if actual != expected {
				t.Fatalf("native TryLock = %t, Go = %t (held %t)", actual, expected, held)
			}
			held = held || actual
		}
		if held {
			got.Unlock()
			want.Unlock()
		}
	})
}
