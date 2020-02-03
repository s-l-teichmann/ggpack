// This is Free Software under the terms of the MIT License.
//
// SPDX-License-Identifier: MIT
// icense-Filename: LICENSE
//
// Copyright (c) 2020 by Sascha L. Teichmann

package ggpack

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

var errTooShort = errors.New("buffer too short")

type ValueType byte

const (
	NullType    ValueType = 1
	HashType    ValueType = 2
	ArrayType   ValueType = 3
	StringType  ValueType = 4
	IntegerType ValueType = 5
	DoubleType  ValueType = 6
)

type HashEntry struct {
	Key   string
	Value *Value
}

type HashEntries []HashEntry

type Value struct {
	typ ValueType

	str     string
	integer int64
	double  float64
	hash    HashEntries
	array   []*Value
}

var Null = &Value{typ: NullType}

func (v *Value) Type() ValueType   { return v.typ }
func (v *Value) String() string    { return v.str }
func (v *Value) Integer() int64    { return v.integer }
func (v *Value) Double() float64   { return v.double }
func (v *Value) Array() []*Value   { return v.array }
func (v *Value) Hash() HashEntries { return v.hash }

func (vt ValueType) String() string {
	switch vt {
	case NullType:
		return "null"
	case HashType:
		return "hash"
	case ArrayType:
		return "array"
	case StringType:
		return "string"
	case DoubleType:
		return "double"
	case IntegerType:
		return "integer"
	default:
		return fmt.Sprintf("unknown (%d)", byte(vt))
	}
}

type Reader struct {
	Reader  io.ReadSeeker
	method  int
	offsets []int32
	entries *Value
}

func (r *Reader) Entries() *Value { return r.entries }

func (v *Value) Find(name string) *Value {
	if v == nil || v.typ != HashType {
		return nil
	}
	idx := sort.Search(len(v.hash), func(i int) bool {
		return v.hash[i].Key >= name
	})
	if idx < len(v.hash) && name == strings.ToLower(v.hash[idx].Key) {
		return v.hash[idx].Value
	}
	return nil
}

var magicBytes = [...]byte{
	0x4f, 0xd0, 0xa0, 0xac,
	0x4a, 0x5b, 0xb9, 0xe5,
	0x93, 0x79, 0x45, 0xa5,
	0xc1, 0xcb, 0x31, 0x93,
}

func (r *Reader) ReadPack() error {

	var offset, size int32

	if err := binary.Read(r.Reader, binary.LittleEndian, &offset); err != nil {
		return err
	}
	if err := binary.Read(r.Reader, binary.LittleEndian, &size); err != nil {
		return err
	}

	buf := make([]byte, size)

	var sign uint32

	for r.method = 3; r.method >= 0; r.method-- {
		if _, err := r.Reader.Seek(int64(offset), io.SeekStart); err != nil {
			return err
		}

		if _, err := io.ReadFull(r.Reader, buf); err != nil {
			return err
		}
		r.DecodeXOR(buf)
		if len(buf) < 4 {
			return errTooShort
		}
		if sign = binary.LittleEndian.Uint32(buf); sign == 0x04030201 {
			goto supported
		}
	}

	return errors.New("unsuported package version")

supported:

	if err := r.readOffsets(buf); err != nil {
		return err
	}

	//ioutil.WriteFile("x.tmp", buf, 0666)
	slice := buf[12:]

	entries, err := r.readHash(&slice, buf)
	if err != nil {
		return err
	}

	r.entries = entries

	return nil
}

func readByte(buf *[]byte) (byte, error) {
	if len(*buf) < 1 {
		return 0, errTooShort
	}
	x := (*buf)[0]
	*buf = (*buf)[1:]
	return x, nil
}

func readInt(buf *[]byte) (int32, error) {
	if len(*buf) < 4 {
		return 0, errTooShort
	}
	x := int32(binary.LittleEndian.Uint32(*buf))
	*buf = (*buf)[4:]
	return x, nil
}

func (r *Reader) readHash(buf *[]byte, orig []byte) (*Value, error) {

	t, err := readByte(buf)
	if err != nil {
		return nil, err
	}

	if ValueType(t) != HashType {
		return nil, errors.New("trying to parse non-hash")
	}

	numEntries, err := readInt(buf)
	if err != nil {
		return nil, err
	}

	if numEntries == 0 {
		return nil, errTooShort
	}

	value := Value{typ: HashType}

	value.hash = make(HashEntries, 0, numEntries)

	for i := int32(0); i < numEntries; i++ {
		offset, err := readInt(buf)
		if err != nil {
			return nil, err
		}

		key, err := r.readString(orig, offset)
		if err != nil {
			return nil, err
		}

		entry, err := r.readValue(buf, orig)
		if err != nil {
			return nil, err
		}
		value.hash = append(value.hash, HashEntry{
			Key:   key,
			Value: entry,
		})
	}
	if t, err = readByte(buf); err != nil {
		return nil, err
	}
	if ValueType(t) != HashType {
		return nil, errors.New("unterminated hash")
	}

	sort.Slice(value.hash, func(i, j int) bool {
		return value.hash[i].Key < value.hash[j].Key
	})

	return &value, nil
}

func (r *Reader) readValue(buf *[]byte, orig []byte) (*Value, error) {

	if len(*buf) < 1 {
		return nil, errTooShort
	}

	v := Value{typ: ValueType((*buf)[0])}

	switch v.typ {
	case NullType:
		*buf = (*buf)[1:]
		return Null, nil
	case HashType:
		return r.readHash(buf, orig)
	case ArrayType:
		*buf = (*buf)[1:]
		numEntries, err := readInt(buf)
		if err != nil {
			return nil, err
		}
		v.array = make([]*Value, 0, numEntries)
		for i := int32(0); i < numEntries; i++ {
			value, err := r.readValue(buf, orig)
			if err != nil {
				return nil, err
			}
			v.array = append(v.array, value)
		}
		t, err := readByte(buf)
		if err != nil {
			return nil, err
		}
		if ValueType(t) != ArrayType {
			return nil, errors.New("unterminated array")
		}

	case StringType:
		*buf = (*buf)[1:]
		ofs, err := readInt(buf)
		if err != nil {
			return nil, err
		}
		if v.str, err = r.readString(orig, ofs); err != nil {
			return nil, err
		}

	case DoubleType, IntegerType:
		*buf = (*buf)[1:]
		ofs, err := readInt(buf)
		if err != nil {
			return nil, err
		}
		num, err := r.readString(orig, ofs)
		if err != nil {
			return nil, err
		}
		if v.typ == IntegerType {
			var err error
			if v.integer, err = strconv.ParseInt(num, 10, 64); err != nil {
				return nil, fmt.Errorf("invalid integer: %s", num)
			}
		} else {
			var err error
			if v.double, err = strconv.ParseFloat(num, 64); err != nil {
				return nil, fmt.Errorf("invalid double: %s", num)
			}
		}

	default:
		return nil, fmt.Errorf("unsupported value: %s", v.typ)
	}

	return &v, nil
}

func (r *Reader) readString(buf []byte, offset int32) (string, error) {

	if offset < 0 || int(offset) >= len(r.offsets) {
		return "", fmt.Errorf("invalid offset index: %d", offset)
	}

	ofs := r.offsets[offset]

	if ofs < 0 || int(ofs) >= len(buf) {
		return "", fmt.Errorf("invalid offset: %d", ofs)
	}

	buf = buf[ofs:]

	end := 0

	for len(buf) > end && buf[end] != 0 {
		end++
	}

	return string(buf[:end]), nil
}

func (r *Reader) readOffsets(buf []byte) error {
	if len(buf) < 12 {
		return errTooShort
	}
	plo := binary.LittleEndian.Uint32(buf[8:])

	if plo < 12 || int(plo) >= len(buf)-4 {
		return errors.New("ggpack plo out of range")
	}
	if buf[plo] != 7 {
		return errors.New("ggpack cannot find plo")
	}

	r.offsets = r.offsets[:0]

	for pos := plo + 1; int(pos+4) < len(buf); pos += 4 {
		offset := binary.LittleEndian.Uint32(buf[pos:])
		if offset == 0xffffffff {
			break
		}
		r.offsets = append(r.offsets, int32(offset))

	}
	return nil
}

func (r *Reader) DecodeXOR(buf []byte) {

	var code int
	if r.method != 2 {
		code = 0x6d
	} else {
		code = 0xad
	}
	prev := byte(len(buf))
	for i, v := range buf {
		x := v ^ magicBytes[i&0xf] ^ byte(i*code)
		buf[i] = x ^ prev
		prev = x
	}
	if r.method != 0 {
		for i := 5; i+1 < len(buf); i += 16 {
			buf[i] ^= 0x0d
			buf[i+1] ^= 0x0d
		}
	}
}
