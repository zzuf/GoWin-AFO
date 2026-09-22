package verify

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

type inventoryComparison struct {
	Symbols         []SymbolResult
	Unmatched       []UnmatchedFact
	Mismatches      []Mismatch
	MismatchedFacts int
}

type comparisonBuilder struct {
	architecture string
	result       inventoryComparison
	symbols      map[string]*SymbolResult
}

func compareInventory(inventory model.Inventory, manifest ProbeManifest, oracle OracleResult) inventoryComparison {
	builder := &comparisonBuilder{architecture: oracle.Architecture, symbols: map[string]*SymbolResult{}}
	functionProbes := make(map[string]FunctionProbe, len(manifest.Functions))
	for _, probe := range manifest.Functions {
		functionProbes[probe.ID] = probe
	}
	bitFieldProbes := make(map[string]BitFieldProbe, len(manifest.BitFields))
	for _, probe := range manifest.BitFields {
		bitFieldProbes[probe.ID] = probe
	}

	for _, record := range oracle.Records {
		compareRecord(builder, inventory, oracle.Architecture, record)
	}
	for _, value := range oracle.Values {
		compareConstant(builder, inventory, oracle.Architecture, value)
	}
	for _, guid := range oracle.GUIDs {
		compareGUID(builder, inventory, oracle.Architecture, guid)
	}
	for _, field := range oracle.BitFields {
		compareBitField(builder, inventory, oracle.Architecture, bitFieldProbes[field.ID], field)
	}
	for _, function := range oracle.Functions {
		compareFunction(builder, inventory, oracle.Architecture, functionProbes[function.ID], function)
	}
	for _, table := range oracle.VTables {
		compareVTable(builder, inventory, oracle.Architecture, table)
	}
	for _, condition := range oracle.Conditions {
		builder.unmatched(nil, condition.ID+".value", "target condition has no corresponding normalized IR symbol")
	}
	for _, symbol := range builder.symbols {
		symbol.Complete = len(symbol.UnmatchedFactIDs) == 0 && len(symbol.MismatchedFactIDs) == 0
		builder.result.Symbols = append(builder.result.Symbols, *symbol)
	}
	sort.Slice(builder.result.Symbols, func(i, j int) bool { return builder.result.Symbols[i].SymbolID < builder.result.Symbols[j].SymbolID })
	return builder.result
}

func compareRecord(builder *comparisonBuilder, inventory model.Inventory, architecture string, record RecordResult) {
	facts := []string{record.ID + ".size", record.ID + ".alignment"}
	for _, field := range record.Fields {
		facts = append(facts, record.ID+".field."+field.ID+".offset")
	}
	symbol := findSymbol(inventory, architecture, record.NativeName, model.KindStruct, model.KindUnion, model.KindGUID)
	if symbol == nil || symbol.Type == nil {
		builder.unmatchedMany(nil, facts, "no matching normalized IR record")
		return
	}
	symbolResult := builder.symbol(symbol)
	kind := string(symbol.Type.Kind)
	if symbol.Kind == model.KindGUID {
		kind = "struct"
	}
	if kind != record.Kind {
		builder.mismatch(symbolResult, "record-kind-mismatch", record.ID+".kind", "/records/"+record.ID+"/kind", record.Kind, kind)
		builder.unmatchedMany(symbolResult, facts, "record kind differs from oracle")
		return
	}
	layout, ok := findLayout(symbol.Type.Layouts, architecture)
	if !ok {
		builder.unmatchedMany(symbolResult, facts, "normalized IR has no layout for this architecture")
		return
	}
	builder.compareUint(symbolResult, record.ID+".size", "/records/"+record.ID+"/size", record.Size, layout.Size)
	builder.compareUint(symbolResult, record.ID+".alignment", "/records/"+record.ID+"/alignment", record.Alignment, layout.Alignment)
	offsets, known := recordOffsets(*symbol.Type, layout, architecture)
	for _, field := range record.Fields {
		factID := record.ID + ".field." + field.ID + ".offset"
		modelField, fieldOK := findModelField(symbol.Type.Fields, field.NativeName)
		if !fieldOK {
			builder.unmatched(symbolResult, factID, "normalized IR record has no matching native field")
			continue
		}
		offset, offsetOK := offsets[modelField.Name]
		if !known || !offsetOK {
			builder.unmatched(symbolResult, factID, "field offset is not explicitly represented or safely derivable")
			continue
		}
		builder.compareUint(symbolResult, factID, "/records/"+record.ID+"/fields/"+field.ID+"/offset", field.Offset, offset)
	}
}

func compareConstant(builder *comparisonBuilder, inventory model.Inventory, architecture string, value ValueResult) {
	factID := value.ID + ".value"
	symbol := findSymbol(inventory, architecture, value.NativeName, model.KindConstant)
	if symbol == nil || symbol.Constant == nil {
		builder.unmatched(nil, factID, "no matching normalized IR constant")
		return
	}
	symbolResult := builder.symbol(symbol)
	expected, expectedOK := new(big.Int).SetString(value.Value, 10)
	actual, actualOK := new(big.Int).SetString(symbol.Constant.Value, 0)
	if !expectedOK || !actualOK {
		builder.unmatched(symbolResult, factID, "constant value is not represented as an exact integer")
		return
	}
	if expected.Cmp(actual) == 0 {
		builder.matched(symbolResult, factID)
		return
	}
	builder.mismatch(symbolResult, "constant-value-mismatch", factID, "/values/"+value.ID+"/value", value.Value, symbol.Constant.Value)
}

func compareGUID(builder *comparisonBuilder, inventory model.Inventory, architecture string, guid GUIDResult) {
	factID := guid.ID + ".value"
	symbol := findGUIDSymbol(inventory, architecture, guid.NativeName)
	if symbol == nil || symbol.GUID == nil {
		builder.unmatched(nil, factID, "no matching normalized IR GUID value")
		return
	}
	symbolResult := builder.symbol(symbol)
	actual := formatGUID(*symbol.GUID)
	if actual == guid.Value {
		builder.matched(symbolResult, factID)
		return
	}
	builder.mismatch(symbolResult, "guid-value-mismatch", factID, "/guids/"+guid.ID+"/value", guid.Value, actual)
}

func compareBitField(builder *comparisonBuilder, inventory model.Inventory, architecture string, probe BitFieldProbe, field BitFieldResult) {
	facts := []string{field.ID + ".maskHex", field.ID + ".bitOffset", field.ID + ".bitWidth"}
	if probe.ID == "" || probe.Record == "" {
		builder.unmatchedMany(nil, facts, "reviewed probe manifest has no matching bit-field declaration")
		return
	}
	symbol := findSymbol(inventory, architecture, probe.Record, model.KindStruct, model.KindUnion)
	if symbol == nil || symbol.Type == nil {
		builder.unmatchedMany(nil, facts, "no matching normalized IR bit-field record")
		return
	}
	symbolResult := builder.symbol(symbol)
	modelField, ok := findModelField(symbol.Type.Fields, probe.Field)
	if !ok || modelField.BitField == nil {
		builder.unmatchedMany(symbolResult, facts, "normalized IR has no matching bit-field metadata")
		return
	}
	builder.compareUint(symbolResult, field.ID+".bitOffset", "/bitFields/"+field.ID+"/bitOffset", field.BitOffset, uint64(modelField.BitField.BitOffset))
	builder.compareUint(symbolResult, field.ID+".bitWidth", "/bitFields/"+field.ID+"/bitWidth", field.BitWidth, uint64(modelField.BitField.BitWidth))
	mask, maskOK := bitMaskHex(uint64(modelField.BitField.BitOffset), uint64(modelField.BitField.BitWidth), len(field.MaskHex)/2)
	if !maskOK {
		builder.unmatched(symbolResult, field.ID+".maskHex", "bit-field mask cannot be represented in the oracle storage width")
	} else if mask == field.MaskHex {
		builder.matched(symbolResult, field.ID+".maskHex")
	} else {
		builder.mismatch(symbolResult, "bit-field-mask-mismatch", field.ID+".maskHex", "/bitFields/"+field.ID+"/maskHex", field.MaskHex, mask)
	}
}

func compareFunction(builder *comparisonBuilder, inventory model.Inventory, architecture string, probe FunctionProbe, function FunctionResult) {
	typeFact := function.ID + ".typeCompatible"
	callingFact := function.ID + ".callingConvention"
	symbol := findSymbol(inventory, architecture, function.NativeName, model.KindFunction)
	if symbol == nil || symbol.Function == nil {
		builder.unmatched(nil, typeFact, "no matching normalized IR function")
		builder.unmatched(nil, callingFact, "no matching normalized IR function")
		return
	}
	symbolResult := builder.symbol(symbol)
	if callingConventionEquivalent(symbol.Function.CallingConvention, function.CallingConvention) {
		builder.matched(symbolResult, callingFact)
	} else {
		builder.mismatch(symbolResult, "function-calling-convention-mismatch", callingFact, "/functions/"+function.ID+"/callingConvention", function.CallingConvention, symbol.Function.CallingConvention)
	}
	if probe.ID == "" || probe.ExpectedType == "" {
		builder.unmatched(symbolResult, typeFact, "reviewed native function type is unavailable")
		return
	}
	expected, parseOK := parseNativeFunctionShape(probe.ExpectedType)
	actual, projectionOK := projectedFunctionShape(*symbol.Function)
	if !parseOK || !projectionOK {
		builder.unmatched(symbolResult, typeFact, "native and projected function shapes are not safely comparable")
		return
	}
	if expected == actual && function.TypeCompatible {
		builder.matched(symbolResult, typeFact)
		return
	}
	builder.mismatch(symbolResult, "function-type-mismatch", typeFact, "/functions/"+function.ID+"/typeCompatible", expected, actual)
}

func compareVTable(builder *comparisonBuilder, inventory model.Inventory, architecture string, table VTableResult) {
	factID := table.ID + ".index"
	symbol := findSymbol(inventory, architecture, table.Interface, model.KindCOMInterface, model.KindWinRTInterface)
	if symbol == nil || symbol.Type == nil {
		builder.unmatched(nil, factID, "no matching normalized IR interface")
		return
	}
	symbolResult := builder.symbol(symbol)
	for _, method := range symbol.Type.Methods {
		if method.Name == table.Method {
			builder.compareUint(symbolResult, factID, "/vtables/"+table.ID+"/index", table.Index, uint64(method.VTableIndex))
			return
		}
	}
	builder.unmatched(symbolResult, factID, "normalized IR interface has no matching method")
}

func (builder *comparisonBuilder) symbol(symbol *model.Symbol) *SymbolResult {
	if result := builder.symbols[symbol.ID]; result != nil {
		return result
	}
	result := &SymbolResult{
		Architecture: builder.architecture, SymbolID: symbol.ID,
		NativeName: symbol.NativeName, Kind: string(symbol.Kind), Complete: true,
		MatchedFactIDs: []string{}, UnmatchedFactIDs: []string{}, MismatchedFactIDs: []string{},
	}
	builder.symbols[symbol.ID] = result
	return result
}

func (builder *comparisonBuilder) matched(symbol *SymbolResult, factID string) {
	symbol.MatchedFactIDs = append(symbol.MatchedFactIDs, factID)
}

func (builder *comparisonBuilder) unmatched(symbol *SymbolResult, factID, reason string) {
	builder.result.Unmatched = append(builder.result.Unmatched, UnmatchedFact{Architecture: builder.architecture, FactID: factID, Reason: reason})
	if symbol != nil {
		symbol.Complete = false
		symbol.UnmatchedFactIDs = append(symbol.UnmatchedFactIDs, factID)
	}
}

func (builder *comparisonBuilder) unmatchedMany(symbol *SymbolResult, facts []string, reason string) {
	for _, fact := range facts {
		builder.unmatched(symbol, fact, reason)
	}
}

func (builder *comparisonBuilder) mismatch(symbol *SymbolResult, code, factID, path, expected, actual string) {
	mismatch := Mismatch{Code: code, Architecture: builder.architecture, FactID: factID, Path: path, Expected: expected, Actual: actual}
	if symbol != nil {
		mismatch.SymbolID = symbol.SymbolID
		symbol.Complete = false
		if strings.HasSuffix(factID, ".kind") {
			// Kind is an identity gate rather than one of the counted ABI facts.
		} else {
			symbol.MismatchedFactIDs = append(symbol.MismatchedFactIDs, factID)
			builder.result.MismatchedFacts++
		}
	}
	builder.result.Mismatches = append(builder.result.Mismatches, mismatch)
}

func (builder *comparisonBuilder) compareUint(symbol *SymbolResult, factID, path string, expected, actual uint64) {
	if expected == actual {
		builder.matched(symbol, factID)
		return
	}
	builder.mismatch(symbol, "abi-value-mismatch", factID, path, strconv.FormatUint(expected, 10), strconv.FormatUint(actual, 10))
}

func findSymbol(inventory model.Inventory, architecture, nativeName string, kinds ...model.SymbolKind) *model.Symbol {
	kindSet := make(map[model.SymbolKind]bool, len(kinds))
	for _, kind := range kinds {
		kindSet[kind] = true
	}
	best := -1
	bestScore := -1
	for index := range inventory.Symbols {
		symbol := &inventory.Symbols[index]
		if symbol.NativeName != nativeName || !kindSet[symbol.Kind] {
			continue
		}
		score := architectureScore(symbol.Architecture, architecture)
		if score < 0 {
			continue
		}
		if score > bestScore || (score == bestScore && (best < 0 || symbol.ID < inventory.Symbols[best].ID)) {
			best, bestScore = index, score
		}
	}
	if best < 0 {
		return nil
	}
	return &inventory.Symbols[best]
}

func findGUIDSymbol(inventory model.Inventory, architecture, nativeName string) *model.Symbol {
	best := -1
	bestScore := -1
	for index := range inventory.Symbols {
		symbol := &inventory.Symbols[index]
		if symbol.NativeName != nativeName || symbol.GUID == nil {
			continue
		}
		score := architectureScore(symbol.Architecture, architecture)
		if score > bestScore || (score == bestScore && score >= 0 && (best < 0 || symbol.ID < inventory.Symbols[best].ID)) {
			best, bestScore = index, score
		}
	}
	if best < 0 {
		return nil
	}
	return &inventory.Symbols[best]
}

func architectureScore(symbol, target string) int {
	if symbol == target {
		return 2
	}
	if symbol == "neutral" || symbol == "all" {
		return 1
	}
	return -1
}

func findLayout(layouts []model.Layout, architecture string) (model.Layout, bool) {
	for _, layout := range layouts {
		if layout.Architecture == architecture {
			return layout, true
		}
	}
	for _, layout := range layouts {
		if layout.Architecture == "neutral" || layout.Architecture == "all" {
			return layout, true
		}
	}
	return model.Layout{}, false
}

func findModelField(fields []model.Field, nativeName string) (model.Field, bool) {
	for _, field := range fields {
		name := field.NativeName
		if name == "" {
			name = field.Name
		}
		if name == nativeName || field.Name == nativeName {
			return field, true
		}
	}
	return model.Field{}, false
}

func recordOffsets(record model.Type, layout model.Layout, architecture string) (map[string]uint64, bool) {
	offsets := map[string]uint64{}
	for _, field := range layout.Fields {
		offsets[field.Name] = field.Offset
		if modelField, ok := findModelField(record.Fields, field.Name); ok {
			offsets[modelField.Name] = field.Offset
		}
	}
	if len(layout.Fields) != 0 {
		return offsets, true
	}
	if record.Kind == model.KindUnion {
		for _, field := range record.Fields {
			offsets[field.Name] = 0
		}
		return offsets, true
	}
	if record.Kind != model.KindStruct {
		return nil, false
	}
	var offset uint64
	for _, field := range record.Fields {
		if field.BitField != nil {
			return nil, false
		}
		size, alignment, ok := typeLayout(field.Type, architecture)
		if !ok || alignment == 0 {
			return nil, false
		}
		if layout.Pack != 0 && alignment > layout.Pack {
			alignment = layout.Pack
		}
		offset = alignUp(offset, alignment)
		offsets[field.Name] = offset
		if ^uint64(0)-offset < size {
			return nil, false
		}
		offset += size
	}
	return offsets, true
}

func typeLayout(value model.Type, architecture string) (uint64, uint64, bool) {
	if value.PointerDepth > 0 || value.Kind == model.KindPointer || value.Kind == model.KindFunctionPtr || value.Kind == model.KindHandle || value.Kind == model.KindOpaque {
		return pointerLayout(architecture)
	}
	if value.Kind == model.KindFixedArray && value.Element != nil && value.Length >= 0 {
		size, alignment, ok := typeLayout(*value.Element, architecture)
		if !ok || uint64(value.Length) > ^uint64(0)/size {
			return 0, 0, false
		}
		return size * uint64(value.Length), alignment, true
	}
	if layout, ok := findLayout(value.Layouts, architecture); ok {
		return layout.Size, layout.Alignment, true
	}
	switch value.GoType {
	case "int8", "uint8", "byte":
		return 1, 1, true
	case "int16", "uint16":
		return 2, 2, true
	case "int32", "uint32", "float32":
		return 4, 4, true
	case "int64", "uint64", "float64":
		return 8, 8, true
	case "uintptr", "unsafe.Pointer":
		return pointerLayout(architecture)
	}
	if value.Bits > 0 && value.Bits%8 == 0 {
		size := uint64(value.Bits / 8)
		if size == 1 || size == 2 || size == 4 || size == 8 {
			return size, size, true
		}
	}
	return 0, 0, false
}

func pointerLayout(architecture string) (uint64, uint64, bool) {
	switch architecture {
	case "386":
		return 4, 4, true
	case "amd64", "arm64", "arm64ec":
		return 8, 8, true
	default:
		return 0, 0, false
	}
}

func alignUp(value, alignment uint64) uint64 {
	if alignment <= 1 {
		return value
	}
	remainder := value % alignment
	if remainder == 0 {
		return value
	}
	return value + alignment - remainder
}

func formatGUID(guid model.GUID) string {
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		guid.Data1, guid.Data2, guid.Data3,
		guid.Data4[0], guid.Data4[1], guid.Data4[2], guid.Data4[3],
		guid.Data4[4], guid.Data4[5], guid.Data4[6], guid.Data4[7])
}

func bitMaskHex(offset, width uint64, byteCount int) (string, bool) {
	if byteCount <= 0 || offset > uint64(byteCount)*8 || width > uint64(byteCount)*8-offset {
		return "", false
	}
	mask := make([]byte, byteCount)
	for bit := offset; bit < offset+width; bit++ {
		mask[bit/8] |= byte(1 << (bit % 8))
	}
	return hex.EncodeToString(mask), true
}

var nativeFunctionPattern = regexp.MustCompile(`^\s*(.+?)\s+\((?:([A-Za-z_][A-Za-z0-9_]*)\s+)?\*\)\s*\((.*)\)\s*$`)

type abiFunctionShape struct {
	Return     string
	Parameters []string
}

func (shape abiFunctionShape) String() string {
	return shape.Return + "(" + strings.Join(shape.Parameters, ",") + ")"
}

func parseNativeFunctionShape(declaration string) (string, bool) {
	match := nativeFunctionPattern.FindStringSubmatch(declaration)
	if match == nil {
		return "", false
	}
	returnType, ok := normalizeNativeType(match[1])
	if !ok {
		return "", false
	}
	shape := abiFunctionShape{Return: returnType, Parameters: []string{}}
	parameters := strings.TrimSpace(match[3])
	if parameters != "" && parameters != "void" {
		for _, parameter := range splitParameters(parameters) {
			normalized, parameterOK := normalizeNativeType(parameter)
			if !parameterOK {
				return "", false
			}
			shape.Parameters = append(shape.Parameters, normalized)
		}
	}
	return shape.String(), true
}

func projectedFunctionShape(function model.Function) (string, bool) {
	returnType, ok := normalizeProjectedType(function.Return)
	if !ok {
		return "", false
	}
	shape := abiFunctionShape{Return: returnType, Parameters: make([]string, 0, len(function.Parameters))}
	for _, parameter := range function.Parameters {
		normalized, parameterOK := normalizeProjectedType(parameter.Type)
		if !parameterOK {
			return "", false
		}
		shape.Parameters = append(shape.Parameters, normalized)
	}
	return shape.String(), true
}

func splitParameters(value string) []string {
	var result []string
	start, depth := 0, 0
	for index, r := range value {
		switch r {
		case '(', '[', '<':
			depth++
		case ')', ']', '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(value[start:index]))
				start = index + 1
			}
		}
	}
	result = append(result, strings.TrimSpace(value[start:]))
	return result
}

func normalizeNativeType(value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, qualifier := range []string{"const ", "volatile ", "struct ", "union ", "_In_ ", "_Out_ "} {
		value = strings.ReplaceAll(value, qualifier, "")
	}
	pointers := 0
	for strings.HasSuffix(strings.TrimSpace(value), "*") {
		pointers++
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "*"))
	}
	base, ok := map[string]string{
		"void": "void", "CHAR": "int8", "BYTE": "uint8", "SHORT": "int16",
		"USHORT": "uint16", "INT": "int32", "UINT": "uint32", "LONG": "int32",
		"ULONG": "uint32", "DWORD": "uint32", "BOOL": "int32", "HRESULT": "int32",
		"LONGLONG": "int64", "ULONGLONG": "uint64", "LARGE_INTEGER": "named:LARGE_INTEGER",
	}[value]
	if !ok {
		if regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(value) {
			base, ok = "named:"+value, true
		} else {
			return "", false
		}
	}
	return base + strings.Repeat("*", pointers), true
}

func normalizeProjectedType(value model.Type) (string, bool) {
	pointers := value.PointerDepth
	if value.Kind == model.KindPointer {
		pointers++
		if value.Element != nil {
			base, ok := normalizeProjectedType(*value.Element)
			if !ok {
				return "", false
			}
			return base + strings.Repeat("*", pointers), true
		}
	}
	base := value.GoType
	if base == "" && value.NativeName == "void" {
		base = "void"
	}
	switch base {
	case "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "float32", "float64", "uintptr", "void":
		return base + strings.Repeat("*", pointers), true
	}
	if value.NativeName != "" {
		return "named:" + value.NativeName + strings.Repeat("*", pointers), true
	}
	if base != "" && regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(base) {
		return "named:" + base + strings.Repeat("*", pointers), true
	}
	return "", false
}

func callingConventionEquivalent(projected, native string) bool {
	projected = strings.ToLower(strings.TrimSpace(projected))
	native = strings.ToLower(strings.TrimSpace(native))
	if projected == native {
		return true
	}
	if native == "winapi" {
		return projected == "platform" || projected == "stdcall" || projected == "winapi"
	}
	return false
}
