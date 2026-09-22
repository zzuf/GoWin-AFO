//go:build amd64 || arm64

package winabi

// SignedPointer is a signed integer with the width of a native pointer.
type SignedPointer int64

type SSIZE_T int64
type INT_PTR int64
type LONG_PTR int64
