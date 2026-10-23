package codegen

import (
	"go/ast"
	"strings"
)

var runtimeTypeNames = map[string]string{
	"java/lang/Object":                    "Object",
	"java/lang/String":                    "String",
	"java/lang/StringBuilder":             "StringBuilder",
	"java/lang/Class":                     "Class",
	"java/lang/Throwable":                 "Throwable",
	"java/lang/Exception":                 "Exception",
	"java/lang/RuntimeException":          "RuntimeException",
	"java/lang/NullPointerException":      "NullPointerException",
	"java/lang/IndexOutOfBoundsException": "IndexOutOfBoundsException",
	"java/lang/ClassCastException":        "ClassCastException",
	"java/lang/Long":                      "Long",
	"java/io/PrintStream":                 "PrintStream",
}

func runtimeTypeName(internalName string) (string, bool) {
	name, ok := runtimeTypeNames[internalName]
	return name, ok
}

func langType(name string) ast.Expr {
	return &ast.StarExpr{X: &ast.SelectorExpr{X: ast.NewIdent("lang"), Sel: ast.NewIdent(name)}}
}

// DescriptorToGoType converts one JVM field descriptor to a Go AST type.
func DescriptorToGoType(desc string) ast.Expr {
	switch desc {
	case "Z":
		return ast.NewIdent("bool")
	case "B":
		return ast.NewIdent("int8")
	case "C":
		return ast.NewIdent("uint16")
	case "S":
		return ast.NewIdent("int16")
	case "I":
		return ast.NewIdent("int32")
	case "J":
		return ast.NewIdent("int64")
	case "F":
		return ast.NewIdent("float32")
	case "D":
		return ast.NewIdent("float64")
	case "V":
		return nil
	case "Ljava/lang/Object;", "Lkotlin/jvm/internal/DefaultConstructorMarker;":
		return ast.NewIdent("any")
	default:
		if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
			raw := desc[1 : len(desc)-1]
			if name, ok := runtimeTypeName(raw); ok {
				return langType(name)
			}
			return &ast.StarExpr{X: ast.NewIdent(CleanClassName(raw))}
		}
		if strings.HasPrefix(desc, "[") {
			return &ast.ArrayType{Elt: DescriptorToGoType(desc[1:])}
		}
		return ast.NewIdent("any")
	}
}

func CleanClassName(internalName string) string {
	if internalName == "" {
		return "Class"
	}
	internalName = strings.ReplaceAll(internalName, "$", "_")
	parts := strings.Split(internalName, "/")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(exportedName(part))
	}
	name := b.String()
	if name == "" {
		return "Class"
	}
	return name
}

func CleanMemberName(name string) string {
	return exportedName(name)
}

func staticFieldName(owner, name string) string {
	field := exportedName(name)
	if owner == "" {
		return field
	}
	return CleanClassName(owner) + field
}
