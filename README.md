# wait-for-tcp

This is a simple Go application with zero external dependencies that checks if a specified TCP target is available. It continuously attempts to connect to the specified target at regular intervals until the target becomes available or the program is terminated.

## Environment Variables

The application requires the following environment variables to be set:

- `TARGET_NAME`: The name of the target to check.
- `TARGET_ADDRESS`: The address of the target in the format `host:port`.
- `INTERVAL`: The interval between connection attempts (optional, default: `2s`).
- `DIAL_TIMEOUT`: The timeout for each connection attempt (optional, default: `1s`).
- `LOG_FIELDS`: Log additional fields (optional, default: `false`).

## Behavior

- The application performs a single connection check to the specified target.
- If the connection attempt fails (within the specified `DIAL_TIMEOUT`), it waits for the specified `INTERVAL` before attempting to connect again.
- The process repeats until one of the following conditions is met:
  - The target becomes available.
  - The program is terminated or canceled.

## Logging

The application uses structured logging to provide clear and consistent log messages. Logs are output in a key-value format with timestamps and log levels.

```
ts=2024-07-05T13:08:20+02:00 level=info msg="Waiting for PostgreSQL to become ready..." dial_timeout="1s" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.18"
ts=2024-07-05T13:08:21+02:00 level=warn msg="PostgreSQL is not ready ✗" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.18"
ts=2024-07-05T13:08:24+02:00 level=warn msg="PostgreSQL is not ready ✗" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.18"
ts=2024-07-05T13:08:27+02:00 level=warn msg="PostgreSQL is not ready ✗" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.18"
```

## Kubernetes Init Container Configuration

Configure your Kubernetes deployment to use this init container:

```yaml
initContainers:
  - name: wait-for-response
    image: containeroo/wait-for-tcp:latest
    env:
      - name: TARGET_NAME
        value: "PostgreSQL"
      - name: TARGET_ADDRESS
        value: "postgres.default.svc.cluster.local:5432"
      - name: INTERVAL
        value: "2s" # Specify the interval duration, e.g., 2 seconds
      - name: DIAL_TIMEOUT
        value: "2s" # Specify the dial timeout duration, e.g., 2 seconds
      - name: LOG_FIELDS
        value: "true"
```
