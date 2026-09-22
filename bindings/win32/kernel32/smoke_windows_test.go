//go:build windows

package kernel32

import (
	"testing"

	"go-windows-api.local/bindings/win32/foundation"
	"go-windows-api.local/runtime/winabi"
)

func TestNondestructiveKernel32Slice(t *testing.T) {
	if GetCurrentProcessId() == 0 || GetCurrentThreadId() == 0 {
		t.Fatal("native process or thread ID was zero")
	}
	var counter foundation.LARGE_INTEGER
	result, lastError := QueryPerformanceCounter(&counter)
	if err := winabi.ErrorIfFalse(int32(result), lastError); err != nil {
		t.Fatalf("QueryPerformanceCounter: %v", err)
	}
	var now foundation.SYSTEMTIME
	GetSystemTime(&now)
	if now.Year < 2000 || now.Month == 0 || now.Month > 12 {
		t.Fatalf("invalid SYSTEMTIME: %#v", now)
	}

	memory, lastError := VirtualAlloc(nil, 4096, MEM_COMMIT|MEM_RESERVE, PAGE_READWRITE)
	if err := winabi.ErrorIfZero(uintptr(memory), lastError); err != nil {
		t.Fatalf("VirtualAlloc: %v", err)
	}
	freed, lastError := VirtualFree(memory, 0, MEM_RELEASE)
	if err := winabi.ErrorIfFalse(int32(freed), lastError); err != nil {
		t.Fatalf("VirtualFree: %v", err)
	}

	if IsGetSystemTimePreciseAsFileTimeAvailable() {
		var precise foundation.FILETIME
		if !GetSystemTimePreciseAsFileTime(&precise) {
			t.Fatal("resolved optional API could not be called")
		}
	}
}
