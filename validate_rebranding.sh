#!/bin/bash

echo "🔍 CHRONOS REBRANDING VALIDATION"
echo "================================"

ERRORS=0

echo "✅ Testing binary functionality..."

# Test binary exists and works
if [[ ! -f "./chronos" ]]; then
    echo "❌ chronos binary not found"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ chronos binary exists"
fi

# Test help command shows Chronos
HELP_OUTPUT=$(./chronos --help 2>&1)
if [[ "$HELP_OUTPUT" == *"Chronos - A terminal-based session manager"* ]]; then
    echo "✅ Help shows 'Chronos' branding"
else
    echo "❌ Help still shows old branding"
    ERRORS=$((ERRORS + 1))
fi

# Test version command shows chronos
VERSION_OUTPUT=$(./chronos version 2>&1)
if [[ "$VERSION_OUTPUT" == *"chronos version"* ]]; then
    echo "✅ Version shows 'chronos' branding"
else
    echo "❌ Version still shows old branding"
    ERRORS=$((ERRORS + 1))
fi

# Test configuration directory
DEBUG_OUTPUT=$(./chronos debug 2>&1)
if [[ "$DEBUG_OUTPUT" == *".chronos"* ]]; then
    echo "✅ Configuration uses .chronos directory"
else
    echo "❌ Configuration still uses old directory"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "🔍 Checking for remaining 'claude-squad' references..."

# Check for claude-squad in Go files (should only be in comments or strings that reference the original)
CLAUDE_SQUAD_REFS=$(rg "claude-squad" --type go . | grep -v "// Originally from claude-squad" | grep -v "# claude-squad" | wc -l)
if [[ $CLAUDE_SQUAD_REFS -eq 0 ]]; then
    echo "✅ No inappropriate claude-squad references in Go files"
else
    echo "❌ Found $CLAUDE_SQUAD_REFS claude-squad references in Go files:"
    rg "claude-squad" --type go . | grep -v "// Originally from claude-squad" | grep -v "# claude-squad"
    ERRORS=$((ERRORS + 1))
fi

# Check that Claude AI references are preserved
CLAUDE_AI_REFS=$(rg "\"claude\"" --type go . | wc -l)
if [[ $CLAUDE_AI_REFS -gt 0 ]]; then
    echo "✅ Claude AI references preserved ($CLAUDE_AI_REFS found)"
else
    echo "⚠️  No Claude AI references found - this might be an issue"
fi

echo ""
echo "🔍 Checking web interface..."

# Check web package.json
if grep -q '"name": "chronos"' web/package.json; then
    echo "✅ Web package.json uses chronos name"
else
    echo "❌ Web package.json still uses old name"
    ERRORS=$((ERRORS + 1))
fi

# Check web layout
if grep -q "title: \"Chronos" web/src/app/layout.tsx; then
    echo "✅ Web layout uses Chronos title"
else
    echo "❌ Web layout still uses old title"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "🔍 Testing import paths..."

# Try to build to check import paths
echo "Building to test import paths..."
if go build -o chronos-test . 2>/dev/null; then
    echo "✅ All import paths updated correctly"
    rm -f chronos-test
else
    echo "❌ Build failed - import paths may be incorrect"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "📊 VALIDATION RESULTS"
echo "===================="

if [[ $ERRORS -eq 0 ]]; then
    echo "🎉 ALL CHECKS PASSED!"
    echo "✅ Chronos rebranding is complete and successful"
    echo ""
    echo "📝 Summary of changes:"
    echo "  • Application name: claude-squad → chronos"
    echo "  • Binary name: csq → chronos"
    echo "  • Config directory: ~/.claude-squad → ~/.chronos"
    echo "  • Log file: claudesquad.log → chronos.log"
    echo "  • All import paths updated"
    echo "  • User-facing text updated"
    echo "  • Claude AI references preserved ✅"
    echo ""
    echo "🚀 Ready to use: ./chronos --help"
else
    echo "❌ $ERRORS ISSUES FOUND"
    echo "Please review and fix the issues above"
    exit 1
fi