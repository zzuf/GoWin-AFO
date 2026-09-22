// Package winabi contains the small, audited ABI runtime used by generated
// Windows bindings. It deliberately supports only integer and pointer-shaped
// syscall arguments; signatures with floating-point, vector, varargs, or
// aggregate-by-value ABI requirements must use another backend.
package winabi
