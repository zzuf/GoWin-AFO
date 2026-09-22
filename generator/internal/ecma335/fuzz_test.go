package ecma335

import "testing"

func FuzzCompressedUint(f *testing.F) {
	for _, seed := range [][]byte{{0}, {0x7f}, {0x80, 0x80}, {0xc0, 0, 0x40, 0}, {0xff}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		r := Reader{Data: b}
		_, _ = r.CompressedUint()
		if r.Off < 0 || r.Off > len(b) {
			t.Fatalf("offset escaped input: %d/%d", r.Off, len(b))
		}
	})
}

func FuzzSignatures(f *testing.F) {
	f.Add([]byte{0, 0, 1})
	f.Add([]byte{6, 8})
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseMethodSignature(b); _, _ = ParseFieldSignature(b) })
}
