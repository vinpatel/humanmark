# Contributing to HumanMark

Thank you for your interest in contributing to HumanMark! This document provides guidelines and information for contributors.

## How to Contribute

### Reporting Bugs

1. Check [existing issues](https://github.com/humanmark/humanmark/issues) first
2. Create a new issue with:
   - Clear title
   - Steps to reproduce
   - Expected vs actual behavior
   - Go version and OS

### Reporting False Positives/Negatives

Help us improve accuracy! When our detection is wrong:

1. Open an issue with label `accuracy`
2. Include:
   - The text/content that was misclassified
   - What HumanMark said (human/AI, score)
   - What it actually was
   - Source (if applicable)

### Suggesting Features

1. Open an issue with label `enhancement`
2. Describe the use case
3. Explain why it would benefit users

### Submitting Code

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `go test ./... -v`
5. Run linter: `go vet ./...`
6. Commit with clear message: `git commit -m "Add feature X"`
7. Push: `git push origin feature/my-feature`
8. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Git

### Setup

```bash
git clone https://github.com/humanmark/humanmark.git
cd humanmark
go mod download
go run cmd/api/main.go
```

### Running Tests

```bash
# All tests
go test ./... -v

# With coverage
go test ./... -cover

# Specific package
go test ./internal/service/... -v
```

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Run `go vet` to catch issues
- Keep functions small and focused
- Add comments for exported functions

## Project Structure

```
humanmark/
├── cmd/api/           # Main application
├── internal/
│   ├── config/        # Configuration
│   ├── handler/       # HTTP handlers
│   ├── middleware/    # HTTP middleware
│   ├── repository/    # Data storage
│   └── service/       # Detection algorithms ← Most contributions here
├── pkg/logger/        # Logging utilities
└── scripts/           # Helper scripts
```

## Areas for Contribution

### High Impact

- **Improve detection algorithms** (`internal/service/*_analyzer.go`)
- **Add test cases** (especially edge cases)
- **Benchmark against real AI outputs**

### Medium Impact

- Documentation improvements
- Error handling improvements
- Performance optimizations

### Good First Issues

Look for issues labeled `good first issue`:
- Typo fixes
- Documentation clarifications
- Simple test additions

## Detection Algorithm Guidelines

If improving detection algorithms:

1. **Document your changes**: Explain what signal you're adding and why
2. **Provide evidence**: Link to research or examples that support the approach
3. **Add tests**: Include test cases showing the improvement
4. **Don't break existing tests**: Run full test suite
5. **Consider false positives**: New signals can incorrectly flag human content

### Adding a New Signal

```go
// In text_analyzer.go

// 1. Add weight constant
const DefaultNewSignalWeight = 0.10

// 2. Add to weights struct
type TextAnalyzerWeights struct {
    // ... existing weights
    NewSignal float64
}

// 3. Implement the signal analysis
func (a *TextAnalyzer) analyzeNewSignal(tokens []string) float64 {
    // Your implementation
    // Return 0.0 (human-like) to 1.0 (AI-like)
}

// 4. Add to main Analyze function
// 5. Add tests in text_analyzer_test.go
```

## Pull Request Checklist

- [ ] Code compiles without errors
- [ ] All tests pass
- [ ] New code has tests
- [ ] Documentation updated (if applicable)
- [ ] Commit messages are clear
- [ ] PR description explains the change

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- No harassment or discrimination

## Questions?

- Open a [Discussion](https://github.com/humanmark/humanmark/discussions)
- Tag maintainers in issues if stuck

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for helping make AI detection transparent and accessible! 🙏
