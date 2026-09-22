package winabi

import (
	"errors"
	"strings"
	"unicode/utf16"
)

var ErrEmbeddedNUL = errors.New("winabi: string contains an embedded NUL")

// UTF16FromString converts a Go string to a NUL-terminated UTF-16 buffer. It
// rejects embedded NUL instead of silently truncating a native C-style string.
func UTF16FromString(value string) ([]uint16, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return nil, ErrEmbeddedNUL
	}
	result := utf16.Encode([]rune(value))
	result = append(result, 0)
	return result, nil
}

// UTF16PtrFromString returns a pointer to a newly allocated NUL-terminated
// UTF-16 buffer. The pointer keeps the backing allocation reachable, but callers
// should still use runtime.KeepAlive after the native call for clear lifetime
// auditing. Embedded NUL is rejected.
func UTF16PtrFromString(value string) (*uint16, error) {
	buffer, err := UTF16FromString(value)
	if err != nil {
		return nil, err
	}
	return &buffer[0], nil
}
