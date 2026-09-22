//go:build windows

package winabi

import "testing"

func TestSystemDLLLazyProcedure(t *testing.T) {
	dll, err := NewSystemDLL("kernel32.dll")
	if err != nil {
		t.Fatal(err)
	}
	proc, err := dll.NewProc("GetCurrentProcessId")
	if err != nil {
		t.Fatal(err)
	}
	if !proc.Available() {
		t.Fatal("GetCurrentProcessId is unavailable")
	}
	result, err := proc.TryCall()
	if err != nil {
		t.Fatal(err)
	}
	if result.R1 == 0 {
		t.Fatal("GetCurrentProcessId returned zero")
	}
}

func TestGeneratedProcedureAPI(t *testing.T) {
	proc := NewSystemProc("kernel32.dll", "GetCurrentProcessId")
	r1, _, _ := proc.Call()
	if r1 == 0 || !proc.Available() {
		t.Fatal("generated procedure API did not call GetCurrentProcessId")
	}
}
