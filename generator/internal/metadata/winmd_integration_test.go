package metadata

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	mswinmd "github.com/microsoft/go-winmd/winmd"
)

func cachedWinMD(t *testing.T, source, file string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sources", "cache", source, file)
	if _, err := filepath.Glob(path); err != nil {
		t.Fatal(err)
	}
	if _, err := mswinmd.Open(path); err != nil {
		t.Skipf("pinned source cache unavailable: %v", err)
	}
	return path
}

func TestOfficialWin32TablesAndSignatures(t *testing.T) {
	path := cachedWinMD(t, "microsoft-win32metadata", "Windows.Win32.winmd")
	m, err := mswinmd.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]uint32{"TypeRef": m.Tables.TypeRef.Len(), "TypeDef": m.Tables.TypeDef.Len(), "MemberRef": m.Tables.MemberRef.Len(), "MethodDef": m.Tables.MethodDef.Len(), "Field": m.Tables.Field.Len(), "Constant": m.Tables.Constant.Len(), "ImplMap": m.Tables.ImplMap.Len(), "CustomAttribute": m.Tables.CustomAttribute.Len()}
	for name, count := range counts {
		if count == 0 {
			t.Errorf("official metadata table %s is empty", name)
		}
	}
	assertRepresentativeRows(t, m)
	parsed := 0
	for i := range m.Tables.MethodDef.Indices() {
		row, e := m.Tables.MethodDef.At(i)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = m.MethodDefSignature(row.Signature); e == nil {
			parsed++
			if parsed == 32 {
				break
			}
		}
	}
	if parsed == 0 {
		t.Fatal("no MethodDef signature decoded")
	}
}

func assertRepresentativeRows(t *testing.T, m *mswinmd.Metadata) {
	t.Helper()

	typeRefDecoded := false
	for i := range m.Tables.TypeRef.Indices() {
		row, err := m.Tables.TypeRef.At(i)
		if err != nil {
			t.Fatalf("TypeRef[%d]: %v", i, err)
		}
		if row.Name.String() != "" {
			typeRefDecoded = true
			break
		}
	}
	if !typeRefDecoded {
		t.Fatal("no representative TypeRef name decoded")
	}

	typeDefDecoded := false
	for i := range m.Tables.TypeDef.Indices() {
		row, err := m.Tables.TypeDef.At(i)
		if err != nil {
			t.Fatalf("TypeDef[%d]: %v", i, err)
		}
		if name := row.Name.String(); name != "" && name != "<Module>" {
			if row.FieldList.End < row.FieldList.Start || row.MethodList.End < row.MethodList.Start {
				t.Fatalf("TypeDef[%d] has descending member ranges", i)
			}
			typeDefDecoded = true
			break
		}
	}
	if !typeDefDecoded {
		t.Fatal("no representative TypeDef decoded")
	}

	memberRefDecoded := false
	for i := range m.Tables.MemberRef.Indices() {
		row, err := m.Tables.MemberRef.At(i)
		if err != nil {
			t.Fatalf("MemberRef[%d]: %v", i, err)
		}
		if row.Name.String() != "" && len(row.Signature) != 0 {
			memberRefDecoded = true
			break
		}
	}
	if !memberRefDecoded {
		t.Fatal("no representative MemberRef name/signature decoded")
	}

	fieldDecoded := false
	for i := range m.Tables.Field.Indices() {
		row, err := m.Tables.Field.At(i)
		if err != nil {
			t.Fatalf("Field[%d]: %v", i, err)
		}
		if row.Name.String() == "" {
			continue
		}
		if _, err = m.FieldSignature(row.Signature); err == nil {
			fieldDecoded = true
			break
		}
	}
	if !fieldDecoded {
		t.Fatal("no representative Field signature decoded")
	}

	constantDecoded := false
	for i := range m.Tables.Constant.Indices() {
		row, err := m.Tables.Constant.At(i)
		if err != nil {
			t.Fatalf("Constant[%d]: %v", i, err)
		}
		if len(row.Value) != 0 && row.Parent.Index != 0 {
			constantDecoded = true
			break
		}
	}
	if !constantDecoded {
		t.Fatal("no representative Constant value/parent decoded")
	}

	implMapDecoded := false
	for i := range m.Tables.ImplMap.Indices() {
		row, err := m.Tables.ImplMap.At(i)
		if err != nil {
			t.Fatalf("ImplMap[%d]: %v", i, err)
		}
		module, err := m.Tables.ModuleRef.At(row.ImportScope)
		if err != nil {
			t.Fatalf("ImplMap[%d] ModuleRef[%d]: %v", i, row.ImportScope, err)
		}
		if row.MemberForwarded.Tag == mswinmd.MemberForwarded_MethodDef && row.ImportName.String() != "" && module.Name.String() != "" && row.MappingFlags.CallingConvention() != 0 {
			implMapDecoded = true
			break
		}
	}
	if !implMapDecoded {
		t.Fatal("no representative P/Invoke map decoded with method, DLL, entry point, and calling convention")
	}

	architectureAttributeDecoded := false
	for i := range m.Tables.CustomAttribute.Indices() {
		row, err := m.Tables.CustomAttribute.At(i)
		if err != nil {
			t.Fatalf("CustomAttribute[%d]: %v", i, err)
		}
		if customAttributeTypeName(m, row) != "SupportedArchitectureAttribute" {
			continue
		}
		if len(row.Value) < 2 || row.Value[0] != 1 || row.Value[1] != 0 {
			t.Fatalf("CustomAttribute[%d] SupportedArchitecture has invalid ECMA-335 prolog", i)
		}
		architectureAttributeDecoded = true
		break
	}
	if !architectureAttributeDecoded {
		t.Fatal("no SupportedArchitectureAttribute constructor/value decoded")
	}
}

func customAttributeTypeName(m *mswinmd.Metadata, attribute mswinmd.CustomAttribute) string {
	if attribute.Type.Tag != mswinmd.CustomAttributeType_MemberRef {
		return ""
	}
	constructor, err := m.Tables.MemberRef.At(attribute.Type.Index)
	if err != nil {
		return ""
	}
	switch constructor.Class.Tag {
	case mswinmd.MemberRefParent_TypeRef:
		typeRef, err := m.Tables.TypeRef.At(constructor.Class.Index)
		if err == nil {
			return typeRef.Name.String()
		}
	case mswinmd.MemberRefParent_TypeDef:
		typeDef, err := m.Tables.TypeDef.At(constructor.Class.Index)
		if err == nil {
			return typeDef.Name.String()
		}
	}
	return ""
}

// Generic signatures in the pinned SDK must be decoded by the upstream reader.
// A regression here would silently turn valid methods into parse errors.
func TestUpstreamGenericSignaturesDecode(t *testing.T) {
	path := cachedWinMD(t, "windows-sdk-winrt", "Windows.winmd")
	m, err := mswinmd.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tables.GenericParam.Len() == 0 || m.Tables.Property.Len() == 0 || m.Tables.Event.Len() == 0 {
		t.Fatal("WinRT generic/property/event tables are unexpectedly empty")
	}
	decoded := false
	for i := range m.Tables.MethodDef.Indices() {
		row, e := m.Tables.MethodDef.At(i)
		if e != nil {
			t.Fatal(e)
		}
		_, e = m.MethodDefSignature(row.Signature)
		if e != nil && strings.Contains(e.Error(), "generic types are not yet supported") {
			t.Fatalf("MethodDef[%d] generic signature still rejected: %v", i, e)
		}
		if e == nil && bytes.Contains(row.Signature, []byte{0x15}) {
			decoded = true
		}
	}
	if !decoded {
		t.Fatal("no representative generic signature decoded")
	}
}
