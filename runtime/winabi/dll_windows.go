//go:build windows

package winabi

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const loadLibrarySearchSystem32 = 0x00000800

var systemLoader struct {
	once    sync.Once
	address uintptr
	err     error
}

// DLL is a lazily loaded DLL constrained to LOAD_LIBRARY_SEARCH_SYSTEM32. Its
// name is validated as a basename and cannot contain a search path.
type DLL struct {
	name   string
	once   sync.Once
	handle syscall.Handle
	err    error
}

// Proc is a lazily resolved DLL export.
type Proc struct {
	name string
	dll  *DLL
	once sync.Once
	addr uintptr
	err  error
}

// NewSystemDLL prepares a System32-safe lazy DLL without loading it.
func NewSystemDLL(name string) (*DLL, error) {
	if err := validateSystemDLLName(name); err != nil {
		return nil, err
	}
	return &DLL{name: name}, nil
}

// MustSystemDLL is for generated package-level, fixed DLL metadata. It does not
// load the DLL and panics only if the generator emitted an unsafe name.
func MustSystemDLL(name string) *DLL {
	mustValidSystemDLL(name)
	dll, _ := NewSystemDLL(name)
	return dll
}

// Name returns the validated DLL basename.
func (d *DLL) Name() string { return d.name }

func loadLibraryExWAddress() (uintptr, error) {
	systemLoader.once.Do(func() {
		// kernel32.dll is in Go's internal system-DLL allowlist, so this bootstrap
		// load already uses LOAD_LIBRARY_SEARCH_SYSTEM32.
		kernel32, err := syscall.LoadDLL("kernel32.dll")
		if err != nil {
			systemLoader.err = err
			return
		}
		proc, err := kernel32.FindProc("LoadLibraryExW")
		if err != nil {
			systemLoader.err = err
			return
		}
		systemLoader.address = proc.Addr()
	})
	return systemLoader.address, systemLoader.err
}

func (d *DLL) load() (syscall.Handle, error) {
	d.once.Do(func() {
		loader, err := loadLibraryExWAddress()
		if err != nil {
			d.err = err
			return
		}
		name, err := syscall.UTF16PtrFromString(d.name)
		if err != nil {
			d.err = err
			return
		}
		handle, _, lastError := syscall.SyscallN(
			loader,
			uintptr(unsafe.Pointer(name)),
			0,
			loadLibrarySearchSystem32,
		)
		runtime.KeepAlive(name)
		if handle == 0 {
			d.err = fmt.Errorf("winabi: load System32 DLL %q: %w", d.name, lastError)
			return
		}
		d.handle = syscall.Handle(handle)
	})
	return d.handle, d.err
}

// NewProc prepares a lazy export without resolving it.
func (d *DLL) NewProc(name string) (*Proc, error) {
	if err := validateProcName(name); err != nil {
		return nil, err
	}
	return &Proc{name: name, dll: d}, nil
}

// MustProc is for generated package-level, fixed export metadata.
func (d *DLL) MustProc(name string) *Proc {
	mustValidProc(name)
	proc, _ := d.NewProc(name)
	return proc
}

// Name returns the native export name.
func (p *Proc) Name() string { return p.name }

// Address resolves the procedure once and returns its address.
func (p *Proc) Address() (uintptr, error) {
	p.once.Do(func() {
		handle, err := p.dll.load()
		if err != nil {
			p.err = err
			return
		}
		p.addr, p.err = syscall.GetProcAddress(handle, p.name)
	})
	return p.addr, p.err
}

// Available reports whether the export can be resolved. Resolution is lazy and
// does not occur merely because the containing package was imported.
func (p *Proc) Available() bool {
	_, err := p.Address()
	return err == nil
}

// TryCall invokes an integer/pointer-only ABI signature and preserves loader or
// resolver errors. Floating-point, vector, varargs, and aggregate-by-value
// signatures must not use this backend.
//
//go:uintptrescapes
func (p *Proc) TryCall(arguments ...uintptr) (CallResult, error) {
	address, err := p.Address()
	if err != nil {
		return CallResult{}, err
	}
	return CallAddress(address, arguments...)
}

// CallAddress invokes an already resolved integer/pointer-only ABI function.
//
//go:uintptrescapes
func CallAddress(address uintptr, arguments ...uintptr) (CallResult, error) {
	if address == 0 {
		return CallResult{}, ErrInvalidAddress
	}
	r1, r2, lastError := syscall.SyscallN(address, arguments...)
	return CallResult{R1: r1, R2: r2, LastError: LastError(lastError)}, nil
}
