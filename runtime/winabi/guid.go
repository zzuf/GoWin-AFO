package winabi

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// GUID is the native 16-byte GUID layout used by Windows, COM, and WinRT.
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// ParseGUID parses the canonical 8-4-4-4-12 GUID form, with optional braces.
func ParseGUID(value string) (GUID, error) {
	var result GUID
	value = strings.TrimSpace(value)
	if len(value) == 38 && value[0] == '{' && value[37] == '}' {
		value = value[1:37]
	}
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return result, errors.New("winabi: GUID must use 8-4-4-4-12 hexadecimal form")
	}
	data1, err := strconv.ParseUint(value[0:8], 16, 32)
	if err != nil {
		return result, fmt.Errorf("winabi: parse GUID Data1: %w", err)
	}
	data2, err := strconv.ParseUint(value[9:13], 16, 16)
	if err != nil {
		return result, fmt.Errorf("winabi: parse GUID Data2: %w", err)
	}
	data3, err := strconv.ParseUint(value[14:18], 16, 16)
	if err != nil {
		return result, fmt.Errorf("winabi: parse GUID Data3: %w", err)
	}
	tail := value[19:23] + value[24:36]
	decoded, err := hex.DecodeString(tail)
	if err != nil {
		return result, fmt.Errorf("winabi: parse GUID Data4: %w", err)
	}
	result.Data1 = uint32(data1)
	result.Data2 = uint16(data2)
	result.Data3 = uint16(data3)
	copy(result.Data4[:], decoded)
	return result, nil
}

// MustParseGUID parses value and panics if a generated, constant GUID is
// malformed. It is intended only for package-level metadata constants.
func MustParseGUID(value string) GUID {
	guid, err := ParseGUID(value)
	if err != nil {
		panic(err)
	}
	return guid
}

func (g GUID) String() string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3],
		g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

// IsZero reports whether every GUID field is zero.
func (g GUID) IsZero() bool { return g == (GUID{}) }
