package winabi

import (
	"errors"
	"strconv"
	"testing"
	"unsafe"
)

func TestGUIDRoundTrip(t *testing.T) {
	const text = "00000000-0000-0000-C000-000000000046"
	guid, err := ParseGUID("{" + text + "}")
	if err != nil {
		t.Fatal(err)
	}
	if got := guid.String(); got != text {
		t.Fatalf("String() = %q, want %q", got, text)
	}
}

func TestCheckedSliceLength(t *testing.T) {
	length, err := CheckedSliceLength(16, 2)
	if err != nil || length != 16 {
		t.Fatalf("CheckedSliceLength(16, 2) = (%d, %v)", length, err)
	}
	_, err = CheckedSliceLength(^uint32(0), 16)
	if strconv.IntSize == 32 && !errors.Is(err, ErrLengthOverflow) {
		t.Fatalf("32-bit overflow error = %v", err)
	}
	if _, err := CheckedSliceLength(1, 0); !errors.Is(err, ErrLengthOverflow) {
		t.Fatalf("zero element size error = %v", err)
	}
}

func TestGUIDRejectsMalformedInput(t *testing.T) {
	if _, err := ParseGUID("not-a-guid"); err == nil {
		t.Fatal("ParseGUID accepted malformed input")
	}
}

func TestUTF16RejectsEmbeddedNUL(t *testing.T) {
	if _, err := UTF16FromString("a\x00b"); !errors.Is(err, ErrEmbeddedNUL) {
		t.Fatalf("UTF16FromString error = %v, want ErrEmbeddedNUL", err)
	}
	buffer, err := UTF16FromString("A\U0001F642")
	if err != nil {
		t.Fatal(err)
	}
	if len(buffer) != 4 || buffer[len(buffer)-1] != 0 {
		t.Fatalf("unexpected UTF-16 buffer: %#v", buffer)
	}
	pointer, err := UTF16PtrFromString("ok")
	if err != nil || pointer == nil || *pointer != uint16('o') {
		t.Fatalf("UTF16PtrFromString = (%v, %v)", pointer, err)
	}
}

func TestStatusSemantics(t *testing.T) {
	if S_FALSE.Failed() || S_FALSE.Err() != nil {
		t.Fatal("S_FALSE must be a successful status")
	}
	if !E_FAIL.Failed() || E_FAIL.Err() == nil {
		t.Fatal("E_FAIL must be a failed status")
	}
	if !STATUS_NOT_IMPLEMENTED.Failed() || STATUS_SUCCESS.Err() != nil {
		t.Fatal("unexpected NTSTATUS success semantics")
	}
}

func TestLastErrorIsConsultedOnlyOnFailure(t *testing.T) {
	if err := ErrorIfFalse(1, ERROR_INVALID_HANDLE); err != nil {
		t.Fatalf("success returned stale last error: %v", err)
	}
	if !errors.Is(ErrorIfFalse(0, 0), ErrNativeFailureWithoutLastError) {
		t.Fatal("missing last error was not distinguished")
	}
	if !errors.Is(ErrorIfFalse(0, ERROR_INVALID_HANDLE), ERROR_INVALID_HANDLE) {
		t.Fatal("failure did not preserve last error")
	}
}

func TestSystemDLLRejectsSearchPaths(t *testing.T) {
	invalid := []string{"kernel32", "../kernel32.dll", `C:\\Windows\\System32\\kernel32.dll`, "kernel32.dll\x00evil"}
	for _, name := range invalid {
		if _, err := NewSystemDLL(name); err == nil {
			t.Errorf("NewSystemDLL(%q) succeeded", name)
		}
	}
	if _, err := NewSystemDLL("api-ms-win-core-synch-l1-2-0.dll"); err != nil {
		t.Fatalf("valid API set rejected: %v", err)
	}
}

func TestPointerSizedTypes(t *testing.T) {
	if unsafe.Sizeof(SignedPointer(0)) != unsafe.Sizeof(uintptr(0)) {
		t.Fatalf("SignedPointer size = %d, pointer size = %d", unsafe.Sizeof(SignedPointer(0)), unsafe.Sizeof(uintptr(0)))
	}
}
