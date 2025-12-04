// Package main реализует генератор методов Reset() для структур.
//
// Утилита сканирует все пакеты проекта, находит структуры с комментарием
// generate:reset и генерирует для них методы Reset(), которые сбрасывают
// состояние структуры к начальным значениям.
//
// Использование:
//
//	go run ./cmd/reset/main.go
//
// Для каждого пакета со структурами создаётся файл reset.gen.go.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// generateComment — маркер комментария для генерации метода Reset().
const generateComment = "generate:reset"

// structInfo содержит информацию о структуре для генерации метода Reset().
//
// name — имя структуры.
// fields — список полей структуры.
// structType — AST-узел структуры.
type structInfo struct {
	name       string
	fields     []*ast.Field
	structType *ast.StructType
}

// "BURN_BABY_BURN" - Apollo 11.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run выполняет основную логику генератора.
//
// Сканирует все пакеты проекта, находит структуры с маркером generate:reset
// и генерирует для них методы Reset() в файлах reset.gen.go.
//
// Возвращает ошибку при неудаче сканирования или генерации.
func run() error {
	// Корневая директория проекта (текущая директория).
	rootDir := "."

	// Находим все пакеты со структурами, которым нужен метод Reset().
	packagesToGenerate := make(map[string][]structInfo)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Пропускаем директории vendor, .git и сгенерированные файлы.
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Обрабатываем только .go файлы (кроме сгененых).
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".gen.go") {
			return nil
		}

		structs, err := findStructsWithResetComment(path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		if len(structs) > 0 {
			dir := filepath.Dir(path)
			packagesToGenerate[dir] = append(packagesToGenerate[dir], structs...)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Генерируем файлы reset.gen.go для каждого пакета
	for pkgDir, structs := range packagesToGenerate {
		if err := generateResetFile(pkgDir, structs); err != nil {
			return fmt.Errorf("failed to generate reset file for %s: %w", pkgDir, err)
		}
		fmt.Printf("Generated reset.gen.go for package %s\n", pkgDir)
	}

	if len(packagesToGenerate) == 0 {
		fmt.Println("No structs with // generate:reset comment found")
	}

	return nil
}

// findStructsWithResetComment находит все структуры в файле с комментарием generate:reset.
//
// filename — путь к .go файлу для анализа.
//
// Возвращает список структур с информацией о полях и саму ошибку парсинга, если есть.
func findStructsWithResetComment(filename string) ([]structInfo, error) {
	// Парсим файл.
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var structs []structInfo

	// Создаём карту комментариев для связывания их с объявлениями.
	cmap := ast.NewCommentMap(fset, node, node.Comments)

	ast.Inspect(node, func(n ast.Node) bool {
		// Ищем общие объявления.
		genDecl, ok := n.(*ast.GenDecl)
		if !ok {
			return true
		}

		// Проверяем комментарии, связанные с объявлением.
		hasResetComment := false
		if genDecl.Doc != nil {
			for _, comment := range genDecl.Doc.List {
				if strings.Contains(comment.Text, generateComment) {
					hasResetComment = true
					break
				}
			}
		}

		// Также проверяем комментарии из карты комментариев.
		if !hasResetComment {
			if comments := cmap[genDecl]; comments != nil {
				for _, commentGroup := range comments {
					for _, comment := range commentGroup.List {
						if strings.Contains(comment.Text, generateComment) {
							hasResetComment = true
							break
						}
					}
					if hasResetComment {
						break
					}
				}
			}
		}

		if !hasResetComment {
			return true
		}

		// Ищем спецификации типов внутри объявления.
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Проверяем, что это структура.
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			structs = append(structs, structInfo{
				name:       typeSpec.Name.Name,
				fields:     structType.Fields.List,
				structType: structType,
			})
		}

		return true
	})

	return structs, nil
}

// generateResetFile генерирует файл reset.gen.go с методами Reset() для структур пакета.
//
// pkgDir — директория пакета.
// structs — список структур для генерации методов.
//
// Возвращает ошибку при неудаче генерации или записи файла.
func generateResetFile(pkgDir string, structs []structInfo) error {
	// Получаем имя пакета из существующих файлов.
	pkgName, err := getPackageName(pkgDir)
	if err != nil {
		return err
	}

	// Собираем необходимые импорты.
	imports := make(map[string]bool)
	for _, s := range structs {
		collectImports(s.fields, imports)
	}

	var buf bytes.Buffer

	// Записываем заголовок файла.
	buf.WriteString("// Code generated by cmd/reset. DO NOT EDIT.\n\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

	// Записываем импорты, если они нужны.
	if len(imports) > 0 {
		buf.WriteString("import (\n")
		for imp := range imports {
			buf.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}
		buf.WriteString(")\n\n")
	}

	// Генерируем метод Reset() для каждой структуры.
	for _, s := range structs {
		buf.WriteString(generateResetMethod(s))
		buf.WriteString("\n")
	}

	// Форматируем сгенерированный код.
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Если форматирование не удалось, выводим неформатированный код для отладки.
		return fmt.Errorf("failed to format generated code: %w\nUnformatted code:\n%s", err, buf.String())
	}

	// Записываем в файл
	outputPath := filepath.Join(pkgDir, "reset.gen.go")
	if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// collectImports собирает импорты, необходимые для полей структуры.
//
// fields — список полей структуры.
// imports — карта для сохранения найденных импортов.
func collectImports(fields []*ast.Field, imports map[string]bool) {
	for _, field := range fields {
		collectImportsFromType(field.Type, imports)
	}
}

// collectImportsFromType рекурсивно собирает импорты из AST-узла типа.
//
// expr — AST-узел типа для анализа.
// imports — карта для сохранения найденных импортов.
func collectImportsFromType(expr ast.Expr, imports map[string]bool) {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		// Квалифицированный идентификатор вроде time.Time.
		if ident, ok := t.X.(*ast.Ident); ok {
			// Распространённые имена пакетов — сопоставляем с полными путями импорта.
			switch ident.Name {
			case "time":
				imports["time"] = true
			case "context":
				imports["context"] = true
			case "sync":
				imports["sync"] = true
			}
		}
	case *ast.StarExpr:
		collectImportsFromType(t.X, imports)
	case *ast.ArrayType:
		collectImportsFromType(t.Elt, imports)
	case *ast.MapType:
		collectImportsFromType(t.Key, imports)
		collectImportsFromType(t.Value, imports)
	case *ast.ChanType:
		collectImportsFromType(t.Value, imports)
	}
}

// getPackageName получает имя пакета из директории.
//
// dir — директория пакета.
//
// Возвращает имя пакета или ошибку, если пакет не найден.
func getPackageName(dir string) (string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") && !strings.HasSuffix(fi.Name(), ".gen.go")
	}, parser.PackageClauseOnly)

	if err != nil {
		return "", err
	}

	// Возвращаем первое найденное имя пакета.
	for name := range pkgs {
		return name, nil
	}

	return "", fmt.Errorf("no package found in directory %s", dir)
}

// generateResetMethod генерирует текст метода Reset() для структуры.
//
// s — информация о структуре (имя и поля).
//
// Возвращает текст метода Reset().
func generateResetMethod(s structInfo) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("func (r *%s) Reset() {\n", s.name))
	buf.WriteString("\tif r == nil {\n")
	buf.WriteString("\t\treturn\n")
	buf.WriteString("\t}\n\n")

	// Генерируем код сброса для каждого поля.
	for _, field := range s.fields {
		if len(field.Names) == 0 {
			// Встроенное поле — пропускаем.
			continue
		}

		for _, fieldName := range field.Names {
			resetCode := generateFieldReset(fieldName.Name, field.Type)
			buf.WriteString(resetCode)
		}
	}

	buf.WriteString("}\n")

	return buf.String()
}

// generateFieldReset генерирует код сброса для отдельного поля структуры.
//
// fieldName — имя поля.
// fieldType — AST-узел типа поля.
//
// Возвращает текст кода для сброса поля.
func generateFieldReset(fieldName string, fieldType ast.Expr) string {
	var buf bytes.Buffer

	switch t := fieldType.(type) {
	case *ast.Ident:
		// Примитивные типы или именованные типы.
		resetValue := getZeroValue(t.Name)
		if resetValue != "" {
			buf.WriteString(fmt.Sprintf("\tr.%s = %s\n", fieldName, resetValue))
		} else {
			// Может быть структурой с методом Reset().
			buf.WriteString(fmt.Sprintf("\tif resetter, ok := interface{}(&r.%s).(interface{ Reset() }); ok {\n", fieldName))
			buf.WriteString("\t\tresetter.Reset()\n")
			buf.WriteString("\t}\n")
		}

	case *ast.StarExpr:
		// Тип-указатель
		buf.WriteString(fmt.Sprintf("\tif r.%s != nil {\n", fieldName))

		switch elem := t.X.(type) {
		case *ast.Ident:
			// Указатель на базовый тип или именованный тип.
			resetValue := getZeroValue(elem.Name)
			if resetValue != "" {
				buf.WriteString(fmt.Sprintf("\t\t*r.%s = %s\n", fieldName, resetValue))
			} else {
				// Указатель на структуру — пытаемся вызвать Reset().
				buf.WriteString(fmt.Sprintf("\t\tif resetter, ok := interface{}(r.%s).(interface{ Reset() }); ok {\n", fieldName))
				buf.WriteString("\t\t\tresetter.Reset()\n")
				buf.WriteString("\t\t}\n")
			}
		case *ast.SelectorExpr:
			// Указатель на квалифицированный тип (например, *time.Time).
			buf.WriteString(fmt.Sprintf("\t\tif resetter, ok := interface{}(r.%s).(interface{ Reset() }); ok {\n", fieldName))
			buf.WriteString("\t\t\tresetter.Reset()\n")
			buf.WriteString("\t\t} else {\n")
			buf.WriteString(fmt.Sprintf("\t\t\t*r.%s = %s{}\n", fieldName, formatType(elem)))
			buf.WriteString("\t\t}\n")
		case *ast.StructType:
			// Указатель на анонимную структуру.
			buf.WriteString(fmt.Sprintf("\t\tif resetter, ok := interface{}(r.%s).(interface{ Reset() }); ok {\n", fieldName))
			buf.WriteString("\t\t\tresetter.Reset()\n")
			buf.WriteString("\t\t}\n")
		default:
			// Прочие типы указателей.
			buf.WriteString(fmt.Sprintf("\t\tif resetter, ok := interface{}(r.%s).(interface{ Reset() }); ok {\n", fieldName))
			buf.WriteString("\t\t\tresetter.Reset()\n")
			buf.WriteString("\t\t}\n")
		}

		buf.WriteString("\t}\n")

	case *ast.ArrayType:
		if t.Len == nil {
			// Слайс
			buf.WriteString(fmt.Sprintf("\tr.%s = r.%s[:0]\n", fieldName, fieldName))
		} else {
			// Массив — сбрасываем каждый элемент.
			// Для простоты пытаемся вызвать Reset() для элементов массива.
			// или устанавливаем нулевое значение.
			buf.WriteString(fmt.Sprintf("\tfor i := range r.%s {\n", fieldName))

			switch elem := t.Elt.(type) {
			case *ast.Ident:
				resetValue := getZeroValue(elem.Name)
				if resetValue != "" {
					buf.WriteString(fmt.Sprintf("\t\tr.%s[i] = %s\n", fieldName, resetValue))
				} else {
					buf.WriteString(fmt.Sprintf("\t\tif resetter, ok := interface{}(&r.%s[i]).(interface{ Reset() }); ok {\n", fieldName))
					buf.WriteString("\t\t\tresetter.Reset()\n")
					buf.WriteString("\t\t}\n")
				}
			case *ast.StarExpr:
				buf.WriteString(fmt.Sprintf("\t\tif r.%s[i] != nil {\n", fieldName))
				buf.WriteString(fmt.Sprintf("\t\t\tif resetter, ok := interface{}(r.%s[i]).(interface{ Reset() }); ok {\n", fieldName))
				buf.WriteString("\t\t\t\tresetter.Reset()\n")
				buf.WriteString("\t\t\t}\n")
				buf.WriteString("\t\t}\n")
			default:
				buf.WriteString(fmt.Sprintf("\t\tif resetter, ok := interface{}(&r.%s[i]).(interface{ Reset() }); ok {\n", fieldName))
				buf.WriteString("\t\t\tresetter.Reset()\n")
				buf.WriteString("\t\t}\n")
			}

			buf.WriteString("\t}\n")
		}

	case *ast.MapType:
		// Мапа.
		buf.WriteString(fmt.Sprintf("\tclear(r.%s)\n", fieldName))

	case *ast.ChanType:
		// Канал — не можем по-настоящему сбросить, оставляем как есть.
		// Или можно установить в nil, но это может быть нежелательно.

	case *ast.InterfaceType:
		// Интерфейс — пытаемся вызвать Reset(), если доступен.
		buf.WriteString(fmt.Sprintf("\tif resetter, ok := r.%s.(interface{ Reset() }); ok && r.%s != nil {\n", fieldName, fieldName))
		buf.WriteString("\t\tresetter.Reset()\n")
		buf.WriteString("\t}\n")

	case *ast.SelectorExpr:
		// Квалифицированный тип (например, time.Time).
		// Пытаемся вызвать Reset() или установить нулевое значение.
		buf.WriteString(fmt.Sprintf("\tif resetter, ok := interface{}(&r.%s).(interface{ Reset() }); ok {\n", fieldName))
		buf.WriteString("\t\tresetter.Reset()\n")
		buf.WriteString("\t} else {\n")
		buf.WriteString(fmt.Sprintf("\t\tr.%s = %s{}\n", fieldName, formatType(t)))
		buf.WriteString("\t}\n")

	case *ast.StructType:
		// Анонимная структура.
		buf.WriteString(fmt.Sprintf("\tif resetter, ok := interface{}(&r.%s).(interface{ Reset() }); ok {\n", fieldName))
		buf.WriteString("\t\tresetter.Reset()\n")
		buf.WriteString("\t}\n")

	default:
		// Для прочих типов пытаемся вызвать метод Reset().
		buf.WriteString(fmt.Sprintf("\tif resetter, ok := interface{}(&r.%s).(interface{ Reset() }); ok {\n", fieldName))
		buf.WriteString("\t\tresetter.Reset()\n")
		buf.WriteString("\t}\n")
	}

	return buf.String()
}

// getZeroValue возвращает нулевое значение для примитивного типа.
//
// typeName — имя типа.
//
// Возвращает строковое представление нулевого значения или пустую строку, если тип не примитивный.
func getZeroValue(typeName string) string {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64":
		return "0"
	case "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return "0"
	case "float32", "float64":
		return "0"
	case "complex64", "complex128":
		return "0"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "byte":
		return "0"
	case "rune":
		return "0"
	default:
		return ""
	}
}

// ********************
// LITLE TRASH FUNCTION
// ********************

// formatType форматирует AST-узел типа в строку.
//
// expr — AST-узел типа.
//
// Возвращает строковое представление типа.
func formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", formatType(t.X), t.Sel.Name)
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + formatType(t.Elt)
		}
		return fmt.Sprintf("[%s]%s", formatType(t.Len), formatType(t.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", formatType(t.Key), formatType(t.Value))
	default:
		return ""
	}
}
