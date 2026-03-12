#!/usr/bin/env sh

COVERAGE_THRESHOLD=90
COVERAGE_OUTPUT=$(mktemp)

forge coverage --report summary > "$COVERAGE_OUTPUT" 2>&1

cat "$COVERAGE_OUTPUT"

TOTAL_LINE=$(grep "^| Total" "$COVERAGE_OUTPUT" || true)

if [ -z "$TOTAL_LINE" ]; then
    echo ""
    echo "❌ Error: Unable to find the Total line in coverage output"
    exit 1
fi

COVERAGE_PERCENT=$(echo "$TOTAL_LINE" | sed -n 's/.*| *\([0-9.]*\)%.*/\1/p' | head -n1)

if [ -z "$COVERAGE_PERCENT" ]; then
    echo ""
    echo "❌ Error: Unable to extract coverage percentage"
    echo "Total Line: $TOTAL_LINE"
    exit 1
fi

# Convert to integer for comparison
COVERAGE_INT=$(echo "$COVERAGE_PERCENT" | cut -d. -f1)
THRESHOLD_INT=$COVERAGE_THRESHOLD

# Calculate difference
DIFFERENCE=$(echo "$THRESHOLD_INT - $COVERAGE_INT" | awk '{print $1 - $3}')

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  📊 CODE COVERAGE VALIDATION REPORT"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Coverage Metrics:"
echo "  ────────────────────────────────────────────────────────────────"
printf "    • Current Coverage:        %s%%\n" "$COVERAGE_PERCENT"
printf "    • Required Threshold:      %s%%\n" "$COVERAGE_THRESHOLD"
echo ""

if [ "$COVERAGE_INT" -lt "$THRESHOLD_INT" ]; then
    echo "  Status: ❌ FAILURE"
    echo "  ────────────────────────────────────────────────────────────────"
    printf "    Coverage falls short by %s%%\n" "$DIFFERENCE"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════"
    echo ""
    exit 1
else
    echo "  Status: ✅ SUCCESS"
    echo "  ────────────────────────────────────────────────────────────────"
    printf "    Coverage exceeds threshold by %s%%\n" "$((COVERAGE_INT - THRESHOLD_INT))"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════"
    echo ""
    exit 0
fi
