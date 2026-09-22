//go:build windows

package main

import (
	"fmt"

	"go-windows-api.local/bindings/win32/kernel32"
)

func main() {
	fmt.Printf("process=%d thread=%d\n", kernel32.GetCurrentProcessId(), kernel32.GetCurrentThreadId())
}
