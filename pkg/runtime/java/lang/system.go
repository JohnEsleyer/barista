package lang

import (
	"fmt"
	"io"
	"os"
	"time"
)

// PrintStream is a minimal native shim for java.io.PrintStream.
type PrintStream struct {
	Object
	writer io.Writer
}

// SystemOut mirrors System.out.
var SystemOut = &PrintStream{writer: os.Stdout}

// SystemErr mirrors System.err.
var SystemErr = &PrintStream{writer: os.Stderr}

// Println writes all arguments followed by a newline.
func (p *PrintStream) Println(args ...any) {
	writeArgs(p.output(), args, true)
}

// Print writes all arguments without adding a newline.
func (p *PrintStream) Print(args ...any) {
	writeArgs(p.output(), args, false)
}

// Flush is a no-op because the runtime writes directly to the underlying writer.
func (p *PrintStream) Flush() {}

func (p *PrintStream) output() io.Writer {
	if p == nil || p.writer == nil {
		return os.Stdout
	}
	return p.writer
}

func writeArgs(w io.Writer, args []any, newline bool) {
	for i, arg := range args {
		if i > 0 {
			fmt.Fprint(w, " ")
		}
		switch v := arg.(type) {
		case nil:
			fmt.Fprint(w, "null")
		case *String:
			if v == nil {
				fmt.Fprint(w, "null")
			} else {
				fmt.Fprint(w, v.GoString())
			}
		default:
			fmt.Fprint(w, v)
		}
	}
	if newline {
		fmt.Fprintln(w)
	}
}

// CurrentTimeMillis returns the current Unix time in milliseconds.
func CurrentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

// Exit terminates the process with the given JVM status code.
func Exit(status int32) {
	os.Exit(int(status))
}
