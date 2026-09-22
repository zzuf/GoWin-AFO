package clangast

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"go-windows-api.local/generator/internal/model"
)

func TestInventoryASTWithoutHeaderRegex(t *testing.T) {
	data := []byte(`{"kind":"TranslationUnitDecl","inner":[{"kind":"RecordDecl","name":"PACKED_BITS","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"Flags","isBitfield":true,"type":{"qualType":"unsigned int"},"inner":[{"kind":"ConstantExpr","value":"3"}]}]},{"kind":"FunctionDecl","name":"HeaderOnly","type":{"qualType":"int (float)"}}]}`)
	var root Node
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	got := Inventory(root, "s", "clang-header", "amd64", "desktop", "fixture.h")
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].Kind != model.KindFunction && got[1].Kind != model.KindFunction {
		t.Fatal("function absent")
	}
	var foundBits bool
	for _, s := range got {
		if s.Type != nil && len(s.Type.Fields) > 0 && s.Type.Fields[0].BitField != nil && s.Type.Fields[0].BitField.BitWidth == 3 {
			foundBits = true
		}
	}
	if !foundBits {
		t.Fatal("bit field width absent")
	}
}

func TestSanitizeRawASTRemovesProcessIDsAndLocalPaths(t *testing.T) {
	local := filepath.Join(t.TempDir(), "include", "fixture.h")
	node := Node{ID: "0x7ffee123"}
	node.Loc.File = local
	node.Inner = []Node{{ID: "0x7ffee456"}}
	node.Inner[0].Range.Begin.File = local
	sanitizeRawNode(&node)
	if node.ID != "" || node.Inner[0].ID != "" || node.Loc.File != "fixture.h" || node.Inner[0].Range.Begin.File != "fixture.h" {
		t.Fatalf("raw AST was not sanitized: %+v", node)
	}
	message := sanitizeClangText(local+":1: error", local, nil)
	if message != "fixture.h:1: error" {
		t.Fatalf("diagnostic path was not sanitized: %q", message)
	}
}

func TestInventoryClangMacroTable(t *testing.T) {
	dump := []byte("#define HEADER_CONST 0x20u\n#define HEADER_FN(x,y) ((x) + (y))\n")
	got := InventoryMacros(dump, "sdk", "clang-header", "amd64", "windows-desktop", "fixture.h")
	if len(got) != 2 {
		t.Fatalf("got %d macros", len(got))
	}
	if got[0].NativeName != "HEADER_CONST" || got[0].Kind != model.KindConstant || got[0].Constant == nil {
		t.Fatalf("object macro = %#v", got[0])
	}
	if got[1].NativeName != "HEADER_FN" || got[1].Kind != model.KindFunction || got[1].Backend != model.BackendUnsupported {
		t.Fatalf("function macro = %#v", got[1])
	}
	if got[1].StatusReason == "" {
		t.Fatal("function macro lacks reason code")
	}
}
