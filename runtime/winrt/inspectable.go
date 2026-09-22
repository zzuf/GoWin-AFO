package winrt

import (
	"runtime"
	"unsafe"

	"github.com/zzuf/GoWin-AFO/runtime/com"
	"github.com/zzuf/GoWin-AFO/runtime/winabi"
)

// TrustLevel is the native WinRT trust level enum.
type TrustLevel int32

const (
	BaseTrust    TrustLevel = 0
	PartialTrust TrustLevel = 1
	FullTrust    TrustLevel = 2
)

// IID_IInspectable identifies the base WinRT interface.
var IID_IInspectable = winabi.MustParseGUID("AF86E2E0-B12D-4C6A-9C5A-D7AA65101E90")

// IInspectable is the base WinRT interface object.
type IInspectable struct {
	VTable *IInspectableVTable
}

// IInspectableVTable preserves IUnknown slots followed by the three WinRT
// inspection slots in native method order.
type IInspectableVTable struct {
	com.IUnknownVTable
	GetIids             uintptr
	GetRuntimeClassName uintptr
	GetTrustLevel       uintptr
}

// QueryInterface delegates to the inherited IUnknown slot.
func (object *IInspectable) QueryInterface(iid *winabi.GUID, result unsafe.Pointer) winabi.HRESULT {
	return (*com.IUnknown)(unsafe.Pointer(object)).QueryInterface(iid, result)
}

// AddRef delegates to the inherited IUnknown slot.
func (object *IInspectable) AddRef() uint32 {
	return (*com.IUnknown)(unsafe.Pointer(object)).AddRef()
}

// Release delegates to the inherited IUnknown slot.
func (object *IInspectable) Release() uint32 {
	return (*com.IUnknown)(unsafe.Pointer(object)).Release()
}

// IIDs obtains and copies the interface IID list, then releases the native
// CoTaskMem allocation before returning.
func (object *IInspectable) IIDs() ([]winabi.GUID, winabi.HRESULT) {
	if object == nil || object.VTable == nil || object.VTable.GetIids == 0 {
		return nil, winabi.E_POINTER
	}
	var count uint32
	var values *winabi.GUID
	call, err := winabi.CallAddress(
		object.VTable.GetIids,
		uintptr(unsafe.Pointer(object)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&values)),
	)
	runtime.KeepAlive(object)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return nil, status
	}
	if count == 0 {
		return nil, status
	}
	if values == nil {
		return nil, winabi.E_FAIL
	}
	sliceLength, err := winabi.CheckedSliceLength(count, unsafe.Sizeof(winabi.GUID{}))
	if err != nil {
		_ = com.FreeTaskMemory(unsafe.Pointer(values))
		return nil, winabi.E_FAIL
	}
	result := append([]winabi.GUID(nil), unsafe.Slice(values, sliceLength)...)
	_ = com.FreeTaskMemory(unsafe.Pointer(values))
	return result, status
}

// RuntimeClassName returns an owned HSTRING supplied by the interface.
func (object *IInspectable) RuntimeClassName() (*HString, winabi.HRESULT) {
	if object == nil || object.VTable == nil || object.VTable.GetRuntimeClassName == 0 {
		return nil, winabi.E_POINTER
	}
	var name uintptr
	call, err := winabi.CallAddress(
		object.VTable.GetRuntimeClassName,
		uintptr(unsafe.Pointer(object)),
		uintptr(unsafe.Pointer(&name)),
	)
	runtime.KeepAlive(object)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return nil, status
	}
	return ownedHString(name), status
}

// GetTrustLevel returns the native trust level without changing its meaning.
func (object *IInspectable) GetTrustLevel() (TrustLevel, winabi.HRESULT) {
	if object == nil || object.VTable == nil || object.VTable.GetTrustLevel == 0 {
		return 0, winabi.E_POINTER
	}
	var trust TrustLevel
	call, err := winabi.CallAddress(
		object.VTable.GetTrustLevel,
		uintptr(unsafe.Pointer(object)),
		uintptr(unsafe.Pointer(&trust)),
	)
	runtime.KeepAlive(object)
	if err != nil {
		return 0, winabi.E_NOTIMPL
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	return trust, status
}
