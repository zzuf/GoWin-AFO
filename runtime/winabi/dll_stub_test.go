//go:build !windows

package winabi

import "testing"

func TestGeneratedProcedureStub(t *testing.T) {
	proc := NewSystemProc("kernel32.dll", "GetCurrentProcessId")
	r1, r2, lastError := proc.Call()
	if r1 != 0 || r2 != 0 || lastError != ERROR_PROC_NOT_FOUND || proc.Available() {
		t.Fatalf("stub result = (%d, %d, %v), available=%v", r1, r2, lastError, proc.Available())
	}
}
