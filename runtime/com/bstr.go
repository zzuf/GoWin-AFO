package com

import (
	"errors"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"go-windows-api.local/runtime/winabi"
)

var (
	errStringTooLong = errors.New("com: UTF-16 string length exceeds UINT32")
	errOutOfMemory   = errors.New("com: native allocation returned nil")

	oleaut32              = winabi.MustSystemDLL("oleaut32.dll")
	procSysAllocStringLen = oleaut32.MustProc("SysAllocStringLen")
	procSysFreeString     = oleaut32.MustProc("SysFreeString")
	procSysStringLen      = oleaut32.MustProc("SysStringLen")
)

// BSTR owns a native BSTR allocated with SysAllocStringLen. Embedded NUL code
// units are preserved because BSTR carries an explicit length. BSTR must not be
// copied; call Close explicitly.
type BSTR struct {
	noCopy noCopy
	value  *uint16
}

// NewBSTR allocates an owned BSTR.
func NewBSTR(value string) (*BSTR, error) {
	units := utf16.Encode([]rune(value))
	if uint64(len(units)) > uint64(^uint32(0)) {
		return nil, errStringTooLong
	}
	var source uintptr
	if len(units) != 0 {
		source = uintptr(unsafe.Pointer(&units[0]))
	}
	call, err := procSysAllocStringLen.TryCall(source, uintptr(uint32(len(units))))
	runtime.KeepAlive(units)
	if err != nil {
		return nil, err
	}
	if call.R1 == 0 {
		return nil, errOutOfMemory
	}
	return &BSTR{value: (*uint16)(winabi.UnsafePointerFromUintptr(call.R1))}, nil
}

// Pointer returns the borrowed native pointer. It remains valid only until the
// next Close call and must not be freed independently.
func (value *BSTR) Pointer() *uint16 {
	if value == nil {
		return nil
	}
	return value.value
}

// String copies the length-delimited BSTR into a Go string.
func (value *BSTR) String() (string, error) {
	if value == nil || value.value == nil {
		return "", nil
	}
	call, err := procSysStringLen.TryCall(uintptr(unsafe.Pointer(value.value)))
	if err != nil {
		return "", err
	}
	length := uint32(call.R1)
	sliceLength, err := winabi.CheckedSliceLength(length, unsafe.Sizeof(uint16(0)))
	if err != nil {
		return "", err
	}
	units := unsafe.Slice(value.value, sliceLength)
	result := string(utf16.Decode(units))
	runtime.KeepAlive(value)
	return result, nil
}

// Close frees the BSTR once and clears its pointer.
func (value *BSTR) Close() error {
	if value == nil || value.value == nil {
		return nil
	}
	if _, err := procSysFreeString.TryCall(uintptr(unsafe.Pointer(value.value))); err != nil {
		return err
	}
	value.value = nil
	return nil
}
