#!/bin/bash
# Script called when reboot completion panic threshold is reached
# Parameters: {fqdn} {cluster}

FQDN="$1"
CLUSTER="$2"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULT_DIR="$(dirname "$SCRIPT_DIR")/results"

mkdir -p "$RESULT_DIR"

echo "[$TIMESTAMP] PANIC: Reboot completion panic threshold reached for FQDN: $FQDN in cluster: $CLUSTER" >> "$RESULT_DIR/reboot_completion_panic.log"
touch "$RESULT_DIR/${CLUSTER}-${FQDN}-panic-triggered"

exit 0