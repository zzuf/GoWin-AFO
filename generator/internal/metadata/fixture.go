package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"go-windows-api.local/generator/internal/model"
)

func LoadFixture(path string) (model.Inventory, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.Inventory{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var inv model.Inventory
	if err = dec.Decode(&inv); err != nil {
		return inv, fmt.Errorf("decode fixture: %w", err)
	}
	return inv, nil
}
