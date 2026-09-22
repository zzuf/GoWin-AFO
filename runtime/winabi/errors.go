package winabi

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedPlatform reports that a Windows-only operation was invoked
	// from a non-Windows build.
	ErrUnsupportedPlatform = errors.New("winabi: Windows ABI is unavailable on this platform")
	// ErrInvalidDLLName reports a DLL name that could influence search paths.
	ErrInvalidDLLName = errors.New("winabi: invalid System32 DLL name")
	// ErrInvalidProcName reports a malformed export name.
	ErrInvalidProcName = errors.New("winabi: invalid procedure name")
	// ErrInvalidAddress reports an attempt to invoke a nil function address.
	ErrInvalidAddress = errors.New("winabi: nil procedure address")
	// ErrNativeFailureWithoutLastError is returned when a documented failure
	// condition occurred but the API did not set a last-error value.
	ErrNativeFailureWithoutLastError = errors.New("winabi: native call failed without setting last error")
	// ErrLengthOverflow reports native length data that cannot be represented by
	// the current Go architecture without overflow.
	ErrLengthOverflow = errors.New("winabi: native buffer length overflows Go address space")
)

// LastError is a raw Win32 thread last-error value. A zero value means
// ERROR_SUCCESS; callers must inspect the API's documented failure sentinel
// before interpreting LastError.
type LastError uint32

const (
	ERROR_SUCCESS           LastError = 0
	ERROR_INVALID_HANDLE    LastError = 6
	ERROR_NOT_ENOUGH_MEMORY LastError = 8
	ERROR_PROC_NOT_FOUND    LastError = 127
)

func (e LastError) Error() string {
	return fmt.Sprintf("Win32 error %d (0x%08X)", uint32(e), uint32(e))
}

// CheckedSliceLength validates a native UINT32 element count before it reaches
// unsafe.Slice. elementSize must be the native size of one element.
func CheckedSliceLength(count uint32, elementSize uintptr) (int, error) {
	if elementSize == 0 {
		return 0, ErrLengthOverflow
	}
	maximumInt := uint64(^uint(0) >> 1)
	maximumAddress := uint64(^uintptr(0))
	if uint64(count) > maximumInt || uint64(count) > maximumAddress/uint64(elementSize) {
		return 0, ErrLengthOverflow
	}
	return int(count), nil
}

// Err converts a nonzero last-error value to error.
func (e LastError) Err() error {
	if e == ERROR_SUCCESS {
		return nil
	}
	return e
}

// ErrorIfFalse interprets lastError only after result has been established as
// the documented BOOL failure value.
func ErrorIfFalse(result int32, lastError LastError) error {
	if result != 0 {
		return nil
	}
	if lastError == ERROR_SUCCESS {
		return ErrNativeFailureWithoutLastError
	}
	return lastError
}

// ErrorIfZero interprets lastError only after a zero integer or pointer return
// has been established as the documented failure value.
func ErrorIfZero(result uintptr, lastError LastError) error {
	if result != 0 {
		return nil
	}
	if lastError == ERROR_SUCCESS {
		return ErrNativeFailureWithoutLastError
	}
	return lastError
}

// ErrorIfInvalidHandle interprets lastError only when result equals the
// pointer-sized INVALID_HANDLE_VALUE sentinel.
func ErrorIfInvalidHandle(result uintptr, lastError LastError) error {
	if result != ^uintptr(0) {
		return nil
	}
	if lastError == ERROR_SUCCESS {
		return ErrNativeFailureWithoutLastError
	}
	return lastError
}
