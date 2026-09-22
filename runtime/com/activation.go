package com

import (
	"runtime"
	"unsafe"

	"go-windows-api.local/runtime/winabi"
)

// CLSCTX is the native COM activation-context flags type.
type CLSCTX uint32

const (
	CLSCTX_INPROC_SERVER  CLSCTX = 0x1
	CLSCTX_INPROC_HANDLER CLSCTX = 0x2
	CLSCTX_LOCAL_SERVER   CLSCTX = 0x4
)

var (
	procCoCreateInstance = ole32.MustProc("CoCreateInstance")

	// CLSID_StdGlobalInterfaceTable is the OS-provided standard Global
	// Interface Table class used by the non-destructive runtime smoke test.
	CLSID_StdGlobalInterfaceTable = winabi.MustParseGUID("00000323-0000-0000-C000-000000000046")
)

// CreateInstance calls CoCreateInstance and writes the requested interface to
// result, which must point to writable pointer storage. No finalizer is added;
// the caller owns and must Release a successful interface reference.
func CreateInstance(clsid *winabi.GUID, outer *IUnknown, context CLSCTX, iid *winabi.GUID, result unsafe.Pointer) winabi.HRESULT {
	if clsid == nil || iid == nil || result == nil {
		return winabi.E_POINTER
	}
	call, err := procCoCreateInstance.TryCall(
		uintptr(unsafe.Pointer(clsid)),
		uintptr(unsafe.Pointer(outer)),
		uintptr(context),
		uintptr(unsafe.Pointer(iid)),
		uintptr(result),
	)
	runtime.KeepAlive(clsid)
	runtime.KeepAlive(outer)
	runtime.KeepAlive(iid)
	if err != nil {
		return winabi.E_NOTIMPL
	}
	return winabi.HRESULT(int32(uint32(call.R1)))
}

// CreateIUnknown requests IID_IUnknown from a COM class and returns an owned
// reference that the caller must Release.
func CreateIUnknown(clsid *winabi.GUID, context CLSCTX) (*IUnknown, winabi.HRESULT) {
	var result *IUnknown
	status := CreateInstance(clsid, nil, context, &IID_IUnknown, unsafe.Pointer(&result))
	if status.Failed() {
		return nil, status
	}
	return result, status
}
