package winrt

import (
	"errors"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"github.com/zzuf/GoWin-AFO/runtime/winabi"
)

var (
	errHStringTooLong = errors.New("winrt: UTF-16 string length exceeds UINT32")

	combase                       = winabi.MustSystemDLL("combase.dll")
	procWindowsCreateString       = combase.MustProc("WindowsCreateString")
	procWindowsDeleteString       = combase.MustProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.MustProc("WindowsGetStringRawBuffer")
)

// HString owns a Windows Runtime HSTRING handle. It has no finalizer and must
// not be copied; call Close explicitly. Unlike NUL-terminated Win32 strings,
// HSTRING can faithfully contain embedded NUL code units.
type HString struct {
	noCopy noCopy
	handle uintptr
}

// NewHString creates an owned HSTRING.
func NewHString(value string) (*HString, error) {
	units := utf16.Encode([]rune(value))
	if uint64(len(units)) > uint64(^uint32(0)) {
		return nil, errHStringTooLong
	}
	var source uintptr
	if len(units) != 0 {
		source = uintptr(unsafe.Pointer(&units[0]))
	}
	var handle uintptr
	call, err := procWindowsCreateString.TryCall(
		source,
		uintptr(uint32(len(units))),
		uintptr(unsafe.Pointer(&handle)),
	)
	runtime.KeepAlive(units)
	if err != nil {
		return nil, err
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return nil, status
	}
	return &HString{handle: handle}, nil
}

// Handle returns the borrowed HSTRING handle valid until Close.
func (value *HString) Handle() uintptr {
	if value == nil {
		return 0
	}
	return value.handle
}

// String copies the length-delimited HSTRING into a Go string.
func (value *HString) String() (string, error) {
	if value == nil || value.handle == 0 {
		return "", nil
	}
	var length uint32
	call, err := procWindowsGetStringRawBuffer.TryCall(
		value.handle,
		uintptr(unsafe.Pointer(&length)),
	)
	if err != nil {
		return "", err
	}
	if call.R1 == 0 || length == 0 {
		return "", nil
	}
	sliceLength, err := winabi.CheckedSliceLength(length, unsafe.Sizeof(uint16(0)))
	if err != nil {
		return "", err
	}
	units := unsafe.Slice((*uint16)(winabi.UnsafePointerFromUintptr(call.R1)), sliceLength)
	result := string(utf16.Decode(units))
	runtime.KeepAlive(value)
	return result, nil
}

// Close deletes the HSTRING once and clears its handle.
func (value *HString) Close() error {
	if value == nil || value.handle == 0 {
		return nil
	}
	call, err := procWindowsDeleteString.TryCall(value.handle)
	if err != nil {
		return err
	}
	status := winabi.HRESULT(int32(uint32(call.R1)))
	if status.Failed() {
		return status
	}
	value.handle = 0
	return nil
}

func ownedHString(handle uintptr) *HString {
	return &HString{handle: handle}
}
