# GoAhead Test Suite

Comprehensive test suite for the goahead server and client implementation, specifically testing the `reboot_completion_check_offset` and `reboot_completion_panic_threshold` features.

## Overview

This test suite validates:
- ✅ **Simple clusters** (no offset, no panic threshold)  
- ✅ **Offset-only clusters** (reboot_completion_check_offset)
- ✅ **Panic-only clusters** (reboot_completion_panic_threshold)
- ✅ **Full feature clusters** (both offset + panic threshold)
- ✅ **Panic timer cancellation** (when reboot completes successfully)

## Architecture

```
/tests/
├── config/           # Server and cluster configurations
│   ├── test_config.yml         # Main server config
│   ├── simple-cluster.yml      # No offset, no panic
│   ├── offset-cluster.yml      # 5s offset only
│   ├── panic-cluster.yml       # 10s panic threshold only
│   ├── full-cluster.yml        # 5s offset + 15s panic threshold
│   └── clients/                # Generated client configs
├── scripts/          # Action scripts called by goahead
│   ├── reboot_goahead_action.sh       # Called when reboot approved
│   ├── reboot_completion_check.sh     # Checks if server is back online  
│   ├── reboot_completion_action.sh    # Called when reboot completed
│   ├── reboot_completion_panic_action.sh # Called on panic threshold
│   └── client_restart_condition.sh   # Client restart condition check
├── ssl/              # Test SSL certificates
├── results/          # Test result files and markers
├── logs/             # Server and test logs
├── test_suite.sh     # Main test runner
├── validate_setup.sh # Setup validation script
└── README.md         # This file
```

## Test Scenarios

### Test 1: Simple Cluster
- **Config**: No offset, no panic threshold
- **Expected**: Immediate reboot completion checking
- **Validates**: Basic goahead + completion actions called

### Test 2: Offset Cluster  
- **Config**: 5s offset, no panic threshold
- **Expected**: 5s delay before first reboot completion check
- **Validates**: Completion actions delayed by offset period

### Test 3: Panic Cluster
- **Config**: No offset, 10s panic threshold  
- **Expected**: Panic triggered after 10s if server doesn't come back
- **Validates**: Automatic panic timer triggers

### Test 4: Full Cluster (Panic Test)
- **Config**: 5s offset + 15s panic threshold
- **Expected**: Panic triggered after 20s total (5s offset + 15s threshold)
- **Validates**: Combined offset + panic threshold timing

### Test 5: Full Cluster (Success Test)  
- **Config**: 5s offset + 15s panic threshold
- **Expected**: Successful reboot cancels panic timer
- **Validates**: Panic timer cancellation on successful completion

## Usage

### Quick Start

1. **Prerequisites**: 
   - Ensure the `goahead_client` directory is in the same parent directory as `goahead`
   - Directory structure should be:
     ```
     work/
     ├── goahead/
     │   └── tests/
     └── goahead_client/
     ```

2. **Validate setup** (recommended first):
   ```bash
   cd goahead/tests
   ./validate_setup.sh
   ```

3. **Run full test suite**:
   ```bash
   ./test_suite.sh
   ```

### Expected Output

```
======================================
  GoAhead Reboot Offset & Panic Test Suite  
======================================

[INFO] Setting up test environment...
[SUCCESS] Binaries compiled successfully
[SUCCESS] Server started with PID: 12345

[INFO] Running Test 1: Simple cluster (no offset, no panic threshold)
[SUCCESS] PASS: Simple cluster - Goahead action called
[SUCCESS] PASS: Simple cluster - Completion action called  
[SUCCESS] PASS: Simple cluster - Panic action not called

[INFO] Running Test 2: Offset cluster (5s offset, no panic threshold)
[SUCCESS] PASS: Offset cluster - Goahead action called
[SUCCESS] PASS: Offset cluster - Completion action delayed by offset
[SUCCESS] PASS: Offset cluster - Completion action called after offset

... (additional tests) ...

==========================
   TEST SUITE SUMMARY
==========================
Tests Run:    15
Tests Passed: 15  
Tests Failed: 0

[SUCCESS] ALL TESTS PASSED!
```

## How It Works

### Action Script Integration

The test suite uses marker files to track when action scripts are called:

- **Reboot approved**: `{cluster}-{fqdn}-goahead-granted`
- **Reboot completed**: `{cluster}-{fqdn}-reboot-completed` 
- **Panic triggered**: `{cluster}-{fqdn}-panic-triggered`

### Server State Simulation

- **Server needs restart**: `client-{id}-needs-restart` file created
- **Server back online**: `{fqdn}-server-online` file created  
- Test runner controls these states to simulate different scenarios

### Timing Validation

Tests use carefully timed delays to validate:
- Actions occur at the right time
- Actions are delayed by the correct offset
- Panic timers trigger at the expected absolute time
- Panic timers are properly cancelled

## Configuration Details

### Cluster Timings

| Cluster | Offset | Panic Threshold | Total Panic Time |
|---------|--------|----------------|------------------|
| simple-cluster | 0s | 0s (disabled) | N/A |
| offset-cluster | 5s | 0s (disabled) | N/A |  
| panic-cluster | 0s | 10s | 10s |
| full-cluster | 5s | 15s | 20s |

### FQDN Patterns

- `simple-*.test.tld` → simple-cluster
- `offset-*.test.tld` → offset-cluster  
- `panic-*.test.tld` → panic-cluster
- `full-*.test.tld` → full-cluster

## Troubleshooting

### Common Issues

1. **Compilation errors**:
   ```bash
   cd ../.. && go build .  # From goahead/tests directory
   cd ../goahead_client && go build .
   ```

2. **Port already in use**:
   ```bash
   pkill -f goahead
   # Wait a moment, then retry
   ```

3. **Permission denied on scripts**:
   ```bash
   chmod +x scripts/*.sh
   ```

4. **SSL certificate issues**:
   ```bash
   # Regenerate certificates
   openssl req -x509 -newkey rsa:2048 -keyout ssl/test.key -out ssl/test.pem -days 365 -nodes -subj "/CN=localhost"
   ```

### Debug Mode

Check logs for detailed information:
- **Server logs**: `logs/server.log`  
- **Action logs**: `results/*.log`

## Extending the Test Suite

### Adding New Test Cases

1. Create cluster config in `config/`
2. Add test function in `test_suite.sh`
3. Call test function from `main()`

### Adding New Action Scripts

1. Create script in `scripts/`  
2. Make executable: `chmod +x scripts/new_script.sh`
3. Reference in cluster config files
4. Add validation logic to test functions

## Dependencies

- **Go 1.24+** (for compilation)
- **OpenSSL** (for certificate generation)  
- **Bash 4+** (for test runner)
- **Standard Unix tools** (timeout, pkill, etc.)

## Integration with CI/CD

The test suite returns appropriate exit codes:
- **Exit 0**: All tests passed
- **Exit 1**: One or more tests failed or setup error

Example CI integration:
```bash
#!/bin/bash
cd goahead/tests
if ./test_suite.sh; then
    echo "✅ All goahead tests passed"
else  
    echo "❌ goahead tests failed"
    exit 1
fi
```