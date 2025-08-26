#!/bin/bash
# Client script to determine if restart is needed
# This script simulates a condition that requires restart

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULT_DIR="$(dirname "$SCRIPT_DIR")/results"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
CLIENT_ID="$1"  # Will be passed by test runner

mkdir -p "$RESULT_DIR"

# Check if this client should request a restart
if [[ -f "$RESULT_DIR/client-${CLIENT_ID}-needs-restart" ]]; then
    echo "[$TIMESTAMP] Client $CLIENT_ID needs restart" >> "$RESULT_DIR/client_restart_conditions.log"
    exit 1  # Exit code 1 means restart is needed
else
    echo "[$TIMESTAMP] Client $CLIENT_ID does not need restart" >> "$RESULT_DIR/client_restart_conditions.log"
    exit 0  # Exit code 0 means no restart needed
fi