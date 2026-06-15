#!/usr/bin/env sh

COVERAGE_THRESHOLD=80

# Keep fuzz runs low so coverage fits a CI time budget. Plain `forge coverage`
# disables the optimizer for accurate source mapping, which makes the heavier
# contracts hit "stack too deep"; `--ir-minimum` enables viaIR with minimum
# optimization to resolve that while keeping coverage data meaningful.
export FOUNDRY_FUZZ_RUNS="${FOUNDRY_FUZZ_RUNS:-256}"

# Coverage exclusions (denominator filter):
#   AutomatedMarketMaker* — AMM belongs to scenario B and is flagged for removal
#     from scenario A; it is excluded from the coverage gate pending that removal.
#   script/             — deploy scripts are integration glue, not core business
#     logic, and should not drag the gate.
COVERAGE_NO_MATCH='(AutomatedMarketMaker|script/)'

COVERAGE_OUTPUT=$(mktemp)

forge coverage --ir-minimum --report summary \
    --no-match-coverage "$COVERAGE_NO_MATCH" > "$COVERAGE_OUTPUT" 2>&1

cat "$COVERAGE_OUTPUT"

TOTAL_LINE=$(grep "^| Total" "$COVERAGE_OUTPUT" || true)

if [ -z "$TOTAL_LINE" ]; then
    echo ""
    echo "❌ Error: Unable to find the Total line in coverage output"
    exit 1
fi

TOTAL_METRICS=$(echo "$TOTAL_LINE" | grep -oE '[0-9]+\.[0-9]+%' | tr -d '%')
LINES_PERCENT=$(echo "$TOTAL_METRICS" | sed -n '1p')
STATEMENTS_PERCENT=$(echo "$TOTAL_METRICS" | sed -n '2p')
BRANCHES_PERCENT=$(echo "$TOTAL_METRICS" | sed -n '3p')
FUNCS_PERCENT=$(echo "$TOTAL_METRICS" | sed -n '4p')

if [ -z "$LINES_PERCENT" ] || [ -z "$STATEMENTS_PERCENT" ] || [ -z "$BRANCHES_PERCENT" ] || [ -z "$FUNCS_PERCENT" ]; then
    echo ""
    echo "❌ Error: Unable to extract one or more Total coverage metrics"
    echo "Total Line: $TOTAL_LINE"
    exit 1
fi

LINES_INT=$(echo "$LINES_PERCENT" | cut -d. -f1)
STATEMENTS_INT=$(echo "$STATEMENTS_PERCENT" | cut -d. -f1)
BRANCHES_INT=$(echo "$BRANCHES_PERCENT" | cut -d. -f1)
FUNCS_INT=$(echo "$FUNCS_PERCENT" | cut -d. -f1)
THRESHOLD_INT=$COVERAGE_THRESHOLD

LINES_DIFF=$((THRESHOLD_INT - LINES_INT))
STATEMENTS_DIFF=$((THRESHOLD_INT - STATEMENTS_INT))
BRANCHES_DIFF=$((THRESHOLD_INT - BRANCHES_INT))
FUNCS_DIFF=$((THRESHOLD_INT - FUNCS_INT))

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  📊 CODE COVERAGE VALIDATION REPORT"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  Coverage Metrics:"
echo "  ────────────────────────────────────────────────────────────────"
printf "    • Lines Coverage:          %s%%\n" "$LINES_PERCENT"
printf "    • Statements Coverage:     %s%%\n" "$STATEMENTS_PERCENT"
printf "    • Branches Coverage:       %s%%\n" "$BRANCHES_PERCENT"
printf "    • Functions Coverage:      %s%%\n" "$FUNCS_PERCENT"
printf "    • Required Threshold:      %s%%\n" "$COVERAGE_THRESHOLD"
echo ""

if [ "$LINES_INT" -lt "$THRESHOLD_INT" ] || [ "$STATEMENTS_INT" -lt "$THRESHOLD_INT" ] || [ "$BRANCHES_INT" -lt "$THRESHOLD_INT" ] || [ "$FUNCS_INT" -lt "$THRESHOLD_INT" ]; then
    echo "  Status: ❌ FAILURE"
    echo "  ────────────────────────────────────────────────────────────────"
    [ "$LINES_INT" -lt "$THRESHOLD_INT" ] && printf "    • Lines below threshold by:      %s%%\n" "$LINES_DIFF"
    [ "$STATEMENTS_INT" -lt "$THRESHOLD_INT" ] && printf "    • Statements below threshold by: %s%%\n" "$STATEMENTS_DIFF"
    [ "$BRANCHES_INT" -lt "$THRESHOLD_INT" ] && printf "    • Branches below threshold by:   %s%%\n" "$BRANCHES_DIFF"
    [ "$FUNCS_INT" -lt "$THRESHOLD_INT" ] && printf "    • Functions below threshold by:  %s%%\n" "$FUNCS_DIFF"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════"
    echo ""
    exit 1
else
    echo "  Status: ✅ SUCCESS"
    echo "  ────────────────────────────────────────────────────────────────"
    printf "    • Lines exceed threshold by:      %s%%\n" "$((LINES_INT - THRESHOLD_INT))"
    printf "    • Statements exceed threshold by: %s%%\n" "$((STATEMENTS_INT - THRESHOLD_INT))"
    printf "    • Branches exceed threshold by:   %s%%\n" "$((BRANCHES_INT - THRESHOLD_INT))"
    printf "    • Functions exceed threshold by:  %s%%\n" "$((FUNCS_INT - THRESHOLD_INT))"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════"
    echo ""
    exit 0
fi
