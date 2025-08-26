#!/bin/bash
# Script to check if a server has completed rebooting
# Parameters: {fqdn}
# Return code 0 = server is back online, non-zero = still rebooting

FQDN="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULT_DIR="$(dirname "$SCRIPT_DIR")/results"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

mkdir -p "$RESULT_DIR"

# Log that check was called
echo "[$TIMESTAMP] Reboot completion check called for FQDN: $FQDN" >> "$RESULT_DIR/reboot_completion_checks.log"

# Check if the "server is back online" marker file exists
# In real scenarios, this would be replaced with actual connectivity checks
if [[ -f "$RESULT_DIR/${FQDN}-server-online" ]]; then
    echo "[$TIMESTAMP] Server $FQDN appears to be back online" >> "$RESULT_DIR/reboot_completion_checks.log"
    exit 0
else
    echo "[$TIMESTAMP] Server $FQDN is still rebooting" >> "$RESULT_DIR/reboot_completion_checks.log"
    exit 1
fi