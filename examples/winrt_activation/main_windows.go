//go:build windows

package main

import (
	"fmt"

	foundation "go-windows-api.local/bindings/winrt/windows/foundation"
	"go-windows-api.local/runtime/winrt"
)

func main() {
	apartment, status := winrt.EnterApartment(winrt.RO_INIT_MULTITHREADED)
	if status.Failed() {
		fmt.Printf("RoInitialize failed: %v\n", status)
		return
	}
	defer apartment.Close()

	factory, status := foundation.GetURIActivationFactory()
	if status.Failed() {
		fmt.Printf("activation factory failed: %v\n", status)
		return
	}
	defer factory.Release()
	fmt.Println("Windows.Foundation.Uri activation factory is available")
}
