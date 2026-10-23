package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/ir"
)

const langRuntimeImport = "github.com/JohnEsleyer/barista/pkg/runtime/java/lang"

// GenerateGo translates a parsed ClassFile and lowered IR into clean Go code.
func GenerateGo(cf *classfile.ClassFile, methods []*ir.MethodIR) ([]byte, error) {
	return GenerateGoWithPackage(cf, methods, packageName(cf.ThisClass))
}

// GenerateGoWithPackage translates a parsed ClassFile and lowered IR into clean Go code
// with a specified Go package name.
func GenerateGoWithPackage(cf *classfile.ClassFile, methods []*ir.MethodIR, pkgName string) ([]byte, error) {
	file := &ast.File{
		Name:  ast.NewIdent(pkgName),
		Decls: make([]ast.Decl, 0),
	}

	importSpecs := make([]*ast.ImportSpec, 0, 3)
	if needsLang(cf, methods) {
		importSpecs = append(importSpecs, &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(langRuntimeImport),
			},
		})
	}
	if needsNet(methods) {
		importSpecs = append(importSpecs, &ast.ImportSpec{
			Name: ast.NewIdent("jnet"),
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(netRuntimeImport),
			},
		})
	}
	if needsKotlin(methods) {
		importSpecs = append(importSpecs, &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(kotlinRuntimeImport),
			},
		})
	}
	if len(importSpecs) > 0 {
		specs := make([]ast.Spec, len(importSpecs))
		for i, spec := range importSpecs {
			specs[i] = spec
		}
		file.Imports = importSpecs
		file.Decls = append(file.Decls, &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: specs,
		})
	}

	file.Decls = append(file.Decls, buildClassStruct(cf))

	for _, field := range cf.Fields {
		if field.IsStatic() {
			file.Decls = append(file.Decls, buildStaticFieldDecl(cf, field))
		}
	}

	usedNames := make(map[string]bool)
	methodNames := make(map[string]string)
	for _, mir := range methods {
		owner := mir.OwnerClass
		if owner == "" {
			owner = cf.ThisClass
		}
		name := methodGoName(mir.Name)
		if name != "init" {
			if usedNames[name] {
				name = uniqueName(name, mir.Descriptor, usedNames)
			}
			usedNames[name] = true
		}
		methodNames[methodKey(owner, mir.Name, mir.Descriptor)] = name
	}

	for _, mir := range methods {
		owner := mir.OwnerClass
		if owner == "" {
			owner = cf.ThisClass
		}
		name := methodNames[methodKey(owner, mir.Name, mir.Descriptor)]
		fnDecl, err := buildFuncDecl(cf, mir, name, methodNames)
		if err != nil {
			return nil, err
		}
		file.Decls = append(file.Decls, fnDecl)
	}

	fset := token.NewFileSet()
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("failed to format generated Go source: %w", err)
	}
	return buf.Bytes(), nil
}

func buildClassStruct(cf *classfile.ClassFile) *ast.GenDecl {
	fields := make([]*ast.Field, 0)
	for _, f := range cf.Fields {
		if f.IsStatic() {
			continue
		}
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(CleanMemberName(f.Name))},
			Type:  descriptorOrAny(f.Descriptor),
		})
	}

	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(CleanClassName(cf.ThisClass)),
				Type: &ast.StructType{
					Fields: &ast.FieldList{List: fields},
				},
			},
		},
	}
}

func buildStaticFieldDecl(cf *classfile.ClassFile, field classfile.MemberInfo) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent(staticFieldName(cf.ThisClass, field.Name))},
				Type:  descriptorOrAny(field.Descriptor),
			},
		},
	}
}

func emitPredeclarations(body *ast.BlockStmt, mir *ir.MethodIR, startSlot, paramSlots int, declared map[string]bool) {
	locals := make(map[int]bool)
	readLocals := make(map[int]bool)
	localTypes := make(map[int]string)
	values := make(map[int]ir.Value)
	readValues := make(map[int]bool)

	md, err := ir.ParseMethodDescriptor(mir.Descriptor)
	if err == nil {
		slot := startSlot
		for _, param := range md.Params {
			localTypes[slot] = param
			slot += descriptorSlots(param)
		}
	}
	if !mir.IsStatic && mir.OwnerClass != "" {
		localTypes[0] = "L" + mir.OwnerClass + ";"
	}

	for _, inst := range mir.Instructions {
		if inst.Result != nil {
			values[inst.Result.ID] = *inst.Result
		}
		for _, arg := range inst.Args {
			readValues[arg.ID] = true
		}
		for _, phi := range inst.PhiAssignments {
			values[phi.Dest.ID] = phi.Dest
			readValues[phi.Src.ID] = true
			readValues[phi.Dest.ID] = true
		}
		for _, c := range inst.Cases {
			for _, phi := range c.PhiAssignments {
				values[phi.Dest.ID] = phi.Dest
				readValues[phi.Src.ID] = true
				readValues[phi.Dest.ID] = true
			}
		}
		for _, phi := range inst.DefaultPhiAssignments {
			values[phi.Dest.ID] = phi.Dest
			readValues[phi.Src.ID] = true
			readValues[phi.Dest.ID] = true
		}
		switch inst.Op {
		case ir.OpLoadVar:
			locals[inst.LocalVar] = true
			readLocals[inst.LocalVar] = true
			if inst.Result != nil && inst.Result.Type != "" {
				localTypes[inst.LocalVar] = inst.Result.Type
			}
		case ir.OpStoreVar:
			locals[inst.LocalVar] = true
			if len(inst.Args) == 1 {
				localTypes[inst.LocalVar] = inst.Args[0].Type
			}
		}
	}

	localIDs := make([]int, 0, len(locals))
	for id := range locals {
		if id >= startSlot+paramSlots {
			localIDs = append(localIDs, id)
		}
	}
	sort.Ints(localIDs)
	for _, id := range localIDs {
		name := fmt.Sprintf("local_%d", id)
		body.List = append(body.List, varDecl(name, descriptorOrAny(localTypes[id])))
		declared[name] = true
		if !readLocals[id] {
			body.List = append(body.List, blankUse(name))
		}
	}

	valueIDs := make([]int, 0, len(values))
	for id := range values {
		valueIDs = append(valueIDs, id)
	}
	sort.Ints(valueIDs)
	for _, id := range valueIDs {
		name := fmt.Sprintf("v%d", id)
		body.List = append(body.List, varDecl(name, descriptorOrAny(values[id].Type)))
		declared[name] = true
		if !readValues[id] {
			body.List = append(body.List, blankUse(name))
		}
	}
}

func descriptorOrAny(desc string) ast.Expr {
	expr := DescriptorToGoType(desc)
	if expr == nil {
		return ast.NewIdent("any")
	}
	return expr
}

func descriptorSlots(desc string) int {
	if desc == "J" || desc == "D" {
		return 2
	}
	return 1
}

func varDecl(name string, typ ast.Expr) ast.Stmt {
	return &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(name)},
					Type:  typ,
				},
			},
		},
	}
}

func blankUse(name string) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("_")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{ast.NewIdent(name)},
	}
}

func assignStmt(lhs string, rhs ast.Expr, declared map[string]bool) ast.Stmt {
	tok := token.DEFINE
	if declared[lhs] {
		tok = token.ASSIGN
	} else {
		declared[lhs] = true
	}
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(lhs)},
		Tok: tok,
		Rhs: []ast.Expr{rhs},
	}
}

func branchExpr(inst ir.Instruction) (ast.Expr, error) {
	if len(inst.Args) == 0 || len(inst.Args) > 2 {
		return nil, fmt.Errorf("branch has %d operands", len(inst.Args))
	}
	tok := branchToken(inst.Condition)
	left := ast.NewIdent(inst.Args[0].String())
	if len(inst.Args) == 1 {
		if inst.Args[0].Type == "Z" {
			switch inst.Condition {
			case ir.CondEQ:
				return &ast.UnaryExpr{Op: token.NOT, X: left}, nil
			case ir.CondNE:
				return left, nil
			default:
				return nil, fmt.Errorf("boolean branch cannot use condition %s", inst.Condition)
			}
		}
		if isReferenceType(inst.Args[0].Type) {
			if inst.Condition != ir.CondEQ && inst.Condition != ir.CondNE {
				return nil, fmt.Errorf("reference branch cannot use condition %s", inst.Condition)
			}
			return &ast.BinaryExpr{
				X:  left,
				Op: tok,
				Y:  ast.NewIdent("nil"),
			}, nil
		}
		return &ast.BinaryExpr{
			X:  left,
			Op: tok,
			Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
		}, nil
	}
	if inst.Args[0].Type == "Z" || inst.Args[1].Type == "Z" {
		if inst.Condition != ir.CondEQ && inst.Condition != ir.CondNE {
			return nil, fmt.Errorf("boolean branch cannot use condition %s", inst.Condition)
		}
	}
	return &ast.BinaryExpr{
		X:  left,
		Op: tok,
		Y:  ast.NewIdent(inst.Args[1].String()),
	}, nil
}

func isReferenceType(desc string) bool {
	return strings.HasPrefix(desc, "L") || strings.HasPrefix(desc, "[")
}

func binaryToken(op ir.OpKind) token.Token {
	switch op {
	case ir.OpAdd:
		return token.ADD
	case ir.OpSub:
		return token.SUB
	case ir.OpMul:
		return token.MUL
	case ir.OpAnd:
		return token.AND
	default:
		return token.QUO
	}
}

func branchToken(cond ir.BranchCondition) token.Token {
	switch cond {
	case ir.CondEQ:
		return token.EQL
	case ir.CondNE:
		return token.NEQ
	case ir.CondLT:
		return token.LSS
	case ir.CondGE:
		return token.GEQ
	case ir.CondGT:
		return token.GTR
	default:
		return token.LEQ
	}
}

func methodGoName(name string) string {
	switch name {
	case "<init>":
		return "Init"
	case "<clinit>":
		return "init"
	default:
		return exportedName(name)
	}
}

func methodKey(owner, name, descriptor string) string {
	return MethodKey(owner, name, descriptor)
}

// MethodKey creates the lookup key for method resolution.
func MethodKey(owner, name, descriptor string) string {
	return owner + "|" + name + "|" + descriptor
}

// ResolveMethodNames builds a lookup table of Go method names for a class and its methods.
func ResolveMethodNames(thisClass string, methods []*ir.MethodIR) map[string]string {
	usedNames := make(map[string]bool)
	methodNames := make(map[string]string)
	for _, mir := range methods {
		owner := mir.OwnerClass
		if owner == "" {
			owner = thisClass
		}
		name := methodGoName(mir.Name)
		if name != "init" {
			if usedNames[name] {
				name = uniqueName(name, mir.Descriptor, usedNames)
			}
			usedNames[name] = true
		}
		methodNames[MethodKey(owner, mir.Name, mir.Descriptor)] = name
	}
	return methodNames
}

func resolveMethodName(names map[string]string, owner, name, descriptor string) string {
	if name == "<clinit>" {
		return "init"
	}
	if resolved, ok := names[MethodKey(owner, name, descriptor)]; ok {
		return resolved
	}
	if name == "<init>" {
		return "Init"
	}
	return exportedName(name)
}

func packageName(internalName string) string {
	return PackageName(internalName)
}

// PackageName computes the Go package name from a JVM internal class name.
func PackageName(internalName string) string {
	if idx := strings.LastIndex(internalName, "/"); idx >= 0 {
		internalName = internalName[idx+1:]
	}
	internalName = strings.ReplaceAll(internalName, "$", "_")
	name := sanitizeIdentifier(internalName)
	if name == "" {
		name = "main"
	}
	name = strings.ToLower(name)
	if name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	if token.Lookup(name).IsKeyword() {
		name += "_"
	}
	return name
}

func exportedName(name string) string {
	name = strings.ReplaceAll(name, "$", "_")
	name = sanitizeIdentifier(name)
	if name == "" {
		return "Method"
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		return "_" + string(runes)
	}
	return string(runes)
}

func uniqueName(base, descriptor string, used map[string]bool) string {
	suffix := sanitizeIdentifier(descriptor)
	if suffix == "" {
		suffix = "overload"
	}
	candidate := base + "_" + suffix
	for i := 2; used[candidate]; i++ {
		candidate = fmt.Sprintf("%s_%s%d", base, suffix, i)
	}
	return candidate
}

func sanitizeIdentifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
