package winrt

import (
	"errors"

	"go-windows-api.local/runtime/winabi"
)

// ErrParameterizedIIDUnsupported makes the current generic projection boundary
// explicit. Returning a guessed or hash-incomplete IID would be ABI-unsafe.
var ErrParameterizedIIDUnsupported = errors.New("winrt: parameterized IID calculation is not implemented")

// ParameterizedIID deliberately refuses to synthesize a WinRT generic IID
// until the canonical signature grammar and UUIDv5 namespace algorithm are
// implemented and verified against official metadata.
func ParameterizedIID(signature string) (winabi.GUID, error) {
	return winabi.GUID{}, ErrParameterizedIIDUnsupported
}
