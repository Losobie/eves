// Package bluemarshal reads EVE's blue.Marshal settings format without executing
// serialized objects. Adapted from TrueBrain/blue-marshal-rs, commit fa99764271f5a70a9565ab6e6f092f57617382a8.
// See LICENSE for the upstream MIT notices. This package deliberately has no encoder.
package bluemarshal

import (
	"encoding/binary"
	"fmt"
	"hash/adler32"
	"math"
	"math/big"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

const MaxFileSize = 64 << 20
const maxNodes = 1_000_000

// Value preserves the distinctions needed to inspect settings (in particular
// tuples versus lists, and byte strings versus Unicode). Do not use it to write files.
type Value struct {
	Kind  byte
	Int   int64
	Float float64
	Text  string
	Items []*Value
	Pairs []Pair
}

type Pair struct{ Key, Value *Value }

func (v *Value) StringValue() (string, bool) {
	if v == nil {
		return "", false
	}
	return v.Text, v.Kind == TY_BUFFER || v.Kind == TY_UTF8
}

func (v *Value) Integer() (int64, bool) {
	if v == nil {
		return 0, false
	}
	if v.Kind == TY_INT64 {
		return v.Int, true
	}
	if v.Kind == TY_LONG {
		n, err := strconv.ParseInt(v.Text, 10, 64)
		return n, err == nil
	}
	return 0, false
}

func (v *Value) Number() (float64, bool) {
	if v != nil && v.Kind == TY_FLOAT {
		return v.Float, true
	}
	n, ok := v.Integer()
	return float64(n), ok
}

func (v *Value) Field(key string) (*Value, bool) {
	if v == nil || v.Kind != TY_DICT {
		return nil, false
	}
	for _, pair := range v.Pairs {
		if text, ok := pair.Key.StringValue(); ok && text == key {
			return pair.Value, true
		}
	}
	return nil, false
}

type reader struct {
	buf                            []byte
	pos, end, version, seen, nodes int
	mapping                        []int
	shared                         []*Value
	err                            error
	crcPos                         int
	crc                            uint32
	hasCRC                         bool
}

func Decode(data []byte) (*Value, error) {
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("marshal file exceeds %d bytes", MaxFileSize)
	}
	r := &reader{buf: data, end: len(data)}
	switch r.byte() {
	case TY_SIGNATURE:
	case TY_SIGNATURE2:
		r.version = int(r.byte())
	default:
		r.fail("missing marshal signature")
	}
	if r.version > 1 {
		r.fail("unsupported marshal version %d", r.version)
	}
	if r.version == 0 && r.err == nil {
		count := int(int32(r.uint(4)))
		if count < 0 || count > (r.end-r.pos)/4 || count > maxNodes {
			r.fail("invalid shared-object table size")
		} else {
			r.end -= count * 4
			r.mapping = make([]int, count)
			r.shared = make([]*Value, count)
			for i := range r.mapping {
				n := int(int32(binary.LittleEndian.Uint32(data[r.end+i*4:])))
				if n < 1 || n > count {
					r.fail("invalid shared-object mapping")
				}
				r.mapping[i] = n - 1
			}
		}
	}
	v := r.object(0)
	if r.err == nil && r.pos != r.end {
		r.fail("trailing data after marshal object")
	}
	if r.err == nil && r.hasCRC && r.version > 0 {
		r.verifyCRC(r.pos)
	}
	if r.err != nil {
		return nil, r.err
	}
	return v, nil
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("marshal at byte %d: %s", r.pos, fmt.Sprintf(format, args...))
	}
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > r.end-r.pos {
		r.fail("truncated data")
		return nil
	}
	b := r.buf[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *reader) uint(n int) uint64 {
	b := r.take(n)
	var v uint64
	for i, x := range b {
		v |= uint64(x) << (8 * i)
	}
	return v
}
func (r *reader) byte() byte { return byte(r.uint(1)) }
func (r *reader) length() int {
	n := int(r.byte())
	if n == 255 {
		n = int(int32(r.uint(4)))
	}
	if n < 0 {
		r.fail("negative length")
		return 0
	}
	return n
}
func (r *reader) buffer() []byte { return r.take(r.length()) }

func (r *reader) verifyCRC(end int) {
	if r.crcPos > end || adler32.Checksum(r.buf[r.crcPos:end]) != r.crc {
		r.fail("checksum mismatch")
	}
}

func (r *reader) slot() int {
	if r.version == 0 {
		if r.seen >= len(r.mapping) {
			r.fail("shared-object table overflow")
			return -1
		}
		i := r.mapping[r.seen]
		r.seen++
		return i
	}
	r.shared = append(r.shared, nil)
	return len(r.shared) - 1
}

func (r *reader) object(depth int) *Value {
	if r.err != nil {
		return nil
	}
	if depth > 256 || r.nodes >= maxNodes {
		r.fail("nesting or object limit exceeded")
		return nil
	}
	r.nodes++
	raw := r.byte()
	tag := raw & TY_TYPEMASK
	v := &Value{Kind: tag}
	// Only these wire types allocate reference slots, even if another tag has
	// the shared bit set. Containers reserve theirs before reading children.
	slot := -1
	if raw&TY_SHAREDFLAG != 0 {
		switch tag {
		case TY_LONG, TY_STR, TY_BUFFER, TY_GLOBAL, TY_TUPLE1, TY_TUPLE2, TY_TUPLE, TY_LIST1, TY_LIST, TY_DICT, TY_INSTANCE, TY_REDUCE, TY_NEWOBJ:
			slot = r.slot()
		}
	}
	switch tag {
	case TY_NONE, TY_TRUE, TY_FALSE:
	case TY_INT_N1:
		v.Kind, v.Int = TY_INT64, -1
	case TY_INT_0:
		v.Kind = TY_INT64
	case TY_INT_1:
		v.Kind, v.Int = TY_INT64, 1
	case TY_INT8:
		v.Kind, v.Int = TY_INT64, int64(int8(r.byte()))
	case TY_INT16:
		v.Kind, v.Int = TY_INT64, int64(int16(r.uint(2)))
	case TY_INT32:
		v.Kind, v.Int = TY_INT64, int64(int32(r.uint(4)))
	case TY_INT64:
		v.Int = int64(r.uint(8))
	case TY_FLOAT_0:
		v.Kind = TY_FLOAT
	case TY_FLOAT:
		v.Float = math.Float64frombits(r.uint(8))
	case TY_LONG:
		b := r.buffer()
		reversed := make([]byte, len(b))
		for i := range b {
			reversed[len(b)-1-i] = b[i]
		}
		n := new(big.Int).SetBytes(reversed)
		if len(b) > 0 && b[len(b)-1]&128 != 0 {
			n.Sub(n, new(big.Int).Lsh(big.NewInt(1), uint(len(b)*8)))
		}
		v.Text = n.String()
	case TY_STR_EMPTY:
		v.Kind = TY_BUFFER
	case TY_STR_CHAR:
		v.Kind, v.Text = TY_BUFFER, string(r.take(1))
	case TY_STR_SHORT:
		v.Kind, v.Text = TY_BUFFER, string(r.take(int(r.byte())))
	case TY_STR_TABLE:
		i := int(r.byte())
		v.Kind = TY_BUFFER
		if i < 1 || i > len(stringTable) {
			r.fail("invalid string-table index %d", i)
		} else {
			v.Text = stringTable[i-1]
		}
	case TY_STR, TY_BUFFER:
		v.Kind, v.Text = TY_BUFFER, string(r.buffer())
	case TY_UTF8, TY_GLOBAL:
		v.Text = string(r.buffer())
		if !utf8.ValidString(v.Text) {
			r.fail("invalid UTF-8")
		}
	case TY_UNICODE_0:
		v.Kind = TY_UTF8
	case TY_UNICODE, TY_UNICODE_1:
		n := 1
		if tag == TY_UNICODE {
			n = r.length()
		}
		if n > (r.end-r.pos)/2 {
			r.fail("truncated UTF-16")
		}
		b := r.take(n * 2)
		units := make([]uint16, len(b)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		for i := 0; i < len(units); i++ {
			if units[i] >= 0xD800 && units[i] <= 0xDBFF {
				if i+1 >= len(units) || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
					r.fail("invalid UTF-16")
					break
				}
				i++
			} else if units[i] >= 0xDC00 && units[i] <= 0xDFFF {
				r.fail("invalid UTF-16")
				break
			}
		}
		v.Kind, v.Text = TY_UTF8, string(utf16.Decode(units))
	case TY_TUPLE0, TY_TUPLE1, TY_TUPLE2, TY_TUPLE, TY_LIST0, TY_LIST1, TY_LIST:
		n := 0
		switch tag {
		case TY_TUPLE1, TY_LIST1:
			n = 1
		case TY_TUPLE2:
			n = 2
		case TY_TUPLE, TY_LIST:
			n = r.length()
		}
		v.Kind = TY_TUPLE
		if tag == TY_LIST0 || tag == TY_LIST1 || tag == TY_LIST {
			v.Kind = TY_LIST
		}
		if n > r.end-r.pos || n > maxNodes-r.nodes {
			r.fail("invalid container size")
		}
		for i := 0; i < n && r.err == nil; i++ {
			v.Items = append(v.Items, r.object(depth+1))
		}
	case TY_DICT:
		n := r.length()
		if n > (r.end-r.pos)/2 || n > (maxNodes-r.nodes)/2 {
			r.fail("invalid dictionary size")
		}
		for i := 0; i < n && r.err == nil; i++ {
			value := r.object(depth + 1)
			key := r.object(depth + 1) // Wire order is value, then key.
			v.Pairs = append(v.Pairs, Pair{key, value})
		}
	case TY_INSTANCE:
		v.Items = append(v.Items, r.object(depth+1), r.object(depth+1))
		if _, ok := v.Items[0].StringValue(); !ok {
			r.fail("instance class is not a string")
		}
	case TY_CALLBACK:
		v.Items = append(v.Items, r.object(depth+1))
	case TY_REDUCE, TY_NEWOBJ:
		payload := r.object(depth + 1)
		if payload == nil || payload.Kind != TY_TUPLE {
			r.fail("invalid object payload")
		} else {
			minimum := 2
			if tag == TY_NEWOBJ {
				minimum = 1
			}
			if len(payload.Items) < minimum {
				r.fail("short object payload")
			}
		}
		v.Items = append(v.Items, payload)
		for !r.mark() && r.err == nil {
			v.Items = append(v.Items, r.object(depth+1))
		}
		for !r.mark() && r.err == nil {
			key := r.object(depth + 1)
			value := r.object(depth + 1)
			v.Pairs = append(v.Pairs, Pair{key, value})
		}
	case TY_REFERENCE:
		i := r.length()
		if r.version == 0 {
			i--
		}
		if i < 0 || i >= len(r.shared) || r.shared[i] == nil {
			r.fail("invalid or cyclic reference %d", i)
		} else {
			v = r.shared[i]
		}
	case TY_CRC_CHECK:
		if r.hasCRC {
			r.fail("multiple checksum markers")
		}
		r.crc = uint32(r.uint(4))
		r.crcPos = r.pos
		r.hasCRC = true
		if r.version == 0 && r.err == nil {
			r.verifyCRC(len(r.buf))
		}
		v = r.object(depth + 1)
	case TY_DBROW, TY_WSTREAM, TY_PICKLE, TY_PICKLER, TY_COMPLEX:
		r.fail("unsupported type tag %d", tag)
	default:
		r.fail("unknown type tag %d", tag)
	}
	if slot >= 0 && r.err == nil {
		r.shared[slot] = v
	}
	return v
}

func (r *reader) mark() bool {
	if r.err != nil {
		return true
	}
	if r.pos >= r.end {
		r.fail("missing iterator terminator")
		return true
	}
	if r.buf[r.pos]&TY_TYPEMASK == TY_MARK {
		r.pos++
		return true
	}
	return false
}
