package winrt

import (
	"runtime"

	"github.com/zzuf/GoWin-AFO/runtime/winabi"
)

// RO_INIT_TYPE is the native WinRT apartment initialization type.
type RO_INIT_TYPE uint32

const (
	RO_INIT_SINGLETHREADED RO_INIT_TYPE = 0
	RO_INIT_MULTITHREADED  RO_INIT_TYPE = 1
)

var (
	procRoInitialize   = combase.MustProc("RoInitialize")
	procRoUninitialize = combase.MustProc("RoUninitialize")
)

// Initialize initializes WinRT on the current OS thread. Goroutine callers
// should use EnterApartment or otherwise pin the goroutine themselves.
func Initialize(kind RO_INIT_TYPE) winabi.HRESULT {
	call, err := procRoInitialize.TryCall(uintptr(kind))
	if err != nil {
		return winabi.E_NOTIMPL
	}
	return winabi.HRESULT(int32(uint32(call.R1)))
}

// Uninitialize balances one successful Initialize call on the current thread.
func Uninitialize() {
	_, _ = procRoUninitialize.TryCall()
}

// Apartment owns a successful RoInitialize call and an OS-thread lock. It must
// not be copied or moved to another goroutine.
type Apartment struct {
	noCopy noCopy
	active bool
}

// EnterApartment locks the goroutine to its current OS thread and initializes
// WinRT. The exact success HRESULT is retained for the caller.
func EnterApartment(kind RO_INIT_TYPE) (*Apartment, winabi.HRESULT) {
	runtime.LockOSThread()
	status := Initialize(kind)
	if status.Failed() {
		runtime.UnlockOSThread()
		return nil, status
	}
	return &Apartment{active: true}, status
}

// Close balances initialization and releases the OS-thread lock.
func (apartment *Apartment) Close() error {
	if apartment == nil || !apartment.active {
		return nil
	}
	Uninitialize()
	apartment.active = false
	runtime.UnlockOSThread()
	return nil
}
