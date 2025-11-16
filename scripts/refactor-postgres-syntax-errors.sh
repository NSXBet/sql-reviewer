#!/bin/bash
# Refactor PostgreSQL rules to convert syntax errors to advice
#
# This script automatically updates all PostgreSQL rule files to convert
# syntax errors to advice objects, matching the pattern used in MySQL rules.
#
# Usage:
#   ./scripts/refactor-postgres-syntax-errors.sh [--dry-run]
#
# Options:
#   --dry-run    Show what would be changed without making modifications

set -euo pipefail

POSTGRES_RULES_DIR="pkg/rules/postgres"
DRY_RUN=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--dry-run]"
            exit 1
            ;;
    esac
done

# Find all .go files that use getANTLRTree (excluding framework.go, utils.go, and test files)
FILES=$(find "$POSTGRES_RULES_DIR" -name "*.go" -type f \
    -not -name "framework.go" \
    -not -name "framework_test.go" \
    -not -name "utils.go" \
    -not -name "*_test.go" \
    -exec grep -l "getANTLRTree" {} \;)

if [ -z "$FILES" ]; then
    echo "No files found that use getANTLRTree"
    exit 0
fi

FILE_COUNT=$(echo "$FILES" | wc -l | tr -d ' ')
echo "Found $FILE_COUNT files to update"
echo

# Process each file
SUCCESS_COUNT=0
SKIPPED_COUNT=0

for file in $FILES; do
    echo "Processing: $file"

    # Check if file already uses ConvertSyntaxErrorToAdvice
    if grep -q "ConvertSyntaxErrorToAdvice" "$file"; then
        echo "  ⏭️  SKIPPED - Already uses ConvertSyntaxErrorToAdvice"
        ((SKIPPED_COUNT++))
        echo
        continue
    fi

    # Check if file has the pattern we're looking for
    if ! grep -q "getANTLRTree" "$file"; then
        echo "  ⏭️  SKIPPED - No getANTLRTree call found"
        ((SKIPPED_COUNT++))
        echo
        continue
    fi

    if $DRY_RUN; then
        # In dry-run mode, just show what would be changed
        echo "  🔍 DRY RUN - Would replace:"
        grep -A3 "tree, err := getANTLRTree" "$file" | head -4 || echo "  (no matching pattern found)"
        echo "  with:"
        echo "    return ConvertSyntaxErrorToAdvice(err)"
        ((SUCCESS_COUNT++))
    else
        # Create backup
        cp "$file" "$file.bak"

        # Perform the replacement using perl for better multiline handling
        # Pattern: getANTLRTree followed by "if err != nil { return nil, err }"
        # The pattern matches tab-indented code with proper whitespace handling
        perl -i -0pe 's/(\ttree, err := getANTLRTree\(checkCtx\)\n\tif err != nil \{\n\t\t)return nil, err(\n\t\})/${1}return ConvertSyntaxErrorToAdvice(err)${2}/g' "$file"

        # Check if the file was modified
        if ! cmp -s "$file" "$file.bak"; then
            echo "  ✅ UPDATED"
            ((SUCCESS_COUNT++))
        else
            echo "  ⏭️  NO CHANGES NEEDED"
            rm "$file.bak"
            ((SKIPPED_COUNT++))
        fi
    fi

    echo
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Summary:"
echo "  Files processed: $FILE_COUNT"
echo "  Successfully updated: $SUCCESS_COUNT"
echo "  Skipped: $SKIPPED_COUNT"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

if $DRY_RUN; then
    echo "🔍 DRY RUN MODE - No files were modified"
    echo "   Run without --dry-run to apply changes"
    exit 0
fi

if [ $SUCCESS_COUNT -gt 0 ]; then
    echo "Running tests to verify changes..."
    if go test ./pkg/rules/postgres/...; then
        echo "✅ All tests passed!"
        echo
        echo "Cleaning up backup files..."
        find "$POSTGRES_RULES_DIR" -name "*.bak" -type f -delete
        echo "✅ Backup files cleaned"
    else
        echo "❌ Tests failed! Restoring backups..."
        find "$POSTGRES_RULES_DIR" -name "*.bak" -type f | while read backup; do
            original="${backup%.bak}"
            mv "$backup" "$original"
            echo "  Restored: $original"
        done
        echo "❌ Refactoring failed - all files restored to original state"
        exit 1
    fi
else
    echo "ℹ️  No files were modified"
fi
