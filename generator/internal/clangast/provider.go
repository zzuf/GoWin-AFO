// Package clangast ingests Clang's machine-readable AST JSON. It never parses
// C/C++ declarations with regular expressions.
package clangast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zzuf/GoWin-AFO/generator/internal/metadata"
	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

type Provider struct {
	Clang        string
	IncludeDirs  []string
	Defines      []string
	Architecture string
	Profile      string
}

func (Provider) Type() string { return "clang-header" }

type Node struct {
	ID                 string `json:"id,omitempty"`
	Kind               string `json:"kind"`
	Name               string `json:"name,omitempty"`
	TagUsed            string `json:"tagUsed,omitempty"`
	CompleteDefinition bool   `json:"completeDefinition,omitempty"`
	IsImplicit         bool   `json:"isImplicit,omitempty"`
	IsBitfield         bool   `json:"isBitfield,omitempty"`
	Type               struct {
		QualType          string `json:"qualType"`
		DesugaredQualType string `json:"desugaredQualType,omitempty"`
	} `json:"type,omitempty"`
	Loc struct {
		File string `json:"file,omitempty"`
		Line int    `json:"line,omitempty"`
		Col  int    `json:"col,omitempty"`
	} `json:"loc,omitempty"`
	Range struct {
		Begin struct {
			File string `json:"file,omitempty"`
			Line int    `json:"line,omitempty"`
			Col  int    `json:"col,omitempty"`
		} `json:"begin"`
	} `json:"range,omitempty"`
	Value string `json:"value,omitempty"`
	Inner []Node `json:"inner,omitempty"`
}

func (p Provider) Ingest(ctx context.Context, req metadata.Request) (metadata.Result, error) {
	arch := p.Architecture
	if arch == "" {
		arch = "amd64"
	}
	profile := p.Profile
	if profile == "" {
		profile = req.ABIProfile
	}
	ast, stderr, err := p.run(ctx, req.Path, arch, profile)
	if err != nil {
		return metadata.Result{}, fmt.Errorf("clang AST: %w: %s", err, strings.TrimSpace(stderr))
	}
	var root Node
	if err = json.Unmarshal(ast, &root); err != nil {
		return metadata.Result{}, fmt.Errorf("decode clang AST JSON: %w", err)
	}
	sanitizeRawNode(&root)
	symbols := Inventory(root, req.Locked.ID, req.Locked.Type, arch, profile, filepath.Base(req.Path))
	macroDump, macroStderr, macroErr := p.runMacros(ctx, req.Path, arch, profile)
	diagnostics := []model.Diagnostic{}
	if macroErr != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			Severity: "warning",
			SourceID: req.Locked.ID,
			Code:     "clang-macro-dump",
			Message:  sanitizeClangText(fmt.Sprintf("macro inventory unavailable: %v: %s", macroErr, strings.TrimSpace(macroStderr)), req.Path, p.IncludeDirs),
		})
	} else {
		symbols = append(symbols, InventoryMacros(macroDump, req.Locked.ID, req.Locked.Type, arch, profile, filepath.Base(req.Path))...)
	}
	return metadata.Result{
		Source:  model.Source{ID: req.Locked.ID, Type: req.Locked.Type, Package: req.Locked.Package, Version: req.Locked.Version, SHA256: req.Locked.SHA256, LicenseIdentifier: req.Locked.LicenseIdentifier, Architectures: []string{arch}, WindowsSDKVersion: req.Locked.WindowsSDKVersion, Files: req.Locked.Files},
		Symbols: symbols, Diagnostics: diagnostics, Raw: map[string]any{"ast": root, "macros": string(macroDump)},
	}, nil
}

// sanitizeRawNode removes Clang process addresses and machine-local paths from
// debug dumps. Neither is semantic input, and both would make a repeated dump
// differ across processes or checkout locations.
func sanitizeRawNode(node *Node) {
	node.ID = ""
	if node.Loc.File != "" {
		node.Loc.File = filepath.Base(node.Loc.File)
	}
	if node.Range.Begin.File != "" {
		node.Range.Begin.File = filepath.Base(node.Range.Begin.File)
	}
	for i := range node.Inner {
		sanitizeRawNode(&node.Inner[i])
	}
}

func sanitizeClangText(text, header string, includeDirs []string) string {
	paths := append([]string{header}, includeDirs...)
	for _, path := range paths {
		if path == "" {
			continue
		}
		text = strings.ReplaceAll(text, path, filepath.Base(path))
		text = strings.ReplaceAll(text, filepath.ToSlash(path), filepath.Base(path))
	}
	return text
}

func (p Provider) run(ctx context.Context, header, arch, profile string) ([]byte, string, error) {
	clang := p.Clang
	if clang == "" {
		clang = "clang"
	}
	args, err := p.compilerArgs(arch, profile)
	if err != nil {
		return nil, "", err
	}
	args = append([]string{"-x", "c++"}, args...)
	args = append(args, "-fsyntax-only", "-Xclang", "-ast-dump=json", header)
	cmd := exec.CommandContext(ctx, clang, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	return out.Bytes(), stderr.String(), err
}

// runMacros asks Clang's preprocessor for a machine-produced macro table. The
// header dump is compared with an otherwise identical empty translation unit,
// so compiler built-ins and command-line profile defines are not attributed to
// the SDK header.
func (p Provider) runMacros(ctx context.Context, header, arch, profile string) ([]byte, string, error) {
	clang := p.Clang
	if clang == "" {
		clang = "clang"
	}
	common, err := p.compilerArgs(arch, profile)
	if err != nil {
		return nil, "", err
	}
	run := func(input string, stdin bool) ([]byte, string, error) {
		args := append([]string{"-x", "c++"}, common...)
		args = append(args, "-E", "-dM", input)
		cmd := exec.CommandContext(ctx, clang, args...)
		if stdin {
			cmd.Stdin = strings.NewReader("\n")
		}
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		return out.Bytes(), stderr.String(), err
	}
	baseline, baseErrOut, err := run("-", true)
	if err != nil {
		return nil, baseErrOut, err
	}
	headerDump, headerErrOut, err := run(header, false)
	if err != nil {
		return nil, headerErrOut, err
	}
	base := macroNames(baseline)
	var filtered strings.Builder
	for _, line := range strings.Split(string(headerDump), "\n") {
		name, _, ok := splitMacroLine(line)
		if ok && !base[name] {
			filtered.WriteString(line)
			filtered.WriteByte('\n')
		}
	}
	return []byte(filtered.String()), headerErrOut, nil
}

func (p Provider) compilerArgs(arch, profile string) ([]string, error) {
	target := map[string]string{"386": "i686-pc-windows-msvc", "amd64": "x86_64-pc-windows-msvc", "arm64": "aarch64-pc-windows-msvc"}[arch]
	if target == "" {
		return nil, fmt.Errorf("unsupported architecture %q", arch)
	}
	args := []string{"--target=" + target, "-DWIN32_LEAN_AND_MEAN=1"}
	switch profile {
	case "windows-appcontainer":
		args = append(args, "-DWINAPI_FAMILY=WINAPI_FAMILY_APP")
	case "windows-wdk":
		args = append(args, "-D_KERNEL_MODE=1")
	default:
		args = append(args, "-DWINAPI_FAMILY=WINAPI_FAMILY_DESKTOP_APP")
	}
	for _, d := range p.Defines {
		args = append(args, "-D"+d)
	}
	for _, d := range p.IncludeDirs {
		args = append(args, "-I", d)
	}
	return args, nil
}

// InventoryMacros parses only Clang's -dM output format; it does not parse C
// source text. Function-like macros remain explicitly unsupported until their
// evaluation count, promotions, overflow and pointer semantics are proven.
func InventoryMacros(dump []byte, sourceID, sourceType, arch, profile, inputFile string) []model.Symbol {
	var out []model.Symbol
	for _, line := range strings.Split(string(dump), "\n") {
		name, value, ok := splitMacroLine(line)
		if !ok {
			continue
		}
		functionLike := strings.Contains(name, "(")
		nativeName := name
		kind := model.KindConstant
		backend := model.BackendTypeOnly
		backendReason := "object-like macro is inventoried as a source constant pending typed evaluation"
		statusReason := "clang-preprocessor-object-macro-awaiting-typed-constant-evaluation"
		if functionLike {
			kind = model.KindFunction
			backend = model.BackendUnsupported
			backendReason = "macro semantics cannot be safely inferred from token text"
			statusReason = "function-like-macro-requires-semantic-safety-proof-or-generated-bridge"
			if i := strings.IndexByte(nativeName, '('); i >= 0 {
				nativeName = nativeName[:i]
			}
		}
		s := model.Symbol{
			SourceID: sourceID, SourceType: sourceType, Kind: kind, NativeName: nativeName,
			GoName: safeName(nativeName), Architecture: arch, ABIProfile: profile,
			CanonicalSignature: "#define " + name + " " + value,
			Status:             model.StatusUnsupportedProjection, StatusReason: statusReason,
			Backend: backend, BackendReason: backendReason,
			Provenance: []model.Provenance{{SourceID: sourceID, InputFile: inputFile, Header: inputFile}},
			Attributes: map[string]string{"macroTokens": value},
		}
		if !functionLike {
			s.Constant = &model.Constant{Type: model.Type{Kind: model.KindOpaque, NativeName: "preprocessor-token-sequence", Opaque: true}, Value: value}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NativeName != out[j].NativeName {
			return out[i].NativeName < out[j].NativeName
		}
		return out[i].CanonicalSignature < out[j].CanonicalSignature
	})
	return out
}

func macroNames(dump []byte) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(string(dump), "\n") {
		if name, _, ok := splitMacroLine(line); ok {
			out[name] = true
		}
	}
	return out
}

func splitMacroLine(line string) (name, value string, ok bool) {
	const prefix = "#define "
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	if rest == "" {
		return "", "", false
	}
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		return rest[:i], strings.TrimSpace(rest[i:]), true
	}
	return rest, "", true
}

func Inventory(root Node, sourceID, sourceType, arch, profile, inputFile string) []model.Symbol {
	var out []model.Symbol
	var visit func(Node, string)
	visit = func(n Node, ns string) {
		if n.IsImplicit {
			return
		}
		namespace := ns
		if n.Kind == "NamespaceDecl" && n.Name != "" {
			namespace = join(ns, n.Name)
		}
		kind, ok := nodeKind(n)
		if ok && n.Name != "" {
			sig := n.Kind + " " + n.Name
			if n.Type.QualType != "" {
				sig += " " + n.Type.QualType
			}
			s := model.Symbol{SourceID: sourceID, SourceType: sourceType, Namespace: namespace, Kind: kind, NativeName: n.Name, GoName: safeName(n.Name), Architecture: arch, ABIProfile: profile, CanonicalSignature: sig, Status: model.StatusUnsupportedProjection, StatusReason: "clang-declaration-inventoried-awaiting-safe-semantic-projection", Backend: model.BackendUnsupported, BackendReason: "header AST declaration is not emitted until ABI semantics are proven", Provenance: []model.Provenance{{SourceID: sourceID, InputFile: inputFile, Header: location(n)}}}
			if kind != model.KindFunction {
				s.Backend = model.BackendTypeOnly
				s.BackendReason = "header declaration is a type or constant"
			}
			if kind == model.KindStruct || kind == model.KindUnion {
				s.Type = &model.Type{Kind: kind, NativeName: n.Name, GoType: safeName(n.Name), Distinct: true}
				for _, child := range n.Inner {
					if child.Kind == "FieldDecl" {
						f := model.Field{Name: safeName(child.Name), NativeName: child.Name, Type: model.Type{Kind: model.KindOpaque, NativeName: child.Type.QualType, GoType: "uintptr", Opaque: true}}
						if child.IsBitfield {
							f.BitField = &model.BitField{StorageType: child.Type.QualType}
							for _, v := range child.Inner {
								if v.Value != "" {
									var width uint8
									_, _ = fmt.Sscan(v.Value, &width)
									f.BitField.BitWidth = width
									break
								}
							}
						}
						s.Type.Fields = append(s.Type.Fields, f)
					}
				}
			}
			out = append(out, s)
		}
		for _, c := range n.Inner {
			visit(c, namespace)
		}
	}
	visit(root, "")
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].NativeName < out[j].NativeName
	})
	return out
}

func nodeKind(n Node) (model.SymbolKind, bool) {
	switch n.Kind {
	case "RecordDecl", "CXXRecordDecl":
		if n.TagUsed == "union" {
			return model.KindUnion, true
		}
		if n.CompleteDefinition {
			return model.KindStruct, true
		}
	case "EnumDecl":
		return model.KindEnum, true
	case "TypedefDecl", "TypeAliasDecl":
		return model.KindTypedef, true
	case "FunctionDecl":
		return model.KindFunction, true
	case "VarDecl", "EnumConstantDecl":
		return model.KindConstant, true
	}
	return "", false
}
func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}
func safeName(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "Unnamed"
	}
	return b.String()
}
func location(n Node) string {
	file, line, col := n.Loc.File, n.Loc.Line, n.Loc.Col
	if file == "" {
		file = n.Range.Begin.File
		line = n.Range.Begin.Line
		col = n.Range.Begin.Col
	}
	if file == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", filepath.Base(file), line, col)
}
