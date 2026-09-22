package winrt

import (
	"errors"
	"testing"
	"unsafe"
)

func TestInspectableVTableLayout(t *testing.T) {
	var table IInspectableVTable
	pointerSize := unsafe.Sizeof(uintptr(0))
	if unsafe.Offsetof(table.GetIids) != pointerSize*3 ||
		unsafe.Offsetof(table.GetRuntimeClassName) != pointerSize*4 ||
		unsafe.Offsetof(table.GetTrustLevel) != pointerSize*5 ||
		unsafe.Sizeof(table) != pointerSize*6 {
		t.Fatalf("unexpected IInspectable vtable layout: size=%d", unsafe.Sizeof(table))
	}
}

func TestActivationFactoryVTableLayout(t *testing.T) {
	var table IActivationFactoryVTable
	pointerSize := unsafe.Sizeof(uintptr(0))
	if unsafe.Offsetof(table.ActivateInstance) != pointerSize*6 || unsafe.Sizeof(table) != pointerSize*7 {
		t.Fatalf("unexpected IActivationFactory vtable layout: size=%d", unsafe.Sizeof(table))
	}
}

func TestParameterizedIIDFailsExplicitly(t *testing.T) {
	guid, err := ParameterizedIID("pinterface({00000000-0000-0000-0000-000000000000};i4)")
	if !errors.Is(err, ErrParameterizedIIDUnsupported) || !guid.IsZero() {
		t.Fatalf("ParameterizedIID = (%v, %v)", guid, err)
	}
}

func TestActivationRejectsNUL(t *testing.T) {
	if _, status := GetActivationFactory("Windows.Foundation\x00.Uri"); !status.Failed() {
		t.Fatalf("embedded NUL status = %v", status)
	}
}
