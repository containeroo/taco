<p align="center">
  <img src=".github/assets/taco.svg" />
</p>

# DEPRECATION WARNING

`TACO` is deprecated and will be removed in the future.
Please use the fully compatible [containeroo/portpatrol](https://github.com/containeroo/portpatrol) instead.


# TACO (TCP Availability Connection Observer)

This is a simple Go application with zero external dependencies that checks if a specified TCP target is available. It continuously attempts to connect to the specified target at regular intervals until the target becomes available or the program is terminated.

## Environment Variables

TACO accepts the following environment variables:

- `TARGET_ADDRESS`: The address of the target in the format `host:port` (required).
- `TARGET_NAME`: The name of the target to check (optional, default: inferred from `TARGET_ADDRESS`)\*.
- `INTERVAL`: The interval between connection attempts (optional, default: `2s`).
- `DIAL_TIMEOUT`: The timeout for each connection attempt (optional, default: `1s`).
- `LOG_EXTRA_FIELDS`: Log additional fields (optional, default: `false`).

**\*** If `TARGET_NAME` is not set, the name will be inferred from the host part of the target address as follows: `postgres.default.svc.cluster.local:5432` will be inferred as `postgres`.

## Behavior Flowchart

```mermaid
graph TD;
    A[Start] --> B[Attempt to connect to TARGET_ADDRESS];
    B -->|Connection successful| C[Target is ready];
    B -->|Connection failed| D[Wait for retry INTERVAL];
    D --> B;
    C --> E[End];
    F[Program terminated or canceled] --> E;
```

## Logging

With the `LOG_EXTRA_FIELDS` environment variable set to `true` additional fields will be logged.

### With additional fields

```text
ts=2024-07-05T13:08:20+02:00 level=INFO msg="Waiting for PostgreSQL to become ready..." dial_timeout="1s" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.22"
ts=2024-07-05T13:08:21+02:00 level=WARN msg="PostgreSQL is not ready ✗" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.22"
ts=2024-07-05T13:08:24+02:00 level=WARN msg="PostgreSQL is not ready ✗" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.22"
ts=2024-07-05T13:08:27+02:00 level=WARN msg="PostgreSQL is not ready ✗" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.22"
ts=2024-07-05T13:08:27+02:00 level=INFO msg="PostgreSQL is ready ✓" dial_timeout="1s" error="dial tcp: lookup postgres.default.svc.cluster.local: i/o timeout" interval="2s" target_address="postgres.default.svc.cluster.local:5432" target_name="PostgreSQL" version="0.0.22"
```

### Without additional fields

```text
time=2024-07-12T12:44:41.494Z level=INFO msg="Waiting for PostgreSQL to become ready..."
time=2024-07-12T12:44:41.512Z level=WARN msg="PostgreSQL is not ready ✗"
time=2024-07-12T12:44:43.532Z level=WARN msg="PostgreSQL is not ready ✗"
time=2024-07-12T12:44:45.552Z level=INFO msg="PostgreSQL is ready ✓"
```

## Kubernetes initContainer Configuration

Configure your Kubernetes deployment to use this init container:

```yaml
initContainers:
  - name: wait-for-valkey
    image: ghcr.io/containeroo/taco:latest
    env:
      - name: TARGET_ADDRESS
        value: valkey.default.svc.cluster.local:6379
      # TARGET_NAME inferred from the target address "valkey.default.svc.cluster.local" which is okay for this use case
      # INTERVAL defaults to 2 seconds which is okay for this use case
      # DIAL_TIMEOUT defaults to 1 seconds which is okay for this use case
      - name: LOG_EXTRA_FIELDS
        value: "true"
  - name: wait-for-postgres
    image: ghcr.io/containeroo/taco:latest
    env:
      - name: TARGET_NAME
        value: PostgreSQL # Use a better name for the target
      - name: TARGET_ADDRESS
        value: postgres.default.svc.cluster.local:5432
      - name: INTERVAL
        value: "4s" # Increase the interval duration, e.g., 4 seconds
      - name: DIAL_TIMEOUT
        value: "2s" # Increase the dial timeout duration, e.g., 2 seconds
```
