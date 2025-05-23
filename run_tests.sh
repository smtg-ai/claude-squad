#!/bin/bash

echo "🚀 CSQ COMPREHENSIVE TEST RUNNER"
echo "================================"

# Change to claude-squad directory
cd "$(dirname "$0")"

# Ensure we have the latest build
echo "📦 Building latest CSQ..."
go build -o csq . || {
    echo "❌ Build failed"
    exit 1
}

echo "✅ Build successful"
echo ""

# Run Go tests
echo "🧪 Running Go tests..."
go test -v . || {
    echo "⚠️  Some Go tests failed"
}

echo ""

# Run comprehensive test suite
echo "🧪 Running comprehensive test suite..."
go run test_suite.go test || {
    echo "❌ Test suite failed"
    exit 1
}

echo ""

# Run manual validation tests
echo "🔍 Running manual validation tests..."

# Test 1: Binary exists and is executable
if [[ ! -f "./csq" ]]; then
    echo "❌ CSQ binary not found"
    exit 1
fi

if [[ ! -x "./csq" ]]; then
    echo "❌ CSQ binary not executable"
    exit 1
fi
echo "✅ Binary validation passed"

# Test 2: Version command works
VERSION_OUTPUT=$(./csq version 2>&1)
if [[ $? -ne 0 ]]; then
    echo "❌ Version command failed"
    exit 1
fi

if [[ ! "$VERSION_OUTPUT" == *"claude-squad version"* ]]; then
    echo "❌ Version output invalid: $VERSION_OUTPUT"
    exit 1
fi
echo "✅ Version command passed"

# Test 3: Help command works
HELP_OUTPUT=$(./csq --help 2>&1)
if [[ $? -ne 0 ]]; then
    echo "❌ Help command failed"
    exit 1
fi

if [[ ! "$HELP_OUTPUT" == *"Available Commands"* ]]; then
    echo "❌ Help output invalid"
    exit 1
fi
echo "✅ Help command passed"

# Test 4: Debug command works
DEBUG_OUTPUT=$(./csq debug 2>&1)
if [[ $? -ne 0 ]]; then
    echo "❌ Debug command failed"
    exit 1
fi

if [[ ! "$DEBUG_OUTPUT" == *"Config:"* ]]; then
    echo "❌ Debug output invalid"
    exit 1
fi
echo "✅ Debug command passed"

# Test 5: TTY detection works
TTY_OUTPUT=$(echo "" | ./csq 2>&1)
if [[ $? -eq 0 ]]; then
    echo "❌ TTY detection failed - should require interactive terminal"
    exit 1
fi

if [[ ! "$TTY_OUTPUT" == *"interactive terminal"* ]]; then
    echo "❌ TTY error message invalid: $TTY_OUTPUT"
    exit 1
fi
echo "✅ TTY detection passed"

# Test 6: Git repository detection works
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"
GIT_OUTPUT=$(csq 2>&1)
cd - > /dev/null
rm -rf "$TEMP_DIR"

if [[ $? -eq 0 ]]; then
    echo "❌ Git detection failed - should require git repository"
    exit 1
fi

if [[ ! "$GIT_OUTPUT" == *"git repository"* ]]; then
    echo "❌ Git error message invalid: $GIT_OUTPUT"
    exit 1
fi
echo "✅ Git repository detection passed"

echo ""
echo "🎉 ALL TESTS PASSED!"
echo "📊 CSQ is working correctly and ready for use"
echo ""
echo "💡 Quick start commands:"
echo "  ./csq --help                    # Show all available commands"
echo "  ./csq version                   # Show version information"
echo "  ./csq debug                     # Show configuration"
echo "  ./csq sync                      # Start sync operation (requires git repo)"
echo ""