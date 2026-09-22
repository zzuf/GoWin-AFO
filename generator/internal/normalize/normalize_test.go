package normalize

import (
	"testing"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

func TestStableIDIncludesABIInputs(t *testing.T) {
	base := model.Symbol{SourceID: "s", Namespace: "N", Kind: model.KindFunction, NativeName: "F", Architecture: "amd64", CanonicalSignature: "F(i4):u4", ABIProfile: "desktop"}
	a := StableID(base)
	base.Architecture = "arm64"
	if b := StableID(base); a == b {
		t.Fatal("architecture must affect stable ID")
	}
	base.Architecture = "amd64"
	base.CanonicalSignature = "F(i8):u4"
	if b := StableID(base); a == b {
		t.Fatal("signature must affect stable ID")
	}
}

func TestInventoryRejectsUnclassified(t *testing.T) {
	inv := model.Inventory{Symbols: []model.Symbol{{SourceID: "s", Kind: model.KindStruct, NativeName: "T", Architecture: "neutral", ABIProfile: "desktop", Status: model.StatusUnclassified, StatusReason: "none", Backend: model.BackendTypeOnly, BackendReason: "type"}}}
	if err := Inventory(&inv); err == nil {
		t.Fatal("expected unclassified rejection")
	}
}
