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

func processFile(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse error: %w", err)
	}

	modified := false

	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			recvType := funcDecl.Recv.List[0].Type
			if starExpr, ok := recvType.(*ast.StarExpr); ok {
				if ident, ok := starExpr.X.(*ast.Ident); ok {
					// Fix Run method
					if strings.HasSuffix(ident.Name, "Step") && funcDecl.Name.Name == "Run" {
						if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) == 1 {
							if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
								result := funcDecl.Type.Results.List[0]
								if resultIdent, ok := result.Type.(*ast.Ident); ok && resultIdent.Name == "error" {
									// Change return type
									result.Type = &ast.StarExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: "types"},
											Sel: &ast.Ident{Name: "StepResult"},
										},
									}
									// Add error return
									funcDecl.Type.Results.List = append(funcDecl.Type.Results.List, &ast.Field{
										Type: &ast.Ident{Name: "error"},
									})
									modified = true
								}
							}
						}
					}
					// Fix Rollback method signature if needed
					if strings.HasSuffix(ident.Name, "Step") && funcDecl.Name.Name == "Rollback" {
						if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) == 1 {
							if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
								result := funcDecl.Type.Results.List[0]
								if _, ok := result.Type.(*ast.Ident); ok {
									// Rollback should return error
								}
							}
						}
					}
				}
			}
		}

		return true
	})

	// Add types import if modified
	if modified {
		hasTypesImport := false
		for _, imp := range node.Imports {
			if imp.Path != nil && strings.Contains(imp.Path.Value, "kubexm/pkg/types") {
				hasTypesImport = true
				break
			}
		}
		if !hasTypesImport {
			node.Imports = append(node.Imports, &ast.ImportSpec{
				Path: &ast.BasicLit{Value: `"github.com/mensylisir/kubexm/pkg/types"`},
			})
		}
	}

	if modified {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, node); err != nil {
			return false, fmt.Errorf("format error: %w", err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
			return false, fmt.Errorf("write error: %w", err)
		}
		return true, nil
	}

	return false, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run fix_steps_v3.go <directory>")
		os.Exit(1)
	}

	dir := os.Args[1]
	fixed := 0
	errors := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fixedFile, err := processFile(path)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", path, err)
			errors++
		} else if fixedFile {
			fmt.Printf("Fixed: %s\n", path)
			fixed++
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFixed %d files, %d errors\n", fixed, errors)
}
