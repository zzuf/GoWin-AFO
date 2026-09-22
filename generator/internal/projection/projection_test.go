package projection

import (
	"testing"

	"go-windows-api.local/generator/internal/model"
)

func TestLLP64(t *testing.T) {
	for native, want := range map[string]string{"INT": "int32", "LONG": "int32", "ULONG": "uint32", "WCHAR": "uint16", "BOOL": "int32"} {
		got, err := GoType(native, "amd64")
		if err != nil || got != want {
			t.Fatalf("%s => %s %v", native, got, err)
		}
	}
	if got, _ := GoType("LONG_PTR", "386"); got != "int32" {
		t.Fatal(got)
	}
	if got, _ := GoType("LONG_PTR", "arm64"); got != "int64" {
		t.Fatal(got)
	}
}

func TestCapabilityMatrix(t *testing.T) {
	basic := model.Function{CallingConvention: "stdcall", Return: model.Type{GoType: "uint32"}}
	if d := Decide(basic); d.Backend != model.BackendPureGoSyscall {
		t.Fatalf("%+v", d)
	}
	basic.Parameters = []model.Parameter{{Name: "x", Type: model.Type{GoType: "float64"}}}
	if d := Decide(basic); d.Backend != model.BackendAssemblyTrampoline {
		t.Fatalf("%+v", d)
	}
	basic.Parameters = nil
	basic.Return = model.Type{Kind: model.KindStruct, GoType: "S"}
	if d := Decide(basic); d.Backend != model.BackendCGOBridge {
		t.Fatalf("%+v", d)
	}
}
