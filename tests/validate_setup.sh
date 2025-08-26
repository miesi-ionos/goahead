#!/bin/bash

# Simple validation script to test individual components
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOAHEAD_DIR="$(dirname "$SCRIPT_DIR")"
# Look for goahead_client in sibling directory first, then in parent directory
if [[ -d "$(dirname "$GOAHEAD_DIR")/goahead_client" ]]; then
    CLIENT_DIR="$(dirname "$GOAHEAD_DIR")/goahead_client"
elif [[ -d "$GOAHEAD_DIR/../goahead_client" ]]; then
    CLIENT_DIR="$GOAHEAD_DIR/../goahead_client"
else
    echo "Error: goahead_client directory not found. Please ensure it's in a sibling directory to goahead"
    exit 1
fi

echo "=== GoAhead Test Suite Setup Validation ==="

# Check if binaries can be compiled
echo "1. Testing goahead server compilation..."
cd "$GOAHEAD_DIR"
if go build -o test_goahead . ; then
    echo "✓ goahead server compiles successfully"
    rm -f test_goahead
else
    echo "✗ goahead server compilation failed"
    exit 1
fi

echo "2. Testing goahead client compilation..."
cd "$CLIENT_DIR"
if go build -o test_goahead_client . ; then
    echo "✓ goahead client compiles successfully"
    rm -f test_goahead_client
else
    echo "✗ goahead client compilation failed"
    exit 1
fi

# Check SSL certificates
echo "3. Checking SSL certificates..."
if [[ -f "$SCRIPT_DIR/ssl/test.pem" && -f "$SCRIPT_DIR/ssl/test.key" ]]; then
    echo "✓ SSL certificates exist"
else
    echo "✗ SSL certificates missing"
    exit 1
fi

# Check configuration files
echo "4. Checking configuration files..."
configs=("simple-cluster.yml" "offset-cluster.yml" "panic-cluster.yml" "full-cluster.yml" "test_config.yml")
for config in "${configs[@]}"; do
    if [[ -f "$SCRIPT_DIR/config/$config" ]]; then
        echo "  ✓ $config exists"
    else
        echo "  ✗ $config missing"
        exit 1
    fi
done

# Check scripts
echo "5. Checking action scripts..."
scripts=("reboot_goahead_action.sh" "reboot_completion_check.sh" "reboot_completion_action.sh" "reboot_completion_panic_action.sh" "client_restart_condition.sh")
for script in "${scripts[@]}"; do
    if [[ -x "$SCRIPT_DIR/scripts/$script" ]]; then
        echo "  ✓ $script exists and is executable"
    else
        echo "  ✗ $script missing or not executable"
        exit 1
    fi
done

# Test script execution
echo "6. Testing script execution..."
mkdir -p "$SCRIPT_DIR/results"
if "$SCRIPT_DIR/scripts/reboot_goahead_action.sh" "test.example.com" "test-cluster"; then
    echo "  ✓ reboot_goahead_action.sh executes successfully"
else
    echo "  ✗ reboot_goahead_action.sh failed"
    exit 1
fi

echo
echo "=== All Setup Validation Tests Passed! ==="
echo
echo "You can now run the full test suite with:"
echo "  $SCRIPT_DIR/test_suite.sh"