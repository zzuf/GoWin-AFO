// Package ecma335 contains strict, bounds-checked helpers missing from the
// upstream WinMD reader. The main PE/table reader remains microsoft/go-winmd.
package ecma335

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrTruncated       = errors.New("ecma-335: truncated input")
	ErrInvalidEncoding = errors.New("ecma-335: invalid encoding")
)

type Reader struct {
	Data []byte
	Off  int
}

func (r *Reader) Remaining() int { return len(r.Data) - r.Off }

func (r *Reader) Byte() (byte, error) {
	if r.Off < 0 || r.Off >= len(r.Data) {
		return 0, ErrTruncated
	}
	v := r.Data[r.Off]
	r.Off++
	return v, nil
}

func (r *Reader) Bytes(n uint32) ([]byte, error) {
	if uint64(n) > uint64(^uint(0)>>1) || r.Off < 0 || int(n) > len(r.Data)-r.Off {
		return nil, ErrTruncated
	}
	v := r.Data[r.Off : r.Off+int(n)]
	r.Off += int(n)
	return v, nil
}

// CompressedUint decodes ECMA-335 II.23.2 compressed unsigned integers and
// rejects reserved prefixes and non-canonical overlong encodings.
func (r *Reader) CompressedUint() (uint32, error) {
	b0, err := r.Byte()
	if err != nil {
		return 0, err
	}
	switch {
	case b0&0x80 == 0:
		return uint32(b0), nil
	case b0&0xC0 == 0x80:
		b1, err := r.Byte()
		if err != nil {
			return 0, err
		}
		v := uint32(b0&0x3F)<<8 | uint32(b1)
		if v < 0x80 {
			return 0, ErrInvalidEncoding
		}
		return v, nil
	case b0&0xE0 == 0xC0:
		bs, err := r.Bytes(3)
		if err != nil {
			return 0, err
		}
		v := uint32(b0&0x1F)<<24 | uint32(bs[0])<<16 | uint32(bs[1])<<8 | uint32(bs[2])
		if v < 0x4000 || v > 0x1FFFFFFF {
			return 0, ErrInvalidEncoding
		}
		return v, nil
	default:
		return 0, ErrInvalidEncoding
	}
}

// CompressedInt decodes the rotated sign representation from II.23.2.
func (r *Reader) CompressedInt() (int32, error) {
	start := r.Off
	u, err := r.CompressedUint()
	if err != nil {
		return 0, err
	}
	encodedBytes := r.Off - start
	v := int32(u >> 1)
	if u&1 != 0 {
		switch encodedBytes {
		case 1:
			v |= ^int32(0x3F)
		case 2:
			v |= ^int32(0x1FFF)
		case 4:
			v |= ^int32(0x0FFFFFFF)
		default:
			return 0, ErrInvalidEncoding
		}
	}
	return v, nil
}

// ReadBlobAt returns a copied view of one #Blob heap entry.
func ReadBlobAt(heap []byte, offset uint32) ([]byte, error) {
	if offset == 0 {
		return []byte{}, nil
	}
	if offset >= uint32(len(heap)) {
		return nil, ErrTruncated
	}
	r := Reader{Data: heap, Off: int(offset)}
	n, err := r.CompressedUint()
	if err != nil {
		return nil, fmt.Errorf("blob length: %w", err)
	}
	b, err := r.Bytes(n)
	if err != nil {
		return nil, fmt.Errorf("blob value: %w", err)
	}
	return append([]byte(nil), b...), nil
}

type TableID uint8

const (
	TableModule          TableID = 0x00
	TableTypeRef         TableID = 0x01
	TableTypeDef         TableID = 0x02
	TableField           TableID = 0x04
	TableMethodDef       TableID = 0x06
	TableParam           TableID = 0x08
	TableMemberRef       TableID = 0x0a
	TableConstant        TableID = 0x0b
	TableCustomAttribute TableID = 0x0c
	TableImplMap         TableID = 0x1c
	TableAssemblyRef     TableID = 0x23
	TableGenericParam    TableID = 0x2a
)

type CodedIndex struct {
	Table TableID
	Row   uint32
}

// DecodeCodedIndex decodes a one-based ECMA table index. zero is the null index.
func DecodeCodedIndex(raw uint32, tagBits uint8, tables []TableID) (CodedIndex, bool, error) {
	if tagBits > 5 || len(tables) == 0 || len(tables) > 1<<tagBits {
		return CodedIndex{}, false, ErrInvalidEncoding
	}
	if raw == 0 {
		return CodedIndex{}, true, nil
	}
	mask := uint32(1<<tagBits) - 1
	tag := int(raw & mask)
	if tag >= len(tables) {
		return CodedIndex{}, false, ErrInvalidEncoding
	}
	rowOneBased := raw >> tagBits
	if rowOneBased == 0 {
		return CodedIndex{}, false, ErrInvalidEncoding
	}
	return CodedIndex{Table: tables[tag], Row: rowOneBased - 1}, false, nil
}

type ElementType byte

const (
	ElementVoid        ElementType = 0x01
	ElementBoolean     ElementType = 0x02
	ElementChar        ElementType = 0x03
	ElementI1          ElementType = 0x04
	ElementU1          ElementType = 0x05
	ElementI2          ElementType = 0x06
	ElementU2          ElementType = 0x07
	ElementI4          ElementType = 0x08
	ElementU4          ElementType = 0x09
	ElementI8          ElementType = 0x0a
	ElementU8          ElementType = 0x0b
	ElementR4          ElementType = 0x0c
	ElementR8          ElementType = 0x0d
	ElementString      ElementType = 0x0e
	ElementPtr         ElementType = 0x0f
	ElementByRef       ElementType = 0x10
	ElementValueType   ElementType = 0x11
	ElementClass       ElementType = 0x12
	ElementVar         ElementType = 0x13
	ElementArray       ElementType = 0x14
	ElementGenericInst ElementType = 0x15
	ElementI           ElementType = 0x18
	ElementU           ElementType = 0x19
	ElementFnPtr       ElementType = 0x1b
	ElementObject      ElementType = 0x1c
	ElementSZArray     ElementType = 0x1d
	ElementMVar        ElementType = 0x1e
)

type TypeSig struct {
	Element     ElementType `json:"element"`
	Index       uint32      `json:"index,omitempty"`
	ElementType *TypeSig    `json:"elementType,omitempty"`
	Rank        uint32      `json:"rank,omitempty"`
	Sizes       []uint32    `json:"sizes,omitempty"`
	LowerBounds []int32     `json:"lowerBounds,omitempty"`
}

type MethodSig struct {
	CallingConvention byte      `json:"callingConvention"`
	HasThis           bool      `json:"hasThis"`
	ExplicitThis      bool      `json:"explicitThis"`
	VarArgs           bool      `json:"varArgs"`
	GenericArity      uint32    `json:"genericArity"`
	Return            TypeSig   `json:"return"`
	Parameters        []TypeSig `json:"parameters"`
}

func ParseMethodSignature(blob []byte) (MethodSig, error) {
	r := Reader{Data: blob}
	cc, err := r.Byte()
	if err != nil {
		return MethodSig{}, err
	}
	if cc&0x0f > 0x05 {
		return MethodSig{}, ErrInvalidEncoding
	}
	sig := MethodSig{CallingConvention: cc & 0x0f, HasThis: cc&0x20 != 0, ExplicitThis: cc&0x40 != 0, VarArgs: cc&0x0f == 0x05}
	if cc&0x10 != 0 {
		sig.GenericArity, err = r.CompressedUint()
		if err != nil {
			return MethodSig{}, err
		}
	}
	paramCount, err := r.CompressedUint()
	if err != nil {
		return MethodSig{}, err
	}
	sig.Return, err = parseType(&r, true)
	if err != nil {
		return MethodSig{}, fmt.Errorf("return type: %w", err)
	}
	if paramCount > uint32(r.Remaining())+1 {
		return MethodSig{}, ErrTruncated
	}
	sig.Parameters = make([]TypeSig, 0, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		t, err := parseType(&r, false)
		if err != nil {
			return MethodSig{}, fmt.Errorf("parameter %d: %w", i, err)
		}
		sig.Parameters = append(sig.Parameters, t)
	}
	if r.Remaining() != 0 {
		return MethodSig{}, fmt.Errorf("%w: trailing signature bytes", ErrInvalidEncoding)
	}
	return sig, nil
}

func ParseFieldSignature(blob []byte) (TypeSig, error) {
	r := Reader{Data: blob}
	b, err := r.Byte()
	if err != nil {
		return TypeSig{}, err
	}
	if b != 0x06 {
		return TypeSig{}, ErrInvalidEncoding
	}
	t, err := parseType(&r, false)
	if err != nil {
		return TypeSig{}, err
	}
	if r.Remaining() != 0 {
		return TypeSig{}, ErrInvalidEncoding
	}
	return t, nil
}

func parseType(r *Reader, allowVoid bool) (TypeSig, error) {
	b, err := r.Byte()
	if err != nil {
		return TypeSig{}, err
	}
	e := ElementType(b)
	t := TypeSig{Element: e}
	switch e {
	case ElementVoid:
		if !allowVoid {
			return TypeSig{}, ErrInvalidEncoding
		}
	case ElementBoolean, ElementChar, ElementI1, ElementU1, ElementI2, ElementU2, ElementI4, ElementU4, ElementI8, ElementU8, ElementR4, ElementR8, ElementString, ElementI, ElementU, ElementObject:
	case ElementPtr, ElementByRef, ElementSZArray:
		inner, err := parseType(r, e == ElementPtr)
		if err != nil {
			return TypeSig{}, err
		}
		t.ElementType = &inner
	case ElementValueType, ElementClass, ElementVar, ElementMVar:
		t.Index, err = r.CompressedUint()
		if err != nil {
			return TypeSig{}, err
		}
	case ElementArray:
		inner, err := parseType(r, false)
		if err != nil {
			return TypeSig{}, err
		}
		t.ElementType = &inner
		t.Rank, err = r.CompressedUint()
		if err != nil {
			return TypeSig{}, err
		}
		n, err := r.CompressedUint()
		if err != nil {
			return TypeSig{}, err
		}
		if n > t.Rank {
			return TypeSig{}, ErrInvalidEncoding
		}
		for i := uint32(0); i < n; i++ {
			v, e := r.CompressedUint()
			if e != nil {
				return TypeSig{}, e
			}
			t.Sizes = append(t.Sizes, v)
		}
		n, err = r.CompressedUint()
		if err != nil {
			return TypeSig{}, err
		}
		if n > t.Rank {
			return TypeSig{}, ErrInvalidEncoding
		}
		for i := uint32(0); i < n; i++ {
			v, e := r.CompressedInt()
			if e != nil {
				return TypeSig{}, e
			}
			t.LowerBounds = append(t.LowerBounds, v)
		}
	case ElementGenericInst, ElementFnPtr:
		return TypeSig{}, fmt.Errorf("%w: complex element 0x%x requires upstream signature parser", ErrInvalidEncoding, b)
	default:
		return TypeSig{}, fmt.Errorf("%w: element 0x%x", ErrInvalidEncoding, b)
	}
	return t, nil
}

type CustomAttribute struct {
	Fixed      []any  `json:"fixed"`
	NamedCount uint16 `json:"namedCount"`
}

// ParseCustomAttributeStrings decodes a custom attribute whose fixed arguments
// are all SerString. The constructor signature determines fixedStringCount.
func ParseCustomAttributeStrings(blob []byte, fixedStringCount int) (CustomAttribute, error) {
	if fixedStringCount < 0 {
		return CustomAttribute{}, ErrInvalidEncoding
	}
	r := Reader{Data: blob}
	prolog, err := r.Bytes(2)
	if err != nil {
		return CustomAttribute{}, err
	}
	if binary.LittleEndian.Uint16(prolog) != 1 {
		return CustomAttribute{}, ErrInvalidEncoding
	}
	a := CustomAttribute{Fixed: make([]any, 0, fixedStringCount)}
	for i := 0; i < fixedStringCount; i++ {
		s, isNull, e := readSerString(&r)
		if e != nil {
			return CustomAttribute{}, e
		}
		if isNull {
			a.Fixed = append(a.Fixed, nil)
		} else {
			a.Fixed = append(a.Fixed, s)
		}
	}
	n, err := r.Bytes(2)
	if err != nil {
		return CustomAttribute{}, err
	}
	a.NamedCount = binary.LittleEndian.Uint16(n)
	if a.NamedCount != 0 {
		return CustomAttribute{}, fmt.Errorf("%w: named arguments require typed decoder", ErrInvalidEncoding)
	}
	if r.Remaining() != 0 {
		return CustomAttribute{}, ErrInvalidEncoding
	}
	return a, nil
}

func readSerString(r *Reader) (string, bool, error) {
	if r.Remaining() < 1 {
		return "", false, ErrTruncated
	}
	if r.Data[r.Off] == 0xff {
		r.Off++
		return "", true, nil
	}
	n, err := r.CompressedUint()
	if err != nil {
		return "", false, err
	}
	b, err := r.Bytes(n)
	if err != nil {
		return "", false, err
	}
	return string(b), false, nil
}

type TableStreamHeader struct {
	Major, Minor, HeapSizes byte
	Valid, Sorted           uint64
	Rows                    map[TableID]uint32
}

func ParseTableStreamHeader(data []byte) (TableStreamHeader, error) {
	// II.24.2.6: reserved(4), major, minor, heap sizes, reserved, valid, sorted.
	if len(data) < 24 {
		return TableStreamHeader{}, ErrTruncated
	}
	h := TableStreamHeader{Major: data[4], Minor: data[5], HeapSizes: data[6], Valid: binary.LittleEndian.Uint64(data[8:16]), Sorted: binary.LittleEndian.Uint64(data[16:24]), Rows: map[TableID]uint32{}}
	off := 24
	for table := uint8(0); table < 64; table++ {
		if h.Valid&(uint64(1)<<table) == 0 {
			continue
		}
		if len(data)-off < 4 {
			return TableStreamHeader{}, ErrTruncated
		}
		h.Rows[TableID(table)] = binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
	}
	return h, nil
}
