package codegen

import (
	"strings"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/ir"
)

const (
	netRuntimeImport    = "github.com/JohnEsleyer/barista/pkg/runtime/java/net"
	kotlinRuntimeImport = "github.com/JohnEsleyer/barista/pkg/runtime/kotlin/jvm"
)

func isRuntimeClass(internalName string) bool {
	_, ok := runtimeTypes[internalName]
	return ok
}

func isNetClass(owner string) bool {
	info, ok := runtimeTypes[owner]
	return ok && info.pkg == "jnet"
}

func runtimeMethodName(owner, name, descriptor string) string {
	if name == "<init>" {
		return "Init"
	}
	switch owner {
	case "java/io/PrintStream":
		switch name {
		case "println":
			return "Println"
		case "print":
			return "Print"
		case "flush":
			return "Flush"
		}
	case "java/lang/System":
		switch name {
		case "currentTimeMillis":
			return "CurrentTimeMillis"
		case "exit":
			return "Exit"
		}
	case "java/lang/String":
		switch name {
		case "length":
			return "Length"
		case "isEmpty":
			return "IsEmpty"
		case "equals":
			return "Equals"
		case "charAt":
			return "CharAt"
		case "substring":
			return "Substring"
		case "concat":
			return "Concat"
		case "hashCode":
			return "HashCode"
		case "toString":
			return "ToString"
		case "getBytes":
			return "GetBytes"
		}
	case "java/lang/Long":
		switch name {
		case "hashCode":
			return "HashCode"
		case "equals":
			return "Equals"
		case "longValue":
			return "LongValue"
		case "toString":
			return "ToString"
		}
	case "java/lang/StringBuilder":
		switch name {
		case "append":
			return stringBuilderAppendName(descriptor)
		case "length":
			return "Length"
		case "toString":
			return "ToString"
		}
	case "java/lang/Throwable", "java/lang/Exception", "java/lang/RuntimeException",
		"java/lang/NullPointerException", "java/lang/IndexOutOfBoundsException",
		"java/lang/ClassCastException", "java/io/IOException":
		switch name {
		case "getMessage":
			return "GetMessage"
		case "toString":
			return "ToString"
		case "printStackTrace":
			return "PrintStackTrace"
		case "initCause":
			return "InitCause"
		}
	case "java/lang/Object":
		switch name {
		case "equals":
			return "Equals"
		case "hashCode":
			return "HashCode"
		case "toString":
			return "ToString"
		}
	case "com/sun/net/httpserver/HttpServer":
		switch name {
		case "create":
			return "HttpServerCreate"
		case "bind":
			return "Bind"
		case "createContext":
			return "CreateContext"
		case "start":
			return "Start"
		case "stop":
			return "Stop"
		case "getAddress":
			return "GetAddress"
		}
	case "com/sun/net/httpserver/HttpExchange":
		switch name {
		case "getRequestMethod":
			return "GetRequestMethod"
		case "getRequestURI":
			return "GetRequestURI"
		case "getRequestHeaders":
			return "GetRequestHeaders"
		case "getResponseHeaders":
			return "GetResponseHeaders"
		case "getRequestBody":
			return "GetRequestBody"
		case "getResponseBody":
			return "GetResponseBody"
		case "sendResponseHeaders":
			return "SendResponseHeaders"
		case "getResponseCode":
			return "GetResponseCode"
		case "close":
			return "Close"
		case "setAttribute":
			return "SetAttribute"
		case "getAttribute":
			return "GetAttribute"
		}
	case "com/sun/net/httpserver/HttpHandler":
		switch name {
		case "handle":
			return "Handle"
		}
	case "com/sun/net/httpserver/Headers":
		switch name {
		case "set":
			return "Set"
		case "add":
			return "Add"
		case "get":
			return "Get"
		case "getFirst":
			return "GetFirst"
		}
	case "java/net/InetSocketAddress":
		switch name {
		case "getPort":
			return "GetPort"
		case "getHostName":
			return "GetHostName"
		case "getAddress":
			return "GetAddress"
		}
	case "java/net/ServerSocket":
		switch name {
		case "accept":
			return "Accept"
		case "close":
			return "Close"
		case "getLocalPort":
			return "GetLocalPort"
		case "isClosed":
			return "IsClosed"
		case "bind":
			return "Bind"
		}
	case "java/net/Socket":
		switch name {
		case "getInputStream":
			return "GetInputStream"
		case "getOutputStream":
			return "GetOutputStream"
		case "close":
			return "Close"
		case "getPort":
			return "GetPort"
		case "getLocalPort":
			return "GetLocalPort"
		case "isClosed":
			return "IsClosed"
		case "isConnected":
			return "IsConnected"
		}
	case "java/net/URI":
		switch name {
		case "create":
			return "URICreate"
		case "getPath":
			return "GetPath"
		case "getQuery":
			return "GetQuery"
		case "getHost":
			return "GetHost"
		case "getPort":
			return "GetPort"
		case "toString":
			return "ToString"
		}
	case "java/net/URL":
		switch name {
		case "openConnection":
			return "OpenConnection"
		case "openStream":
			return "OpenStream"
		case "getPath":
			return "GetPath"
		case "toString":
			return "ToString"
		}
	case "java/net/HttpURLConnection":
		switch name {
		case "setRequestMethod":
			return "SetRequestMethod"
		case "setRequestProperty":
			return "SetRequestProperty"
		case "getResponseCode":
			return "GetResponseCode"
		case "getInputStream":
			return "GetInputStream"
		case "getOutputStream":
			return "GetOutputStream"
		case "disconnect":
			return "Disconnect"
		}
	case "java/io/InputStream", "java/io/ByteArrayInputStream":
		switch name {
		case "read":
			return "Read"
		case "readAllBytes":
			return "ReadAllBytes"
		case "close":
			return "Close"
		}
	case "java/io/OutputStream", "java/io/ByteArrayOutputStream":
		switch name {
		case "write":
			return "Write"
		case "flush":
			return "Flush"
		case "close":
			return "Close"
		case "toByteArray":
			return "ToByteArray"
		case "toString":
			return "ToString"
		case "reset":
			return "Reset"
		case "size":
			return "Size"
		}
	}
	return exportedName(name)
}

func stringBuilderAppendName(descriptor string) string {
	md, err := ir.ParseMethodDescriptor(descriptor)
	if err == nil && len(md.Params) > 0 {
		if md.Params[0] == "Ljava/lang/String;" {
			return "AppendString"
		}
		if md.Params[0] == "I" {
			return "AppendInt"
		}
		if md.Params[0] == "C" {
			return "AppendChar"
		}
	}
	return "AppendAny"
}

func needsLang(cf *classfile.ClassFile, methods []*ir.MethodIR) bool {
	for _, field := range cf.Fields {
		if usesLangDescriptor(field.Descriptor) {
			return true
		}
	}
	for _, mir := range methods {
		if len(mir.ExceptionTraps) > 0 {
			return true
		}
		md, err := ir.ParseMethodDescriptor(mir.Descriptor)
		if err == nil {
			if usesLangDescriptor(md.Return) {
				return true
			}
			for _, param := range md.Params {
				if usesLangDescriptor(param) {
					return true
				}
			}
		}
		for _, inst := range mir.Instructions {
			switch inst.Op {
			case ir.OpConstString, ir.OpConstClass, ir.OpLoadException, ir.OpArrayLoad, ir.OpArrayStore,
				ir.OpCheckCast, ir.OpInstanceOf, ir.OpCmpLong:
				return true
			case ir.OpNewArray, ir.OpANewArray:
				if usesLangDescriptor(inst.Descriptor) {
					return true
				}
			case ir.OpNew:
				if isRuntimeClass(inst.OwnerClass) && runtimeTypePkg(inst.OwnerClass) == "lang" {
					return true
				}
			case ir.OpInvokeVirtual, ir.OpInvokeSpecial:
				if inst.OwnerClass == "java/lang/Object" && inst.MemberName != "<init>" {
					return true
				}
				if inst.OwnerClass != "java/lang/Object" && isRuntimeClass(inst.OwnerClass) && runtimeTypePkg(inst.OwnerClass) == "lang" {
					return true
				}
			case ir.OpInvokeStatic:
				if inst.OwnerClass == "kotlin/io/ConsoleKt" {
					return true
				}
				if inst.OwnerClass != "java/lang/Object" && isRuntimeClass(inst.OwnerClass) && runtimeTypePkg(inst.OwnerClass) == "lang" {
					return true
				}
			case ir.OpGetStatic:
				if inst.OwnerClass == "java/lang/System" {
					return true
				}
			}
		}
	}
	return false
}

func usesLangDescriptor(desc string) bool {
	if desc == "" {
		return false
	}
	if strings.HasPrefix(desc, "[") {
		return usesLangDescriptor(desc[1:])
	}
	if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
		raw := desc[1 : len(desc)-1]
		if raw == "java/lang/Object" {
			return false
		}
		return isRuntimeClass(raw) && runtimeTypePkg(raw) == "lang"
	}
	return false
}

func usesNetDescriptor(desc string) bool {
	if desc == "" {
		return false
	}
	if strings.HasPrefix(desc, "[") {
		return usesNetDescriptor(desc[1:])
	}
	if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
		raw := desc[1 : len(desc)-1]
		return isNetClass(raw)
	}
	return false
}

func needsNet(methods []*ir.MethodIR) bool {
	for _, mir := range methods {
		md, err := ir.ParseMethodDescriptor(mir.Descriptor)
		if err == nil {
			if usesNetDescriptor(md.Return) {
				return true
			}
			for _, param := range md.Params {
				if usesNetDescriptor(param) {
					return true
				}
			}
		}
		for _, inst := range mir.Instructions {
			if usesNetDescriptor(inst.Descriptor) {
				return true
			}
			if isNetClass(inst.OwnerClass) {
				return true
			}
		}
	}
	return false
}

func isKotlinIntrinsicClass(owner string) bool {
	return owner == "kotlin/jvm/internal/Intrinsics"
}

func kotlinIntrinsicName(name string) (string, bool) {
	switch name {
	case "checkNotNullParameter", "checkNotNullExpressionValue":
		return "CheckNotNullParameter", true
	case "checkNotNull":
		return "CheckNotNull", true
	case "areEqual":
		return "AreEqual", true
	case "throwUninitializedPropertyAccessException":
		return "ThrowUninitializedPropertyAccessException", true
	case "throwParameterIsNullException":
		return "ThrowParameterIsNullException", true
	case "stringPlus":
		return "StringPlus", true
	default:
		return "", false
	}
}

func needsKotlin(methods []*ir.MethodIR) bool {
	for _, mir := range methods {
		for _, inst := range mir.Instructions {
			if inst.Op == ir.OpInvokeStatic && isKotlinIntrinsicClass(inst.OwnerClass) {
				if _, ok := kotlinIntrinsicName(inst.MemberName); ok {
					return true
				}
			}
		}
	}
	return false
}
