# WUT Tests

This directory contains all tests for the WUT project, organized by type.

## Directory Structure

```
tests/
├── unit/              # Unit tests for individual components
├── integration/       # Integration tests for component interactions
├── benchmarks/        # Performance benchmarks
└── README.md         # This file
```

## Running Tests

### All Tests
```bash
go test ./tests/...
```

### Unit Tests Only
```bash
go test ./tests/unit/...
```

### Integration Tests Only
```bash
go test ./tests/integration/...
```

### Benchmarks
```bash
go test -bench=. ./tests/benchmarks/...
```

### With Coverage
```bash
go test -cover ./tests/...
```

### Verbose Output
```bash
go test -v ./tests/...
```

### Race Detection
```bash
go test -race ./tests/...
```

## Test Organization

### Unit Tests (`tests/unit/`)
Tests for individual functions and methods in isolation.

**Files:**
- `ai_embeddings_test.go` - AI embedding functionality
- `util_math_test.go` - Utility math functions
- `fuzzy_matcher_test.go` - Fuzzy matching algorithms
- `parser_test.go` - Command parser

**Coverage Target:** 80%+

### Integration Tests (`tests/integration/`)
Tests for interactions between multiple components.

**Files:**
- `context_search_test.go` - Context-aware search integration

**Coverage Target:** 60%+

### Benchmarks (`tests/benchmarks/`)
Performance benchmarks for critical paths.

**Files:**
- `benchmark_test.go` - All performance benchmarks

**Metrics:**
- Command embedding performance
- Fuzzy matching speed
- Parser throughput
- Search engine performance

## Writing Tests

### Unit Test Template
```go
package unit

import (
    "testing"
    "wut/internal/yourpackage"
)

func TestYourFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := yourpackage.YourFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Benchmark Template
```go
package benchmarks

import (
    "testing"
    "wut/internal/yourpackage"
)

func BenchmarkYourFunction(b *testing.B) {
    // Setup
    input := "test"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        yourpackage.YourFunction(input)
    }
}
```

## Test Coverage

Current coverage by package:
- `internal/util`: 100%
- `internal/core`: 58.6%
- `pkg/fuzzy`: 55.6%
- `internal/search`: 44.0%
- `internal/ai`: 22.5%

**Overall:** 45%+

## CI/CD Integration

Tests are automatically run on:
- Every push to main/develop
- Every pull request
- Before releases

See `.github/workflows/ci.yml` for details.

## Best Practices

1. **Use table-driven tests** for multiple test cases
2. **Test edge cases** (empty input, nil values, etc.)
3. **Use descriptive test names** that explain what's being tested
4. **Keep tests independent** - no shared state
5. **Mock external dependencies** in unit tests
6. **Add benchmarks** for performance-critical code
7. **Aim for 80%+ coverage** in unit tests
8. **Document complex test scenarios**

## Troubleshooting

### Tests Failing Locally
```bash
# Clean test cache
go clean -testcache

# Run with verbose output
go test -v ./tests/...
```

### Slow Tests
```bash
# Run with timeout
go test -timeout 30s ./tests/...

# Skip slow tests
go test -short ./tests/...
```

### Coverage Issues
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./tests/...
go tool cover -html=coverage.out
```

## Contributing

When adding new features:
1. Write tests first (TDD)
2. Ensure tests pass locally
3. Check coverage doesn't decrease
4. Add benchmarks for performance-critical code
5. Update this README if adding new test categories

## Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Go Test Comments](https://github.com/golang/go/wiki/TestComments)
