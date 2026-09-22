package foundation

import (
	"testing"
	"unsafe"
)

func TestLargeInteger386IsNotClaimedDirect(t *testing.T) {
	if LargeIntegerDirectABI {
		t.Fatal("Go/386 cannot represent native LARGE_INTEGER alignment")
	}
	if unsafe.Sizeof(LARGE_INTEGER{}) != 8 {
		t.Fatalf("LARGE_INTEGER size = %d", unsafe.Sizeof(LARGE_INTEGER{}))
	}
}
