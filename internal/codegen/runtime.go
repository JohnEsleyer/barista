package codegen

import (
	"strings"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/ir"
)

func isRuntimeClass(internalName string) bool {
	_, ok := runtimeTypeName(internalName)
	return ok
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
		"java/lang/ClassCastException":
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
				if isRuntimeClass(inst.OwnerClass) {
					return true
				}
			case ir.OpInvokeVirtual, ir.OpInvokeSpecial:
				if inst.OwnerClass == "java/lang/Object" && inst.MemberName != "<init>" {
					return true
				}
				if inst.OwnerClass != "java/lang/Object" && isRuntimeClass(inst.OwnerClass) {
					return true
				}
			case ir.OpInvokeStatic:
				if inst.OwnerClass == "kotlin/io/ConsoleKt" {
					return true
				}
				if inst.OwnerClass != "java/lang/Object" && isRuntimeClass(inst.OwnerClass) {
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
		return isRuntimeClass(raw)
	}
	return false
}

const kotlinRuntimeImport = "github.com/JohnEsleyer/barista/pkg/runtime/kotlin/jvm"

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
