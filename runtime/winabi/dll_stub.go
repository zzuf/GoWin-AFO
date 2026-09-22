//go:build !windows

package winabi

// DLL is a validated System32 DLL descriptor. Non-Windows builds never load it.
type DLL struct{ name string }

// Proc is a validated export descriptor. Non-Windows builds never resolve it.
type Proc struct{ name string }

func NewSystemDLL(name string) (*DLL, error) {
	if err := validateSystemDLLName(name); err != nil {
		return nil, err
	}
	return &DLL{name: name}, nil
}

func MustSystemDLL(name string) *DLL {
	mustValidSystemDLL(name)
	dll, _ := NewSystemDLL(name)
	return dll
}

func (d *DLL) Name() string { return d.name }

func (d *DLL) NewProc(name string) (*Proc, error) {
	if err := validateProcName(name); err != nil {
		return nil, err
	}
	return &Proc{name: name}, nil
}

func (d *DLL) MustProc(name string) *Proc {
	mustValidProc(name)
	proc, _ := d.NewProc(name)
	return proc
}

func (p *Proc) Name() string { return p.name }

func (p *Proc) Address() (uintptr, error) { return 0, ErrUnsupportedPlatform }

func (p *Proc) Available() bool { return false }

//go:uintptrescapes
func (p *Proc) TryCall(arguments ...uintptr) (CallResult, error) {
	return CallResult{}, ErrUnsupportedPlatform
}

//go:uintptrescapes
func CallAddress(address uintptr, arguments ...uintptr) (CallResult, error) {
	if address == 0 {
		return CallResult{}, ErrInvalidAddress
	}
	return CallResult{}, ErrUnsupportedPlatform
}
