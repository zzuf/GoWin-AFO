package winabi

import "unsafe"

// UnsafePointerFromUintptr bit-casts a native pointer result returned in an ABI
// register. It must never be used to round-trip a Go heap pointer through
// uintptr; its only supported input is an address produced by native code.
func UnsafePointerFromUintptr(value uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&value))
}
