package foundation

import (
	"testing"
	"unsafe"
)

func TestFoundationLayouts(t *testing.T) {
	if unsafe.Sizeof(GUID{}) != 16 || unsafe.Alignof(GUID{}) != 4 {
		t.Fatalf("GUID layout = size %d align %d", unsafe.Sizeof(GUID{}), unsafe.Alignof(GUID{}))
	}
	if unsafe.Sizeof(FILETIME{}) != 8 || unsafe.Sizeof(SYSTEMTIME{}) != 16 {
		t.Fatalf("unexpected time layouts: FILETIME=%d SYSTEMTIME=%d", unsafe.Sizeof(FILETIME{}), unsafe.Sizeof(SYSTEMTIME{}))
	}
	if unsafe.Sizeof(HANDLE(0)) != unsafe.Sizeof(uintptr(0)) || unsafe.Sizeof(SSIZE_T(0)) != unsafe.Sizeof(uintptr(0)) {
		t.Fatal("pointer-sized type mismatch")
	}
	pointerSize := unsafe.Sizeof(uintptr(0))
	wantSecurityAttributes := uintptr(12)
	if pointerSize == 8 {
		wantSecurityAttributes = 24
	}
	if unsafe.Sizeof(SECURITY_ATTRIBUTES{}) != wantSecurityAttributes {
		t.Fatalf("SECURITY_ATTRIBUTES size = %d, want %d", unsafe.Sizeof(SECURITY_ATTRIBUTES{}), wantSecurityAttributes)
	}
}

func TestLargeIntegerUnionAccessors(t *testing.T) {
	var value LARGE_INTEGER
	value.SetParts(0x89abcdef, -2)
	if value.LowPart() != 0x89abcdef || value.HighPart() != -2 {
		t.Fatalf("union views = (%08x, %d)", value.LowPart(), value.HighPart())
	}
}
