#!/bin/bash
set -e

# GoAhead Test Suite
# Comprehensive testing of reboot_completion_check_offset and reboot_completion_panic_threshold features

# Configuration
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
TEST_DIR="$SCRIPT_DIR"
RESULTS_DIR="$TEST_DIR/results"
LOGS_DIR="$TEST_DIR/logs"
CONFIG_DIR="$TEST_DIR/config"
SERVER_PORT=8444
SERVER_URL="https://127.0.0.1:$SERVER_PORT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Test result tracking
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Function to increment test counters
test_result() {
    local result="$1"
    local test_name="$2"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    
    if [[ "$result" == "PASS" ]]; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        log_success "PASS: $test_name"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        log_error "FAIL: $test_name"
    fi
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test environment..."
    
    # Kill server if running
    if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
        log_info "Terminating goahead server (PID: $SERVER_PID)"
        kill "$SERVER_PID"
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    
    # Clean up client processes
    pkill -f "goahead_client" 2>/dev/null || true
    
    # Clean up result files
    rm -rf "$RESULTS_DIR"/*
    rm -rf "$LOGS_DIR"/*
}

# Setup function
setup() {
    log_info "Setting up test environment..."
    
    # Create necessary directories
    mkdir -p "$RESULTS_DIR" "$LOGS_DIR" "$CONFIG_DIR/clients"
    
    # Clean previous results
    cleanup
    
    # Compile goahead server
    log_info "Compiling goahead server..."
    cd "$GOAHEAD_DIR"
    go build -o goahead .
    if [[ $? -ne 0 ]]; then
        log_error "Failed to compile goahead server"
        exit 1
    fi
    
    # Compile goahead client
    log_info "Compiling goahead client..."
    cd "$CLIENT_DIR"
    go build -o goahead_client .
    if [[ $? -ne 0 ]]; then
        log_error "Failed to compile goahead client"
        exit 1
    fi
    
    log_success "Binaries compiled successfully"
}

# Start goahead server
start_server() {
    log_info "Starting goahead server on port $SERVER_PORT..."
    
    cd "$GOAHEAD_DIR"
    ./goahead -config "$CONFIG_DIR/test_config.yml" -debug > "$LOGS_DIR/server.log" 2>&1 &
    SERVER_PID=$!
    
    # Wait for server to start
    sleep 3
    
    # Check if server is running
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        log_error "Failed to start goahead server"
        cat "$LOGS_DIR/server.log"
        exit 1
    fi
    
    log_success "Server started with PID: $SERVER_PID"
}

# Create client configuration
create_client_config() {
    local fqdn="$1"
    local client_id="$2"
    
    cat > "$CONFIG_DIR/clients/client_${client_id}.yml" << EOF
---
service_url: $SERVER_URL/
service_url_ca_file: $TEST_DIR/ssl/test.pem
requesting_fqdn: $fqdn
restart_condition_script: $TEST_DIR/scripts/client_restart_condition.sh $client_id
restart_condition_script_exit_code_for_reboot: 1
os_restart_hooks_dir: $TEST_DIR/scripts/
EOF
}

# Run client and return response
run_client() {
    local client_id="$1"
    local config_file="$CONFIG_DIR/clients/client_${client_id}.yml"
    
    cd "$CLIENT_DIR"
    timeout 30s ./goahead_client -config "$config_file" -debug 2>&1
}

# Wait for file to appear (with timeout)
wait_for_file() {
    local file_path="$1"
    local timeout="${2:-10}"
    local count=0
    
    while [[ $count -lt $timeout ]]; do
        if [[ -f "$file_path" ]]; then
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    return 1
}

# Validate that action scripts were called correctly
validate_action_called() {
    local action_type="$1"
    local fqdn="$2"
    local cluster="$3"
    local should_exist="${4:-true}"
    
    case "$action_type" in
        "goahead")
            local marker_file="$RESULTS_DIR/${cluster}-${fqdn}-goahead-granted"
            ;;
        "completion")
            local marker_file="$RESULTS_DIR/${cluster}-${fqdn}-reboot-completed"
            ;;
        "panic")
            local marker_file="$RESULTS_DIR/${cluster}-${fqdn}-panic-triggered"
            ;;
        *)
            log_error "Unknown action type: $action_type"
            return 1
            ;;
    esac
    
    if [[ "$should_exist" == "true" ]]; then
        if [[ -f "$marker_file" ]]; then
            return 0
        else
            log_error "Expected $action_type action marker file not found: $marker_file"
            return 1
        fi
    else
        if [[ ! -f "$marker_file" ]]; then
            return 0
        else
            log_error "Unexpected $action_type action marker file found: $marker_file"
            return 1
        fi
    fi
}

# Test 1: Simple cluster (no offset, no panic threshold)
test_simple_cluster() {
    log_info "Running Test 1: Simple cluster (no offset, no panic threshold)"
    
    local fqdn="simple-server01.test.tld"
    local client_id="simple01"
    local cluster="simple-cluster"
    
    # Create client config
    create_client_config "$fqdn" "$client_id"
    
    # Mark client as needing restart
    touch "$RESULTS_DIR/client-${client_id}-needs-restart"
    
    # Run client to get approval
    log_info "Step 1: Client requesting restart approval..."
    local response=$(run_client "$client_id" | tail -1)
    
    # Validate goahead action was called
    if validate_action_called "goahead" "$fqdn" "$cluster"; then
        test_result "PASS" "Simple cluster - Goahead action called"
    else
        test_result "FAIL" "Simple cluster - Goahead action called"
    fi
    
    # Wait a moment then simulate server coming back online
    sleep 3
    log_info "Step 2: Simulating server coming back online..."
    touch "$RESULTS_DIR/${fqdn}-server-online"
    
    # Wait for completion action (should happen quickly since no offset)
    sleep 5
    
    # Validate completion action was called
    if validate_action_called "completion" "$fqdn" "$cluster"; then
        test_result "PASS" "Simple cluster - Completion action called"
    else
        test_result "FAIL" "Simple cluster - Completion action called"
    fi
    
    # Validate panic action was NOT called (no panic threshold)
    if validate_action_called "panic" "$fqdn" "$cluster" "false"; then
        test_result "PASS" "Simple cluster - Panic action not called"
    else
        test_result "FAIL" "Simple cluster - Panic action not called"
    fi
}

# Test 2: Offset cluster (5s offset, no panic threshold)
test_offset_cluster() {
    log_info "Running Test 2: Offset cluster (5s offset, no panic threshold)"
    
    local fqdn="offset-server01.test.tld"
    local client_id="offset01"
    local cluster="offset-cluster"
    
    # Clean previous results
    rm -f "$RESULTS_DIR"/*
    
    create_client_config "$fqdn" "$client_id"
    touch "$RESULTS_DIR/client-${client_id}-needs-restart"
    
    log_info "Step 1: Client requesting restart approval..."
    run_client "$client_id" > /dev/null 2>&1 &
    
    # Validate goahead action was called
    sleep 2
    if validate_action_called "goahead" "$fqdn" "$cluster"; then
        test_result "PASS" "Offset cluster - Goahead action called"
    else
        test_result "FAIL" "Offset cluster - Goahead action called"
    fi
    
    # Immediately simulate server online, but completion should wait for offset
    log_info "Step 2: Immediately simulating server online (should wait for 5s offset)..."
    touch "$RESULTS_DIR/${fqdn}-server-online"
    
    # Check completion action is NOT called yet (within offset period)
    sleep 3
    if validate_action_called "completion" "$fqdn" "$cluster" "false"; then
        test_result "PASS" "Offset cluster - Completion action delayed by offset"
    else
        test_result "FAIL" "Offset cluster - Completion action delayed by offset"
    fi
    
    # Wait for offset to pass, then completion should happen
    log_info "Step 3: Waiting for offset to pass..."
    sleep 4
    
    if validate_action_called "completion" "$fqdn" "$cluster"; then
        test_result "PASS" "Offset cluster - Completion action called after offset"
    else
        test_result "FAIL" "Offset cluster - Completion action called after offset"
    fi
}

# Test 3: Panic cluster (no offset, 10s panic threshold)
test_panic_cluster() {
    log_info "Running Test 3: Panic cluster (no offset, 10s panic threshold)"
    
    local fqdn="panic-server01.test.tld"
    local client_id="panic01"
    local cluster="panic-cluster"
    
    # Clean previous results
    rm -f "$RESULTS_DIR"/*
    
    create_client_config "$fqdn" "$client_id"
    touch "$RESULTS_DIR/client-${client_id}-needs-restart"
    
    log_info "Step 1: Client requesting restart approval..."
    run_client "$client_id" > /dev/null 2>&1 &
    
    sleep 2
    if validate_action_called "goahead" "$fqdn" "$cluster"; then
        test_result "PASS" "Panic cluster - Goahead action called"
    else
        test_result "FAIL" "Panic cluster - Goahead action called"
    fi
    
    # Do NOT simulate server coming back online - should trigger panic after 10s
    log_info "Step 2: NOT bringing server online - should trigger panic after 10s..."
    
    # Wait for panic threshold to be reached
    sleep 12
    
    if validate_action_called "panic" "$fqdn" "$cluster"; then
        test_result "PASS" "Panic cluster - Panic action triggered after threshold"
    else
        test_result "FAIL" "Panic cluster - Panic action triggered after threshold"
    fi
    
    # Completion should NOT be called since server never came online
    if validate_action_called "completion" "$fqdn" "$cluster" "false"; then
        test_result "PASS" "Panic cluster - Completion action not called (server offline)"
    else
        test_result "FAIL" "Panic cluster - Completion action not called (server offline)"
    fi
}

# Test 4: Full cluster (5s offset + 15s panic threshold)
test_full_cluster() {
    log_info "Running Test 4: Full cluster (5s offset + 15s panic threshold)"
    
    local fqdn="full-server01.test.tld"
    local client_id="full01"
    local cluster="full-cluster"
    
    # Clean previous results
    rm -f "$RESULTS_DIR"/*
    
    create_client_config "$fqdn" "$client_id"
    touch "$RESULTS_DIR/client-${client_id}-needs-restart"
    
    log_info "Step 1: Client requesting restart approval..."
    run_client "$client_id" > /dev/null 2>&1 &
    
    sleep 2
    if validate_action_called "goahead" "$fqdn" "$cluster"; then
        test_result "PASS" "Full cluster - Goahead action called"
    else
        test_result "FAIL" "Full cluster - Goahead action called"
    fi
    
    # Do NOT bring server online - should trigger panic after offset(5s) + threshold(15s) = 20s
    log_info "Step 2: NOT bringing server online - should trigger panic after 20s total (5s offset + 15s threshold)..."
    
    # Check that panic doesn't trigger too early (before 18s)
    sleep 17
    if validate_action_called "panic" "$fqdn" "$cluster" "false"; then
        test_result "PASS" "Full cluster - Panic not triggered too early"
    else
        test_result "FAIL" "Full cluster - Panic not triggered too early"
    fi
    
    # Wait for panic threshold to be reached
    sleep 5
    
    if validate_action_called "panic" "$fqdn" "$cluster"; then
        test_result "PASS" "Full cluster - Panic triggered after offset + threshold"
    else
        test_result "FAIL" "Full cluster - Panic triggered after offset + threshold"
    fi
}

# Test 5: Full cluster with successful reboot (panic cancellation test)
test_full_cluster_success() {
    log_info "Running Test 5: Full cluster with successful reboot (panic cancellation)"
    
    local fqdn="full-server02.test.tld"
    local client_id="full02"
    local cluster="full-cluster"
    
    # Clean previous results
    rm -f "$RESULTS_DIR"/*
    
    create_client_config "$fqdn" "$client_id"
    touch "$RESULTS_DIR/client-${client_id}-needs-restart"
    
    log_info "Step 1: Client requesting restart approval..."
    run_client "$client_id" > /dev/null 2>&1 &
    
    sleep 2
    if validate_action_called "goahead" "$fqdn" "$cluster"; then
        test_result "PASS" "Full cluster success - Goahead action called"
    else
        test_result "FAIL" "Full cluster success - Goahead action called"
    fi
    
    # Wait for offset to pass, then bring server online
    log_info "Step 2: Waiting for offset (5s), then bringing server online..."
    sleep 6
    touch "$RESULTS_DIR/${fqdn}-server-online"
    
    # Wait for completion
    sleep 4
    
    if validate_action_called "completion" "$fqdn" "$cluster"; then
        test_result "PASS" "Full cluster success - Completion action called"
    else
        test_result "FAIL" "Full cluster success - Completion action called"
    fi
    
    # Wait to ensure panic doesn't trigger (panic timer should be cancelled)
    log_info "Step 3: Waiting to ensure panic timer was cancelled..."
    sleep 12
    
    if validate_action_called "panic" "$fqdn" "$cluster" "false"; then
        test_result "PASS" "Full cluster success - Panic timer cancelled after successful reboot"
    else
        test_result "FAIL" "Full cluster success - Panic timer cancelled after successful reboot"
    fi
}

# Print test summary
print_summary() {
    echo
    echo "=========================="
    echo "   TEST SUITE SUMMARY"
    echo "=========================="
    echo "Tests Run:    $TESTS_RUN"
    echo "Tests Passed: $TESTS_PASSED"
    echo "Tests Failed: $TESTS_FAILED"
    echo
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        log_success "ALL TESTS PASSED!"
        return 0
    else
        log_error "$TESTS_FAILED TEST(S) FAILED!"
        return 1
    fi
}

# Main execution
main() {
    echo "======================================"
    echo "  GoAhead Reboot Offset & Panic Test Suite"
    echo "======================================"
    echo
    
    # Set trap for cleanup
    trap cleanup EXIT
    
    # Setup
    setup
    start_server
    
    # Run tests
    test_simple_cluster
    test_offset_cluster  
    test_panic_cluster
    test_full_cluster
    test_full_cluster_success
    
    # Print summary and exit
    if print_summary; then
        exit 0
    else
        exit 1
    fi
}

# Execute main function
main "$@"