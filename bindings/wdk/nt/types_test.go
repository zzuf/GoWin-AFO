package nt

import (
	"testing"
	"unsafe"
)

func TestWDKTypeOnlyLayouts(t *testing.T) {
	pointerSize := unsafe.Sizeof(uintptr(0))
	wantSize := pointerSize * 2
	if unsafe.Sizeof(UNICODE_STRING{}) != wantSize {
		t.Fatalf("UNICODE_STRING size = %d, want %d", unsafe.Sizeof(UNICODE_STRING{}), wantSize)
	}
	wantObjectAttributes := uintptr(24)
	if pointerSize == 8 {
		wantObjectAttributes = 48
	}
	if unsafe.Sizeof(OBJECT_ATTRIBUTES{}) != wantObjectAttributes {
		t.Fatalf("OBJECT_ATTRIBUTES size = %d, want %d", unsafe.Sizeof(OBJECT_ATTRIBUTES{}), wantObjectAttributes)
	}
	for _, symbol := range SliceSymbolInventory {
		if symbol.Status == "unclassified" || symbol.Status == "" || symbol.Reason == "" {
			t.Fatalf("unaccounted WDK symbol: %#v", symbol)
		}
	}
}
