package com

import (
	"runtime"
	"unsafe"

	"go-windows-api.local/runtime/winabi"
)

// IID_IUnknown is 00000000-0000-0000-C000-000000000046.
var IID_IUnknown = winabi.MustParseGUID("00000000-0000-0000-C000-000000000046")

// IUnknown is the native COM object header. Its sole field points to a vtable.
// Release must be called explicitly for every owned reference.
type IUnknown struct {
	VTable *IUnknownVTable
}

// IUnknownVTable preserves the required first three COM method slots.
type IUnknownVTable struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

// QueryInterface invokes vtable slot 0. object must point to writable pointer
// storage (for example, unsafe.Pointer(&result)).
func (object *IUnknown) QueryInterface(iid *winabi.GUID, result unsafe.Pointer) winabi.HRESULT {
	if object == nil || object.VTable == nil || iid == nil || result == nil {
		return winabi.E_POINTER
	}
	if object.VTable.QueryInterface == 0 {
		return winabi.E_NOINTERFACE
	}
	call, err := winabi.CallAddress(
		object.VTable.QueryInterface,
		uintptr(unsafe.Pointer(object)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(result),
	)
	runtime.KeepAlive(object)
	runtime.KeepAlive(iid)
	if err != nil {
		return winabi.E_NOTIMPL
	}
	return winabi.HRESULT(int32(uint32(call.R1)))
}

// AddRef invokes vtable slot 1 and returns the diagnostic reference count.
func (object *IUnknown) AddRef() uint32 {
	if object == nil || object.VTable == nil || object.VTable.AddRef == 0 {
		return 0
	}
	call, err := winabi.CallAddress(object.VTable.AddRef, uintptr(unsafe.Pointer(object)))
	runtime.KeepAlive(object)
	if err != nil {
		return 0
	}
	return uint32(call.R1)
}

// Release invokes vtable slot 2. The receiver must not be used afterward when
// the returned count is zero.
func (object *IUnknown) Release() uint32 {
	if object == nil || object.VTable == nil || object.VTable.Release == 0 {
		return 0
	}
	call, err := winabi.CallAddress(object.VTable.Release, uintptr(unsafe.Pointer(object)))
	runtime.KeepAlive(object)
	if err != nil {
		return 0
	}
	return uint32(call.R1)
}
