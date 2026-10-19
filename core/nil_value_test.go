package core

import (
	"os"
	"testing"
)

func TestIsNilPreservesGoInterfaceAndContainerPresence(t *testing.T) {
	t.Parallel()
	var absent error
	var pathError *os.PathError
	var wrapped error = pathError
	if !IsNil(absent) || !IsNil(pathError) || !IsNil(wrapped) {
		t.Fatal("Go nil capability admitted as present")
	}
	value := 0
	if IsNil(&value) || IsNil(value) || IsNil("") || IsNil(struct{}{}) {
		t.Fatal("present zero/scalar was classified as absent")
	}
	var pointer *int
	var data []byte
	var dictionary map[string]int
	var channel chan int
	var function func()
	if !IsNil(pointer) || !IsNil(data) || !IsNil(dictionary) || !IsNil(channel) || !IsNil(function) {
		t.Fatal("native typed nil kind was classified as present")
	}
	if IsNil([]byte{}) || IsNil(map[string]int{}) || IsNil(make(chan int)) || IsNil(func() {}) || IsNil([0]byte{}) {
		t.Fatal("present empty value was classified as absent")
	}
}

func FuzzIsNilMatchesNativeRepresentation(f *testing.F) {
	f.Add(uint8(0), false, int64(0))
	f.Add(uint8(1), true, int64(1))
	f.Add(uint8(2), false, int64(-1))
	f.Add(uint8(3), true, int64(0))
	f.Add(uint8(4), false, int64(42))
	f.Fuzz(func(t *testing.T, kind uint8, present bool, value int64) {
		var got bool
		switch kind % 5 {
		case 0:
			var pointer *int64
			if present {
				pointer = &value
			}
			got = IsNil(pointer)
		case 1:
			var data []int64
			if present {
				data = []int64{value}
			}
			got = IsNil(data)
		case 2:
			var function func() int64
			if present {
				function = func() int64 { return value }
			}
			got = IsNil(function)
		case 3:
			var cause *os.PathError
			if present {
				cause = &os.PathError{Op: "native representation", Path: "fixture"}
			}
			var capability error = cause
			got = IsNil(capability)
		case 4:
			var channel chan int64
			if present {
				channel = make(chan int64)
			}
			got = IsNil(channel)
		}
		if got == present || IsNil(value) {
			t.Fatalf("native kind=%d present=%v scalar=%d: absent=%v", kind%5, present, value, got)
		}
	})
}
