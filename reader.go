package ggpack

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
)

type ValueType byte

const (
	NullType    ValueType = 1
	HashType    ValueType = 2
	ArrayType   ValueType = 3
	StringType  ValueType = 4
	IntegerType ValueType = 5
	DoubleType  ValueType = 6
)

type Value struct {
	typ ValueType

	str     string
	integer int
	double  float64
	//hash    map[string]*Value
	array []*Value
}

var Null = Value{typ: NullType}

func (v *Value) GetType() ValueType { return v.typ }

func (vt ValueType) isNull() bool    { return vt == NullType }
func (vt ValueType) isHash() bool    { return vt == HashType }
func (vt ValueType) isString() bool  { return vt == StringType }
func (vt ValueType) isInteger() bool { return vt == IntegerType }
func (vt ValueType) isDouble() bool  { return vt == DoubleType }
func (vt ValueType) isArray() bool   { return vt == ArrayType }

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
		return "unknown"
	}
}

type Reader struct {
	Reader  io.ReadSeeker
	method  int
	offsets []int32
	entries []*Value
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

	log.Printf("offset: %d size: %d\n", offset, size)

	buf := make([]byte, size)

	var sign uint32

	for r.method = 3; r.method >= 0; r.method-- {
		if _, err := r.Reader.Seek(int64(offset), io.SeekStart); err != nil {
			return err
		}

		if _, err := io.ReadFull(r.Reader, buf); err != nil {
			return err
		}
		r.decodeXOR(buf)
		if sign = binary.LittleEndian.Uint32(buf[:4]); sign == 0x04030201 {
			log.Printf("using method: %d\n", r.method)
			goto supported
		}
	}

	return errors.New("unsuported package version")

supported:

	if err := r.readOffsets(buf); err != nil {
		return err
	}

	r.clearEntries()

	entries, err := r.readHash(buf)
	if err != nil {
		return err
	}

	_ = entries

	return nil
}

func (r *Reader) readHash(buf []byte) (*Value, error) {

	if len(buf) < 12 {
		return nil, errors.New("directory too short")
	}

	value := Value{typ: HashType}

	if err := value.readHash(buf[12:], r.offsets); err != nil {
		return nil, err
	}

	return &value, nil
}

func (v *Value) readHash(buf []byte, offsets []int32) error {
	if ValueType(buf[0]) != HashType {
		return errors.New("trying to parse non-hash")
	}
	numEntries := int32(binary.LittleEndian.Uint32(buf[1:]))
	log.Printf("num entries: %d\n", numEntries)

	if numEntries == 0 {
		return errors.New("empty hash")
	}

	buf = buf[1+4:]

	for i := int32(0); i < numEntries; i++ {
		keyIdx := int32(binary.LittleEndian.Uint32(buf))

		keyOfs := offsets[keyIdx]

		key := readString(buf[keyOfs:])

		log.Printf("key: '%s'\n", key)

		buf = buf[4:]
		// TODO: Read value
	}
	return nil
}

func readString(buf []byte) string {

	end := 0

	for len(buf) > end && buf[end] != 0 {
		end++
	}

	return string(buf[:end])
}

func (r *Reader) clearEntries() {
	for i := range r.entries {
		r.entries[i] = nil
	}
	r.entries = r.entries[:0]
}

func (r *Reader) readOffsets(buf []byte) error {
	if len(buf) < 12 {
		return errors.New("directory too short")
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
	log.Printf("num offsets: %d\n", len(r.offsets))
	return nil
}

func (r *Reader) decodeXOR(buf []byte) {

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
