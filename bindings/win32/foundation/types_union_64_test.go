//go:build amd64 || arm64

package foundation

import (
	"testing"
	"unsafe"
)

func TestLargeInteger64DirectLayout(t *testing.T) {
	if !LargeIntegerDirectABI {
		t.Fatal("64-bit LARGE_INTEGER should be direct ABI")
	}
	if unsafe.Sizeof(LARGE_INTEGER{}) != 8 || unsafe.Alignof(LARGE_INTEGER{}) != 8 {
		t.Fatalf("LARGE_INTEGER layout = size %d align %d", unsafe.Sizeof(LARGE_INTEGER{}), unsafe.Alignof(LARGE_INTEGER{}))
	}
}
