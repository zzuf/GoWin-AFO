package source

import (
	"fmt"
	"strings"
)

// MetadataFile returns the unique WinMD archive member explicitly pinned by the
// lock. Directory layout is part of the input identity, not a provider guess.
func (s Locked) MetadataFile() (string, error) {
	var selected string
	for _, file := range s.Files {
		if !strings.HasSuffix(strings.ToLower(file), ".winmd") {
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("source %s has multiple locked WinMD files", s.ID)
		}
		selected = file
	}
	if selected == "" {
		return "", fmt.Errorf("source %s has no locked WinMD file", s.ID)
	}
	return selected, nil
}
