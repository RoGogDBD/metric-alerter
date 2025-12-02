package linter

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "linter",
	Doc:  "reports uses of builtin panic and log.Fatal/os.Exit outside main.main",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Проходим по всем файлам в пакете.
	for _, file := range pass.Files {
		// Записываем имя пакета.
		pkgName := file.Name.Name
		// Проходим по всем объявлениям в файле.
		for _, decl := range file.Decls {
			// Если это функция, то чекаем её тело.
			if fDecl, ok := decl.(*ast.FuncDecl); ok && fDecl.Body != nil {
				funcName := fDecl.Name.Name

				ast.Inspect(fDecl.Body, func(node ast.Node) bool {
					checkCall(pass, node, funcName, pkgName)
					return true
				})
			} else {
				ast.Inspect(decl, func(node ast.Node) bool {
					checkCall(pass, node, "", pkgName)
					return true
				})
			}
		}
	}
	return nil, nil
}

// *************************************************************************************************************************************************
// Решил протестить, через, напрямую инспектировать *ast.CallExpr и использовать pass.TypesInfo для определения вызываемой функции и ее контекста :o
// *************************************************************************************************************************************************
func checkCall(pass *analysis.Pass, node ast.Node, funcName string, pkgName string) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}
	// Делаем проверку на вызов panic.
	if id, ok := call.Fun.(*ast.Ident); ok {
		if id.Name == "panic" {
			// Проверяем, что это именно встроенный panic, а не переопределённый.
			obj := pass.TypesInfo.Uses[id]
			if obj != nil && obj.Pkg() == nil {
				// Если это встроенный panic, то сообщаем об этом.
				pass.Reportf(id.Pos(), "use of builtin panic is discouraged")
			}
			return
		}
	}
	// Типо проверка на вызов log.Fatal или os.Exit.
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	// ************************************************
	// УЖАСТНЫЙ БЛОК: Безопасная проверка на *ast.Ident
	// ************************************************
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		// Если не идентификатор - выходим.
		return
	}

	// Получили объект пакета из селектора.
	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return
	}
	// Проверяем, что это именно пакет.
	pkgNameObj, ok := obj.(*types.PkgName)
	if !ok {
		return
	}

	// ************************************************
	// ************************************************

	// Получили путь пакета.
	pkgPath := pkgNameObj.Imported().Path()

	// Проверка условий ТЗ.
	if !(pkgName == "main" && funcName == "main") {
		// Если живы, значит вызов происходит вне main.main.
		switch pkgPath {
		// Проверяем на log.Fatal.
		case "log":
			if sel.Sel.Name == "Fatal" || sel.Sel.Name == "Fatalf" || sel.Sel.Name == "Fatalln" {
				pass.Reportf(sel.Sel.Pos(), "call to log.Fatal or os.Exit outside main.main")
			}
		// Проверяем на os.Exit.
		case "os":
			if sel.Sel.Name == "Exit" {
				pass.Reportf(sel.Sel.Pos(), "call to log.Fatal or os.Exit outside main.main")
			}
		}
	}
}
