package com

import (
	"testing"
	"unsafe"
)

func TestIUnknownVTableLayout(t *testing.T) {
	var table IUnknownVTable
	pointerSize := unsafe.Sizeof(uintptr(0))
	if unsafe.Offsetof(table.QueryInterface) != 0 ||
		unsafe.Offsetof(table.AddRef) != pointerSize ||
		unsafe.Offsetof(table.Release) != pointerSize*2 ||
		unsafe.Sizeof(table) != pointerSize*3 {
		t.Fatalf("unexpected IUnknown vtable layout: size=%d", unsafe.Sizeof(table))
	}
}

func TestIUnknownIID(t *testing.T) {
	if got := IID_IUnknown.String(); got != "00000000-0000-0000-C000-000000000046" {
		t.Fatalf("IID_IUnknown = %s", got)
	}
}
