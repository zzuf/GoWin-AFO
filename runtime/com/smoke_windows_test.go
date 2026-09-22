//go:build windows

package com

import (
	"testing"

	"github.com/zzuf/GoWin-AFO/runtime/winabi"
)

func TestApartmentAndAllocators(t *testing.T) {
	apartment, status := EnterApartment(COINIT_MULTITHREADED)
	if status.Failed() {
		t.Fatalf("CoInitializeEx failed: %v", status)
	}
	defer apartment.Close()

	bstr, err := NewBSTR("a\x00b\U0001F642")
	if err != nil {
		t.Fatal(err)
	}
	defer bstr.Close()
	got, err := bstr.String()
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\x00b\U0001F642" {
		t.Fatalf("BSTR round trip = %q", got)
	}

	memory, err := AllocTaskMemory(64)
	if err != nil {
		t.Fatal(err)
	}
	if memory.Pointer() == nil || memory.Size() != 64 {
		t.Fatal("invalid CoTaskMem allocation")
	}
	if err := memory.Close(); err != nil {
		t.Fatal(err)
	}
	if status != winabi.S_OK && status != winabi.S_FALSE {
		t.Fatalf("unexpected successful CoInitializeEx status: %v", status)
	}

	unknown, activationStatus := CreateIUnknown(&CLSID_StdGlobalInterfaceTable, CLSCTX_INPROC_SERVER)
	if activationStatus.Failed() {
		t.Fatalf("CoCreateInstance(Standard GIT) failed: %v", activationStatus)
	}
	if unknown == nil {
		t.Fatal("CoCreateInstance returned nil IUnknown")
	}
	unknown.Release()
}
