package testsnake

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// Analyzer checks that test names in t.Run use snake_case
var Analyzer = &analysis.Analyzer{
	Name:             "testsnake",
	Doc:              "checks that test names passed to t.Run follow snake_case convention",
	Run:              run,
	RunDespiteErrors: true,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		// Only check test files
		if !strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if this is a call to *.Run()
			selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			// Check if the method name is "Run"
			if selExpr.Sel.Name != "Run" {
				return true
			}

			// Check if the receiver is a testing type (testing.T, testing.B, testing.F)
			if !isTestingType(pass, selExpr.X) {
				return true
			}

			// Check if there are at least 2 arguments (name and function)
			if len(callExpr.Args) < 2 {
				return true
			}

			// Get the first argument (test name)
			firstArg := callExpr.Args[0]

			// Check if it's a selector expression (table-driven test pattern like tt.name)
			if selExpr, ok := firstArg.(*ast.SelectorExpr); ok {
				// Handle table-driven tests
				testNames := extractTableTestNames(pass, file, selExpr, callExpr)
				for name, pos := range testNames {
					if !isValidSnakeCase(name) {
						pass.Reportf(pos, "test name %q should use snake_case (e.g., \"my_test_case\")", name)
					}
				}
				return true
			}

			// Try to get the string value (either from literal or constant variable)
			testName := getStringValue(pass, firstArg)
			if testName == "" {
				return true
			}

			// Check if the test name follows snake_case
			if !isValidSnakeCase(testName) {
				pass.Reportf(callExpr.Pos(), "test name %q should use snake_case (e.g., \"my_test_case\")", testName)
			}

			return true
		})
	}

	return nil, nil
}

// getStringValue extracts the string value from an expression
// It handles both string literals and variables/constants
func getStringValue(pass *analysis.Pass, expr ast.Expr) string {
	// Case 1: String literal
	if lit, ok := expr.(*ast.BasicLit); ok {
		return strings.Trim(lit.Value, "\"")
	}

	// Case 2: Identifier (variable or constant)
	if ident, ok := expr.(*ast.Ident); ok {
		// First check if it's a constant
		if obj := pass.TypesInfo.ObjectOf(ident); obj != nil {
			if konst, ok := obj.(*types.Const); ok {
				return strings.Trim(konst.Val().String(), "\"")
			}

			// For variables, try to find the assignment
			if _, ok := obj.(*types.Var); ok {
				// Look for the variable's initialization in the same function
				if decl := findVarDecl(pass, ident); decl != "" {
					return decl
				}
			}
		}
	}

	// Case 3: Constant expression
	tv, ok := pass.TypesInfo.Types[expr]
	if ok && tv.Value != nil {
		return strings.Trim(tv.Value.String(), "\"")
	}

	return ""
}

// findVarDecl tries to find the string literal value assigned to a variable
func findVarDecl(pass *analysis.Pass, ident *ast.Ident) string {
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return ""
	}

	var result string

	// Find the declaration
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			// Look for assignment statements
			if assign, ok := n.(*ast.AssignStmt); ok {
				for i, lhs := range assign.Lhs {
					if lhsIdent, ok := lhs.(*ast.Ident); ok {
						if pass.TypesInfo.ObjectOf(lhsIdent) == obj && i < len(assign.Rhs) {
							// Found the assignment
							if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok {
								result = strings.Trim(lit.Value, "\"")
								return false // Stop searching
							}
						}
					}
				}
			}
			return true
		})
		if result != "" {
			break
		}
	}

	return result
}

// extractTableTestNames extracts test names from table-driven tests
// It handles patterns like: for _, tt := range tests { t.Run(tt.name, ...) }
func extractTableTestNames(pass *analysis.Pass, file *ast.File, selExpr *ast.SelectorExpr, callExpr *ast.CallExpr) map[string]token.Pos {
	result := make(map[string]token.Pos)

	// Get the field name being accessed (e.g., "name" from tt.name)
	fieldName := selExpr.Sel.Name

	// Find the enclosing for/range statement
	var rangeStmt *ast.RangeStmt
	var found bool

	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}

		if rs, ok := n.(*ast.RangeStmt); ok {
			// Check if our callExpr is inside this range statement
			if rs.Pos() < callExpr.Pos() && callExpr.End() < rs.End() {
				rangeStmt = rs
				found = true
				return false
			}
		}
		return true
	})

	if rangeStmt == nil {
		return result
	}

	// Get the identifier of the slice being ranged over
	var sliceIdent *ast.Ident
	switch x := rangeStmt.X.(type) {
	case *ast.Ident:
		sliceIdent = x
	default:
		return result
	}

	// Find the slice declaration
	obj := pass.TypesInfo.ObjectOf(sliceIdent)
	if obj == nil {
		return result
	}

	// Find the composite literal (slice initialization)
	ast.Inspect(file, func(n ast.Node) bool {
		// Look for assignment or declaration
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				if lhsIdent, ok := lhs.(*ast.Ident); ok {
					if pass.TypesInfo.ObjectOf(lhsIdent) == obj && i < len(stmt.Rhs) {
						// Found the assignment - extract test names
						if compLit, ok := stmt.Rhs[i].(*ast.CompositeLit); ok {
							extractNamesFromCompositeLit(compLit, fieldName, result)
						}
					}
				}
			}
		case *ast.ValueSpec:
			for i, name := range stmt.Names {
				if pass.TypesInfo.ObjectOf(name) == obj && i < len(stmt.Values) {
					// Found the declaration - extract test names
					if compLit, ok := stmt.Values[i].(*ast.CompositeLit); ok {
						extractNamesFromCompositeLit(compLit, fieldName, result)
					}
				}
			}
		}
		return true
	})

	return result
}

// extractNamesFromCompositeLit extracts field values from a composite literal
func extractNamesFromCompositeLit(compLit *ast.CompositeLit, fieldName string, result map[string]token.Pos) {
	for _, elt := range compLit.Elts {
		if innerLit, ok := elt.(*ast.CompositeLit); ok {
			// Look through the key-value pairs in the struct literal
			for _, innerElt := range innerLit.Elts {
				if kv, ok := innerElt.(*ast.KeyValueExpr); ok {
					// Check if this is the field we're looking for
					if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == fieldName {
						// Extract the string value
						if lit, ok := kv.Value.(*ast.BasicLit); ok {
							testName := strings.Trim(lit.Value, "\"")
							result[testName] = lit.Pos()
						}
					}
				}
			}
		}
	}
}

// isTestingType checks if the expression is a testing type (testing.T, testing.B, testing.F)
func isTestingType(pass *analysis.Pass, expr ast.Expr) bool {
	typ := pass.TypesInfo.TypeOf(expr)
	if typ == nil {
		return false
	}

	// Get the string representation of the type
	typeStr := typ.String()
	types.Identical(typ, &types.Named{})

	// Check if it's a pointer to testing.T, testing.B, or testing.F
	return typeStr == "*testing.T" || typeStr == "*testing.B" || typeStr == "*testing.F"
}

// isValidSnakeCase checks if a string follows snake_case convention
// Valid examples: my_function, calculate_sum, http_handler, test123
// Invalid examples: MyFunction, CalculateSum, My_Function, _leading, trailing_
func isValidSnakeCase(name string) bool {
	// Empty string is invalid
	if name == "" {
		return false
	}

	// Check for uppercase letters
	for _, ch := range name {
		if unicode.IsUpper(ch) {
			return false
		}
	}

	// Check for valid snake_case pattern:
	// - lowercase letters, numbers, and underscores only
	// - cannot start or end with underscore
	// - no consecutive underscores
	snakeCasePattern := regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)
	return snakeCasePattern.MatchString(name)
}

