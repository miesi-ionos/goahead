# goahead
Simple service that allows or denies server / OS restarts.

Client can be found here: https://github.com/xorpaul/goahead_client

## Building

```sh
go mod tidy
go build
```

### workflow

#### Client inquires or requests restart
Client sends request to service with URI `/v1/request/restart/os` with JSON payload:
```
{"fqdn":"foobar-server1.domain.tld","uptime":"2255h27m43s"}
```

#### `goahead` service checks and decides if client may restart

The `fqdn` from the client payload gets matched against definied cluster nodes name patterns in cluster config:

```
---
foobar-server:
  enabled: true
  name_pattern: "^(foobar-server-).*[[:digit:]]{2}.(domain).(tld)$"
  blacklist_name_pattern:
    - ".*-standalone-.*"
    - ".*-black-.*"
  minimum_uptime: 24h
  cluster_type: active/active
  allowed_parallel_restarts: 2
  reboot_goahead_actions:
    - /etc/goahead/goahead_hooks.d/notify_admins.sh {:%fqdn%:} {:%cluster%:}
  reboot_completion_check: /etc/goahead/reboot_completion_checks.d/check.sh {:%fqdn%:}
  reboot_completion_check_interval: 15s
  reboot_completion_check_consecutive_successes: 3
  reboot_completion_check_offset: 15m
  reboot_completion_actions:
    - /etc/goahead/goahead_hooks.d/notify_admins.sh {:%fqdn%:} {:%cluster%:}
  reboot_completion_panic_threshold: 3h
```

## Logging

GoAhead provides structured logging with advanced file management and configurable rotation:

### Log Files

- Each cluster gets its own log file: `{log_base_dir}/{cluster-name}.log`
- Main application logs: `{log_base_dir}/goahead.log`
- Unknown host logs: `{log_base_dir}/unknown.log`  
- Checker process logs: `{log_base_dir}/checker.log`

### Log Rotation

GoAhead includes intelligent log rotation with timestamp-based naming:

- **Configurable size threshold**: Set maximum file size before rotation (e.g., `100M`, `1G`)
- **Timestamp-based rotation**: Rotated files use format `logfile.log.2025-08-26_17-21-11`
- **Configurable retention**: Keep a specified number of historical log files
- **Automatic cleanup**: Optionally delete old log files beyond retention count
- **Graceful shutdown**: All log file handles are properly closed on SIGINT/SIGTERM

#### Log Rotation Behavior

**Default Configuration (1 active + 5 historical files):**
```
goahead.log                    # Current active log
goahead.log.2025-08-26_17-21-11  # Most recent rotation
goahead.log.2025-08-26_16-15-33  # Older rotation  
goahead.log.2025-08-26_14-10-22  # Older rotation
goahead.log.2025-08-26_12-05-11  # Older rotation
goahead.log.2025-08-26_10-02-44  # Oldest kept rotation
```

When a new rotation occurs and `delete_old_log_files` is enabled, the oldest file is automatically deleted.

### Configuration

```yaml
# Basic logging
log_base_dir: /var/log/goahead/

# Advanced log rotation settings  
log_max_size: 100M              # Maximum file size before rotation
log_rotation_count: 5           # Number of rotated files to keep  
delete_old_log_files: false     # Auto-delete files beyond retention count
```

#### Configuration Parameters

| Parameter | Default | Description | Examples |
|-----------|---------|-------------|----------|
| `log_max_size` | `100M` | Maximum log file size before rotation | `50M`, `1G`, `500K`, `2048` (bytes) |
| `log_rotation_count` | `5` | Number of rotated log files to retain | `3`, `10`, `0` (disable retention) |
| `delete_old_log_files` | `false` | Automatically delete old files beyond count | `true`, `false` |

#### Size Format Examples

- `100M` - 100 megabytes
- `1G` - 1 gigabyte  
- `500K` - 500 kilobytes
- `2048` - 2048 bytes
- `1T` - 1 terabyte

### Debug Mode

- **Structured format**: Uses logrus for consistent structured logging
- **Debug mode**: Use `-debug` flag for detailed debug output
- **Performance optimized**: Proper printf formatting instead of string concatenation

**Note**: Ensure the log directory exists and is writable by the goahead user.

## Action Scripts

GoAhead supports multiple types of action scripts that are executed at different phases of the reboot process:

### reboot_goahead_actions

**When executed:** Immediately when GoAhead approves a reboot request (before the client reboots)

**Purpose:** Execute actions that should happen when a reboot is granted but before the actual reboot occurs

**Common use cases:**
- Send notifications that a server is about to reboot
- Update monitoring systems to expect downtime
- Log reboot approvals for audit trails
- Prepare dependent services for the upcoming downtime

**Configuration:**
```yaml
cluster-name:
  reboot_goahead_actions:
    - /path/to/scripts/notify_admins.sh {:%fqdn%:} {:%cluster%:}
    - /path/to/scripts/update_monitoring.sh {:%fqdn%:}
    - /path/to/scripts/log_reboot_approval.sh {:%fqdn%:} {:%cluster%:}
```

### reboot_completion_actions

**When executed:** After GoAhead determines the server has successfully rebooted and is back online

**Purpose:** Execute actions that should happen once a reboot is confirmed complete

**Common use cases:**
- Send notifications that a server is back online
- Update monitoring systems that downtime is over
- Run post-reboot verification checks
- Update status dashboards
- Trigger dependent service restarts

**Configuration:**
```yaml
cluster-name:
  reboot_completion_actions:
    - /path/to/scripts/notify_reboot_complete.sh {:%fqdn%:} {:%cluster%:}
    - /path/to/scripts/update_status_dashboard.sh {:%fqdn%:}
    - /path/to/scripts/run_post_reboot_checks.sh {:%fqdn%:}
```

### Action Script Parameters

All action scripts receive standardized parameters:

- `{:%fqdn%:}` - The fully qualified domain name of the rebooting server
- `{:%cluster%:}` - The cluster name the server belongs to
- `{:%uptime%:}` - Current server uptime (available for some actions)

### Action Script Best Practices

1. **Make scripts idempotent** - They may be called multiple times
2. **Handle failures gracefully** - Use proper exit codes (0 for success, non-zero for failure)
3. **Keep execution time minimal** - Long-running actions can delay the reboot process
4. **Log appropriately** - Include timestamps and meaningful error messages
5. **Test scripts independently** - Ensure they work correctly outside of GoAhead

**Example script structure:**
```bash
#!/bin/bash
FQDN="$1"
CLUSTER="$2"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# Your action logic here
echo "[$TIMESTAMP] Processing $FQDN in cluster $CLUSTER"

# Always exit with appropriate code
exit 0
```

Either allows a restart with `"go_ahead":true`:
```
{"timestamp":"2018-11-22T15:06:35.538017828Z","go_ahead":true,"unknown_host":false,"request_id":"BSporAsx","found_cluster":"foobar-server","requesting_fqdn":"foobar-server1.domain.tld","message":"","reported_uptime":"2255h27m43s"}
```

or denies a restart with the reason:
```
{"timestamp":"2018-11-22T15:06:35.538017828Z","go_ahead":false,"unknown_host":false,"request_id":"BSporAsx","found_cluster":"foobar-server","requesting_fqdn":"foobar-server1.domain.tld","message":"Denied restart request as the current_ongoing_restarts of cluster foobar-server is larger than the allowed_parallel_restarts: 1 >= 1 Currently restarting hosts: foobar-server2.domain.tld","reported_uptime":"2255h27m43s"}
```

#### Client processes the response

If the restart request was denied via `"go_ahead":false` then the client terminates and will/should ask again later.

If the restart was allowed then optionally the `goahead` service and/or the client can trigger certian hooks.
E.g. Clean removal from load-balancing/cluster or notify monitoring of upcoming restart etc.


#### `goahead` service checks for successful restart

The configured `reboot_completion_check` gets triggered, when the first contact from the previous client gets recieved.
When the check returns with the expected return code for the configured `reboot_completion_check_consecutive_successes` times, then the client is considered as successfully rebooted and the amount of currently restarting cluster nodes is decremented.

## Configuration Reference

### Reboot Completion Check Parameters

#### `reboot_completion_check_offset`
**Type:** Duration (e.g., `15m`, `2h`, `30s`)
**Default:** `0s` (no delay)

Defines the initial delay before starting reboot completion checks. This parameter allows systems time to fully shut down and begin the reboot process before goahead starts checking if the reboot is complete.

**Timeline:** The first reboot completion check will start at `reboot_approval_time + reboot_completion_check_offset`.

**Use cases:**
- Allow time for graceful shutdown processes
- Account for hardware initialization time
- Prevent false negatives from checking too early

#### `reboot_completion_panic_threshold`
**Type:** Duration (e.g., `3h`, `90m`, `7200s`)
**Default:** `0s` (disabled)

Sets the maximum time to wait for a server to complete its reboot before triggering panic actions. The panic timer is calculated as an absolute deadline: `reboot_approval_time + reboot_completion_check_offset + reboot_completion_panic_threshold`.

**Automatic Triggering:** Unlike other checks that are triggered by incoming requests, the panic threshold automatically triggers at the calculated time if the server hasn't completed its reboot.

**Timeline:**
1. `T+0`: Reboot approved
2. `T+offset`: First reboot completion check starts
3. `T+offset+panic_threshold`: Panic actions trigger automatically if server still rebooting

**Panic Actions:** When triggered, executes the configured `reboot_completion_panic_actions` scripts and sends notifications as defined.

**Example Configuration:**
```yaml
foobar-cluster:
  reboot_completion_check_offset: 15m      # Wait 15 minutes before first check
  reboot_completion_panic_threshold: 3h   # Panic if not rebooted after 3h 15m total
  reboot_completion_panic_actions:
    scripts:
      - /path/to/scripts/alert_admins.sh {:%fqdn%:} {:%cluster%:}
      - /path/to/scripts/create_incident.sh {:%fqdn%:}
```
