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

func processFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	modified := false
	needsTypesImport := false

	// Check if types import is needed
	for _, imp := range node.Imports {
		if imp.Path != nil && strings.Contains(imp.Path.Value, "kubexm/pkg/types") {
			needsTypesImport = true
			break
		}
	}

	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			recvType := funcDecl.Recv.List[0].Type
			if starExpr, ok := recvType.(*ast.StarExpr); ok {
				if ident, ok := starExpr.X.(*ast.Ident); ok {
					if strings.HasSuffix(ident.Name, "Step") && funcDecl.Name.Name == "Run" {
						if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) == 1 {
							if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
								result := funcDecl.Type.Results.List[0]
								if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "error" {
									result.Type = &ast.StarExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: "types"},
											Sel: &ast.Ident{Name: "StepResult"},
										},
									}
									funcDecl.Type.Results.List = append(funcDecl.Type.Results.List, &ast.Field{
										Type: &ast.Ident{Name: "error"},
									})
									modified = true
									needsTypesImport = true
								}
							}
						}
					}
					// Also fix Rollback if it returns (error) instead of (error)
					if ident.Name == "Step" && funcDecl.Name.Name == "Rollback" {
						if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
							result := funcDecl.Type.Results.List[0]
							if _, ok := result.Type.(*ast.Ident); ok {
								// Rollback signature is already correct: (ctx) error
							}
						}
					}
				}
			}
		}

		return true
	})

	// Add types import if needed
	if needsTypesImport {
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
			modified = true
		}
	}

	if modified {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, node); err != nil {
			return fmt.Errorf("format error: %w", err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
		fmt.Printf("Fixed: %s\n", path)
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run fix_steps_v2.go <directory>")
		os.Exit(1)
	}

	dir := os.Args[1]
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if err := processFile(path); err != nil {
			fmt.Printf("Error processing %s: %v\n", path, err)
		} else {
			count++
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processed %d files\n", count)
}
