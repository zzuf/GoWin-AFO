package metadata

import (
	"testing"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

func TestSafePureGoPInvokeGate(t *testing.T) {
	fn := model.Function{
		DLL:               "kernel32.dll",
		EntryPoint:        "GetCurrentProcessId",
		CallingConvention: "stdcall",
		Return:            model.Type{Kind: model.KindPrimitive, GoType: "uint32", Bits: 32},
	}
	if !safePureGoPInvoke(fn) {
		t.Fatal("integer-only fixed System32 import was rejected")
	}
	fn.DLL = `..\untrusted.dll`
	if safePureGoPInvoke(fn) {
		t.Fatal("unsafe DLL path was accepted")
	}
	fn.DLL = "kernel32.dll"
	fn.CallingConvention = "cdecl"
	if safePureGoPInvoke(fn) {
		t.Fatal("cdecl was accepted for the shared 386 projection")
	}
	fn.CallingConvention = "stdcall"
	fn.Parameters = []model.Parameter{{Name: "value", Type: model.Type{Kind: model.KindPrimitive, GoType: "float64", Bits: 64}}}
	if safePureGoPInvoke(fn) {
		t.Fatal("floating ABI was accepted by the integer syscall backend")
	}
	fn.Parameters = []model.Parameter{{Name: "value", Type: model.Type{Kind: model.KindPrimitive, GoType: "uint64", Bits: 64}}}
	if !hasWideScalarByValue(fn) || safePureGoPInvoke(fn) {
		t.Fatal("windows/386 multiword uint64 value was accepted")
	}
	fn.Parameters[0].Type = model.Type{Kind: model.KindPointer, GoType: "*uint64", PointerDepth: 1, Element: &model.Type{Kind: model.KindPrimitive, GoType: "uint64", Bits: 64}}
	if hasWideScalarByValue(fn) || !safeWordType(fn.Parameters[0].Type, false) || !hasPointerABI(fn) {
		t.Fatal("pointer to uint64 is one ABI word but must be blocked by the separate lifetime/alignment policy")
	}
}

func TestGoIdentifierKeywordsAndDuplicates(t *testing.T) {
	if got := goIdentifier("type"); got != "type_" {
		t.Fatalf("keyword = %q", got)
	}
	used := map[string]int{}
	if a, b := uniqueParameterName("value", used), uniqueParameterName("value", used); a != "value" || b != "value_1" {
		t.Fatalf("duplicate names = %q, %q", a, b)
	}
	if got := goPublicIdentifier("send"); got != "Send" {
		t.Fatalf("public identifier = %q", got)
	}
}
