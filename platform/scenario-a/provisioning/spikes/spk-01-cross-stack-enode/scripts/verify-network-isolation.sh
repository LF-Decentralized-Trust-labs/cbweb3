#!/usr/bin/env bash
set -euo pipefail

echo "[verify-network-isolation] checking that joiner is not on spk01_found_net"

# Check if spk01_found_net exists
if ! docker network inspect spk01_found_net > /dev/null 2>&1; then
    echo "FAIL T4 network-isolation spk01_found_net does not exist"
    exit 1
fi

# Get containers on found network
FOUND_CONTAINERS=$(docker network inspect spk01_found_net --format '{{json .Containers}}' 2>/dev/null || echo "{}")

# Check if any container name contains "joiner"
if echo "${FOUND_CONTAINERS}" | jq -e 'to_entries[].value.Name | select(test("joiner"; "i"))' > /dev/null 2>&1; then
    echo "FAIL T4 network-isolation JOINER_ON_FOUND_NET"
    exit 1
fi

echo "PASS T4 network-isolation join_net_isolated"
