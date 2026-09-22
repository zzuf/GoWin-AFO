package ecma335

import (
	"encoding/binary"
	"errors"
	"testing"
)

func TestCompressedUint(t *testing.T) {
	tests := []struct {
		b    []byte
		want uint32
	}{{[]byte{0x7f}, 0x7f}, {[]byte{0x80, 0x80}, 0x80}, {[]byte{0xbf, 0xff}, 0x3fff}, {[]byte{0xc0, 0, 0x40, 0}, 0x4000}, {[]byte{0xdf, 0xff, 0xff, 0xff}, 0x1fffffff}}
	for _, tt := range tests {
		r := Reader{Data: tt.b}
		got, err := r.CompressedUint()
		if err != nil || got != tt.want {
			t.Fatalf("%x: got %x err %v", tt.b, got, err)
		}
	}
}

func TestCompressedUintRejectsMalformedAndTruncated(t *testing.T) {
	for _, b := range [][]byte{{0x80}, {0xc0, 0}, {0xe0}, {0x80, 1}, {0xc0, 0, 0, 1}} {
		r := Reader{Data: b}
		if _, err := r.CompressedUint(); err == nil {
			t.Fatalf("accepted %x", b)
		}
	}
}

func TestBlob(t *testing.T) {
	heap := []byte{0, 3, 'a', 'b', 'c'}
	got, err := ReadBlobAt(heap, 1)
	if err != nil || string(got) != "abc" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err = ReadBlobAt(heap, 2); err == nil {
		t.Fatal("accepted truncated blob")
	}
}

func TestCodedIndex(t *testing.T) {
	idx, null, err := DecodeCodedIndex((4<<2)|1, 2, []TableID{TableTypeDef, TableTypeRef, TableField})
	if err != nil || null || idx.Table != TableTypeRef || idx.Row != 3 {
		t.Fatalf("%+v %v %v", idx, null, err)
	}
	_, null, err = DecodeCodedIndex(0, 2, []TableID{TableTypeDef})
	if err != nil || !null {
		t.Fatal("null coded index")
	}
}

func TestMethodAndFieldSignatures(t *testing.T) {
	sig, err := ParseMethodSignature([]byte{0x00, 0x02, byte(ElementI4), byte(ElementU4), byte(ElementPtr), byte(ElementU2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(sig.Parameters) != 2 || sig.Return.Element != ElementI4 || sig.Parameters[1].ElementType.Element != ElementU2 {
		t.Fatalf("%+v", sig)
	}
	f, err := ParseFieldSignature([]byte{0x06, byte(ElementI8)})
	if err != nil || f.Element != ElementI8 {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err = ParseMethodSignature([]byte{0, 1, byte(ElementVoid)}); !errors.Is(err, ErrTruncated) {
		t.Fatalf("expected truncated: %v", err)
	}
}

func TestCustomAttribute(t *testing.T) {
	blob := []byte{1, 0, 3, 's', 'd', 'k', 0, 0}
	a, err := ParseCustomAttributeStrings(blob, 1)
	if err != nil || a.Fixed[0] != "sdk" {
		t.Fatalf("%+v %v", a, err)
	}
	blob[0] = 0
	if _, err = ParseCustomAttributeStrings(blob, 1); err == nil {
		t.Fatal("accepted bad prolog")
	}
}

func TestTableHeaderAndTruncation(t *testing.T) {
	b := make([]byte, 28)
	b[4] = 2
	b[5] = 0
	b[6] = 1
	binary.LittleEndian.PutUint64(b[8:16], 1<<uint(TableTypeDef))
	binary.LittleEndian.PutUint32(b[24:28], 7)
	h, err := ParseTableStreamHeader(b)
	if err != nil || h.Rows[TableTypeDef] != 7 {
		t.Fatalf("%+v %v", h, err)
	}
	if _, err = ParseTableStreamHeader(b[:26]); !errors.Is(err, ErrTruncated) {
		t.Fatalf("%v", err)
	}
}
