package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/ir"
)

type funcEmitter struct {
	cf          *classfile.ClassFile
	mir         *ir.MethodIR
	methodNames map[string]string
	declared    map[string]bool
	used        map[int]bool
	body        *ast.BlockStmt
	pending     []int
	inTry       bool
}

func (e *funcEmitter) append(stmt ast.Stmt) {
	for i := len(e.pending) - 1; i >= 0; i-- {
		stmt = &ast.LabeledStmt{
			Label: ast.NewIdent(fmt.Sprintf("L%d", e.pending[i])),
			Stmt:  stmt,
		}
	}
	e.pending = nil
	e.body.List = append(e.body.List, stmt)
}

func (e *funcEmitter) finish() error {
	if len(e.pending) != 0 {
		return fmt.Errorf("method %s ends with an unbound label", e.mir.Name)
	}
	return nil
}

func (e *funcEmitter) assign(name string, rhs ast.Expr) {
	e.append(assignStmt(name, rhs, e.declared))
}

func (e *funcEmitter) assignResult(result *ir.Value, rhs ast.Expr) {
	if result == nil {
		return
	}
	e.assign(result.String(), rhs)
	if e.used != nil && !e.used[result.ID] {
		e.append(blankUse(result.String()))
	}
}

func (e *funcEmitter) emit(inst ir.Instruction) error {
	switch inst.Op {
	case ir.OpConstInt:
		if inst.Result == nil {
			return fmt.Errorf("CONST_INT without a result in %s", e.mir.Name)
		}
		valueExpr := ast.Expr(&ast.BasicLit{
			Kind:  token.INT,
			Value: strconv.FormatInt(int64(inst.Constant), 10),
		})
		if !e.declared[inst.Result.String()] {
			valueExpr = &ast.CallExpr{Fun: ast.NewIdent("int32"), Args: []ast.Expr{valueExpr}}
		}
		e.assignResult(inst.Result, valueExpr)

	case ir.OpLoadVar:
		if inst.Result == nil {
			return fmt.Errorf("LOAD_VAR without a result in %s", e.mir.Name)
		}
		localName := fmt.Sprintf("local_%d", inst.LocalVar)
		if !e.mir.IsStatic && inst.LocalVar == 0 {
			localName = "this"
		}
		e.assignResult(inst.Result, ast.NewIdent(localName))

	case ir.OpStoreVar:
		if len(inst.Args) != 1 {
			return fmt.Errorf("STORE_VAR without a value in %s", e.mir.Name)
		}
		localName := fmt.Sprintf("local_%d", inst.LocalVar)
		if !e.mir.IsStatic && inst.LocalVar == 0 {
			localName = "this"
		}
		e.assign(localName, ast.NewIdent(inst.Args[0].String()))

	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
		if len(inst.Args) != 2 || inst.Result == nil {
			return fmt.Errorf("binary operation with invalid operands in %s", e.mir.Name)
		}
		e.assignResult(inst.Result, &ast.BinaryExpr{
			X:  ast.NewIdent(inst.Args[0].String()),
			Op: binaryToken(inst.Op),
			Y:  ast.NewIdent(inst.Args[1].String()),
		})

	case ir.OpNeg:
		if len(inst.Args) != 1 || inst.Result == nil {
			return fmt.Errorf("NEG with invalid operands in %s", e.mir.Name)
		}
		e.assignResult(inst.Result, &ast.UnaryExpr{
			Op: token.SUB,
			X:  ast.NewIdent(inst.Args[0].String()),
		})

	case ir.OpLabel:
		e.pending = append(e.pending, inst.Label)

	case ir.OpGoto:
		e.append(&ast.BranchStmt{
			Tok:   token.GOTO,
			Label: ast.NewIdent(fmt.Sprintf("L%d", inst.Target)),
		})

	case ir.OpBranch:
		cond, err := branchExpr(inst)
		if err != nil {
			return fmt.Errorf("method %s: %w", e.mir.Name, err)
		}
		e.append(&ast.IfStmt{
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
			return fmt.Errorf("NEW with invalid metadata in %s", e.mir.Name)
		}
		objType, ok := runtimeTypeName(inst.OwnerClass)
		var compositeType ast.Expr
		if ok {
			compositeType = &ast.SelectorExpr{X: ast.NewIdent("lang"), Sel: ast.NewIdent(objType)}
		} else {
			compositeType = ast.NewIdent(CleanClassName(inst.OwnerClass))
		}
		e.assignResult(inst.Result, &ast.UnaryExpr{
			Op: token.AND,
			X:  &ast.CompositeLit{Type: compositeType},
		})

	case ir.OpGetField:
		if inst.Result == nil || len(inst.Args) != 1 {
			return fmt.Errorf("GET_FIELD with invalid operands in %s", e.mir.Name)
		}
		if inst.OwnerClass != "" && inst.OwnerClass != e.cf.ThisClass {
			return fmt.Errorf("GET_FIELD %s.%s refers to external class %s; only single-class field access is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
		}
		e.assignResult(inst.Result, &ast.SelectorExpr{
			X:   ast.NewIdent(inst.Args[0].String()),
			Sel: ast.NewIdent(CleanMemberName(inst.MemberName)),
		})

	case ir.OpPutField:
		if len(inst.Args) != 2 {
			return fmt.Errorf("PUT_FIELD with invalid operands in %s", e.mir.Name)
		}
		if inst.OwnerClass != "" && inst.OwnerClass != e.cf.ThisClass {
			return fmt.Errorf("PUT_FIELD %s.%s refers to external class %s; only single-class field access is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
		}
		e.append(&ast.AssignStmt{
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
			return fmt.Errorf("GET_STATIC without a result in %s", e.mir.Name)
		}
		if inst.OwnerClass == "java/lang/System" {
			switch inst.MemberName {
			case "out":
				e.assignResult(inst.Result, langSelector("SystemOut"))
				return nil
			case "err":
				e.assignResult(inst.Result, langSelector("SystemErr"))
				return nil
			}
		}
		if inst.OwnerClass != "" && inst.OwnerClass != e.cf.ThisClass {
			return fmt.Errorf("GET_STATIC %s.%s refers to external class %s; only single-class static state is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
		}
		e.assignResult(inst.Result, ast.NewIdent(staticFieldName(inst.OwnerClass, inst.MemberName)))

	case ir.OpPutStatic:
		if len(inst.Args) != 1 {
			return fmt.Errorf("PUT_STATIC without a value in %s", e.mir.Name)
		}
		if inst.OwnerClass != "" && inst.OwnerClass != e.cf.ThisClass {
			return fmt.Errorf("PUT_STATIC %s.%s refers to external class %s; only single-class static state is supported yet", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
		}
		e.append(&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(staticFieldName(inst.OwnerClass, inst.MemberName))},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent(inst.Args[0].String())},
		})

	case ir.OpInvokeVirtual, ir.OpInvokeSpecial:
		if len(inst.Args) == 0 {
			return fmt.Errorf("method invocation without a receiver in %s", e.mir.Name)
		}
		if inst.OwnerClass == "java/lang/Object" && inst.MemberName == "<init>" {
			e.append(blankUse(inst.Args[0].String()))
			return nil
		}
		var callName string
		if isRuntimeClass(inst.OwnerClass) {
			callName = runtimeMethodName(inst.OwnerClass, inst.MemberName, inst.Descriptor)
		} else {
			if inst.OwnerClass == "java/lang/Object" {
				return fmt.Errorf("java/lang/Object.%s is not supported yet", inst.MemberName)
			}
			if inst.OwnerClass != "" && inst.OwnerClass != e.cf.ThisClass {
				return fmt.Errorf("%s on external class %s is not supported yet; only single-class dispatch is implemented", inst.MemberName, inst.OwnerClass)
			}
			callName = resolveMethodName(e.methodNames, inst.OwnerClass, inst.MemberName, inst.Descriptor)
		}
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
			e.assignResult(inst.Result, call)
		} else {
			e.append(&ast.ExprStmt{X: call})
		}

	case ir.OpInvokeStatic:
		var call *ast.CallExpr
		if isRuntimeClass(inst.OwnerClass) {
			callName := runtimeMethodName(inst.OwnerClass, inst.MemberName, inst.Descriptor)
			call = &ast.CallExpr{
				Fun:  langSelector(callName),
				Args: make([]ast.Expr, 0, len(inst.Args)),
			}
		} else {
			if inst.OwnerClass != "" && inst.OwnerClass != e.cf.ThisClass {
				return fmt.Errorf("static call %s.%s refers to external class %s; only single-class dispatch is implemented", inst.OwnerClass, inst.MemberName, inst.OwnerClass)
			}
			call = &ast.CallExpr{
				Fun:  ast.NewIdent(resolveMethodName(e.methodNames, inst.OwnerClass, inst.MemberName, inst.Descriptor)),
				Args: make([]ast.Expr, 0, len(inst.Args)),
			}
		}
		for _, arg := range inst.Args {
			call.Args = append(call.Args, ast.NewIdent(arg.String()))
		}
		if inst.Result != nil {
			e.assignResult(inst.Result, call)
		} else {
			e.append(&ast.ExprStmt{X: call})
		}

	case ir.OpConstString:
		if inst.Result == nil {
			return fmt.Errorf("CONST_STRING without a result in %s", e.mir.Name)
		}
		e.assignResult(inst.Result, &ast.CallExpr{
			Fun:  langSelector("NewString"),
			Args: []ast.Expr{stringLit(inst.Descriptor)},
		})

	case ir.OpConstClass:
		if inst.Result == nil {
			return fmt.Errorf("CONST_CLASS without a result in %s", e.mir.Name)
		}
		e.assignResult(inst.Result, &ast.CallExpr{
			Fun:  langSelector("NewClass"),
			Args: []ast.Expr{stringLit(inst.Descriptor)},
		})

	case ir.OpConstNull:
		if inst.Result == nil {
			return fmt.Errorf("CONST_NULL without a result in %s", e.mir.Name)
		}
		if !e.declared[inst.Result.String()] {
			e.append(varDecl(inst.Result.String(), ast.NewIdent("any")))
			e.declared[inst.Result.String()] = true
		}
		e.assignResult(inst.Result, ast.NewIdent("nil"))

	case ir.OpNewArray, ir.OpANewArray:
		if inst.Result == nil || len(inst.Args) != 1 {
			return fmt.Errorf("array allocation with invalid operands in %s", e.mir.Name)
		}
		arrayType := DescriptorToGoType(inst.Descriptor)
		if arrayType == nil {
			return fmt.Errorf("array allocation has invalid descriptor %q in %s", inst.Descriptor, e.mir.Name)
		}
		e.assignResult(inst.Result, &ast.CallExpr{
			Fun:  ast.NewIdent("make"),
			Args: []ast.Expr{arrayType, ast.NewIdent(inst.Args[0].String())},
		})

	case ir.OpArrayLength:
		if inst.Result == nil || len(inst.Args) != 1 {
			return fmt.Errorf("ARRAY_LENGTH with invalid operands in %s", e.mir.Name)
		}
		length := &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{ast.NewIdent(inst.Args[0].String())}}
		e.assignResult(inst.Result, &ast.CallExpr{Fun: ast.NewIdent("int32"), Args: []ast.Expr{length}})

	case ir.OpArrayLoad:
		if inst.Result == nil || len(inst.Args) != 2 {
			return fmt.Errorf("ARRAY_LOAD with invalid operands in %s", e.mir.Name)
		}
		e.assignResult(inst.Result, &ast.CallExpr{
			Fun:  langSelector("ArrayLoad"),
			Args: []ast.Expr{ast.NewIdent(inst.Args[0].String()), ast.NewIdent(inst.Args[1].String())},
		})

	case ir.OpArrayStore:
		if len(inst.Args) != 3 {
			return fmt.Errorf("ARRAY_STORE with invalid operands in %s", e.mir.Name)
		}
		e.append(&ast.ExprStmt{X: &ast.CallExpr{
			Fun: langSelector("ArrayStore"),
			Args: []ast.Expr{
				ast.NewIdent(inst.Args[0].String()),
				ast.NewIdent(inst.Args[1].String()),
				ast.NewIdent(inst.Args[2].String()),
			},
		}})

	case ir.OpThrow:
		if len(inst.Args) != 1 {
			return fmt.Errorf("THROW without an exception in %s", e.mir.Name)
		}
		e.append(panicStmt(ast.NewIdent(inst.Args[0].String())))

	case ir.OpLoadException:
		if inst.Result == nil {
			return fmt.Errorf("LOAD_EXCEPTION without a result in %s", e.mir.Name)
		}
		e.assignResult(inst.Result, ast.NewIdent("exc"))

	case ir.OpReturn:
		if e.inTry {
			return fmt.Errorf("method %s returns from inside a protected try range; this is not supported yet", e.mir.Name)
		}
		retStmt := &ast.ReturnStmt{}
		if len(inst.Args) > 0 {
			retStmt.Results = []ast.Expr{ast.NewIdent(inst.Args[0].String())}
		}
		e.append(retStmt)

	default:
		return fmt.Errorf("unhandled IR opcode %s in %s", inst.Op, e.mir.Name)
	}
	return nil
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

	branchy := len(mir.ExceptionTraps) > 0
	if !branchy {
		for _, inst := range mir.Instructions {
			if inst.Op == ir.OpLabel || inst.Op == ir.OpGoto || inst.Op == ir.OpBranch {
				branchy = true
				break
			}
		}
	}
	if branchy {
		emitPredeclarations(fnDecl.Body, mir, startSlot, md.ParamSlots, declared)
	}
	if len(mir.ExceptionTraps) > 0 {
		fnDecl.Body.List = append(fnDecl.Body.List, varDecl("exc", langType("Throwable")))
		declared["exc"] = true
		fnDecl.Body.List = append(fnDecl.Body.List,
			varDeclInit("handlerPC", ast.NewIdent("int32"), intLit(-1)))
		declared["handlerPC"] = true
	}

	groups, err := groupExceptionTraps(mir.ExceptionTraps)
	if err != nil {
		return nil, fmt.Errorf("method %s: %w", mir.Name, err)
	}
	groupAt := make(map[int]tryGroup, len(groups))
	for _, group := range groups {
		groupAt[group.StartPC] = group
	}

	labelPC := make(map[int]int)
	labelAtPC := make(map[int]int)
	for _, inst := range mir.Instructions {
		if inst.Op == ir.OpLabel {
			labelPC[inst.Label] = inst.PC
			labelAtPC[inst.PC] = inst.Label
		}
	}

	emitter := &funcEmitter{
		cf:          cf,
		mir:         mir,
		methodNames: methodNames,
		declared:    declared,
		used:        usedValues(mir),
		body:        fnDecl.Body,
	}

	found := make(map[int]bool)
	for i := 0; i < len(mir.Instructions); i++ {
		inst := mir.Instructions[i]
		group, ok := groupAt[inst.PC]
		if !ok {
			if err := emitter.emit(inst); err != nil {
				return nil, err
			}
			continue
		}
		end := i
		for end < len(mir.Instructions) && mir.Instructions[end].PC >= group.StartPC && mir.Instructions[end].PC < group.EndPC {
			end++
		}
		if end == i {
			return nil, fmt.Errorf("method %s: exception range [%d,%d) has no instructions", mir.Name, group.StartPC, group.EndPC)
		}
		if err := emitTryGroup(emitter, group, mir.Instructions[i:end], labelPC, labelAtPC); err != nil {
			return nil, fmt.Errorf("method %s: %w", mir.Name, err)
		}
		found[group.StartPC] = true
		i = end - 1
	}
	for _, group := range groups {
		if !found[group.StartPC] {
			return nil, fmt.Errorf("method %s: exception range start pc %d does not align with an instruction", mir.Name, group.StartPC)
		}
	}
	if err := emitter.finish(); err != nil {
		return nil, err
	}
	return fnDecl, nil
}

type tryGroup struct {
	StartPC int
	EndPC   int
	Traps   []ir.ExceptionTrap
}

func groupExceptionTraps(traps []ir.ExceptionTrap) ([]tryGroup, error) {
	if len(traps) == 0 {
		return nil, nil
	}
	sorted := append([]ir.ExceptionTrap(nil), traps...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].StartPC != sorted[j].StartPC {
			return sorted[i].StartPC < sorted[j].StartPC
		}
		return sorted[i].EndPC < sorted[j].EndPC
	})
	groups := make([]tryGroup, 0)
	for _, trap := range sorted {
		if len(groups) > 0 {
			last := &groups[len(groups)-1]
			if last.StartPC == trap.StartPC && last.EndPC == trap.EndPC {
				last.Traps = append(last.Traps, trap)
				continue
			}
			if trap.StartPC < last.EndPC {
				return nil, fmt.Errorf("overlapping exception ranges are not supported yet: [%d,%d) and [%d,%d)",
					last.StartPC, last.EndPC, trap.StartPC, trap.EndPC)
			}
		}
		groups = append(groups, tryGroup{
			StartPC: trap.StartPC,
			EndPC:   trap.EndPC,
			Traps:   []ir.ExceptionTrap{trap},
		})
	}
	return groups, nil
}

func emitTryGroup(main *funcEmitter, group tryGroup, insts []ir.Instruction, labelPC, labelAtPC map[int]int) error {
	for _, inst := range insts {
		switch inst.Op {
		case ir.OpReturn:
			return fmt.Errorf("return inside protected try range [%d,%d) is not supported yet", group.StartPC, group.EndPC)
		case ir.OpGoto, ir.OpBranch:
			targetPC, ok := labelPC[inst.Target]
			if !ok {
				return fmt.Errorf("branch label L%d has no bytecode offset", inst.Target)
			}
			if targetPC < group.StartPC || targetPC >= group.EndPC {
				return fmt.Errorf("branch from protected range [%d,%d) to pc %d is not supported yet", group.StartPC, group.EndPC, targetPC)
			}
		}
	}

	main.append(&ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("handlerPC")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{intLit(-1)},
	})

	closureBody := &ast.BlockStmt{List: make([]ast.Stmt, 0, len(insts)+1)}
	tryEmitter := &funcEmitter{
		cf:          main.cf,
		mir:         main.mir,
		methodNames: main.methodNames,
		declared:    main.declared,
		used:        main.used,
		body:        closureBody,
		inTry:       true,
	}
	closureBody.List = append(closureBody.List, recoverDefer(group.Traps))
	for _, inst := range insts {
		if err := tryEmitter.emit(inst); err != nil {
			return err
		}
	}
	if err := tryEmitter.finish(); err != nil {
		return err
	}

	main.append(&ast.ExprStmt{X: &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{Params: &ast.FieldList{}},
			Body: closureBody,
		},
	}})

	dispatch := &ast.SwitchStmt{
		Tag:  ast.NewIdent("handlerPC"),
		Body: &ast.BlockStmt{List: make([]ast.Stmt, 0)},
	}
	seen := make(map[int]bool)
	for _, trap := range group.Traps {
		if seen[trap.HandlerPC] {
			continue
		}
		seen[trap.HandlerPC] = true
		label, ok := labelAtPC[trap.HandlerPC]
		if !ok {
			return fmt.Errorf("handler pc %d has no IR label", trap.HandlerPC)
		}
		dispatch.Body.List = append(dispatch.Body.List, &ast.CaseClause{
			List: []ast.Expr{intLit(int64(trap.HandlerPC))},
			Body: []ast.Stmt{&ast.BranchStmt{
				Tok:   token.GOTO,
				Label: ast.NewIdent(fmt.Sprintf("L%d", label)),
			}},
		})
	}
	main.append(dispatch)
	return nil
}

func recoverDefer(traps []ir.ExceptionTrap) ast.Stmt {
	switchBody := &ast.BlockStmt{List: make([]ast.Stmt, 0, len(traps)+1)}
	for _, trap := range traps {
		switchBody.List = append(switchBody.List, &ast.CaseClause{
			List: []ast.Expr{&ast.CallExpr{
				Fun:  langSelector("MatchesException"),
				Args: []ast.Expr{ast.NewIdent("r"), stringLit(trap.CatchClass)},
			}},
			Body: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("exc")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun:  langSelector("AsThrowable"),
						Args: []ast.Expr{ast.NewIdent("r")},
					}},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("handlerPC")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{intLit(int64(trap.HandlerPC))},
				},
			},
		})
	}
	switchBody.List = append(switchBody.List, &ast.CaseClause{
		Body: []ast.Stmt{panicStmt(ast.NewIdent("r"))},
	})

	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("r")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent("recover")}},
		},
		Cond: &ast.BinaryExpr{X: ast.NewIdent("r"), Op: token.NEQ, Y: ast.NewIdent("nil")},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.SwitchStmt{Body: switchBody},
		}},
	}
	return &ast.DeferStmt{Call: &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{Params: &ast.FieldList{}},
			Body: &ast.BlockStmt{List: []ast.Stmt{ifStmt}},
		},
	}}
}

func langSelector(name string) ast.Expr {
	return &ast.SelectorExpr{X: ast.NewIdent("lang"), Sel: ast.NewIdent(name)}
}

func intLit(value int64) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(value, 10)}
}

func stringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func varDeclInit(name string, typ, value ast.Expr) ast.Stmt {
	return &ast.DeclStmt{Decl: &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{ast.NewIdent(name)},
				Type:   typ,
				Values: []ast.Expr{value},
			},
		},
	}}
}

func panicStmt(expr ast.Expr) ast.Stmt {
	return &ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic"), Args: []ast.Expr{expr}}}
}

func usedValues(mir *ir.MethodIR) map[int]bool {
	used := make(map[int]bool)
	for _, inst := range mir.Instructions {
		for _, arg := range inst.Args {
			used[arg.ID] = true
		}
	}
	return used
}
