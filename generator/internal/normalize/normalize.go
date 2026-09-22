// Package normalize creates stable IDs and deterministic ordering.
package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go-windows-api.local/generator/internal/model"
)

func StableID(s model.Symbol) string {
	parts := []string{
		s.SourceID,
		s.Namespace,
		string(s.Kind),
		s.NativeName,
		s.Architecture,
		s.CanonicalSignature,
		fmt.Sprintf("%d", s.GenericArity),
		s.ABIProfile,
	}
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func Inventory(inv *model.Inventory) error {
	inv.SchemaVersion = model.SchemaVersion
	seen := make(map[string]string, len(inv.Symbols))
	for i := range inv.Symbols {
		s := &inv.Symbols[i]
		s.Namespace = strings.TrimSpace(s.Namespace)
		s.NativeName = strings.TrimSpace(s.NativeName)
		if s.CanonicalSignature == "" {
			s.CanonicalSignature = string(s.Kind) + " " + s.QualifiedName()
		}
		s.ID = StableID(*s)
		if err := s.Validate(); err != nil {
			return err
		}
		if previous, ok := seen[s.ID]; ok {
			return fmt.Errorf("stable ID collision %s between %s and %s", s.ID, previous, s.QualifiedName())
		}
		seen[s.ID] = s.QualifiedName()
		sort.Slice(s.Provenance, func(i, j int) bool {
			a, b := s.Provenance[i], s.Provenance[j]
			if a.SourceID != b.SourceID {
				return a.SourceID < b.SourceID
			}
			if a.InputFile != b.InputFile {
				return a.InputFile < b.InputFile
			}
			if a.MetadataTable != b.MetadataTable {
				return a.MetadataTable < b.MetadataTable
			}
			return a.MetadataRow < b.MetadataRow
		})
	}
	sort.Slice(inv.Sources, func(i, j int) bool { return inv.Sources[i].ID < inv.Sources[j].ID })
	sort.Slice(inv.Symbols, func(i, j int) bool { return inv.Symbols[i].ID < inv.Symbols[j].ID })
	sort.Slice(inv.Diagnostics, func(i, j int) bool {
		a, b := inv.Diagnostics[i], inv.Diagnostics[j]
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.SymbolID != b.SymbolID {
			return a.SymbolID < b.SymbolID
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	inv.ManifestHash = ""
	b, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("marshal normalized inventory: %w", err)
	}
	h := sha256.Sum256(b)
	inv.ManifestHash = hex.EncodeToString(h[:])
	return nil
}
