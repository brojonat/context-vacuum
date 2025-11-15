# Contributing to context-vacuum

Thank you for your interest in contributing to context-vacuum!

## Development Setup

1. **Prerequisites**
   - Go 1.25.1 or higher
   - sqlc (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
   - golangci-lint (optional, for linting)

2. **Clone and Build**
   ```bash
   git clone https://github.com/yourusername/context-vacuum.git
   cd context-vacuum
   go mod download
   make build
   ```

3. **Run Tests**
   ```bash
   make test
   ```

## Code Style

This project follows the guidelines in `CLAUDE.md`:

- Use explicit dependency injection
- Follow TDD (write tests first)
- Use `sqlc` for database operations
- Use `slog` for structured logging
- Keep functions focused and composable
- Avoid global state

## Making Changes

1. Create a feature branch: `git checkout -b feature/my-feature`
2. Make your changes following the code style
3. Add tests for new functionality
4. Run tests: `make test`
5. Run linter: `make lint` (if available)
6. Commit with a descriptive message
7. Push and create a pull request

## Testing

- Write unit tests for all new packages
- Ensure test coverage is maintained
- Use table-driven tests where appropriate
- Mock external dependencies

## Documentation

- Update README.md for user-facing changes
- Update CLAUDE.md for development patterns
- Add code comments for complex logic
- Document all public APIs

## Questions?

Open an issue or start a discussion!
