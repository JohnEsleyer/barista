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

// GenerateGo translates a parsed ClassFile and lowered IR into clean Go code.
func GenerateGo(cf *classfile.ClassFile, methods []*ir.MethodIR) ([]byte, error) {
	file := &ast.File{
		Name:  ast.NewIdent(packageName(cf.ThisClass)),
		Decls: make([]ast.Decl, 0),
	}

	if needsJavaLangString(cf, methods) {
		file.Decls = append(file.Decls, javaLangStringDecl())
	}

	file.Decls = append(file.Decls, buildClassStruct(cf))

	for _, field := range cf.Fields {
		if field.IsStatic() {
			file.Decls = append(file.Decls, buildStaticFieldDecl(cf, field))
		}
	}

	// Assign one stable Go name per JVM method. Overloads receive a descriptor
	// suffix so call sites can resolve the exact target.
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

func buildFuncDecl(cf *classfile.ClassFile, mir *ir.MethodIR, fnName string, methodNames map[string]string) (*ast.FuncDecl, error) {
	md, err := ir.ParseMethodDescriptor(mir.Descriptor)
	if err != nil {
		return nil, fmt.Errorf("method %s: %w", mir.Name, err)
	}

	startSlot := 0
	if !mir.IsStatic {
		startSlot = 1
	}

	params := make([]*ast.Field, 0, len(md.Params))
	declared := make(map[string]bool)
	if !mir.IsStatic {
		declared["this"] = true
	}

	slot := startSlot
	for _, param := range md.Params {
		name := fmt.Sprintf("local_%d", slot)
		declared[name] = true
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(name)},
			Type:  descriptorOrAny(param),
		})
		slot += descriptorSlots(param)
	}

	fnDecl := &ast.FuncDecl{
		Name: ast.NewIdent(fnName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: params},
		},
		Body: &ast.BlockStmt{List: make([]ast.Stmt, 0)},
	}
	if !mir.IsStatic {
		fnDecl.Recv = &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("this")},
					Type:  &ast.StarExpr{X: ast.NewIdent(CleanClassName(cf.ThisClass))},
				},
			},
		}
	}
	if md.Return != "V" {
		fnDecl.Type.Results = &ast.FieldList{
			List: []*ast.Field{{Type: descriptorOrAny(md.Return)}},
		}
	}

	branchy := false
	for _, inst := range mir.Instructions {
		if inst.Op == ir.OpLabel || inst.Op == ir.OpGoto || inst.Op == ir.OpBranch {
			branchy = true
			break
		}
	}
	if branchy {
		emitPredeclarations(fnDecl.Body, mir, startSlot, md.ParamSlots, declared)
	}

	var pendingLabels []int
	appendStmt := func(stmt ast.Stmt) {
		for i := len(pendingLabels) - 1; i >= 0; i-- {
			stmt = &ast.LabeledStmt{
				Label: ast.NewIdent(fmt.Sprintf("L%d", pendingLabels[i])),
				Stmt:  stmt,
			}
		}
		pendingLabels = nil
		fnDecl.Body.List = append(fnDecl.Body.List, stmt)
	}

	for _, inst := range mir.Instructions {
		switch inst.Op {
		case ir.OpConstInt:
			if inst.Result == nil {
				return nil, fmt.Errorf("CONST_INT without a result in %s", mir.Name)
			}
			valueExpr := ast.Expr(&ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.FormatInt(int64(inst.Constant), 10),
			})
			// In branchless methods the SSA value is introduced with :=, so a
			// plain literal would infer Go's int instead of int32. In branchy
			// methods emitPredeclarations already fixed the type.
			if !declared[inst.Result.String()] {
				valueExpr = &ast.CallExpr{
					Fun:  ast.NewIdent("int32"),
					Args: []ast.Expr{valueExpr},
				}
			}
			appendStmt(assignStmt(inst.Result.String(), valueExpr, declared))

		case ir.OpLoadVar:
			if inst.Result == nil {
				return nil, fmt.Errorf("LOAD_VAR without a result in %s", mir.Name)
			}
			localName := fmt.Sprintf("local_%d", inst.LocalVar)
			if !mir.IsStatic && inst.LocalVar == 0 {
				localName = "this"
			}
			appendStmt(assignStmt(inst.Result.String(), ast.NewIdent(localName), declared))

		case ir.OpStoreVar:
			if len(inst.Args) != 1 {
				return nil, fmt.Errorf("STORE_VAR without a value in %s", mir.Name)
			}
			localName := fmt.Sprintf("local_%d", inst.LocalVar)
			if !mir.IsStatic && inst.LocalVar == 0 {
				localName = "this"
			}
			appendStmt(assignStmt(localName, ast.NewIdent(inst.Args[0].String()), declared))

		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
			if len(inst.Args) != 2 || inst.Result == nil {
				return nil, fmt.Errorf("binary operation with invalid operands in %s", mir.Name)
			}
			appendStmt(assignStmt(inst.Result.String(), &ast.BinaryExpr{
				X:  ast.NewIdent(inst.Args[0].String()),
				Op: binaryToken(inst.Op),
				Y:  ast.NewIdent(inst.Args[1].String()),
			}, declared))

		case ir.OpNeg:
			if len(inst.Args) != 1 || inst.Result == nil {
				return nil, fmt.Errorf("NEG with invalid operands in %s", mir.Name)
			}
			appendStmt(assignStmt(inst.Result.String(), &ast.UnaryExpr{
				Op: token.SUB,
				X:  ast.NewIdent(inst.Args[0].String()),
			}, declared))

		case ir.OpLabel:
			pendingLabels = append(pendingLabels, inst.Label)

		case ir.OpGoto:
			appendStmt(&ast.BranchStmt{
				Tok:   token.GOTO,
				Label: ast.NewIdent(fmt.Sprintf("L%d", inst.Target)),
			})

		case ir.OpBranch:
			cond, err := branchExpr(inst)
			if err != nil {
				return nil, fmt.Errorf("method %s: %w", mir.Name, err)
			}
			appendStmt(&ast.IfStmt{
				Cond: cond,
				Body: &ast.BlockStmt{List: []ast.Stmt{
					&ast.BranchStmt{
						Tok:   token.GOTO,
						Label: ast.NewIdent(fmt.Sprintf("L%d", inst.Target)),
					},
				}},
			})

		case ir.OpNew:
			if inst.Result == nil || inst.OwnerClass == "" {
				return nil, fmt.Errorf("NEW with invalid metadata in %s", mir.Name)
			}
			appendStmt(assignStmt(inst.Result.String(), &ast.UnaryExpr{
				Op: token.AND,
				X: &ast.CompositeLit{
					Type: ast.NewIdent(CleanClassName(inst.OwnerClass)),
				},
			}, declared))

		case ir.OpGetField:
			if inst.Result == nil || len(inst.Args) != 1 {
				return nil, fmt.Errorf("GET_FIELD with invalid operands in %s", mir.Name)
			}
			if inst.OwnerClass != "" && inst.OwnerClass != cf.ThisClass {
				return nil, fmt.Errorf("GET_FIELD %s.%s refers to external class %s; only single-class field access is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
			}
			appendStmt(assignStmt(inst.Result.String(), &ast.SelectorExpr{
				X:   ast.NewIdent(inst.Args[0].String()),
				Sel: ast.NewIdent(CleanMemberName(inst.MemberName)),
			}, declared))

		case ir.OpPutField:
			if len(inst.Args) != 2 {
				return nil, fmt.Errorf("PUT_FIELD with invalid operands in %s", mir.Name)
			}
			if inst.OwnerClass != "" && inst.OwnerClass != cf.ThisClass {
				return nil, fmt.Errorf("PUT_FIELD %s.%s refers to external class %s; only single-class field access is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
			}
			appendStmt(&ast.AssignStmt{
				Lhs: []ast.Expr{
					&ast.SelectorExpr{
						X:   ast.NewIdent(inst.Args[0].String()),
						Sel: ast.NewIdent(CleanMemberName(inst.MemberName)),
					},
				},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{ast.NewIdent(inst.Args[1].String())},
			})

		case ir.OpGetStatic:
			if inst.Result == nil {
				return nil, fmt.Errorf("GET_STATIC without a result in %s", mir.Name)
			}
			if inst.OwnerClass != "" && inst.OwnerClass != cf.ThisClass {
				return nil, fmt.Errorf("GET_STATIC %s.%s refers to external class %s; only single-class static state is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
			}
			appendStmt(assignStmt(inst.Result.String(), ast.NewIdent(
				staticFieldName(inst.OwnerClass, inst.MemberName),
			), declared))

		case ir.OpPutStatic:
			if len(inst.Args) != 1 {
				return nil, fmt.Errorf("PUT_STATIC without a value in %s", mir.Name)
			}
			if inst.OwnerClass != "" && inst.OwnerClass != cf.ThisClass {
				return nil, fmt.Errorf("PUT_STATIC %s.%s refers to external class %s; only single-class static state is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
			}
			appendStmt(&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent(staticFieldName(inst.OwnerClass, inst.MemberName))},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{ast.NewIdent(inst.Args[0].String())},
			})

		case ir.OpInvokeVirtual, ir.OpInvokeSpecial:
			if len(inst.Args) == 0 {
				return nil, fmt.Errorf("method invocation without a receiver in %s", mir.Name)
			}
			// java/lang/Object has no generated body. The allocation already
			// produced a zeroed Go struct, so Object.<init> is a no-op. Keep a
			// blank use so the receiver SSA value does not become unused.
			if inst.OwnerClass == "java/lang/Object" && inst.MemberName == "<init>" {
				appendStmt(&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("_")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(inst.Args[0].String())},
				})
				continue
			}
			if inst.OwnerClass == "java/lang/Object" {
				return nil, fmt.Errorf("java/lang/Object.%s is not supported yet", inst.MemberName)
			}
			if inst.OwnerClass != "" && inst.OwnerClass != cf.ThisClass {
				return nil, fmt.Errorf("%s on external class %s is not supported yet; only single-class dispatch is implemented", inst.MemberName, inst.OwnerClass)
			}
			callName := resolveMethodName(methodNames, inst.OwnerClass, inst.MemberName, inst.Descriptor)
			call := &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent(inst.Args[0].String()),
					Sel: ast.NewIdent(callName),
				},
				Args: make([]ast.Expr, 0, len(inst.Args)-1),
			}
			for _, arg := range inst.Args[1:] {
				call.Args = append(call.Args, ast.NewIdent(arg.String()))
			}
			if inst.Result != nil {
				appendStmt(assignStmt(inst.Result.String(), call, declared))
			} else {
				appendStmt(&ast.ExprStmt{X: call})
			}

		case ir.OpInvokeStatic:
			if inst.OwnerClass != "" && inst.OwnerClass != cf.ThisClass {
				return nil, fmt.Errorf("static call %s.%s refers to external class %s; only single-class dispatch is implemented", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
			}
			callName := resolveMethodName(methodNames, inst.OwnerClass, inst.MemberName, inst.Descriptor)
			call := &ast.CallExpr{
				Fun:  ast.NewIdent(callName),
				Args: make([]ast.Expr, 0, len(inst.Args)),
			}
			for _, arg := range inst.Args {
				call.Args = append(call.Args, ast.NewIdent(arg.String()))
			}
			if inst.Result != nil {
				appendStmt(assignStmt(inst.Result.String(), call, declared))
			} else {
				appendStmt(&ast.ExprStmt{X: call})
			}

		case ir.OpReturn:
			retStmt := &ast.ReturnStmt{}
			if len(inst.Args) > 0 {
				retStmt.Results = []ast.Expr{ast.NewIdent(inst.Args[0].String())}
			}
			appendStmt(retStmt)

		default:
			return nil, fmt.Errorf("unhandled IR opcode %s in %s", inst.Op, mir.Name)
		}
	}

	if len(pendingLabels) != 0 {
		return nil, fmt.Errorf("method %s ends with an unbound label", mir.Name)
	}
	return fnDecl, nil
}

// emitPredeclarations emits var declarations for every SSA value and local used
// by a branchy method. Go forbids goto statements that jump over declarations,
// so all names must be declared before the first label.
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
		return &ast.BinaryExpr{
			X:  left,
			Op: tok,
			Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
		}, nil
	}
	return &ast.BinaryExpr{
		X:  left,
		Op: tok,
		Y:  ast.NewIdent(inst.Args[1].String()),
	}, nil
}

func binaryToken(op ir.OpKind) token.Token {
	switch op {
	case ir.OpAdd:
		return token.ADD
	case ir.OpSub:
		return token.SUB
	case ir.OpMul:
		return token.MUL
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
	return owner + "|" + name + "|" + descriptor
}

func resolveMethodName(names map[string]string, owner, name, descriptor string) string {
	if name == "<clinit>" {
		return "init"
	}
	if resolved, ok := names[methodKey(owner, name, descriptor)]; ok {
		return resolved
	}
	if name == "<init>" {
		return "Init"
	}
	return exportedName(name)
}

func needsJavaLangString(cf *classfile.ClassFile, methods []*ir.MethodIR) bool {
	for _, field := range cf.Fields {
		if strings.Contains(field.Descriptor, "Ljava/lang/String;") {
			return true
		}
	}
	for _, mir := range methods {
		md, err := ir.ParseMethodDescriptor(mir.Descriptor)
		if err != nil {
			continue
		}
		if strings.Contains(md.Return, "Ljava/lang/String;") {
			return true
		}
		for _, param := range md.Params {
			if strings.Contains(param, "Ljava/lang/String;") {
				return true
			}
		}
	}
	return false
}

func javaLangStringDecl() *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent("JavaLangString"),
				Type: &ast.StructType{
					Fields: &ast.FieldList{List: []*ast.Field{
						{
							Names: []*ast.Ident{ast.NewIdent("Value")},
							Type:  ast.NewIdent("string"),
						},
					}},
				},
			},
		},
	}
}

func packageName(internalName string) string {
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
