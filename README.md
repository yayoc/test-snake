# testsnake - golangci-lint plugin for snake_case test names

A golangci-lint plugin that enforces snake_case naming convention for test names passed to `t.Run()`.

## Overview

This linter checks that all test names in `t.Run()` calls follow the snake_case convention instead of camelCase or PascalCase.

### Valid Examples

```go
t.Run("add_positive_numbers", func(t *testing.T) { ... })
t.Run("calculate_sum", func(t *testing.T) { ... })
t.Run("http_handler_test", func(t *testing.T) { ... })
```

### Invalid Examples

```go
t.Run("AddPositiveNumbers", func(t *testing.T) { ... })  // PascalCase
t.Run("calculateSum", func(t *testing.T) { ... })        // camelCase
t.Run("_leading_underscore", func(t *testing.T) { ... }) // leading underscore
t.Run("trailing_", func(t *testing.T) { ... })           // trailing underscore
t.Run("double__underscore", func(t *testing.T) { ... })  // consecutive underscores
```

## Installation

### Building the Plugin

```bash
go build -buildmode=plugin -o testsnake.so testsnake.go
```

### Configuration

Add the following to your `.golangci.yml`:

```yaml
linters-settings:
  custom:
    testsnake:
      path: ./testsnake.so
      description: Checks that test names passed to t.Run follow snake_case convention
      original-url: github.com/yayoc/test-snake

linters:
  enable:
    - testsnake
```

## Usage

Run golangci-lint as usual:

```bash
golangci-lint run
```

The plugin will automatically check all `*_test.go` files for snake_case violations in `t.Run()` calls.

## Development

### Running Tests

```bash
go test -v
```

### Project Structure

```
.
├── testsnake.go           # Main analyzer implementation
├── testsnake_test.go      # Analyzer tests
├── testsnake.so           # Compiled plugin (generated)
├── .golangci.yml          # Configuration example
├── testdata/              # Test fixtures
│   └── src/
│       └── testlintdata/
│           └── example_test.go
├── go.mod
└── README.md
```

## License

MIT
