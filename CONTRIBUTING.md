# Contributing

Thank you for considering contributing to our project! This document outlines the process for contributing.

## Development Setup

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR-USERNAME/claude-squad.git`
3. Add the upstream repository: `git remote add upstream https://github.com/smtg-ai/claude-squad.git`
4. Install dependencies: `go mod download`

## Code Standards

### Lint

You can run the following command to lint the code:

```bash
gofmt -w .
```

### Testing

Please include tests for new features or bug fixes.

### Adding a new AI assistant

To add a new AI assistant, you'll need to:

1.  Update `config/config.go` to detect the new assistant's command-line tool.
2.  Update `session/tmux/tmux.go` to handle the new assistant's prompts.
3.  Update `README.md` to include instructions for using the new assistant.

## Questions?

Feel free to open an issue for any questions about contributing.

