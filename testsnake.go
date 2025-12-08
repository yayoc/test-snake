package testsnake

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

const (
	name = "testsnake"
	doc = "testsnake checks that test names in t.Run use snake_case convention"
	msg = "test name %q should use snake_case (e.g., \"my_test_case\")"
)

// Analyzer checks that test names in t.Run use snake_case
var Analyzer = &analysis.Analyzer{
	Name:             name,
	Doc:              doc,
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

			// Check if this is a selector expression (table-driven test)
			if sel, ok := firstArg.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					// Try to extract all test names from the table with their positions
					testCases := extractValuesWithPosFromRange(pass, ident, sel.Sel.Name)
					for _, tc := range testCases {
						if tc.value != "" && !isValidSnakeCase(tc.value) {
							pass.Reportf(tc.pos, msg, tc.value)
						}
					}
					if len(testCases) > 0 {
						return true
					}
				}
			}

			// Try to get the string value (either from literal or constant variable)
			testName := strVal(pass, firstArg)
			if testName == "" {
				return true
			}

			// Check if the test name follows snake_case
			if !isValidSnakeCase(testName) {
				pass.Reportf(callExpr.Pos(), msg, testName)
			}

			return true
		})
	}

	return nil, nil
}

type value interface{} // string | structConst

// valueWithPos holds a string value and its position in the source
type valueWithPos struct {
	value string
	pos   token.Pos
}

// strVal extracts the string value from an expression
func strVal(pass *analysis.Pass, expr ast.Expr) string {
	val, ok := eval(pass, expr)
	if ok {
		return val
	}

	return ""
}

// fieldName extracts the field name from a key expression
func fieldName(key ast.Expr) string {
	if ident, ok := key.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// eval evaluates an expression and returns its value
func eval(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	// string literal
	if lit, ok := expr.(*ast.BasicLit); ok {
		return strings.Trim(lit.Value, "\""), true
	}

	// constant value
	if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.Value != nil {
		if tv.Value.Kind() == constant.String {
			return strings.Trim(tv.Value.String(), "\""), true
		}
	}

	// selector expression (e.g., tt.name in table-driven tests)
	// This is handled at the call site in the main run function
	if _, ok := expr.(*ast.SelectorExpr); ok {
		return "", false
	}

	// identifier
	if ident, ok := expr.(*ast.Ident); ok {
		if obj := pass.TypesInfo.ObjectOf(ident); obj != nil {
			// Check if it's a constant
			if konst, ok := obj.(*types.Const); ok {
				if konst.Val().Kind() == constant.String {
					return strings.Trim(konst.Val().String(), "\""), true
				}
			}

			// Check if it's a variable
			if _, ok := obj.(*types.Var); ok {
				// Look for the variable's initialization
				if decl := findVarDecl(pass, ident); decl != "" {
					return decl, true
				}
			}
		}
	}

	return "", false
}

// extractFieldValueWithPos extracts a string field value and its position from a composite literal
func extractFieldValueWithPos(pass *analysis.Pass, comp *ast.CompositeLit, targetField string) (string, token.Pos) {
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			// Get the field name
			key := fieldName(kv.Key)
			if key == targetField {
				// Recursively evaluate the value
				if val, ok := eval(pass, kv.Value); ok {
					return val, kv.Value.Pos()
				}
			}
		}
	}
	return "", token.NoPos
}

// extractValuesWithPosFromRange extracts field values with positions from a slice used in a range statement
func extractValuesWithPosFromRange(pass *analysis.Pass, rangeVar *ast.Ident, fieldName string) []valueWithPos {
	obj := pass.TypesInfo.ObjectOf(rangeVar)
	if obj == nil {
		return nil
	}

	var values []valueWithPos
	var rangeExpr ast.Expr

	// Find the range statement where this variable is defined
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if rangeStmt, ok := n.(*ast.RangeStmt); ok {
				// Check if this range statement defines our variable
				if ident, ok := rangeStmt.Value.(*ast.Ident); ok {
					if pass.TypesInfo.ObjectOf(ident) == obj {
						rangeExpr = rangeStmt.X
						return false // Found it, stop searching
					}
				}
			}
			return true
		})
		if rangeExpr != nil {
			break
		}
	}

	if rangeExpr == nil {
		return nil
	}

	// Get the slice being ranged over
	var sliceLit *ast.CompositeLit

	// If rangeExpr is an identifier, find its declaration
	if ident, ok := rangeExpr.(*ast.Ident); ok {
		sliceObj := pass.TypesInfo.ObjectOf(ident)
		if sliceObj == nil {
			return nil
		}

		// Find the slice declaration
		for _, file := range pass.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if assign, ok := n.(*ast.AssignStmt); ok {
					for i, lhs := range assign.Lhs {
						if lhsIdent, ok := lhs.(*ast.Ident); ok {
							if pass.TypesInfo.ObjectOf(lhsIdent) == sliceObj && i < len(assign.Rhs) {
								if comp, ok := assign.Rhs[i].(*ast.CompositeLit); ok {
									sliceLit = comp
									return false
								}
							}
						}
					}
				}
				return true
			})
			if sliceLit != nil {
				break
			}
		}
	} else if comp, ok := rangeExpr.(*ast.CompositeLit); ok {
		// Direct composite literal in range
		sliceLit = comp
	}

	if sliceLit == nil {
		return nil
	}

	// Extract values with positions from each element in the slice
	for _, elt := range sliceLit.Elts {
		if comp, ok := elt.(*ast.CompositeLit); ok {
			// Extract the field value and position from this struct
			fieldVal, fieldPos := extractFieldValueWithPos(pass, comp, fieldName)
			if fieldVal != "" {
				values = append(values, valueWithPos{value: fieldVal, pos: fieldPos})
			}
		}
	}

	return values
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

