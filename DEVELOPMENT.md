# Development Guide

## Quick Setup

### Install Development Version
```bash
# Install development version alongside production
make install-dev
```

This installs the development binary as `cs-dev`, allowing you to test changes without affecting your production setup.

### Usage
```bash
# Production version (from brew)
cs

# Development version (your local build)
cs-dev
```

### Development Workflow
```bash
# 1. Make changes to code
# 2. Test and build
make install-dev

# 3. Test your changes
cs-dev debug  # Check configuration
cs-dev         # Run with your changes

# 4. Compare with production
cs debug      # Production version
```

### Cleanup
```bash
# Remove development version
make uninstall-dev
```

## Installation Details

The `install-dev` target:
- 🍺 **With Homebrew**: Installs to `$(brew --prefix)/bin/cs-dev`
- 🐧 **Without Homebrew**: Installs to `/usr/local/bin/cs-dev`
- ✅ **Coexistence**: Works alongside production `cs` from `brew install claude-squad`
- 🔧 **Development**: Perfect for testing local changes

## Key Mapping Development

Test custom key mappings with:
```bash
cs-dev debug  # Show current key mappings
```

Your local `~/.claude-squad/config.json` will be used by `cs-dev`.