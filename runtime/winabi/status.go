package winabi

import "fmt"

// HRESULT preserves the signed 32-bit Windows HRESULT representation.
type HRESULT int32

const (
	S_OK    HRESULT = 0
	S_FALSE HRESULT = 1

	E_NOTIMPL     HRESULT = -2147467263 // 0x80004001
	E_NOINTERFACE HRESULT = -2147467262 // 0x80004002
	E_POINTER     HRESULT = -2147467261 // 0x80004003
	E_FAIL        HRESULT = -2147467259 // 0x80004005
	E_INVALIDARG  HRESULT = -2147024809 // 0x80070057
	E_OUTOFMEMORY HRESULT = -2147024882 // 0x8007000E
)

// Failed reports whether the HRESULT severity bit denotes failure.
func (h HRESULT) Failed() bool { return h < 0 }

// Succeeded reports whether the HRESULT severity bit denotes success.
func (h HRESULT) Succeeded() bool { return h >= 0 }

// Severity returns the high severity bit.
func (h HRESULT) Severity() uint32 { return uint32(h) >> 31 }

// Facility matches the SDK HRESULT_FACILITY macro's 13-bit mask.
func (h HRESULT) Facility() uint16 { return uint16((uint32(h) >> 16) & 0x1fff) }

// Code returns the low 16-bit HRESULT code.
func (h HRESULT) Code() uint16 { return uint16(uint32(h)) }

func (h HRESULT) Error() string {
	return fmt.Sprintf("HRESULT 0x%08X (facility=%d, code=%d)", uint32(h), h.Facility(), h.Code())
}

// Err returns h as an error only when it represents failure. Success statuses,
// including S_FALSE, return nil without discarding the original h value.
func (h HRESULT) Err() error {
	if h.Succeeded() {
		return nil
	}
	return h
}

// NTSTATUS preserves the signed 32-bit native status value.
type NTSTATUS int32

const (
	STATUS_SUCCESS         NTSTATUS = 0
	STATUS_NOT_IMPLEMENTED NTSTATUS = -1073741822 // 0xC0000002
)

// Failed reports whether the NTSTATUS severity class is error or warning as
// observed by the conventional NT_SUCCESS macro (status >= 0 succeeds).
func (s NTSTATUS) Failed() bool { return s < 0 }

// Severity returns the two-bit NTSTATUS severity field.
func (s NTSTATUS) Severity() uint32 { return uint32(s) >> 30 }

// Facility returns the 12-bit NTSTATUS facility field.
func (s NTSTATUS) Facility() uint16 { return uint16((uint32(s) >> 16) & 0xfff) }

// Code returns the low 16-bit status code.
func (s NTSTATUS) Code() uint16 { return uint16(uint32(s)) }

func (s NTSTATUS) Error() string {
	return fmt.Sprintf("NTSTATUS 0x%08X (facility=%d, code=%d)", uint32(s), s.Facility(), s.Code())
}

// Err returns s as an error only when NT_SUCCESS(s) is false.
func (s NTSTATUS) Err() error {
	if !s.Failed() {
		return nil
	}
	return s
}
