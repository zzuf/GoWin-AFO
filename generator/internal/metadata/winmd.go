package metadata

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	mswinmd "github.com/microsoft/go-winmd/winmd"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

type WinMDProvider struct{ SourceType string }

func (p WinMDProvider) Type() string { return p.SourceType }

type WinMDRawDump struct {
	MetadataVersion string            `json:"metadataVersion"`
	Tables          map[string]uint32 `json:"tables"`
	TypeDefs        []RawTypeDef      `json:"typeDefs"`
}
type RawTypeDef struct {
	Name, Namespace                              string
	Flags                                        uint32
	FieldStart, FieldEnd, MethodStart, MethodEnd uint32
}

func (p WinMDProvider) Ingest(_ context.Context, req Request) (Result, error) {
	m, err := mswinmd.Open(req.Path)
	if err != nil {
		return Result{}, fmt.Errorf("open winmd: %w", err)
	}
	r := Result{Source: model.Source{ID: req.Locked.ID, Type: req.Locked.Type, Package: req.Locked.Package, Version: req.Locked.Version, SHA256: req.Locked.SHA256, LicenseIdentifier: req.Locked.LicenseIdentifier, Architectures: append([]string(nil), req.Locked.Architectures...), WindowsSDKVersion: req.Locked.WindowsSDKVersion, Files: append([]string(nil), req.Locked.Files...)}}
	raw := WinMDRawDump{MetadataVersion: m.Version, Tables: map[string]uint32{}}
	for name, n := range map[string]uint32{
		"ClassLayout":            m.Tables.ClassLayout.Len(),
		"Constant":               m.Tables.Constant.Len(),
		"CustomAttribute":        m.Tables.CustomAttribute.Len(),
		"Event":                  m.Tables.Event.Len(),
		"EventMap":               m.Tables.EventMap.Len(),
		"Field":                  m.Tables.Field.Len(),
		"FieldLayout":            m.Tables.FieldLayout.Len(),
		"GenericParam":           m.Tables.GenericParam.Len(),
		"GenericParamConstraint": m.Tables.GenericParamConstraint.Len(),
		"ImplMap":                m.Tables.ImplMap.Len(),
		"InterfaceImpl":          m.Tables.InterfaceImpl.Len(),
		"MemberRef":              m.Tables.MemberRef.Len(),
		"MethodDef":              m.Tables.MethodDef.Len(),
		"MethodSemantics":        m.Tables.MethodSemantics.Len(),
		"ModuleRef":              m.Tables.ModuleRef.Len(),
		"NestedClass":            m.Tables.NestedClass.Len(),
		"Param":                  m.Tables.Param.Len(),
		"Property":               m.Tables.Property.Len(),
		"PropertyMap":            m.Tables.PropertyMap.Len(),
		"TypeDef":                m.Tables.TypeDef.Len(),
		"TypeRef":                m.Tables.TypeRef.Len(),
		"TypeSpec":               m.Tables.TypeSpec.Len(),
	} {
		raw.Tables[name] = n
	}
	moduleRefs := map[mswinmd.Index]string{}
	for i := range m.Tables.ModuleRef.Indices() {
		mr, e := m.Tables.ModuleRef.At(i)
		if e == nil {
			moduleRefs[i] = mr.Name.String()
		}
	}
	implMaps := map[mswinmd.Index]mswinmd.ImplMap{}
	for i := range m.Tables.ImplMap.Indices() {
		im, e := m.Tables.ImplMap.At(i)
		if e == nil && im.MemberForwarded.Tag == mswinmd.MemberForwarded_MethodDef {
			implMaps[im.MemberForwarded.Index] = im
		}
	}
	constants := map[mswinmd.Index]mswinmd.Constant{}
	for i := range m.Tables.Constant.Indices() {
		c, e := m.Tables.Constant.At(i)
		if e == nil && c.Parent.Tag == mswinmd.HasConstant_Field {
			constants[c.Parent.Index] = c
		}
	}
	nestedParents := map[mswinmd.Index]mswinmd.Index{}
	for i := range m.Tables.NestedClass.Indices() {
		n, e := m.Tables.NestedClass.At(i)
		if e == nil {
			nestedParents[n.NestedClass] = n.EnclosingClass
		}
	}
	propertyOwners := map[mswinmd.Index]mswinmd.Index{}
	for i := range m.Tables.PropertyMap.Indices() {
		pm, e := m.Tables.PropertyMap.At(i)
		if e != nil {
			r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("PropertyMap[%d]: %v", i, e)))
			continue
		}
		for property := range pm.PropertyList.All() {
			propertyOwners[property] = pm.Parent
		}
	}
	eventOwners := map[mswinmd.Index]mswinmd.Index{}
	for i := range m.Tables.EventMap.Indices() {
		em, e := m.Tables.EventMap.At(i)
		if e != nil {
			r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("EventMap[%d]: %v", i, e)))
			continue
		}
		for event := range em.EventList.All() {
			eventOwners[event] = em.Parent
		}
	}
	for ti := range m.Tables.TypeDef.Indices() {
		td, e := m.Tables.TypeDef.At(ti)
		if e != nil {
			r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("TypeDef[%d]: %v", ti, e)))
			r.Symbols = append(r.Symbols, metadataRowError(req, "TypeDef", uint32(ti), "", "", e))
			continue
		}
		rawName := td.Name.String()
		if rawName == "<Module>" {
			continue
		}
		name, ns := ownedTypeName(m, ti, nestedParents)
		raw.TypeDefs = append(raw.TypeDefs, RawTypeDef{Name: name, Namespace: ns, Flags: uint32(td.Flags), FieldStart: uint32(td.FieldList.Start), FieldEnd: uint32(td.FieldList.End), MethodStart: uint32(td.MethodList.Start), MethodEnd: uint32(td.MethodList.End)})
		kind := typeDefKind(m, td)
		arity := genericArity(name)
		baseCanonical := "typedef " + ns + "." + name
		typeStatus, statusReason := model.StatusUnsupportedProjection, "full-type-layout-and-attribute-projection-not-yet-emitted"
		if req.Locked.Type == "wdk-winmd" && kind != model.KindCOMInterface {
			statusReason = "wdk-type-inventoried-awaiting-safe-layout-projection"
		}
		typeSym := newMetadataSymbol(req, ns, kind, name, arity, baseCanonical, typeStatus, statusReason, model.BackendTypeOnly, "metadata type is inventory-only in current full-source pass", "TypeDef", uint32(ti))
		typeSym.Type = &model.Type{Kind: kind, NativeName: name, GoType: goIdentifier(name), Distinct: true}
		r.Symbols = append(r.Symbols, typeSym)
		for fi := range td.FieldList.All() {
			field, e := m.Tables.Field.At(fi)
			if e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("Field[%d]: %v", fi, e)))
				r.Symbols = append(r.Symbols, metadataRowError(req, "Field", uint32(fi), ns, name, e))
				continue
			}
			fsig, serr := m.FieldSignature(field.Signature)
			canonical := "field " + ns + "." + name + "." + field.Name.String() + ":" + hex.EncodeToString(field.Signature)
			typ := model.Type{Kind: model.KindOpaque, NativeName: "unparsed", GoType: "uintptr", Opaque: true}
			status, reason := model.StatusUnsupportedProjection, "field-layout-projection-not-yet-emitted"
			if serr == nil {
				canonical = "field " + ns + "." + name + "." + field.Name.String() + ":" + canonicalSigType(m, fsig.Type)
				typ = projectSigType(m, fsig.Type)
			} else {
				status = model.StatusSourceParseError
				reason = "microsoft-go-winmd-field-signature:" + serr.Error()
			}
			fkind := model.KindTypedef
			var constant *model.Constant
			if field.Flags.HasAll(mswinmd.FieldFlags_Literal) {
				fkind = model.KindConstant
				if c, ok := constants[fi]; ok {
					constant = &model.Constant{Type: typ, Value: "0x" + hex.EncodeToString(c.Value)}
					canonical += "=" + hex.EncodeToString(c.Value)
				}
			}
			fs := newMetadataSymbol(req, ns, fkind, field.Name.String(), 0, canonical, status, reason, model.BackendTypeOnly, "field or constant is not callable", "Field", uint32(fi))
			fs.Attributes = map[string]string{"declaringType": name}
			fs.Type = &typ
			fs.Constant = constant
			r.Symbols = append(r.Symbols, fs)
		}
		vtable := 0
		for mi := range td.MethodList.All() {
			method, e := m.Tables.MethodDef.At(mi)
			if e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("MethodDef[%d]: %v", mi, e)))
				r.Symbols = append(r.Symbols, metadataRowError(req, "MethodDef", uint32(mi), ns, name, e))
				continue
			}
			sig, serr := m.MethodDefSignature(method.Signature)
			canonical := "method " + ns + "." + method.Name.String() + ":" + hex.EncodeToString(method.Signature)
			status, reason := model.StatusUnsupportedProjection, "method-projection-not-yet-emitted"
			backend, backendReason := model.BackendUnsupported, "full-source method emitter is conservative"
			fn := &model.Function{CallingConvention: "platform", Parameters: []model.Parameter{}, Return: model.Type{Kind: model.KindPrimitive, GoType: "uintptr"}}
			if serr == nil {
				canonical = name + "::" + canonicalMethod(m, method.Name.String(), sig)
				fn = projectMethod(m, sig, method)
			} else {
				status = model.StatusSourceParseError
				reason = "microsoft-go-winmd-method-signature:" + serr.Error()
				backendReason = "signature parser rejected method"
			}
			if im, ok := implMaps[mi]; ok {
				fn.DLL = moduleRefs[im.ImportScope]
				fn.EntryPoint = im.ImportName.String()
				fn.SetLastError = im.MappingFlags.HasAll(mswinmd.PInvokeFlags_SupportsLastError)
				fn.CallingConvention = callingConvention(im.MappingFlags)
				candidate, candidateReason := capability(*fn)
				backendReason = "candidate=" + string(candidate) + ":" + candidateReason
				if serr != nil {
					// Preserve the parser error classification established above.
				} else if req.Locked.Type == "wdk-winmd" && isKernelDLL(fn.DLL) {
					status = model.StatusKernelModeOnly
					reason = "wdk-import-targets-kernel-module"
					backendReason = "ordinary Go process cannot call kernel export"
				} else if candidate == model.BackendPureGoSyscall && hasPointerABI(*fn) {
					status = model.StatusUnsupportedProjection
					reason = "pinvoke-pointer-lifetime-attributes-not-projected"
					backend = model.BackendUnsupported
					backendReason = "NativeArrayInfo/SAL/ownership/retention/alignment semantics are not yet proven; Go pointer wrapper withheld"
				} else if candidate == model.BackendPureGoSyscall && hasWideScalarByValue(*fn) {
					status = model.StatusUnsupportedGoABI
					reason = "windows-386-wide-scalar-requires-multiword-abi"
					backend = model.BackendUnsupported
					backendReason = "int64/uint64 by-value calls cannot be represented as one uintptr on windows/386; architecture trampoline pending"
				} else if candidate == model.BackendPureGoSyscall && safePureGoPInvoke(*fn) {
					status = model.StatusGeneratedPureGo
					reason = "official-winmd-pinvoke-fixed-integer-no-pointer-signature"
					backend = model.BackendPureGoSyscall
					backendReason = "fixed-width integer ABI with no pointer or wide-scalar values; DLL and entry point are fixed metadata"
					// Availability custom attributes are not yet fully projected. Treat
					// official imports as conditionally resolvable and emit IsXAvailable
					// instead of claiming the export exists on every Windows version.
					fn.OptionalExport = true
					if fn.SetLastError {
						fn.FailureRule = "metadata does not supply a failure sentinel; inspect the API result before using last error"
					} else {
						fn.FailureRule = "raw native return value"
					}
				} else if candidate == model.BackendPureGoSyscall {
					status = model.StatusUnsupportedProjection
					reason = "pinvoke-has-unresolved-or-unsafe-projected-type"
					backend = model.BackendUnsupported
					backendReason += "; emitted call withheld until every projected type is proven"
				} else {
					status = model.StatusUnsupportedGoABI
					reason = "pinvoke-requires-unimplemented-special-abi-backend"
					backend = model.BackendUnsupported
					backendReason += "; no false uintptr wrapper emitted"
				}
			}
			if kind == model.KindCOMInterface || kind == model.KindWinRTInterface {
				idx := vtable
				fn.VTableIndex = &idx
				vtable++
			}
			ms := newMetadataSymbol(req, ns, model.KindFunction, method.Name.String(), sigGeneric(sig, serr), canonical, status, reason, backend, backendReason, "MethodDef", uint32(mi))
			ms.Function = fn
			ms.Attributes = map[string]string{"declaringType": name}
			r.Symbols = append(r.Symbols, ms)
		}
	}
	for pi := range m.Tables.Property.Indices() {
		property, e := m.Tables.Property.At(pi)
		if e != nil {
			r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("Property[%d]: %v", pi, e)))
			r.Symbols = append(r.Symbols, metadataRowError(req, "Property", uint32(pi), "", "", e))
			continue
		}
		owner, ns := "<unknown-owner>", ""
		if typeIndex, ok := propertyOwners[pi]; ok {
			owner, ns = ownedTypeName(m, typeIndex, nestedParents)
		}
		name := property.Name.String()
		canonical := "property " + joinName(ns, owner) + "." + name + ":" + hex.EncodeToString(property.Type)
		s := newMetadataSymbol(req, ns, model.KindProperty, name, 0, canonical, model.StatusUnsupportedProjection, "property-signature-and-accessor-projection-not-yet-emitted", model.BackendTypeOnly, "property remains inventoried until accessor ownership and generic signatures are projected", "Property", uint32(pi))
		s.Attributes = map[string]string{"declaringType": owner}
		r.Symbols = append(r.Symbols, s)
	}
	for ei := range m.Tables.Event.Indices() {
		event, e := m.Tables.Event.At(ei)
		if e != nil {
			r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("Event[%d]: %v", ei, e)))
			r.Symbols = append(r.Symbols, metadataRowError(req, "Event", uint32(ei), "", "", e))
			continue
		}
		owner, ns := "<unknown-owner>", ""
		if typeIndex, ok := eventOwners[ei]; ok {
			owner, ns = ownedTypeName(m, typeIndex, nestedParents)
		}
		name := event.Name.String()
		canonical := "event " + joinName(ns, owner) + "." + name + ":" + resolveTypeHandle(m, event.EventType)
		s := newMetadataSymbol(req, ns, model.KindEvent, name, 0, canonical, model.StatusUnsupportedProjection, "event-delegate-and-token-projection-not-yet-emitted", model.BackendTypeOnly, "event remains inventoried until add/remove token semantics are projected", "Event", uint32(ei))
		s.Attributes = map[string]string{"declaringType": owner}
		r.Symbols = append(r.Symbols, s)
	}
	for gi := range m.Tables.GenericParam.Indices() {
		parameter, e := m.Tables.GenericParam.At(gi)
		if e != nil {
			r.Diagnostics = append(r.Diagnostics, diag(req.Locked.ID, "metadata-table-row", fmt.Sprintf("GenericParam[%d]: %v", gi, e)))
			r.Symbols = append(r.Symbols, metadataRowError(req, "GenericParam", uint32(gi), "", "", e))
			continue
		}
		name := parameter.Name.String()
		if name == "" {
			name = fmt.Sprintf("GenericParam_%d", gi)
		}
		owner := fmt.Sprintf("tag=%v,index=%d", parameter.Owner.Tag, parameter.Owner.Index)
		canonical := fmt.Sprintf("generic-param %s owner(%s) number=%d flags=0x%x", name, owner, parameter.Number, uint16(parameter.Flags))
		s := newMetadataSymbol(req, "", model.KindGenericType, name, 0, canonical, model.StatusUnsupportedProjection, "generic-parameter-constraint-projection-not-yet-emitted", model.BackendTypeOnly, "generic parameter is inventory-only", "GenericParam", uint32(gi))
		s.Attributes = map[string]string{"owner": owner}
		r.Symbols = append(r.Symbols, s)
	}
	r.Raw = raw
	return r, nil
}

func diag(source, code, msg string) model.Diagnostic {
	return model.Diagnostic{Severity: "warning", SourceID: source, Code: code, Message: msg}
}

func metadataRowError(req Request, table string, row uint32, namespace, owner string, parseErr error) model.Symbol {
	name := fmt.Sprintf("%sError_%d", table, row)
	if owner != "" {
		name = owner + "_" + name
	}
	s := newMetadataSymbol(req, namespace, model.KindOpaque, name, 0, fmt.Sprintf("metadata-row-error %s[%d]", table, row), model.StatusSourceParseError, "ecma335-table-row-read-error", model.BackendUnsupported, "row could not be decoded and no projection was emitted", table, row)
	s.Attributes = map[string]string{"parseError": parseErr.Error()}
	return s
}

func newMetadataSymbol(req Request, ns string, kind model.SymbolKind, name string, arity int, sig string, status model.Status, reason string, backend model.Backend, backendReason, table string, row uint32) model.Symbol {
	return model.Symbol{SourceID: req.Locked.ID, SourceType: req.Locked.Type, Namespace: ns, Kind: kind, NativeName: name, GoName: goPublicIdentifier(name), Architecture: "neutral", ABIProfile: req.ABIProfile, CanonicalSignature: sig, GenericArity: arity, Status: status, StatusReason: reason, Backend: backend, BackendReason: backendReason, Provenance: []model.Provenance{{SourceID: req.Locked.ID, InputFile: filepathBase(req.Path), MetadataTable: table, MetadataRow: row}}}
}

func filepathBase(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}
func genericArity(name string) int {
	if i := strings.LastIndexByte(name, '`'); i >= 0 {
		n, _ := strconv.Atoi(name[i+1:])
		return n
	}
	return 0
}
func sigGeneric(sig mswinmd.SigMethodDef, err error) int {
	if err != nil {
		return 0
	}
	return int(sig.Generic)
}

func typeDefKind(m *mswinmd.Metadata, td mswinmd.TypeDef) model.SymbolKind {
	if td.Flags.Semantics() == mswinmd.TypeSemantics_Interface {
		if strings.HasPrefix(td.Namespace.String(), "Windows.") && !strings.HasPrefix(td.Namespace.String(), "Windows.Win32.") {
			return model.KindWinRTInterface
		}
		return model.KindCOMInterface
	}
	base := resolveTypeHandle(m, td.Extends)
	switch base {
	case "System.Enum":
		return model.KindEnum
	case "System.ValueType":
		return model.KindStruct
	case "System.MulticastDelegate":
		return model.KindDelegate
	}
	if strings.HasPrefix(td.Namespace.String(), "Windows.") && !strings.HasPrefix(td.Namespace.String(), "Windows.Win32.") {
		return model.KindRuntimeClass
	}
	return model.KindStruct
}

func resolveTypeHandle(m *mswinmd.Metadata, idx mswinmd.CodedIndex[mswinmd.TypeDefOrRef]) string {
	switch idx.Tag {
	case mswinmd.TypeDefOrRef_TypeRef:
		r, e := m.Tables.TypeRef.At(idx.Index)
		if e == nil {
			return joinName(r.Namespace.String(), r.Name.String())
		}
	case mswinmd.TypeDefOrRef_TypeDef:
		r, e := m.Tables.TypeDef.At(idx.Index)
		if e == nil {
			return joinName(r.Namespace.String(), r.Name.String())
		}
	}
	return ""
}
func resolveSigHandle(m *mswinmd.Metadata, idx mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]) string {
	switch idx.Tag {
	case mswinmd.TypeDefOrRefOrSpec_TypeRef:
		r, e := m.Tables.TypeRef.At(idx.Index)
		if e == nil {
			return joinName(r.Namespace.String(), r.Name.String())
		}
	case mswinmd.TypeDefOrRefOrSpec_TypeDef:
		r, e := m.Tables.TypeDef.At(idx.Index)
		if e == nil {
			return joinName(r.Namespace.String(), r.Name.String())
		}
	case mswinmd.TypeDefOrRefOrSpec_TypeSpec:
		return "typespec#" + strconv.FormatUint(uint64(idx.Index), 10)
	}
	return "unknown"
}
func joinName(ns, n string) string {
	if ns == "" {
		return n
	}
	return ns + "." + n
}

func ownedTypeName(m *mswinmd.Metadata, index mswinmd.Index, parents map[mswinmd.Index]mswinmd.Index) (string, string) {
	seen := map[mswinmd.Index]bool{}
	var names []string
	current := index
	ns := ""
	for {
		if seen[current] {
			return strings.Join(names, "."), ns
		}
		seen[current] = true
		td, err := m.Tables.TypeDef.At(current)
		if err != nil {
			return strings.Join(names, "."), ns
		}
		names = append([]string{td.Name.String()}, names...)
		if td.Namespace.String() != "" {
			ns = td.Namespace.String()
		}
		parent, ok := parents[current]
		if !ok {
			break
		}
		current = parent
	}
	return strings.Join(names, "."), ns
}

func canonicalMethod(m *mswinmd.Metadata, name string, s mswinmd.SigMethodDef) string {
	params := make([]string, len(s.Param))
	for i, p := range s.Param {
		params[i] = canonicalSigType(m, p.Type)
	}
	return name + "(" + strings.Join(params, ",") + ")->" + retCanonical(m, s.RetType)
}
func retCanonical(m *mswinmd.Metadata, r mswinmd.SigRetType) string {
	if r.Kind == mswinmd.SigRetTypeKind_Void && len(r.Type.Mod) == 0 {
		return "void"
	}
	return canonicalSigType(m, r.Type)
}
func canonicalSigType(m *mswinmd.Metadata, t mswinmd.SigType) string {
	base := canonicalSigTypeUnmodified(m, t)
	if len(t.Mod) == 0 {
		return base
	}
	mods := make([]string, len(t.Mod))
	for i, mod := range t.Mod {
		kind := "modopt"
		switch mod.Kind {
		case mswinmd.SigCustomModKind_Reqd:
			kind = "modreq"
		case mswinmd.SigCustomModKind_Opt:
		default:
			kind = fmt.Sprintf("modkind#%d", mod.Kind)
		}
		mods[i] = kind + "<" + resolveSigHandle(m, mod.Index) + ">"
	}
	return strings.Join(mods, " ") + " " + base
}

func canonicalSigTypeUnmodified(m *mswinmd.Metadata, t mswinmd.SigType) string {
	switch t.Kind {
	case mswinmd.ElementType_PTR, mswinmd.ElementType_BYREF:
		if inner, ok := t.Value.(mswinmd.SigType); ok {
			return t.Kind.String() + "<" + canonicalSigType(m, inner) + ">"
		}
	case mswinmd.ElementType_VALUETYPE, mswinmd.ElementType_CLASS:
		if idx, ok := t.Value.(mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]); ok {
			return t.Kind.String() + "<" + resolveSigHandle(m, idx) + ">"
		}
	case mswinmd.ElementType_ARRAY:
		if a, ok := t.Value.(mswinmd.SigArray); ok {
			sizes := make([]string, len(a.Sizes))
			for i, size := range a.Sizes {
				sizes[i] = strconv.FormatUint(uint64(size), 10)
			}
			bounds := make([]string, len(a.LowerBounds))
			for i, bound := range a.LowerBounds {
				bounds[i] = strconv.FormatInt(int64(bound), 10)
			}
			return fmt.Sprintf("array[rank=%d;sizes=%s;lower=%s]<%s>", a.Rank, strings.Join(sizes, ","), strings.Join(bounds, ","), canonicalSigType(m, a.Type))
		}
	case mswinmd.ElementType_SZARRAY:
		if element, ok := t.Value.(mswinmd.SigType); ok {
			return "SZARRAY<" + canonicalSigType(m, element) + ">"
		}
	case mswinmd.ElementType_GENERICINST:
		if instance, ok := t.Value.(mswinmd.SigGenericInst); ok {
			kind := "valuetype"
			if instance.Class {
				kind = "class"
			}
			arguments := make([]string, len(instance.Type))
			for i, argument := range instance.Type {
				arguments[i] = canonicalSigType(m, argument)
			}
			return "GENERICINST<" + kind + " " + resolveSigHandle(m, instance.Index) + "<" + strings.Join(arguments, ",") + ">>"
		}
	case mswinmd.ElementType_VAR, mswinmd.ElementType_MVAR:
		if parameter, ok := t.Value.(uint32); ok {
			return fmt.Sprintf("%s(%d)", t.Kind, parameter)
		}
	case mswinmd.ElementType_FNPTR:
		if fn, ok := t.Value.(mswinmd.SigStandAloneMethod); ok {
			fixed := make([]string, len(fn.Param))
			for i, param := range fn.Param {
				fixed[i] = canonicalSigType(m, param.Type)
			}
			optional := make([]string, len(fn.VariableParam))
			for i, param := range fn.VariableParam {
				optional[i] = canonicalSigType(m, param.Type)
			}
			return fmt.Sprintf("FNPTR<cc=%d;this=%t;explicit=%t;varargs=%t;generic=%d;fixed=(%s);optional=(%s);ret=%s>",
				fn.CallingConvention, fn.HasThis, fn.ExplicitThis, fn.VarArgs, fn.Generic,
				strings.Join(fixed, ","), strings.Join(optional, ","), retCanonical(m, fn.RetType))
		}
	}
	return t.Kind.String()
}

func projectMethod(m *mswinmd.Metadata, s mswinmd.SigMethodDef, method mswinmd.MethodDef) *model.Function {
	fn := &model.Function{CallingConvention: "platform", VarArgs: s.VarArgs}
	fn.Return = projectRet(m, s.RetType)
	usedNames := map[string]int{}
	for i, p := range s.Param {
		name := "arg" + strconv.Itoa(i)
		if uint32(i) < method.ParamList.Len() {
			row := method.ParamList.Start + mswinmd.Index(i)
			if pr, e := m.Tables.Param.At(row); e == nil && pr.Name.String() != "" {
				name = pr.Name.String()
			}
		}
		name = uniqueParameterName(goIdentifier(name), usedNames)
		dir := model.DirectionIn
		if p.Kind == mswinmd.SigParamKind_ByRef {
			dir = model.DirectionInOut
		}
		fn.Parameters = append(fn.Parameters, model.Parameter{Name: name, Type: projectSigType(m, p.Type), Direction: dir})
	}
	return fn
}
func projectRet(m *mswinmd.Metadata, r mswinmd.SigRetType) model.Type {
	if r.Kind == mswinmd.SigRetTypeKind_Void {
		return model.Type{Kind: model.KindPrimitive, NativeName: "void", Opaque: len(r.Type.Mod) != 0}
	}
	// The upstream reader preserves the BYREF wrapper in r.Type, so adding
	// another pointer here would turn a native T* return into T**.
	return projectSigType(m, r.Type)
}
func projectSigType(m *mswinmd.Metadata, t mswinmd.SigType) model.Type {
	native, goType, bits, signed := primitive(t.Kind)
	out := model.Type{Kind: model.KindPrimitive, NativeName: native, GoType: goType, Bits: bits, Signed: signed, Opaque: len(t.Mod) != 0}
	switch t.Kind {
	case mswinmd.ElementType_PTR, mswinmd.ElementType_BYREF:
		out.Kind = model.KindPointer
		if inner, ok := t.Value.(mswinmd.SigType); ok {
			x := projectSigType(m, inner)
			out.Element = &x
			if x.GoType == "" {
				out.GoType = "unsafe.Pointer"
			} else {
				out.GoType = "*" + x.GoType
			}
			out.PointerDepth = x.PointerDepth + 1
		}
	case mswinmd.ElementType_CLASS, mswinmd.ElementType_VALUETYPE:
		if idx, ok := t.Value.(mswinmd.CodedIndex[mswinmd.TypeDefOrRefOrSpec]); ok {
			out.NativeName = resolveSigHandle(m, idx)
			out.GoType = goIdentifier(lastName(out.NativeName))
			out.Kind = model.KindNativeTypedef
			out.Distinct = true
		}
	case mswinmd.ElementType_ARRAY:
		out.Kind = model.KindFixedArray
		if a, ok := t.Value.(mswinmd.SigArray); ok {
			x := projectSigType(m, a.Type)
			out.Element = &x
			if len(a.Sizes) == 1 {
				out.Length = int(a.Sizes[0])
				out.GoType = fmt.Sprintf("[%d]%s", out.Length, x.GoType)
			} else {
				out.GoType = "uintptr"
			}
		}
	}
	if !knownPrimitiveElement(t.Kind) && t.Kind != mswinmd.ElementType_PTR && t.Kind != mswinmd.ElementType_BYREF && t.Kind != mswinmd.ElementType_CLASS && t.Kind != mswinmd.ElementType_VALUETYPE && t.Kind != mswinmd.ElementType_ARRAY {
		out.Opaque = true
	}
	if out.GoType == "" {
		out.GoType = "uintptr"
		out.Opaque = true
	}
	return out
}
func primitive(k mswinmd.ElementType) (string, string, int, bool) {
	switch k {
	case mswinmd.ElementType_VOID:
		return "void", "", 0, false
	case mswinmd.ElementType_BOOLEAN:
		return "BOOLEAN", "uint8", 8, false
	case mswinmd.ElementType_CHAR:
		return "WCHAR", "uint16", 16, false
	case mswinmd.ElementType_I1:
		return "CHAR", "int8", 8, true
	case mswinmd.ElementType_U1:
		return "BYTE", "uint8", 8, false
	case mswinmd.ElementType_I2:
		return "SHORT", "int16", 16, true
	case mswinmd.ElementType_U2:
		return "USHORT", "uint16", 16, false
	case mswinmd.ElementType_I4:
		return "LONG", "int32", 32, true
	case mswinmd.ElementType_U4:
		return "ULONG", "uint32", 32, false
	case mswinmd.ElementType_I8:
		return "LONGLONG", "int64", 64, true
	case mswinmd.ElementType_U8:
		return "ULONGLONG", "uint64", 64, false
	case mswinmd.ElementType_R4:
		return "FLOAT", "float32", 32, true
	case mswinmd.ElementType_R8:
		return "DOUBLE", "float64", 64, true
	case mswinmd.ElementType_I:
		return "INT_PTR", "winabi.INT_PTR", 0, true
	case mswinmd.ElementType_U:
		return "UINT_PTR", "uintptr", 0, false
	case mswinmd.ElementType_STRING:
		return "PWSTR", "*uint16", 0, false
	case mswinmd.ElementType_OBJECT:
		return "IUnknown", "uintptr", 0, false
	}
	return k.String(), "uintptr", 0, false
}

func knownPrimitiveElement(k mswinmd.ElementType) bool {
	switch k {
	case mswinmd.ElementType_VOID, mswinmd.ElementType_BOOLEAN, mswinmd.ElementType_CHAR,
		mswinmd.ElementType_I1, mswinmd.ElementType_U1, mswinmd.ElementType_I2,
		mswinmd.ElementType_U2, mswinmd.ElementType_I4, mswinmd.ElementType_U4,
		mswinmd.ElementType_I8, mswinmd.ElementType_U8, mswinmd.ElementType_R4,
		mswinmd.ElementType_R8, mswinmd.ElementType_I, mswinmd.ElementType_U,
		mswinmd.ElementType_STRING:
		return true
	default:
		return false
	}
}

func safePureGoPInvoke(fn model.Function) bool {
	if fn.DLL == "" || fn.EntryPoint == "" || !safeSystemDLLLiteral(fn.DLL) {
		return false
	}
	if fn.CallingConvention != "platform" && fn.CallingConvention != "stdcall" {
		// cdecl is not interchangeable with stdcall on the supported 386 target.
		return false
	}
	if !safeWordType(fn.Return, true) {
		return false
	}
	for _, p := range fn.Parameters {
		if !safeWordType(p.Type, false) {
			return false
		}
	}
	return true
}

func safeWordType(t model.Type, allowVoid bool) bool {
	if t.Opaque || t.Bits > 64 {
		return false
	}
	if t.Kind == model.KindPointer {
		return t.Element != nil && safePointerElement(*t.Element)
	}
	if t.Kind != model.KindPrimitive {
		return false
	}
	switch t.GoType {
	case "":
		return allowVoid && t.NativeName == "void"
	case "int8", "uint8", "int16", "uint16", "int32", "uint32", "uintptr", "winabi.INT_PTR", "*uint16":
		return true
	default:
		return false
	}
}

func safePointerElement(t model.Type) bool {
	if t.Opaque {
		return false
	}
	if t.Kind == model.KindPointer {
		return t.Element != nil && safePointerElement(*t.Element)
	}
	if t.Kind != model.KindPrimitive {
		return false
	}
	switch t.GoType {
	case "", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "uintptr", "winabi.INT_PTR", "*uint16":
		return true
	default:
		return false
	}
}

func hasWideScalarByValue(fn model.Function) bool {
	if wideScalar(fn.Return) {
		return true
	}
	for _, p := range fn.Parameters {
		if wideScalar(p.Type) {
			return true
		}
	}
	return false
}

func hasPointerABI(fn model.Function) bool {
	if pointerABIType(fn.Return) {
		return true
	}
	for _, p := range fn.Parameters {
		if pointerABIType(p.Type) {
			return true
		}
	}
	return false
}

func pointerABIType(t model.Type) bool {
	return t.Kind == model.KindPointer || t.PointerDepth > 0 || strings.HasPrefix(t.GoType, "*") || t.GoType == "unsafe.Pointer"
}

func wideScalar(t model.Type) bool {
	return t.Kind == model.KindPrimitive && t.PointerDepth == 0 && (t.GoType == "int64" || t.GoType == "uint64")
}

func safeSystemDLLLiteral(name string) bool {
	if len(name) < 5 || len(name) > 255 || !strings.EqualFold(name[len(name)-4:], ".dll") || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func capability(fn model.Function) (model.Backend, string) {
	if fn.VarArgs {
		return model.BackendCGOBridge, "varargs requires generated C adapter"
	}
	if fn.CallingConvention == "vectorcall" || fn.CallingConvention == "thiscall" {
		return model.BackendCGOBridge, "calling convention not represented by syscall"
	}
	hasFloat := fn.Return.GoType == "float32" || fn.Return.GoType == "float64"
	complex := byValueComplex(fn.Return)
	for _, p := range fn.Parameters {
		hasFloat = hasFloat || p.Type.GoType == "float32" || p.Type.GoType == "float64"
		complex = complex || byValueComplex(p.Type)
	}
	if complex {
		return model.BackendCGOBridge, "aggregate value ABI requires compiler bridge"
	}
	if hasFloat {
		return model.BackendPureGoSyscallFloat, "floating register ABI requires architecture trampoline"
	}
	return model.BackendPureGoSyscall, "integer and pointer ABI is representable"
}
func byValueComplex(t model.Type) bool {
	return (t.Kind == model.KindStruct || t.Kind == model.KindUnion) && t.PointerDepth == 0
}
func callingConvention(f mswinmd.PInvokeAttributes) string {
	switch f.CallingConvention() {
	case mswinmd.PInvokeCallingConvention_Cdecl:
		return "cdecl"
	case mswinmd.PInvokeCallingConvention_Stdcall:
		return "stdcall"
	case mswinmd.PInvokeCallingConvention_Thiscall:
		return "thiscall"
	case mswinmd.PInvokeCallingConvention_Fastcall:
		return "fastcall"
	default:
		return "platform"
	}
}
func isKernelDLL(dll string) bool {
	d := strings.ToLower(dll)
	return strings.Contains(d, "ntoskrnl") || strings.Contains(d, "hal.dll") || strings.Contains(d, "ndis.sys") || strings.HasSuffix(d, ".sys")
}
func lastName(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}
func goIdentifier(s string) string {
	s = lastName(s)
	s = strings.TrimSuffix(s, "`"+strconv.Itoa(genericArity(s)))
	var b strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "Unnamed"
	}
	identifier := b.String()
	if goKeywords[identifier] {
		identifier += "_"
	}
	return identifier
}

func goPublicIdentifier(s string) string {
	identifier := goIdentifier(s)
	if identifier[0] >= 'a' && identifier[0] <= 'z' {
		identifier = strings.ToUpper(identifier[:1]) + identifier[1:]
	} else if identifier[0] == '_' || identifier[0] >= '0' && identifier[0] <= '9' {
		identifier = "X" + identifier
	}
	return identifier
}

func uniqueParameterName(name string, used map[string]int) string {
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	ordinal := used[name]
	used[name]++
	return name + "_" + strconv.Itoa(ordinal)
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}
