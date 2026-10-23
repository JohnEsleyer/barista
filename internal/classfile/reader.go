package classfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	ErrUnexpectedEOF = errors.New("unexpected end of classfile stream")
	ErrBadMagic      = errors.New("invalid class file magic number (must be 0xCAFEBABE)")
)

// Reader is a bounds-checked, big-endian binary stream reader.
type Reader struct {
	data   []byte
	offset int
}

// NewReader wraps a raw byte slice.
func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

// Offset returns the current read position in bytes.
func (r *Reader) Offset() int { return r.offset }

// Remaining returns how many bytes are left unread.
func (r *Reader) Remaining() int { return len(r.data) - r.offset }

// ReadU1 reads one unsigned byte.
func (r *Reader) ReadU1() (uint8, error) {
	if r.offset >= len(r.data) {
		return 0, ErrUnexpectedEOF
	}
	b := r.data[r.offset]
	r.offset++
	return b, nil
}

// ReadU2 reads one big-endian unsigned 16-bit integer.
func (r *Reader) ReadU2() (uint16, error) {
	if r.offset+2 > len(r.data) {
		return 0, ErrUnexpectedEOF
	}
	v := binary.BigEndian.Uint16(r.data[r.offset : r.offset+2])
	r.offset += 2
	return v, nil
}

// ReadU4 reads one big-endian unsigned 32-bit integer.
func (r *Reader) ReadU4() (uint32, error) {
	if r.offset+4 > len(r.data) {
		return 0, ErrUnexpectedEOF
	}
	v := binary.BigEndian.Uint32(r.data[r.offset : r.offset+4])
	r.offset += 4
	return v, nil
}

// ReadBytes reads exactly n bytes and returns a copy.
func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if n < 0 || r.offset+n > len(r.data) {
		return nil, ErrUnexpectedEOF
	}
	out := make([]byte, n)
	copy(out, r.data[r.offset:r.offset+n])
	r.offset += n
	return out, nil
}

// Skip advances the reader by n bytes.
func (r *Reader) Skip(n int) error {
	if n < 0 || r.offset+n > len(r.data) {
		return ErrUnexpectedEOF
	}
	r.offset += n
	return nil
}

// ReadAll reads an entire io.Reader into a Reader instance.
func ReadAll(r io.Reader) (*Reader, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read class file: %w", err)
	}
	return NewReader(buf), nil
}
