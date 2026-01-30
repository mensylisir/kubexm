package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func fixStepFiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", path, err)
			return nil
		}

		modified := false

		ast.Inspect(node, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Check if this is a Run method of a Step
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				if sel, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
					typeName := sel.X.(*ast.Ident).Name
					if strings.HasSuffix(typeName, "Step") && funcDecl.Name.Name == "Run" {
						// Check signature: (ctx runtime.ExecutionContext) error
						if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) == 1 {
							if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
								// Change from error to (*types.StepResult, error)
								result := funcDecl.Type.Results.List[0]
								if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "error" {
									result.Type = &ast.StarExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: "types"},
											Sel: &ast.Ident{Name: "StepResult"},
										},
									}
									// Add second return value: error
									funcDecl.Type.Results.List = append(funcDecl.Type.Results.List, &ast.Field{
										Type: &ast.Ident{Name: "error"},
									})
									modified = true
								}
							}
						}
					}
				}
			}

			return true
		})

		if modified {
			fmt.Printf("Modified: %s\n", path)
			// Write the file back
			// Note: This is a simplified version; proper implementation would need more careful formatting
		}

		return nil
	})
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run fix_steps.go <directory>")
		os.Exit(1)
	}
	dir := os.Args[1]
	if err := fixStepFiles(dir); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
