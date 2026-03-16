# Contributing

Thank you for your interest in Kube Ops Agent! Contributions via Issues and Pull Requests are welcome.

## Development Environment

- Go 1.25 or higher
- Configured Kubernetes cluster (optional, for integration tests)

## Development Workflow

1. **Fork this repository** and clone locally
2. **Create a branch**: `git checkout -b feature/your-feature` or `fix/your-fix`
3. **Make changes**, ensure `go build ./...` and `go test ./...` pass
4. **Commit changes**: Use clear commit messages
5. **Push branch**: `git push origin feature/your-feature`
6. **Open a Pull Request** describing the changes and motivation

## Code Style

- Follow Go [Effective Go](https://go.dev/doc/effective_go) and [Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- Format with `gofmt` or `goimports`
- Add tests for new features

## Testing

```bash
go test ./...
```

## Pull Request Checklist

- [ ] Code builds successfully
- [ ] Tests pass
- [ ] Documentation updated (if needed)
- [ ] Commit messages are clear and concise
