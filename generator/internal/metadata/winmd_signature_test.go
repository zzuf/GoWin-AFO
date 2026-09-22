package metadata

import (
	"testing"

	mswinmd "github.com/microsoft/go-winmd/winmd"
	"go-windows-api.local/generator/internal/model"
)

// Distinct generic instantiations must retain distinct canonical signatures.
// Otherwise their stable IDs can collide after the upstream parser decodes them.
func TestCanonicalGenericSignaturePreservesArguments(t *testing.T) {
	index := mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]{Tag: mswinmd.TypeDefOrRefOrSpec_TypeSpec, Index: 7}
	makeType := func(param uint32) mswinmd.SigType {
		return mswinmd.SigType{Kind: mswinmd.ElementType_GENERICINST, Value: mswinmd.SigGenericInst{
			Class: true, Index: index, Type: []mswinmd.SigType{{Kind: mswinmd.ElementType_VAR, Value: param}},
		}}
	}
	if got := canonicalSigType(nil, makeType(0)); got != "GENERICINST<class typespec#7<VAR(0)>>" {
		t.Fatalf("generic argument 0 canonicalized as %q", got)
	}
	if got := canonicalSigType(nil, makeType(1)); got != "GENERICINST<class typespec#7<VAR(1)>>" {
		t.Fatalf("generic argument 1 canonicalized as %q", got)
	}
	if got := canonicalSigType(nil, mswinmd.SigType{Kind: mswinmd.ElementType_SZARRAY, Value: makeType(0)}); got != "SZARRAY<GENERICINST<class typespec#7<VAR(0)>>>" {
		t.Fatalf("SZARRAY element canonicalized as %q", got)
	}
	if typ := projectSigType(nil, makeType(0)); !typ.Opaque {
		t.Fatal("generic instantiation was treated as a safe callable ABI type")
	}
}

func TestByRefReturnIsOnePointer(t *testing.T) {
	ret := mswinmd.SigRetType{Kind: mswinmd.SigRetTypeKind_ByRef, Type: mswinmd.SigType{
		Kind:  mswinmd.ElementType_BYREF,
		Value: mswinmd.SigType{Kind: mswinmd.ElementType_I4},
	}}
	typ := projectRet(nil, ret)
	if typ.PointerDepth != 1 || typ.GoType != "*int32" {
		t.Fatalf("BYREF int32 projected as depth %d, type %q", typ.PointerDepth, typ.GoType)
	}
	if got := retCanonical(nil, ret); got != "BYREF<I4>" {
		t.Fatalf("BYREF return canonicalized as %q", got)
	}
}

func TestCanonicalArrayShapePreservesSizesAndBounds(t *testing.T) {
	makeArray := func(sizes []uint32, bounds []int32) mswinmd.SigType {
		return mswinmd.SigType{Kind: mswinmd.ElementType_ARRAY, Value: mswinmd.SigArray{
			Type: mswinmd.SigType{Kind: mswinmd.ElementType_I4}, Rank: 2,
			Sizes: sizes, LowerBounds: bounds,
		}}
	}
	if got := canonicalSigType(nil, makeArray([]uint32{3, 4}, []int32{-1, 2})); got != "array[rank=2;sizes=3,4;lower=-1,2]<I4>" {
		t.Fatalf("array shape canonicalized as %q", got)
	}
	if got := canonicalSigType(nil, makeArray([]uint32{3, 5}, []int32{-1, 2})); got != "array[rank=2;sizes=3,5;lower=-1,2]<I4>" {
		t.Fatalf("different array size collapsed to %q", got)
	}
	if got := canonicalSigType(nil, makeArray([]uint32{3, 4}, []int32{-1, 3})); got != "array[rank=2;sizes=3,4;lower=-1,3]<I4>" {
		t.Fatalf("different lower bound collapsed to %q", got)
	}
}

func TestCanonicalCustomModifiersPreserveKindOrderAndType(t *testing.T) {
	opt := mswinmd.SigCustomMod{Kind: mswinmd.SigCustomModKind_Opt, Index: mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]{Tag: mswinmd.TypeDefOrRefOrSpec_TypeSpec, Index: 7}}
	reqd := mswinmd.SigCustomMod{Kind: mswinmd.SigCustomModKind_Reqd, Index: mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]{Tag: mswinmd.TypeDefOrRefOrSpec_TypeSpec, Index: 8}}
	if got := canonicalSigType(nil, mswinmd.SigType{Kind: mswinmd.ElementType_I4}); got != "I4" {
		t.Fatalf("unmodified primitive changed to %q", got)
	}
	if got := canonicalSigType(nil, mswinmd.SigType{Kind: mswinmd.ElementType_I4, Mod: []mswinmd.SigCustomMod{opt, reqd}}); got != "modopt<typespec#7> modreq<typespec#8> I4" {
		t.Fatalf("custom modifiers canonicalized as %q", got)
	}
	if got := canonicalSigType(nil, mswinmd.SigType{Kind: mswinmd.ElementType_I4, Mod: []mswinmd.SigCustomMod{reqd, opt}}); got != "modreq<typespec#8> modopt<typespec#7> I4" {
		t.Fatalf("modifier order collapsed to %q", got)
	}
}

func TestCanonicalFunctionPointerPreservesInnerSignature(t *testing.T) {
	makeFn := func(cc mswinmd.SigCallingConvention) mswinmd.SigType {
		return mswinmd.SigType{Kind: mswinmd.ElementType_FNPTR, Value: mswinmd.SigStandAloneMethod{
			CallingConvention: cc,
			SigMethodRef: mswinmd.SigMethodRef{
				SigMethodDef: mswinmd.SigMethodDef{
					HasThis: true, ExplicitThis: false, VarArgs: true, Generic: 2,
					RetType: mswinmd.SigRetType{Kind: mswinmd.SigRetTypeKind_ByValue, Type: mswinmd.SigType{Kind: mswinmd.ElementType_I4}},
					Param:   []mswinmd.SigParam{{Kind: mswinmd.SigParamKind_ByValue, Type: mswinmd.SigType{Kind: mswinmd.ElementType_I2}}},
				},
				VariableParam: []mswinmd.SigParam{{Kind: mswinmd.SigParamKind_ByValue, Type: mswinmd.SigType{Kind: mswinmd.ElementType_U1}}},
			},
		}}
	}
	if got := canonicalSigType(nil, makeFn(mswinmd.SigCallingConvention_Cdecl)); got != "FNPTR<cc=1;this=true;explicit=false;varargs=true;generic=2;fixed=(I2);optional=(U1);ret=I4>" {
		t.Fatalf("function pointer canonicalized as %q", got)
	}
	if got := canonicalSigType(nil, makeFn(mswinmd.SigCallingConvention_Stdcall)); got != "FNPTR<cc=2;this=true;explicit=false;varargs=true;generic=2;fixed=(I2);optional=(U1);ret=I4>" {
		t.Fatalf("calling convention collapsed to %q", got)
	}
}

func TestModifiedSignatureTypesAreNotCallable(t *testing.T) {
	mod := mswinmd.SigCustomMod{Kind: mswinmd.SigCustomModKind_Reqd, Index: mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]{Tag: mswinmd.TypeDefOrRefOrSpec_TypeSpec, Index: 7}}
	modifiedInt := projectSigType(nil, mswinmd.SigType{Kind: mswinmd.ElementType_I4, Mod: []mswinmd.SigCustomMod{mod}})
	if !modifiedInt.Opaque {
		t.Fatal("modified primitive is not opaque")
	}
	fn := model.Function{DLL: "kernel32.dll", EntryPoint: "GetVersion", CallingConvention: "stdcall", Return: modifiedInt}
	if safePureGoPInvoke(fn) {
		t.Fatal("modified primitive passed the callable gate")
	}
	modifiedVoid := projectRet(nil, mswinmd.SigRetType{Kind: mswinmd.SigRetTypeKind_Void, Type: mswinmd.SigType{Kind: mswinmd.ElementType_VOID, Mod: []mswinmd.SigCustomMod{mod}}})
	if !modifiedVoid.Opaque {
		t.Fatal("modified void return is not opaque")
	}
	fn.Return = modifiedVoid
	if safePureGoPInvoke(fn) {
		t.Fatal("modified void return passed the callable gate")
	}
	if plainVoid := projectRet(nil, mswinmd.SigRetType{Kind: mswinmd.SigRetTypeKind_Void}); plainVoid.Opaque {
		t.Fatal("unmodified void return became opaque")
	}
}
