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

	usedNames := make(map[string]bool)
	for _, mir := range methods {
		// Phase 1 only supports static methods. Instance methods and
		// constructors require the object model introduced in Phase 2.
		if !mir.IsStatic || strings.HasPrefix(mir.Name, "<") {
			continue
		}
		name := exportedName(mir.Name)
		if usedNames[name] {
			name = uniqueName(name, mir.Descriptor, usedNames)
		}
		usedNames[name] = true

		fnDecl, err := buildFuncDecl(mir, name)
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

func buildFuncDecl(mir *ir.MethodIR, fnName string) (*ast.FuncDecl, error) {
	params, returnType, paramSlots, err := parseMethodDescriptor(mir.Descriptor)
	if err != nil {
		return nil, fmt.Errorf("method %s: %w", mir.Name, err)
	}

	fnDecl := &ast.FuncDecl{
		Name: ast.NewIdent(fnName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: params},
		},
		Body: &ast.BlockStmt{List: make([]ast.Stmt, 0)},
	}
	if returnType != nil {
		fnDecl.Type.Results = &ast.FieldList{
			List: []*ast.Field{{Type: returnType}},
		}
	}

	branchy := false
	for _, inst := range mir.Instructions {
		if inst.Op == ir.OpLabel || inst.Op == ir.OpGoto || inst.Op == ir.OpBranch {
			branchy = true
			break
		}
	}

	declared := make(map[string]bool)
	for i := 0; i < paramSlots; i++ {
		declared[fmt.Sprintf("local_%d", i)] = true
	}

	if branchy {
		emitPredeclarations(fnDecl.Body, mir, paramSlots, declared)
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
			appendStmt(assignStmt(inst.Result.String(), &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.FormatInt(int64(inst.Constant), 10),
			}, declared))

		case ir.OpLoadVar:
			if inst.Result == nil {
				return nil, fmt.Errorf("LOAD_VAR without a result in %s", mir.Name)
			}
			appendStmt(assignStmt(inst.Result.String(), ast.NewIdent(fmt.Sprintf("local_%d", inst.LocalVar)), declared))

		case ir.OpStoreVar:
			if len(inst.Args) != 1 {
				return nil, fmt.Errorf("STORE_VAR without a value in %s", mir.Name)
			}
			appendStmt(assignStmt(fmt.Sprintf("local_%d", inst.LocalVar), ast.NewIdent(inst.Args[0].String()), declared))

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
func emitPredeclarations(body *ast.BlockStmt, mir *ir.MethodIR, paramSlots int, declared map[string]bool) {
	locals := make(map[int]bool)
	readLocals := make(map[int]bool)
	values := make(map[int]bool)
	readValues := make(map[int]bool)

	for _, inst := range mir.Instructions {
		if inst.Result != nil {
			values[inst.Result.ID] = true
		}
		for _, arg := range inst.Args {
			readValues[arg.ID] = true
		}
		switch inst.Op {
		case ir.OpLoadVar:
			locals[inst.LocalVar] = true
			readLocals[inst.LocalVar] = true
		case ir.OpStoreVar:
			locals[inst.LocalVar] = true
		}
	}

	localIDs := make([]int, 0, len(locals))
	for id := range locals {
		if id >= paramSlots {
			localIDs = append(localIDs, id)
		}
	}
	sort.Ints(localIDs)
	for _, id := range localIDs {
		name := fmt.Sprintf("local_%d", id)
		body.List = append(body.List, varDecl(name, "int32"))
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
		body.List = append(body.List, varDecl(name, "int32"))
		declared[name] = true
		if !readValues[id] {
			body.List = append(body.List, blankUse(name))
		}
	}
}

func varDecl(name, typeName string) ast.Stmt {
	return &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(name)},
					Type:  ast.NewIdent(typeName),
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

func parseMethodDescriptor(desc string) ([]*ast.Field, ast.Expr, int, error) {
	if !strings.HasPrefix(desc, "(") {
		return nil, nil, 0, fmt.Errorf("invalid method descriptor: %s", desc)
	}
	closingParen := strings.Index(desc, ")")
	if closingParen == -1 {
		return nil, nil, 0, fmt.Errorf("invalid method descriptor missing closing paren: %s", desc)
	}

	paramDesc := desc[1:closingParen]
	returnDesc := desc[closingParen+1:]

	fields := make([]*ast.Field, 0)
	idx := 0
	for i := 0; i < len(paramDesc); i++ {
		switch paramDesc[i] {
		case 'I':
			name := fmt.Sprintf("local_%d", idx)
			idx++
			fields = append(fields, &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(name)},
				Type:  ast.NewIdent("int32"),
			})
		default:
			return nil, nil, 0, fmt.Errorf("unsupported parameter type %q in descriptor %s (Phase 1 supports int only)", paramDesc[i], desc)
		}
	}

	var retType ast.Expr
	switch returnDesc {
	case "I":
		retType = ast.NewIdent("int32")
	case "V":
		retType = nil
	default:
		return nil, nil, 0, fmt.Errorf("unsupported return type %q in descriptor %s (Phase 1 supports int and void only)", returnDesc, desc)
	}
	return fields, retType, idx, nil
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
