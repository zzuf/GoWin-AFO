//go:build windows

package winrt

import "testing"

func TestHStringAndActivationFactory(t *testing.T) {
	apartment, status := EnterApartment(RO_INIT_MULTITHREADED)
	if status.Failed() {
		t.Fatalf("RoInitialize failed: %v", status)
	}
	defer apartment.Close()

	value, err := NewHString("a\x00b\U0001F642")
	if err != nil {
		t.Fatal(err)
	}
	defer value.Close()
	got, err := value.String()
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\x00b\U0001F642" {
		t.Fatalf("HSTRING round trip = %q", got)
	}

	factory, status := GetActivationFactory("Windows.Foundation.Uri")
	if status.Failed() {
		t.Fatalf("RoGetActivationFactory failed: %v", status)
	}
	if factory == nil {
		t.Fatal("RoGetActivationFactory returned nil")
	}
	factory.Release()
}
