#!/bin/bash
# Script called when a server has successfully completed rebooting
# Parameters: {fqdn} {cluster}

FQDN="$1"
CLUSTER="$2"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULT_DIR="$(dirname "$SCRIPT_DIR")/results"

mkdir -p "$RESULT_DIR"

echo "[$TIMESTAMP] Reboot completion action called for FQDN: $FQDN in cluster: $CLUSTER" >> "$RESULT_DIR/reboot_completion_actions.log"
touch "$RESULT_DIR/${CLUSTER}-${FQDN}-reboot-completed"

exit 0