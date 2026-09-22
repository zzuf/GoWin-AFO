package generator

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	internalcoverage "go-windows-api.local/generator/internal/coverage"
	"go-windows-api.local/generator/internal/emit"
	"go-windows-api.local/generator/internal/metadata"
	"go-windows-api.local/generator/internal/model"
	"go-windows-api.local/generator/internal/normalize"
	"go-windows-api.local/generator/internal/slice"
)

const Version = "0.1.0"

type GenerateOptions struct {
	Root    string
	All     bool
	DumpRaw bool
	Write   bool
}

type GenerateResult struct {
	Inventory model.Inventory
	Coverage  internalcoverage.Report
	Files     int
	Packages  int
}

func Generate(ctx context.Context, opt GenerateOptions) (GenerateResult, error) {
	root := opt.Root
	if root == "" {
		var err error
		root, err = findProjectRoot()
		if err != nil {
			return GenerateResult{}, err
		}
	}
	inv, err := metadata.LoadFixture(filepath.Join(root, "generator", "testdata", "e2e", "source.json"))
	if err != nil {
		return GenerateResult{}, err
	}
	inv.GeneratorVersion = Version
	if err = normalize.Inventory(&inv); err != nil {
		return GenerateResult{}, fmt.Errorf("normalize fixture: %w", err)
	}
	var rawDumps map[string][]byte
	if opt.DumpRaw {
		rawDumps = make(map[string][]byte)
	}
	if opt.All {
		if err = ingestLocked(ctx, root, &inv, rawDumps); err != nil {
			return GenerateResult{}, err
		}
		disambiguateExactMetadataDuplicates(inv.Symbols)
	}
	overrides, err := metadata.LoadOverrides(filepath.Join(root, "overrides"))
	if err != nil {
		return GenerateResult{}, err
	}
	if err = metadata.ApplyOverrides(inv.Symbols, inv.Sources, overrides); err != nil {
		return GenerateResult{}, err
	}
	if err = normalize.Inventory(&inv); err != nil {
		return GenerateResult{}, fmt.Errorf("normalize IR: %w", err)
	}
	module := modulePath(root)
	tree, err := emit.Render(inv, module)
	if err != nil {
		return GenerateResult{}, err
	}
	sliceTree, err := slice.Render(root, module, Version)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("render reviewed slice: %w", err)
	}
	report := internalcoverage.Build(inv)
	slicePackages := map[string]bool{}
	for path := range sliceTree.Files {
		if strings.HasSuffix(path, ".go") {
			slicePackages[filepath.ToSlash(filepath.Dir(path))] = true
		}
	}
	result := GenerateResult{Inventory: inv, Coverage: report, Files: len(tree.Files) + len(sliceTree.Files), Packages: generatedPackageCount(tree.Files) + len(slicePackages)}
	if opt.Write {
		if err = writeGeneration(root, inv, report, tree, sliceTree, rawDumps); err != nil {
			return GenerateResult{}, err
		}
	}
	return result, nil
}

func ingestLocked(ctx context.Context, root string, inv *model.Inventory, rawDumps map[string][]byte) error {
	m, err := OpenSourceManager(root)
	if err != nil {
		return err
	}
	verify, verifyErr := m.Verify()
	states := map[string]SourceResult{}
	for _, r := range verify {
		states[r.ID] = r
	}
	if verifyErr != nil {
		return fmt.Errorf("locked inputs are not ready; run winapisource fetch: %w", verifyErr)
	}
	sourceSeen := map[string]bool{}
	for _, s := range inv.Sources {
		sourceSeen[s.ID] = true
	}
	for _, locked := range m.List() {
		if !sourceSeen[locked.ID] {
			inv.Sources = append(inv.Sources, model.Source{ID: locked.ID, Type: locked.Type, Package: locked.Package, Version: locked.Version, SHA256: locked.SHA256, LicenseIdentifier: locked.LicenseIdentifier, Architectures: locked.Architectures, WindowsSDKVersion: locked.WindowsSDKVersion, Files: locked.Files})
			sourceSeen[locked.ID] = true
		}
		state := states[locked.ID]
		if state.State != "verified" {
			inv.Symbols = append(inv.Symbols, sourceSentinel(locked.ID, locked.Type, locked.Package, "external-sdk-not-installed", state.Message))
			continue
		}
		var file string
		switch locked.Type {
		case "win32-winmd":
			file = "Windows.Win32.winmd"
		case "wdk-winmd":
			file = "Windows.Wdk.winmd"
		case "winrt-winmd":
			file = "Windows.winmd"
		case "windows-app-sdk":
			inv.Symbols = append(inv.Symbols, sourceSentinel(locked.ID, locked.Type, locked.Package, "missing-upstream-metadata", "meta package is verified, but transitive API packages do not yet have separately hashed provider locks"))
			continue
		default:
			inv.Symbols = append(inv.Symbols, sourceSentinel(locked.ID, locked.Type, locked.Package, "unsupported-projection", "provider is registered but not selected in this run"))
			continue
		}
		provider := metadata.WinMDProvider{SourceType: locked.Type}
		res, err := provider.Ingest(ctx, metadata.Request{Root: root, Locked: locked, Path: filepath.Join(m.CacheDir(locked), file), ABIProfile: profileFor(locked.Type)})
		if err != nil {
			return fmt.Errorf("ingest %s: %w", locked.ID, err)
		}
		inv.Symbols = append(inv.Symbols, res.Symbols...)
		inv.Diagnostics = append(inv.Diagnostics, res.Diagnostics...)
		if rawDumps != nil {
			b, e := json.MarshalIndent(res.Raw, "", "  ")
			if e != nil {
				return e
			}
			b = append(b, '\n')
			if filepath.Base(locked.ID) != locked.ID || strings.ContainsAny(locked.ID, `/\`) {
				return fmt.Errorf("unsafe locked source ID for raw dump: %q", locked.ID)
			}
			rawDumps[filepath.ToSlash(filepath.Join("coverage", "raw", locked.ID+".json"))] = b
		}
	}
	markDuplicateOrigins(inv.Symbols)
	return nil
}

func sourceSentinel(id, sourceType, pkg, reason, message string) model.Symbol {
	status := model.StatusUnsupportedProjection
	switch reason {
	case "external-sdk-not-installed":
		status = model.StatusExternalSDKNotInstalled
	case "missing-upstream-metadata":
		status = model.StatusMissingUpstreamMetadata
	}
	if message == "" {
		message = reason
	}
	return model.Symbol{SourceID: id, SourceType: sourceType, Namespace: "", Kind: model.KindOpaque, NativeName: pkg + "::<source>", GoName: "", Architecture: "neutral", ABIProfile: "source-provider", CanonicalSignature: "source-provider-sentinel:" + pkg, Status: status, StatusReason: reason + ":" + message, Backend: model.BackendUnsupported, BackendReason: "no callable symbol is emitted for a source-level sentinel", Provenance: []model.Provenance{{SourceID: id, InputFile: "sources.lock.json"}}, Attributes: map[string]string{"inventorySentinel": "true"}}
}
func profileFor(sourceType string) string {
	if sourceType == "wdk-winmd" {
		return "windows-wdk"
	}
	return "windows-desktop"
}

func markDuplicateOrigins(symbols []model.Symbol) {
	first := map[string]string{}
	for i := range symbols {
		s := &symbols[i]
		key := s.Namespace + "\x00" + string(s.Kind) + "\x00" + s.NativeName + "\x00" + s.CanonicalSignature + "\x00" + s.Architecture
		if id, ok := first[key]; ok {
			if s.Attributes == nil {
				s.Attributes = map[string]string{}
			}
			s.Attributes["duplicateProjectionOf"] = id
			s.BackendReason += "; duplicate projection suppressed while provenance remains source-specific"
		} else {
			first[key] = s.SourceID
		}
	}
}

func disambiguateExactMetadataDuplicates(symbols []model.Symbol) {
	groups := map[string][]int{}
	for i, s := range symbols {
		key := strings.Join([]string{s.SourceID, s.Namespace, string(s.Kind), s.NativeName, s.Architecture, s.CanonicalSignature, fmt.Sprintf("%d", s.GenericArity), s.ABIProfile}, "\x00")
		groups[key] = append(groups[key], i)
	}
	for _, indices := range groups {
		if len(indices) < 2 {
			continue
		}
		sort.Slice(indices, func(i, j int) bool {
			a, b := symbols[indices[i]], symbols[indices[j]]
			ap, bp := model.Provenance{}, model.Provenance{}
			if len(a.Provenance) > 0 {
				ap = a.Provenance[0]
			}
			if len(b.Provenance) > 0 {
				bp = b.Provenance[0]
			}
			if ap.MetadataTable != bp.MetadataTable {
				return ap.MetadataTable < bp.MetadataTable
			}
			return ap.MetadataRow < bp.MetadataRow
		})
		for ordinal, index := range indices {
			symbols[index].CanonicalSignature += fmt.Sprintf(" #upstream-duplicate=%d", ordinal)
			if symbols[index].Attributes == nil {
				symbols[index].Attributes = map[string]string{}
			}
			symbols[index].Attributes["upstreamDuplicateOrdinal"] = fmt.Sprintf("%d/%d", ordinal, len(indices))
		}
	}
}

func writeReports(root string, inv model.Inventory, r internalcoverage.Report) error {
	inventoryPath := filepath.Join(root, "coverage", "inventory.json.gz")
	if err := writeAtomicStream(inventoryPath, func(w io.Writer) error {
		gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(gz)
		if err = enc.Encode(inv); err != nil {
			_ = gz.Close()
			return err
		}
		return gz.Close()
	}); err != nil {
		return err
	}
	// Schema v1 originally wrote a very large uncompressed file. It is an owned
	// generated artifact and is removed only after the compressed replacement is durable.
	_ = os.Remove(filepath.Join(root, "coverage", "inventory.json"))
	b, err := internalcoverage.MarshalJSON(r)
	if err != nil {
		return err
	}
	if err = writeAtomicFile(filepath.Join(root, "coverage", "latest.json"), b); err != nil {
		return err
	}
	return writeAtomicFile(filepath.Join(root, "coverage", "latest.md"), []byte(internalcoverage.Markdown(r)))
}

func writeAtomicStream(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".winapigen-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = write(f); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replaceFile(name, path)
}

func writeAtomicFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".winapigen-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return replaceFile(name, path)
}

func replaceFile(name, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return os.Rename(name, path)
	} else if err != nil {
		return err
	}
	backup := path + ".old"
	if _, e := os.Stat(backup); e == nil {
		return fmt.Errorf("stale report backup %s", backup)
	}
	if e := os.Rename(path, backup); e != nil {
		return e
	}
	if e := os.Rename(name, path); e != nil {
		_ = os.Rename(backup, path)
		return e
	}
	return os.Remove(backup)
}

func findProjectRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err = os.Stat(filepath.Join(d, "project.yaml")); err == nil {
			return d, nil
		}
		p := filepath.Dir(d)
		if p == d {
			return "", errors.New("project.yaml not found")
		}
		d = p
	}
}
func modulePath(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "go-windows-api.local"
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "module" {
			return f[1]
		}
	}
	return "go-windows-api.local"
}
func generatedPackageCount(files map[string][]byte) int {
	m := map[string]bool{}
	for path := range files {
		m[filepath.ToSlash(filepath.Dir(path))] = true
	}
	return len(m)
}

func ReadInventory(path string) (model.Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Inventory{}, err
	}
	defer f.Close()
	var reader io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, e := gzip.NewReader(f)
		if e != nil {
			return model.Inventory{}, e
		}
		defer gz.Close()
		reader = gz
	}
	var inv model.Inventory
	err = json.NewDecoder(reader).Decode(&inv)
	return inv, err
}
func FindSymbols(inv model.Inventory, qualified, native, id string) []model.Symbol {
	var out []model.Symbol
	for _, s := range inv.Symbols {
		if qualified != "" && s.QualifiedName() != qualified {
			continue
		}
		if native != "" && s.NativeName != native {
			continue
		}
		if id != "" && s.ID != id {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
