package jnet

import (
	"bytes"
	"io"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

// InputStream is the native shim for java.io.InputStream.
type InputStream struct {
	lang.Object
	reader io.Reader
	closer io.Closer
}

// NewInputStream creates an InputStream wrapping a Go reader.
func NewInputStream(r io.Reader) *InputStream {
	var c io.Closer
	if closer, ok := r.(io.Closer); ok {
		c = closer
	}
	return &InputStream{reader: r, closer: c}
}

// Read implements InputStream.read(), read(byte[]), and read(byte[], int, int).
func (in *InputStream) Read(args ...any) int32 {
	if in == nil || in.reader == nil {
		return -1
	}
	if len(args) == 0 {
		var buf [1]byte
		n, err := in.reader.Read(buf[:])
		if n == 0 || err != nil {
			return -1
		}
		return int32(buf[0])
	}
	if len(args) == 1 {
		buf, ok := args[0].([]int8)
		if !ok {
			return -1
		}
		goBuf := make([]byte, len(buf))
		n, err := in.reader.Read(goBuf)
		if n > 0 {
			for i := 0; i < n; i++ {
				buf[i] = int8(goBuf[i])
			}
		}
		if n == 0 && err != nil {
			return -1
		}
		return int32(n)
	}
	if len(args) >= 3 {
		buf, ok := args[0].([]int8)
		off := args[1].(int32)
		length := args[2].(int32)
		if !ok || off < 0 || length < 0 || int(off+length) > len(buf) {
			return -1
		}
		goBuf := make([]byte, length)
		n, err := in.reader.Read(goBuf)
		if n > 0 {
			for i := 0; i < n; i++ {
				buf[int(off)+i] = int8(goBuf[i])
			}
		}
		if n == 0 && err != nil {
			return -1
		}
		return int32(n)
	}
	return -1
}

// ReadAllBytes implements InputStream.readAllBytes() added in Java 9.
func (in *InputStream) ReadAllBytes() []int8 {
	if in == nil || in.reader == nil {
		return []int8{}
	}
	data, err := io.ReadAll(in.reader)
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	out := make([]int8, len(data))
	for i, b := range data {
		out[i] = int8(b)
	}
	return out
}

// Close closes the underlying reader if it implements io.Closer.
func (in *InputStream) Close() {
	if in != nil && in.closer != nil {
		_ = in.closer.Close()
	}
}

// OutputStream is the native shim for java.io.OutputStream.
type OutputStream struct {
	lang.Object
	writer io.Writer
	closer io.Closer
}

// NewOutputStream creates an OutputStream wrapping a Go writer.
func NewOutputStream(w io.Writer) *OutputStream {
	var c io.Closer
	if closer, ok := w.(io.Closer); ok {
		c = closer
	}
	return &OutputStream{writer: w, closer: c}
}

// Write implements OutputStream.write(int), write(byte[]), and write(byte[], int, int).
func (out *OutputStream) Write(args ...any) {
	if out == nil || out.writer == nil || len(args) == 0 {
		return
	}
	switch v := args[0].(type) {
	case int32:
		_, err := out.writer.Write([]byte{byte(v)})
		if err != nil {
			panic(lang.NewIOException(lang.NewString(err.Error())))
		}
	case []int8:
		if len(args) >= 3 {
			off := args[1].(int32)
			length := args[2].(int32)
			if off >= 0 && length >= 0 && int(off+length) <= len(v) {
				buf := make([]byte, length)
				for i := 0; i < int(length); i++ {
					buf[i] = byte(v[int(off)+i])
				}
				_, err := out.writer.Write(buf)
				if err != nil {
					panic(lang.NewIOException(lang.NewString(err.Error())))
				}
			}
		} else {
			buf := make([]byte, len(v))
			for i, b := range v {
				buf[i] = byte(b)
			}
			_, err := out.writer.Write(buf)
			if err != nil {
				panic(lang.NewIOException(lang.NewString(err.Error())))
			}
		}
	case []byte:
		_, err := out.writer.Write(v)
		if err != nil {
			panic(lang.NewIOException(lang.NewString(err.Error())))
		}
	}
}

// Flush flushes any buffered data.
func (out *OutputStream) Flush() {}

// Close closes the underlying writer if it implements io.Closer.
func (out *OutputStream) Close() {
	if out != nil && out.closer != nil {
		_ = out.closer.Close()
	}
}

// ByteArrayOutputStream is the native shim for java.io.ByteArrayOutputStream.
type ByteArrayOutputStream struct {
	OutputStream
	buf bytes.Buffer
}

// Init initializes the ByteArrayOutputStream.
func (b *ByteArrayOutputStream) Init(args ...any) *ByteArrayOutputStream {
	if b == nil {
		return b
	}
	b.buf.Reset()
	b.writer = &b.buf
	return b
}

// ToByteArray returns the buffered contents as []int8.
func (b *ByteArrayOutputStream) ToByteArray() []int8 {
	if b == nil {
		return []int8{}
	}
	raw := b.buf.Bytes()
	out := make([]int8, len(raw))
	for i, v := range raw {
		out[i] = int8(v)
	}
	return out
}

// ToString returns the buffered contents as a lang.String.
func (b *ByteArrayOutputStream) ToString() *lang.String {
	if b == nil {
		return lang.NewString("")
	}
	return lang.NewString(b.buf.String())
}

// Reset resets the buffer.
func (b *ByteArrayOutputStream) Reset() {
	if b != nil {
		b.buf.Reset()
	}
}

// Size returns the count of bytes in the buffer.
func (b *ByteArrayOutputStream) Size() int32 {
	if b == nil {
		return 0
	}
	return int32(b.buf.Len())
}

// ByteArrayInputStream is the native shim for java.io.ByteArrayInputStream.
type ByteArrayInputStream struct {
	InputStream
	raw []byte
}

// Init initializes the stream with the provided []int8 array.
func (b *ByteArrayInputStream) Init(args ...any) *ByteArrayInputStream {
	if b == nil || len(args) == 0 {
		return b
	}
	if data, ok := args[0].([]int8); ok {
		buf := make([]byte, len(data))
		for i, v := range data {
			buf[i] = byte(v)
		}
		b.raw = buf
		b.reader = bytes.NewReader(buf)
	}
	return b
}
