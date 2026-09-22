package winrt

import (
	"runtime"
	"strings"
	"unsafe"

	"go-windows-api.local/runtime/winabi"
)

var (
	// IID_IActivationFactory identifies Windows.Foundation.IActivationFactory.
	IID_IActivationFactory = winabi.MustParseGUID("00000035-0000-0000-C000-000000000046")

	procRoGetActivationFactory = combase.MustProc("RoGetActivationFactory")
	procRoActivateInstance     = combase.MustProc("RoActivateInstance")
)

// IActivationFactory is the WinRT activation factory interface.
type IActivationFactory struct {
	VTable *IActivationFactoryVTable
}

// IActivationFactoryVTable extends IInspectable with ActivateInstance.
type IActivationFactoryVTable struct {
	IInspectableVTable
	ActivateInstance uintptr
}

// QueryInterface delegates to the inherited IInspectable/IUnknown slot.
func (factory *IActivationFactory) QueryInterface(iid *winabi.GUID, result unsafe.Pointer) winabi.HRESULT {
	return (*IInspectable)(unsafe.Pointer(factory)).QueryInterface(iid, result)
}

// AddRef delegates to IUnknown.
func (factory *IActivationFactory) AddRef() uint32 {
	return (*IInspectable)(unsafe.Pointer(factory)).AddRef()
}

// Release delegates to IUnknown.
func (factory *IActivationFactory) Release() uint32 {
	return (*IInspectable)(unsafe.Pointer(factory)).Release()
}

// Activate creates a default-constructible runtime-class instance.
func (factory *IActivationFactory) Activate() (*IInspectable, winabi.HRESULT) {
	if factory == nil || factory.VTable == nil || factory.VTable.ActivateInstance == 0 {
		return nil, winabi.E_POINTER
	}
	var result *IInspectable
	call, err := winabi.CallAddress(
		factory.VTable.ActivateInstance,
		uintptr(unsafe.Pointer(factory)),
		uintptr(unsafe.Pointer(&result)),
	)
	runtime.KeepAlive(factory)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return nil, status
	}
	return result, status
}

// GetActivationFactory obtains the standard IActivationFactory for className.
// Runtime class names reject embedded NUL even though HSTRING itself can hold it.
func GetActivationFactory(className string) (*IActivationFactory, winabi.HRESULT) {
	if className == "" || strings.IndexByte(className, 0) >= 0 {
		return nil, winabi.E_INVALIDARG
	}
	name, err := NewHString(className)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	defer name.Close()
	var factory *IActivationFactory
	call, err := procRoGetActivationFactory.TryCall(
		name.Handle(),
		uintptr(unsafe.Pointer(&IID_IActivationFactory)),
		uintptr(unsafe.Pointer(&factory)),
	)
	runtime.KeepAlive(name)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return nil, status
	}
	return factory, status
}

// ActivateInstance asks WinRT to activate a default-constructible runtime class.
func ActivateInstance(className string) (*IInspectable, winabi.HRESULT) {
	if className == "" || strings.IndexByte(className, 0) >= 0 {
		return nil, winabi.E_INVALIDARG
	}
	name, err := NewHString(className)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	defer name.Close()
	var instance *IInspectable
	call, err := procRoActivateInstance.TryCall(
		name.Handle(),
		uintptr(unsafe.Pointer(&instance)),
	)
	runtime.KeepAlive(name)
	if err != nil {
		return nil, winabi.E_NOTIMPL
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return nil, status
	}
	return instance, status
}
