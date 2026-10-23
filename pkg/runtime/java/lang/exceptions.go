package lang

import "fmt"

// Throwable is the native shim for java.lang.Throwable.
type Throwable struct {
	Object
	Message *String
	Cause   *Throwable
}

// NewThrowable constructs a Throwable with the given message.
func NewThrowable(message *String) *Throwable {
	return &Throwable{Message: message}
}

// Init implements the JVM <init>(String) and <init>() calls.
func (t *Throwable) Init(args ...*String) {
	if t == nil {
		return
	}
	if len(args) > 0 {
		t.Message = args[0]
	}
}

// GetMessage returns the detail message.
func (t *Throwable) GetMessage() *String {
	if t == nil {
		return nil
	}
	return t.Message
}

// InitCause sets and returns the cause.
func (t *Throwable) InitCause(cause *Throwable) *Throwable {
	if t != nil {
		t.Cause = cause
	}
	return t
}

// ToString renders the exception in a Java-like form.
func (t *Throwable) ToString() *String {
	if t == nil {
		return NewString("java.lang.Throwable")
	}
	name := throwableClassName(t)
	if t.Message == nil {
		return NewString(name)
	}
	return NewString(name + ": " + t.Message.GoString())
}

// Error lets Throwable satisfy error.
func (t *Throwable) Error() string {
	return t.ToString().GoString()
}

// String lets Throwable satisfy fmt.Stringer.
func (t *Throwable) String() string {
	return t.Error()
}

// PrintStackTrace writes a compact stack trace to standard error.
func (t *Throwable) PrintStackTrace() {
	fmt.Println(t.Error())
}

// Exception is the native shim for java.lang.Exception.
type Exception struct {
	Throwable
}

// NewException constructs an Exception with the given message.
func NewException(message *String) *Exception {
	return &Exception{Throwable: Throwable{Message: message}}
}

// RuntimeException is the native shim for java.lang.RuntimeException.
type RuntimeException struct {
	Exception
}

// NewRuntimeException constructs a RuntimeException with the given message.
func NewRuntimeException(message *String) *RuntimeException {
	return &RuntimeException{Exception: Exception{Throwable: Throwable{Message: message}}}
}

// NullPointerException is the native shim for
// java.lang.NullPointerException.
type NullPointerException struct {
	RuntimeException
}

// NewNullPointerException constructs a NullPointerException.
func NewNullPointerException(message *String) *NullPointerException {
	return &NullPointerException{RuntimeException: RuntimeException{Exception: Exception{Throwable: Throwable{Message: message}}}}
}

// IndexOutOfBoundsException is the native shim for
// java.lang.IndexOutOfBoundsException.
type IndexOutOfBoundsException struct {
	RuntimeException
}

// NewIndexOutOfBoundsException constructs an IndexOutOfBoundsException.
func NewIndexOutOfBoundsException(message *String) *IndexOutOfBoundsException {
	return &IndexOutOfBoundsException{RuntimeException: RuntimeException{Exception: Exception{Throwable: Throwable{Message: message}}}}
}

func throwableClassName(t *Throwable) string {
	if t == nil {
		return "java.lang.Throwable"
	}
	return "java.lang.Throwable"
}

// AsThrowable converts any runtime exception value into the common Throwable
// base. It accepts only Barista runtime exceptions.
func AsThrowable(value any) *Throwable {
	switch v := value.(type) {
	case nil:
		return nil
	case *Throwable:
		return v
	case *Exception:
		return &v.Throwable
	case *RuntimeException:
		return &v.Exception.Throwable
	case *NullPointerException:
		return &v.RuntimeException.Exception.Throwable
	case *IndexOutOfBoundsException:
		return &v.RuntimeException.Exception.Throwable
	default:
		return nil
	}
}

// MatchesException reports whether value is assignable to the named JVM catch
// class. The standard java.lang hierarchy is supported.
func MatchesException(value any, className string) bool {
	if value == nil {
		return false
	}
	switch className {
	case "", "java/lang/Throwable":
		return AsThrowable(value) != nil
	case "java/lang/Exception":
		switch value.(type) {
		case *Exception, *RuntimeException, *NullPointerException, *IndexOutOfBoundsException:
			return true
		default:
			return false
		}
	case "java/lang/RuntimeException":
		switch value.(type) {
		case *RuntimeException, *NullPointerException, *IndexOutOfBoundsException:
			return true
		default:
			return false
		}
	case "java/lang/NullPointerException":
		_, ok := value.(*NullPointerException)
		return ok
	case "java/lang/IndexOutOfBoundsException":
		_, ok := value.(*IndexOutOfBoundsException)
		return ok
	default:
		return false
	}
}
