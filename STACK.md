# Tech Stack

## Language
Go 1.24+

## Dependencies
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **Config:** [Viper](https://github.com/spf13/viper) for `.envref.yaml`
- **Linting:** [golangci-lint](https://golangci-lint.run/) with `.golangci.yml`
- **Testing:** Standard `testing` + [testify](https://github.com/stretchr/testify)
- **Build:** Makefile + [GoReleaser](https://goreleaser.com/)

## Empty Workspace Signal
No `go.mod` file in the repository root.

## Bootstrapping
1. `go mod init github.com/xcke/envref`
2. `mkdir -p cmd/envref internal pkg`
3. Create minimal `cmd/envref/main.go` with a Cobra root command
4. Verify: `go build ./...`

## Verification
Run in order before every commit:

```bash
go build ./...       # compiles without errors
go vet ./...         # catches common mistakes
go test ./...        # all tests pass
golangci-lint run    # lint checks pass
```

## Smoke Test

```bash
go run ./cmd/envref --help
```

Confirm the CLI help output looks correct for the current state.

## Conventions

### Project Layout
- `cmd/` — binary entrypoints
- `internal/` — not importable by external consumers
- `pkg/` — importable by other projects

### Code Style
- All exported functions and types must have doc comments
- Use `error` return values — do not panic in library code
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Table-driven tests with `t.Run()` subtests
- No global mutable state — pass dependencies via constructor injection
- Keep `main.go` minimal — wire things together and call `cmd.Execute()`
- Prefer composition over inheritance (interfaces over embedding chains)

### CLI Changes
For CLI command changes, run the Smoke Test above to verify the command structure.
