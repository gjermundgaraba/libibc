# CLAUDE.md - libibc Coding Guidelines

## Build Commands
- Build: `go build ./...`
- Test all: `go test ./...`
- Test single file: `go test ./chains/cosmos/cosmos_test.go`
- Test single function: `go test -run TestGetPacketV1 ./chains/cosmos`
- Lint: `go vet ./...`
- Format code: `gofmt -w .`

## Code Style Guidelines
- **Error Handling**: Use `github.com/pkg/errors` for error wrapping - always wrap errors with context
- **Imports**: Group standard library, external dependencies, and internal imports
- **Naming**: Use CamelCase for exported symbols, camelCase for non-exported
- **Testing**: Use `github.com/stretchr/testify` for assertions - prefer `require` for test failures
- **Comments**: Don't add a bunch of useless comments. Clear code over comments. Always.
- **File Organization**: Place interfaces at package level, implementation below
- **Coding Style**: Only implement what is required, never add unnecessary functionality or checks that are not required.

## Project Structure
- **cmd/**: Command line tools
- **chains/**: Chain-specific implementations 
- **ibc/**: Core IBC data types and functionality

Always use absolute paths for file operations and full package paths for imports.

## AI Behavior Guidelines
- Always make sure code builds before considering yourself done
