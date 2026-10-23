package codegen

import (
	"go/ast"
	"strings"
)

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
	default:
		if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
			raw := desc[1 : len(desc)-1]
			if raw == "java/lang/String" {
				return &ast.StarExpr{X: ast.NewIdent("JavaLangString")}
			}
			return &ast.StarExpr{X: ast.NewIdent(CleanClassName(raw))}
		}
		if strings.HasPrefix(desc, "[") {
			return &ast.ArrayType{Elt: DescriptorToGoType(desc[1:])}
		}
		return ast.NewIdent("any")
	}
}

// CleanClassName turns the JVM internal name into an exported Go identifier.
// For example "com/example/User" becomes "ComExampleUser".
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

// CleanMemberName turns a JVM field or method name into an exported Go name.
func CleanMemberName(name string) string {
	return exportedName(name)
}

// staticFieldName creates a package-level name for a JVM static field. The
// class prefix prevents collisions with generated functions and instance
// fields, which live in different Go namespaces.
func staticFieldName(owner, name string) string {
	field := exportedName(name)
	if owner == "" {
		return field
	}
	return CleanClassName(owner) + field
}
