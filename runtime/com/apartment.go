package com

import (
	"runtime"

	"github.com/zzuf/GoWin-AFO/runtime/winabi"
)

// COINIT is the native COM apartment initialization flag type.
type COINIT uint32

const (
	COINIT_MULTITHREADED     COINIT = 0
	COINIT_APARTMENTTHREADED COINIT = 2
	COINIT_DISABLE_OLE1DDE   COINIT = 4
	COINIT_SPEED_OVER_MEMORY COINIT = 8
)

var (
	ole32              = winabi.MustSystemDLL("ole32.dll")
	procCoInitializeEx = ole32.MustProc("CoInitializeEx")
	procCoUninitialize = ole32.MustProc("CoUninitialize")
)

// Initialize calls CoInitializeEx for the current OS thread. Callers that use
// goroutines must pin the goroutine with runtime.LockOSThread, or use
// EnterApartment, and balance every successful call with Uninitialize.
func Initialize(flags COINIT) winabi.HRESULT {
	call, err := procCoInitializeEx.TryCall(0, uintptr(flags))
	if err != nil {
		return winabi.E_NOTIMPL
	}
	return winabi.HRESULT(int32(uint32(call.R1)))
}

// Uninitialize calls CoUninitialize on the current OS thread.
func Uninitialize() {
	_, _ = procCoUninitialize.TryCall()
}

// Apartment owns one successful CoInitializeEx call and keeps its goroutine on
// that OS thread until Close. It must not be copied or transferred to another
// goroutine.
type Apartment struct {
	noCopy noCopy
	active bool
}

// EnterApartment pins the current goroutine, initializes COM, and returns an
// explicit owner. The HRESULT is preserved even when successful (for example,
// S_FALSE). On failure the goroutine is unpinned before returning.
func EnterApartment(flags COINIT) (*Apartment, winabi.HRESULT) {
	runtime.LockOSThread()
	status := Initialize(flags)
	if status.Failed() {
		runtime.UnlockOSThread()
		return nil, status
	}
	return &Apartment{active: true}, status
}

// Close balances the successful initialization and unpins the goroutine. It is
// idempotent but must run on the goroutine that called EnterApartment.
func (apartment *Apartment) Close() error {
	if apartment == nil || !apartment.active {
		return nil
	}
	Uninitialize()
	apartment.active = false
	runtime.UnlockOSThread()
	return nil
}
