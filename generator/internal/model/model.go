// Package model defines the language-neutral normalized binding IR.
package model

import "fmt"

const SchemaVersion = 1

type Status string

const (
	StatusUnclassified            Status = "unclassified"
	StatusGeneratedPureGo         Status = "generated-purego"
	StatusGeneratedAssembly       Status = "generated-assembly"
	StatusGeneratedCGOBridge      Status = "generated-cgo-bridge"
	StatusGeneratedTypeOnly       Status = "generated-type-only"
	StatusGeneratedManualOverride Status = "generated-manual-override"
	StatusUnsupportedGoABI        Status = "unsupported-go-abi"
	StatusUnsupportedProjection   Status = "unsupported-projection"
	StatusKernelModeOnly          Status = "kernel-mode-only"
	StatusMissingUpstreamMetadata Status = "missing-upstream-metadata"
	StatusExternalSDKNotInstalled Status = "external-sdk-not-installed"
	StatusUndocumentedOutOfScope  Status = "undocumented-out-of-scope"
	StatusLicenseRestricted       Status = "license-restricted"
	StatusSourceParseError        Status = "source-parse-error"
)

func (s Status) Valid() bool {
	switch s {
	case StatusGeneratedPureGo, StatusGeneratedAssembly, StatusGeneratedCGOBridge,
		StatusGeneratedTypeOnly, StatusGeneratedManualOverride, StatusUnsupportedGoABI,
		StatusUnsupportedProjection, StatusKernelModeOnly, StatusMissingUpstreamMetadata,
		StatusExternalSDKNotInstalled, StatusUndocumentedOutOfScope,
		StatusLicenseRestricted, StatusSourceParseError:
		return true
	default:
		return false
	}
}

type Backend string

const (
	BackendPureGoSyscall      Backend = "purego-syscall"
	BackendPureGoSyscallFloat Backend = "purego-syscall-float"
	BackendAssemblyTrampoline Backend = "assembly-trampoline"
	BackendCGOBridge          Backend = "cgo-bridge"
	BackendTypeOnly           Backend = "type-only"
	BackendUnsupported        Backend = "unsupported"
)

type SymbolKind string

const (
	KindPrimitive      SymbolKind = "primitive"
	KindPointer        SymbolKind = "pointer"
	KindFixedArray     SymbolKind = "fixed-array"
	KindFlexibleArray  SymbolKind = "flexible-array"
	KindFunctionPtr    SymbolKind = "function-pointer"
	KindStruct         SymbolKind = "struct"
	KindUnion          SymbolKind = "union"
	KindEnum           SymbolKind = "enum"
	KindFlags          SymbolKind = "flags-enum"
	KindTypedef        SymbolKind = "typedef"
	KindNativeTypedef  SymbolKind = "native-typedef"
	KindOpaque         SymbolKind = "opaque"
	KindHandle         SymbolKind = "handle"
	KindGUID           SymbolKind = "guid"
	KindFunction       SymbolKind = "function"
	KindCallback       SymbolKind = "callback"
	KindCOMInterface   SymbolKind = "com-interface"
	KindWinRTInterface SymbolKind = "winrt-interface"
	KindRuntimeClass   SymbolKind = "runtime-class"
	KindDelegate       SymbolKind = "delegate"
	KindGenericType    SymbolKind = "generic-type"
	KindConstant       SymbolKind = "constant"
	KindProperty       SymbolKind = "property"
	KindEvent          SymbolKind = "event"
	KindCoClass        SymbolKind = "coclass"
	KindDispInterface  SymbolKind = "dispinterface"
	KindAlias          SymbolKind = "alias"
)

type Direction string

const (
	DirectionIn    Direction = "in"
	DirectionOut   Direction = "out"
	DirectionInOut Direction = "in-out"
)

type Inventory struct {
	SchemaVersion    int          `json:"schemaVersion"`
	GeneratorVersion string       `json:"generatorVersion"`
	ManifestHash     string       `json:"manifestHash"`
	Sources          []Source     `json:"sources"`
	Symbols          []Symbol     `json:"symbols"`
	Diagnostics      []Diagnostic `json:"diagnostics,omitempty"`
}

type Source struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Package           string   `json:"package"`
	Version           string   `json:"version"`
	SHA256            string   `json:"sha256"`
	LicenseIdentifier string   `json:"licenseIdentifier"`
	Architectures     []string `json:"architectures"`
	WindowsSDKVersion string   `json:"windowsSDKVersion,omitempty"`
	Files             []string `json:"files"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	SourceID string `json:"sourceId,omitempty"`
	SymbolID string `json:"symbolId,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type Symbol struct {
	ID                 string            `json:"id"`
	SourceID           string            `json:"sourceId"`
	SourceType         string            `json:"sourceType"`
	Namespace          string            `json:"namespace"`
	Kind               SymbolKind        `json:"kind"`
	NativeName         string            `json:"nativeName"`
	GoName             string            `json:"goName,omitempty"`
	Architecture       string            `json:"architecture"`
	ABIProfile         string            `json:"abiProfile"`
	CanonicalSignature string            `json:"canonicalSignature"`
	GenericArity       int               `json:"genericArity"`
	Status             Status            `json:"status"`
	StatusReason       string            `json:"statusReason"`
	Backend            Backend           `json:"backend"`
	BackendReason      string            `json:"backendReason"`
	Type               *Type             `json:"type,omitempty"`
	Function           *Function         `json:"function,omitempty"`
	Constant           *Constant         `json:"constant,omitempty"`
	GUID               *GUID             `json:"guid,omitempty"`
	Availability       Availability      `json:"availability,omitempty"`
	Ownership          *Ownership        `json:"ownership,omitempty"`
	SourceLocation     *SourceLocation   `json:"sourceLocation,omitempty"`
	Provenance         []Provenance      `json:"provenance"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	ManualOverride     bool              `json:"manualOverride"`
	ABIVerified        bool              `json:"abiVerified"`
	RuntimeSmokeTested bool              `json:"runtimeSmokeTested"`
}

type Type struct {
	Kind            SymbolKind  `json:"kind"`
	NativeName      string      `json:"nativeName,omitempty"`
	GoType          string      `json:"goType,omitempty"`
	Underlying      string      `json:"underlying,omitempty"`
	PointerDepth    int         `json:"pointerDepth,omitempty"`
	Optional        bool        `json:"optional,omitempty"`
	Signed          bool        `json:"signed,omitempty"`
	Bits            int         `json:"bits,omitempty"`
	Length          int         `json:"length,omitempty"`
	Element         *Type       `json:"element,omitempty"`
	Fields          []Field     `json:"fields,omitempty"`
	Layouts         []Layout    `json:"layouts,omitempty"`
	EnumValues      []EnumValue `json:"enumValues,omitempty"`
	Flags           bool        `json:"flags,omitempty"`
	Distinct        bool        `json:"distinct,omitempty"`
	Opaque          bool        `json:"opaque,omitempty"`
	CloseFunction   string      `json:"closeFunction,omitempty"`
	InvalidValue    string      `json:"invalidValue,omitempty"`
	FlexibleElement *Type       `json:"flexibleElement,omitempty"`
	Interfaces      []string    `json:"interfaces,omitempty"`
	Methods         []Method    `json:"methods,omitempty"`
}

type Layout struct {
	Architecture string        `json:"architecture"`
	Size         uint64        `json:"size"`
	Alignment    uint64        `json:"alignment"`
	Pack         uint64        `json:"pack,omitempty"`
	TailPadding  uint64        `json:"tailPadding,omitempty"`
	Fields       []FieldLayout `json:"fields,omitempty"`
}

type Field struct {
	Name       string    `json:"name"`
	NativeName string    `json:"nativeName,omitempty"`
	Type       Type      `json:"type"`
	Anonymous  bool      `json:"anonymous,omitempty"`
	BitField   *BitField `json:"bitField,omitempty"`
}

type FieldLayout struct {
	Name      string `json:"name"`
	Offset    uint64 `json:"offset"`
	BitOffset uint8  `json:"bitOffset,omitempty"`
	BitWidth  uint8  `json:"bitWidth,omitempty"`
}

type BitField struct {
	StorageType string `json:"storageType"`
	BitOffset   uint8  `json:"bitOffset"`
	BitWidth    uint8  `json:"bitWidth"`
	Signed      bool   `json:"signed"`
}

type EnumValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Function struct {
	DLL               string      `json:"dll,omitempty"`
	EntryPoint        string      `json:"entryPoint,omitempty"`
	Ordinal           uint16      `json:"ordinal,omitempty"`
	ForwardedExport   string      `json:"forwardedExport,omitempty"`
	CallingConvention string      `json:"callingConvention"`
	Parameters        []Parameter `json:"parameters"`
	Return            Type        `json:"return"`
	SetLastError      bool        `json:"setLastError"`
	OptionalExport    bool        `json:"optionalExport"`
	VarArgs           bool        `json:"varArgs"`
	FailureRule       string      `json:"failureRule,omitempty"`
	VTableIndex       *int        `json:"vtableIndex,omitempty"`
	BridgeName        string      `json:"bridgeName,omitempty"`
}

type Method struct {
	Name        string      `json:"name"`
	VTableIndex int         `json:"vtableIndex"`
	Parameters  []Parameter `json:"parameters"`
	Return      Type        `json:"return"`
}

type Parameter struct {
	Name         string    `json:"name"`
	Type         Type      `json:"type"`
	Direction    Direction `json:"direction"`
	Nullability  string    `json:"nullability,omitempty"`
	ByteCount    string    `json:"byteCount,omitempty"`
	ElementCount string    `json:"elementCount,omitempty"`
	RetVal       bool      `json:"retVal,omitempty"`
	FreeWith     string    `json:"freeWith,omitempty"`
}

type Constant struct {
	Type  Type   `json:"type"`
	Value string `json:"value"`
	Alias string `json:"alias,omitempty"`
}

type GUID struct {
	Data1 uint32   `json:"data1"`
	Data2 uint16   `json:"data2"`
	Data3 uint16   `json:"data3"`
	Data4 [8]uint8 `json:"data4"`
}

type Availability struct {
	MinimumWindowsVersion string   `json:"minimumWindowsVersion,omitempty"`
	Contract              string   `json:"contract,omitempty"`
	ContractVersion       uint32   `json:"contractVersion,omitempty"`
	Architectures         []string `json:"architectures,omitempty"`
	Profiles              []string `json:"profiles,omitempty"`
	Deprecated            bool     `json:"deprecated,omitempty"`
	DeprecationMessage    string   `json:"deprecationMessage,omitempty"`
	ThreadingModel        string   `json:"threadingModel,omitempty"`
	MarshalingBehavior    string   `json:"marshalingBehavior,omitempty"`
}

type Ownership struct {
	Kind        string `json:"kind"`
	Allocator   string `json:"allocator,omitempty"`
	Deallocator string `json:"deallocator,omitempty"`
	Retained    bool   `json:"retained,omitempty"`
}

type SourceLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type Provenance struct {
	SourceID      string `json:"sourceId"`
	InputFile     string `json:"inputFile"`
	MetadataTable string `json:"metadataTable,omitempty"`
	MetadataRow   uint32 `json:"metadataRow,omitempty"`
	Header        string `json:"header,omitempty"`
}

func (s Symbol) QualifiedName() string {
	if s.Namespace == "" {
		return s.NativeName
	}
	return s.Namespace + "." + s.NativeName
}

func (s Symbol) Validate() error {
	if s.SourceID == "" || s.Kind == "" || s.NativeName == "" || s.Architecture == "" || s.ABIProfile == "" {
		return fmt.Errorf("symbol %q lacks stable-ID input", s.QualifiedName())
	}
	if !s.Status.Valid() {
		return fmt.Errorf("symbol %q has invalid status %q", s.QualifiedName(), s.Status)
	}
	if s.StatusReason == "" {
		return fmt.Errorf("symbol %q has no status reason", s.QualifiedName())
	}
	if s.Backend == "" || s.BackendReason == "" {
		return fmt.Errorf("symbol %q has incomplete backend decision", s.QualifiedName())
	}
	return nil
}
