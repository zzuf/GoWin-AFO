package winabi

import (
	"fmt"
	"strings"
)

// CallResult preserves both machine-word return registers and the raw
// thread-local last-error snapshot from an integer/pointer ABI call.
type CallResult struct {
	R1        uintptr
	R2        uintptr
	LastError LastError
}

// NewSystemProc creates a generated-code procedure descriptor using the
// System32-only lazy loader. dll and name are expected to be fixed metadata;
// malformed values panic as generator invariants before any DLL is loaded.
func NewSystemProc(dll, name string) *Proc {
	return MustSystemDLL(dll).MustProc(name)
}

// Call invokes an integer/pointer-only signature and returns the conventional
// two result registers plus last error. A loader/resolver failure is represented
// as ERROR_PROC_NOT_FOUND; runtime code that must distinguish it uses TryCall.
//
//go:uintptrescapes
func (p *Proc) Call(arguments ...uintptr) (r1, r2 uintptr, lastError LastError) {
	result, err := p.TryCall(arguments...)
	if err != nil {
		return 0, 0, ERROR_PROC_NOT_FOUND
	}
	return result.R1, result.R2, result.LastError
}

func validateSystemDLLName(name string) error {
	if len(name) < 5 || len(name) > 255 || !strings.EqualFold(name[len(name)-4:], ".dll") {
		return ErrInvalidDLLName
	}
	for _, character := range name {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return ErrInvalidDLLName
	}
	if strings.Contains(name, "..") {
		return ErrInvalidDLLName
	}
	return nil
}

func validateProcName(name string) error {
	if name == "" || len(name) > 512 {
		return ErrInvalidProcName
	}
	for _, character := range name {
		if character < 0x21 || character > 0x7e || character == '/' || character == '\\' || character == ':' {
			return ErrInvalidProcName
		}
	}
	return nil
}

func mustValidSystemDLL(name string) {
	if err := validateSystemDLLName(name); err != nil {
		panic(fmt.Errorf("%w: %q", err, name))
	}
}

func mustValidProc(name string) {
	if err := validateProcName(name); err != nil {
		panic(fmt.Errorf("%w: %q", err, name))
	}
}
