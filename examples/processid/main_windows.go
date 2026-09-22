//go:build windows

package main

import (
	"fmt"

	"github.com/zzuf/GoWin-AFO/bindings/win32/kernel32"
)

func main() {
	fmt.Printf("process=%d thread=%d\n", kernel32.GetCurrentProcessId(), kernel32.GetCurrentThreadId())
}
