package codegen

import (
	"go/ast"
	"strings"
)

type runtimeTypeInfo struct {
	pkg  string
	name string
}

var runtimeTypes = map[string]runtimeTypeInfo{
	"java/lang/Object":                    {"lang", "Object"},
	"java/lang/String":                    {"lang", "String"},
	"java/lang/StringBuilder":             {"lang", "StringBuilder"},
	"java/lang/Class":                     {"lang", "Class"},
	"java/lang/Throwable":                 {"lang", "Throwable"},
	"java/lang/Exception":                 {"lang", "Exception"},
	"java/lang/RuntimeException":          {"lang", "RuntimeException"},
	"java/lang/NullPointerException":      {"lang", "NullPointerException"},
	"java/lang/IndexOutOfBoundsException": {"lang", "IndexOutOfBoundsException"},
	"java/lang/ClassCastException":        {"lang", "ClassCastException"},
	"java/lang/Long":                      {"lang", "Long"},
	"java/io/PrintStream":                 {"lang", "PrintStream"},
	"java/io/IOException":                 {"lang", "IOException"},

	// Phase 5: Networking & I/O
	"java/net/Socket":                     {"jnet", "Socket"},
	"java/net/ServerSocket":               {"jnet", "ServerSocket"},
	"java/net/InetSocketAddress":          {"jnet", "InetSocketAddress"},
	"java/net/SocketAddress":              {"jnet", "SocketAddress"},
	"java/net/URI":                        {"jnet", "URI"},
	"java/net/URL":                        {"jnet", "URL"},
	"java/net/HttpURLConnection":          {"jnet", "HttpURLConnection"},
	"com/sun/net/httpserver/HttpServer":   {"jnet", "HttpServer"},
	"com/sun/net/httpserver/HttpExchange": {"jnet", "HttpExchange"},
	"com/sun/net/httpserver/HttpHandler":  {"jnet", "HttpHandler"},
	"com/sun/net/httpserver/Headers":      {"jnet", "Headers"},
	"com/sun/net/httpserver/HttpContext":  {"jnet", "HttpContext"},
	"java/io/InputStream":                 {"jnet", "InputStream"},
	"java/io/OutputStream":                {"jnet", "OutputStream"},
	"java/io/ByteArrayOutputStream":       {"jnet", "ByteArrayOutputStream"},
	"java/io/ByteArrayInputStream":        {"jnet", "ByteArrayInputStream"},
}

func runtimeTypeName(internalName string) (string, bool) {
	info, ok := runtimeTypes[internalName]
	if !ok {
		return "", false
	}
	return info.name, true
}

func runtimeTypePkg(internalName string) string {
	info, ok := runtimeTypes[internalName]
	if !ok {
		return "lang"
	}
	return info.pkg
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
			if info, ok := runtimeTypes[raw]; ok {
				return &ast.StarExpr{X: &ast.SelectorExpr{X: ast.NewIdent(info.pkg), Sel: ast.NewIdent(info.name)}}
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
