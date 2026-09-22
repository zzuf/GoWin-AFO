// Package metadata ingests official source formats into the normalized IR.
package metadata

import (
	"context"

	"go-windows-api.local/generator/internal/model"
	"go-windows-api.local/generator/internal/source"
)

type Request struct {
	Root       string
	Locked     source.Locked
	Path       string
	ABIProfile string
}
type Result struct {
	Source      model.Source       `json:"source"`
	Symbols     []model.Symbol     `json:"symbols"`
	Raw         any                `json:"raw"`
	Diagnostics []model.Diagnostic `json:"diagnostics,omitempty"`
}

type Provider interface {
	Type() string
	Ingest(context.Context, Request) (Result, error)
}

// ExternalSDKProvider makes packages such as WebView2, DirectX Agility and
// DirectStorage additive sources with separate coverage denominators.
type ExternalSDKProvider interface {
	Provider
	Detect(context.Context) (installed bool, version string, err error)
}

// TypeLibraryProvider keeps product type libraries outside Windows OS coverage.
type TypeLibraryProvider interface {
	Provider
	InventoryTypeLibrary(context.Context, string) (TypeLibraryInventory, error)
}

type TypeLibraryInventory struct {
	LibraryID      string        `json:"libid"`
	Name           string        `json:"name"`
	Version        string        `json:"version"`
	CoClasses      []TLCoClass   `json:"coclasses"`
	Interfaces     []TLInterface `json:"interfaces"`
	DispInterfaces []TLInterface `json:"dispinterfaces"`
	Enums          []TLNamed     `json:"enums"`
	Records        []TLNamed     `json:"records"`
	Aliases        []TLNamed     `json:"aliases"`
}
type TLNamed struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}
type TLCoClass struct {
	Name, CLSID string
	Interfaces  []string
}
type TLInterface struct {
	Name, IID  string
	Methods    []TLMember
	Properties []TLMember
	Events     []TLMember
}
type TLMember struct {
	Name      string
	DISPID    int32
	Signature string
}
