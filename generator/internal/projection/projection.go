// Package projection contains explicit Windows ABI capability decisions.
package projection

import (
	"fmt"
	"strings"

	"go-windows-api.local/generator/internal/model"
)

type Decision struct {
	Backend model.Backend `json:"backend"`
	Status  model.Status  `json:"status"`
	Reason  string        `json:"reason"`
}

func Decide(fn model.Function) Decision {
	cc := strings.ToLower(fn.CallingConvention)
	if fn.VarArgs {
		return Decision{model.BackendCGOBridge, model.StatusGeneratedCGOBridge, "varargs requires a generated fixed-signature C adapter"}
	}
	if cc == "vectorcall" || cc == "thiscall" || cc == "fastcall" {
		return Decision{model.BackendCGOBridge, model.StatusGeneratedCGOBridge, "calling convention requires a compiler-owned ABI bridge"}
	}
	if cc != "" && cc != "platform" && cc != "stdcall" && cc != "cdecl" {
		return Decision{model.BackendUnsupported, model.StatusUnsupportedGoABI, "unknown calling convention"}
	}
	if forbidden(fn.Return) {
		return Decision{model.BackendUnsupported, model.StatusUnsupportedGoABI, "return type is not representable by supported backends"}
	}
	hasFloat := isFloat(fn.Return)
	aggregate := isAggregateValue(fn.Return)
	vector := isVector(fn.Return)
	for _, p := range fn.Parameters {
		if forbidden(p.Type) {
			return Decision{model.BackendUnsupported, model.StatusUnsupportedGoABI, "parameter " + p.Name + " is not representable by supported backends"}
		}
		hasFloat = hasFloat || isFloat(p.Type)
		aggregate = aggregate || isAggregateValue(p.Type)
		vector = vector || isVector(p.Type)
	}
	if vector || aggregate {
		return Decision{model.BackendCGOBridge, model.StatusGeneratedCGOBridge, "vector or aggregate value passing requires generated C bridge"}
	}
	if hasFloat {
		return Decision{model.BackendAssemblyTrampoline, model.StatusGeneratedAssembly, "floating-point register ABI requires architecture trampoline"}
	}
	return Decision{model.BackendPureGoSyscall, model.StatusGeneratedPureGo, "integer/pointer signature is supported by the isolated syscall runtime"}
}

func isFloat(t model.Type) bool {
	return t.GoType == "float32" || t.GoType == "float64" || t.NativeName == "FLOAT" || t.NativeName == "DOUBLE"
}
func isAggregateValue(t model.Type) bool {
	return t.PointerDepth == 0 && (t.Kind == model.KindStruct || t.Kind == model.KindUnion || t.Kind == model.KindFixedArray)
}
func isVector(t model.Type) bool  { return strings.Contains(strings.ToLower(t.NativeName), "vector") }
func forbidden(t model.Type) bool { return t.Bits > 64 && t.PointerDepth == 0 && !isAggregateValue(t) }

var LLP64 = map[string]string{
	"CHAR": "int8", "BYTE": "uint8", "SHORT": "int16", "USHORT": "uint16",
	"INT": "int32", "UINT": "uint32", "LONG": "int32", "ULONG": "uint32",
	"LONGLONG": "int64", "ULONGLONG": "uint64", "WCHAR": "uint16",
	"BOOL": "int32", "BOOLEAN": "uint8", "HRESULT": "int32", "NTSTATUS": "int32",
	"SIZE_T": "uintptr", "UINT_PTR": "uintptr", "ULONG_PTR": "uintptr",
	"HANDLE": "uintptr",
}

func GoType(native, arch string) (string, error) {
	if v, ok := LLP64[native]; ok {
		return v, nil
	}
	switch native {
	case "SSIZE_T", "INT_PTR", "LONG_PTR":
		if arch == "386" {
			return "int32", nil
		}
		if arch == "amd64" || arch == "arm64" {
			return "int64", nil
		}
		return "", fmt.Errorf("pointer-sized type %s requires concrete architecture", native)
	}
	return "", fmt.Errorf("no proven LLP64 projection for %s", native)
}
