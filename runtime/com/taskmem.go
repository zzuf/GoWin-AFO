package com

import (
	"unsafe"

	"github.com/zzuf/GoWin-AFO/runtime/winabi"
)

var (
	procCoTaskMemAlloc = ole32.MustProc("CoTaskMemAlloc")
	procCoTaskMemFree  = ole32.MustProc("CoTaskMemFree")
)

// TaskMemory owns a CoTaskMemAlloc allocation. It has no finalizer and must not
// be copied; call Close explicitly.
type TaskMemory struct {
	noCopy noCopy
	value  unsafe.Pointer
	size   uintptr
}

// AllocTaskMemory allocates a COM task allocator block. A zero-size allocation
// is allowed to return nil without being treated as failure.
func AllocTaskMemory(size uintptr) (*TaskMemory, error) {
	call, err := procCoTaskMemAlloc.TryCall(size)
	if err != nil {
		return nil, err
	}
	if call.R1 == 0 && size != 0 {
		return nil, errOutOfMemory
	}
	return &TaskMemory{value: winabi.UnsafePointerFromUintptr(call.R1), size: size}, nil
}

// Pointer returns a borrowed native pointer valid until Close.
func (memory *TaskMemory) Pointer() unsafe.Pointer {
	if memory == nil {
		return nil
	}
	return memory.value
}

// Size returns the requested allocation size.
func (memory *TaskMemory) Size() uintptr {
	if memory == nil {
		return 0
	}
	return memory.size
}

// Close releases the allocation once.
func (memory *TaskMemory) Close() error {
	if memory == nil || memory.value == nil {
		return nil
	}
	if err := FreeTaskMemory(memory.value); err != nil {
		return err
	}
	memory.value = nil
	memory.size = 0
	return nil
}

// FreeTaskMemory releases a pointer returned through a COM out parameter.
func FreeTaskMemory(pointer unsafe.Pointer) error {
	if pointer == nil {
		return nil
	}
	_, err := procCoTaskMemFree.TryCall(uintptr(pointer))
	return err
}
