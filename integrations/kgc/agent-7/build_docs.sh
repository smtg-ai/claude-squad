#!/bin/bash
#
# build_docs.sh - Validates KGC documentation structure
#
# This script:
# 1. Validates all markdown files exist and are well-formed
# 2. Checks for broken internal links
# 3. Generates index of all documentation
# 4. Errors on any validation failures
#
# Usage: ./build_docs.sh
# Exit codes:
#   0 - All validations passed
#   1 - Validation errors found

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TOTAL_FILES=0
TOTAL_LINKS=0
BROKEN_LINKS=0
ERRORS=0

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_DIR="${SCRIPT_DIR}/docs"

echo "KGC Documentation Build & Validation"
echo "====================================="
echo ""

# Check if docs directory exists
if [ ! -d "$DOCS_DIR" ]; then
    echo -e "${RED}ERROR: docs/ directory not found${NC}"
    exit 1
fi

# Function: Check if file exists
check_file_exists() {
    local file=$1
    local context=$2

    if [ ! -f "$file" ]; then
        echo -e "${RED}✗ MISSING: $file (referenced from $context)${NC}"
        ((ERRORS++))
        return 1
    fi
    return 0
}

# Function: Validate markdown file structure
validate_markdown() {
    local file=$1

    if [ ! -f "$file" ]; then
        echo -e "${RED}✗ File not found: $file${NC}"
        ((ERRORS++))
        return 1
    fi

    # Check if file is empty
    if [ ! -s "$file" ]; then
        echo -e "${RED}✗ Empty file: $file${NC}"
        ((ERRORS++))
        return 1
    fi

    # Check if file starts with heading
    if ! head -n 1 "$file" | grep -q '^#'; then
        echo -e "${YELLOW}⚠ Warning: $file doesn't start with heading${NC}"
    fi

    ((TOTAL_FILES++))
    return 0
}

# Function: Extract and validate links
validate_links() {
    local file=$1
    local file_dir=$(dirname "$file")

    # For now, just count links and skip validation
    # Full validation would require all agent code to exist
    # We validate that docs/ internal links work

    # Count total links
    local link_count=$(grep -o '\[.*\](.*\.md)' "$file" 2>/dev/null | wc -l || echo 0)
    TOTAL_LINKS=$((TOTAL_LINKS + link_count))

    # Validate only links to files within docs/
    while IFS= read -r line; do
        if [[ -z "$line" ]]; then
            continue
        fi

        # Extract path from markdown link
        local path=$(echo "$line" | sed -n 's/.*(\(.*\.md\)).*/\1/p')

        # Skip if no path
        if [[ -z "$path" ]]; then
            continue
        fi

        # Skip external links
        if [[ "$path" =~ ^https?:// ]]; then
            continue
        fi

        # Skip anchors only
        if [[ "$path" =~ ^# ]]; then
            continue
        fi

        # Construct target file path
        if [[ "$path" =~ ^\.\. ]]; then
            local target="${file_dir}/${path}"
        else
            local target="${file_dir}/${path}"
        fi

        # Remove anchor
        target="${target%%#*}"

        # Only check if target is within docs/
        if [[ "$target" =~ /docs/ ]]; then
            if [ ! -f "$target" ]; then
                echo -e "${RED}✗ Broken link in $(basename $file): $path${NC}"
                ((BROKEN_LINKS++))
                ((ERRORS++))
            fi
        fi
    done < <(grep -o '\[.*\](.*\.md)' "$file" 2>/dev/null || true)
}

echo "Step 1: Validating directory structure..."
echo "==========================================="

# Required directories
REQUIRED_DIRS=(
    "$DOCS_DIR/tutorial"
    "$DOCS_DIR/how_to"
    "$DOCS_DIR/reference"
    "$DOCS_DIR/explanation"
)

for dir in "${REQUIRED_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        echo -e "${GREEN}✓${NC} $dir"
    else
        echo -e "${RED}✗ Missing directory: $dir${NC}"
        ((ERRORS++))
    fi
done

echo ""
echo "Step 2: Validating required files..."
echo "====================================="

# Required files
REQUIRED_FILES=(
    "$DOCS_DIR/index.md"
    "$DOCS_DIR/tutorial/getting_started.md"
    "$DOCS_DIR/how_to/create_knowledge_store.md"
    "$DOCS_DIR/how_to/verify_receipts.md"
    "$DOCS_DIR/how_to/run_multi_agent_demo.md"
    "$DOCS_DIR/reference/substrate_interfaces.md"
    "$DOCS_DIR/reference/api.md"
    "$DOCS_DIR/reference/cli.md"
    "$DOCS_DIR/explanation/why_determinism.md"
    "$DOCS_DIR/explanation/receipt_chaining.md"
    "$DOCS_DIR/explanation/composition_laws.md"
)

for file in "${REQUIRED_FILES[@]}"; do
    if validate_markdown "$file"; then
        echo -e "${GREEN}✓${NC} $file"
    fi
done

echo ""
echo "Step 3: Validating internal links..."
echo "====================================="

# Find all markdown files and validate links
while IFS= read -r -d '' file; do
    validate_links "$file"
done < <(find "$DOCS_DIR" -name "*.md" -type f -print0)

echo ""
echo "Step 4: Generating documentation index..."
echo "=========================================="

# Generate index file
INDEX_FILE="${SCRIPT_DIR}/DOCUMENTATION_INDEX.txt"

cat > "$INDEX_FILE" <<EOF
KGC Documentation Index
Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
===============================================

Directory Structure:
EOF

# Add directory tree
find "$DOCS_DIR" -type f -name "*.md" | sort | while read -r file; do
    relative_path="${file#$DOCS_DIR/}"
    echo "  $relative_path" >> "$INDEX_FILE"
done

echo "" >> "$INDEX_FILE"
echo "Statistics:" >> "$INDEX_FILE"
echo "  Total files: $TOTAL_FILES" >> "$INDEX_FILE"
echo "  Total links: $TOTAL_LINKS" >> "$INDEX_FILE"
echo "  Broken links: $BROKEN_LINKS" >> "$INDEX_FILE"

echo -e "${GREEN}✓${NC} Index generated: $INDEX_FILE"

echo ""
echo "Step 5: Final validation report..."
echo "==================================="

echo ""
echo "Statistics:"
echo "  Total markdown files: $TOTAL_FILES"
echo "  Total internal links: $TOTAL_LINKS"
echo "  Broken links: $BROKEN_LINKS"
echo "  Total errors: $ERRORS"
echo ""

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ All validations passed!${NC}"
    echo ""
    echo "Documentation is ready for use."
    exit 0
else
    echo -e "${RED}✗ Validation failed with $ERRORS error(s)${NC}"
    echo ""
    echo "Please fix the errors above before committing."
    exit 1
fi
