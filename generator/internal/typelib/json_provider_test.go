package typelib

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zzuf/GoWin-AFO/generator/internal/metadata"
	"github.com/zzuf/GoWin-AFO/generator/internal/source"
)

func TestTypeLibraryInventory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(p, []byte(`{"libid":"{1}","name":"Product","version":"1","coclasses":[{"Name":"App","CLSID":"{2}","Interfaces":["IApp"]}],"interfaces":[{"Name":"IApp","IID":"{3}","Methods":[{"Name":"Run","DISPID":1,"Signature":"HRESULT()"}],"Properties":null,"Events":null}],"dispinterfaces":[],"enums":[],"records":[],"aliases":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	inv, err := (JSONProvider{}).InventoryTypeLibrary(context.Background(), p)
	if err != nil || inv.LibraryID != "{1}" {
		t.Fatalf("%+v %v", inv, err)
	}
	result, err := (JSONProvider{}).Ingest(context.Background(), metadata.Request{Path: p, ABIProfile: "windows-desktop", Locked: source.Locked{ID: "product-tlb", Version: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Symbols) == 0 || result.Symbols[0].Provenance[0].InputFile != "x.json" {
		t.Fatalf("machine-local path leaked into provenance: %+v", result.Symbols)
	}
}
