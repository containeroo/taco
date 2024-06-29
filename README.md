# Service Checker

This is a simple Go application that checks if a specified TCP service is available. It continuously attempts to connect to the specified service at regular intervals until the service becomes available or the program is terminated.

## Environment Variables

The application requires the following environment variables to be set:

- `TARGET_NAME`: The name of the service to check.
- `TARGET_ADDRESS`: The address of the service in the format `host:port`.
- `INTERVAL`: The interval between connection attempts (optional, default: `2s`).
- `DIAL_TIMEOUT`: The timeout for each connection attempt (optional, default: `2s`).

## Kubernetes Init Container Configuration

Configure your Kubernetes deployment to use this init container:

```yaml
initContainers:
  - name: wait-for-response
    image: containeroo/wait-for-response:latest
    env:
      - name: TARGET_NAME
        value: "PostgreSQL"
      - name: TARGET_ADDRESS
        value: "postgres.default.svc.cluster.local:5432"
      - name: INTERVAL
        value: "2s" # Specify the interval duration, e.g., 2 seconds
      - name: DIAL_TIMEOUT
        value: "2s" # Specify the dial timeout duration, e.g., 2 seconds
```
