// Package typelib defines a deterministic inventory interchange for TLB/OLB
// readers. Native COM loading can feed this format without mixing product APIs
// into Windows OS coverage.
package typelib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/zzuf/GoWin-AFO/generator/internal/metadata"
	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

type JSONProvider struct{}

func (JSONProvider) Type() string { return "typelib" }
func (JSONProvider) Detect(context.Context) (bool, string, error) {
	return true, "json-interchange-v1", nil
}
func (JSONProvider) InventoryTypeLibrary(_ context.Context, path string) (metadata.TypeLibraryInventory, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return metadata.TypeLibraryInventory{}, err
	}
	var inv metadata.TypeLibraryInventory
	if err = json.Unmarshal(b, &inv); err != nil {
		return inv, err
	}
	if inv.LibraryID == "" {
		return inv, fmt.Errorf("type library has no LIBID")
	}
	return inv, nil
}
func (p JSONProvider) Ingest(ctx context.Context, req metadata.Request) (metadata.Result, error) {
	inv, err := p.InventoryTypeLibrary(ctx, req.Path)
	if err != nil {
		return metadata.Result{}, err
	}
	var syms []model.Symbol
	inputFile := filepath.Base(req.Path)
	add := func(kind model.SymbolKind, name, sig string) {
		syms = append(syms, model.Symbol{SourceID: req.Locked.ID, SourceType: "typelib", Namespace: inv.Name, Kind: kind, NativeName: name, GoName: name, Architecture: "neutral", ABIProfile: req.ABIProfile, CanonicalSignature: sig, Status: model.StatusUnsupportedProjection, StatusReason: "external-product-typelib-inventory-is-separate-from-os-generation", Backend: model.BackendTypeOnly, BackendReason: "type library member inventory only", Provenance: []model.Provenance{{SourceID: req.Locked.ID, InputFile: inputFile}}})
	}
	for _, v := range inv.CoClasses {
		add(model.KindCoClass, v.Name, "CLSID="+v.CLSID)
	}
	for _, v := range inv.Interfaces {
		add(model.KindCOMInterface, v.Name, "IID="+v.IID)
		for _, m := range append(append([]metadata.TLMember{}, v.Methods...), append(v.Properties, v.Events...)...) {
			add(model.KindFunction, v.Name+"."+m.Name, fmt.Sprintf("DISPID=%d %s", m.DISPID, m.Signature))
		}
	}
	for _, v := range inv.DispInterfaces {
		add(model.KindDispInterface, v.Name, "IID="+v.IID)
	}
	for _, v := range inv.Enums {
		add(model.KindEnum, v.Name, v.ID)
	}
	for _, v := range inv.Records {
		add(model.KindStruct, v.Name, v.ID)
	}
	for _, v := range inv.Aliases {
		add(model.KindAlias, v.Name, v.ID)
	}
	sort.Slice(syms, func(i, j int) bool { return syms[i].CanonicalSignature < syms[j].CanonicalSignature })
	return metadata.Result{Source: model.Source{ID: req.Locked.ID, Type: "typelib", Package: req.Locked.Package, Version: req.Locked.Version, SHA256: req.Locked.SHA256, LicenseIdentifier: req.Locked.LicenseIdentifier, Architectures: req.Locked.Architectures, Files: req.Locked.Files}, Symbols: syms, Raw: inv}, nil
}
