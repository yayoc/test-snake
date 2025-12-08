package testlintdata

import "testing"

// Runner is a type with a Run method (not related to testing)
type Runner struct{}

func (r *Runner) Run(name string, fn func()) {
	// Some custom Run implementation
	fn()
}

func TestExample(t *testing.T) {
	// Good: snake_case test names
	t.Run("add_positive_numbers", func(t *testing.T) {
		// This should pass
	})

	t.Run("multiply_by_zero", func(t *testing.T) {
		// This should pass
	})

	t.Run("calculate_sum_with_negative", func(t *testing.T) {
		// This should pass
	})

	// Bad: camelCase test names
	t.Run("AddPositiveNumbers", func(t *testing.T) { // want "test name \"AddPositiveNumbers\" should use snake_case"
		// This should fail
	})

	t.Run("MultiplyNumbers", func(t *testing.T) { // want "test name \"MultiplyNumbers\" should use snake_case"
		// This should fail
	})

	// Bad: mixed case
	t.Run("Add_PositiveNumbers", func(t *testing.T) { // want "test name \"Add_PositiveNumbers\" should use snake_case"
		// This should fail
	})

	// Bad: leading underscore
	t.Run("_leading_underscore", func(t *testing.T) { // want "test name \"_leading_underscore\" should use snake_case"
		// This should fail
	})

	// Bad: trailing underscore
	t.Run("trailing_underscore_", func(t *testing.T) { // want "test name \"trailing_underscore_\" should use snake_case"
		// This should fail
	})

	// Bad: consecutive underscores
	t.Run("double__underscore", func(t *testing.T) { // want "test name \"double__underscore\" should use snake_case"
		// This should fail
	})

	// Good: This is NOT a t.Run call, so it should be ignored by the linter
	// even though the name is not snake_case
	runner := &Runner{}
	runner.Run("ThisIsNotATest", func() {
		// This should NOT be flagged
	})

	valid_name := "valid_snake"
	t.Run(valid_name, func(t *testing.T) {
		// This should pass
	})

	invalid_name := "invalidSnake"
	t.Run(invalid_name, func(t *testing.T) { // want "test name \"invalidSnake\" should use snake_case"
		// This should fail
	})

	valid_concat_name := "valid" + "_snake"
	t.Run(valid_concat_name, func(t *testing.T) {
		// This should pass
	})

	invalid_concat_name := "invalid" + "Snake"
	t.Run(invalid_name, func(t *testing.T) { // want "test name \"invalidSnake\" should use snake_case"
		// This should fail
	})
}

func TestParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{
			name: "invalid snake case", // want "test name \"invalid snake case\" should use snake_case"
			want: "foobar"
		},
		{
			name: "_invalid_snake_case_", // want "test name \"_invalid_snake_case_\" should use snake_case"
			want: "foobar"
		},
		{
			name: "valid_snake_case",
			want: "foobar"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
